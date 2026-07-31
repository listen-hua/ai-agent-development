package iam

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	CodeTokenInvalid     = 510000
	CodePermissionDenied = 510001
	CodeInvalidRequest   = 511000
	CodeDatabaseError    = 511001
	CodeUnavailable      = 519000
	CodeIdentityMismatch = 519002
)

type APIError struct {
	Code    int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("iam error %d: %s", e.Code, e.Message)
}

func ErrorCode(err error) (int, bool) {
	var value *APIError
	if errors.As(err, &value) {
		return value.Code, true
	}
	return 0, false
}

type UserCommonInfo struct {
	ID                int64 `json:"id"`
	FeishuAccountInfo struct {
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
		UserID    string `json:"user_id"`
	} `json:"feishu_account_info"`
}

type EffectivePermissionResult struct {
	IAMUserID       *int64
	PermissionKeys  []string
	PolicyVersion   string
	SourceUpdatedAt *time.Time
}

type Client struct {
	baseURL  string
	appID    string
	secret   string
	http     *http.Client
	cache    *redis.Client
	cacheTTL time.Duration
}

func New(baseURL, appID, secret, redisAddr string, cacheTTL, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	if cacheTTL <= 0 {
		cacheTTL = 30 * time.Second
	}
	value := &Client{
		baseURL:  strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		appID:    strings.TrimSpace(appID),
		secret:   secret,
		http:     &http.Client{Timeout: timeout},
		cacheTTL: cacheTTL,
	}
	if strings.TrimSpace(redisAddr) != "" {
		value.cache = redis.NewClient(&redis.Options{Addr: redisAddr})
	}
	return value
}

func (c *Client) Configured() bool {
	return c != nil && c.baseURL != "" && c.appID != "" && c.secret != ""
}

func (c *Client) AppID() string {
	if c == nil {
		return ""
	}
	return c.appID
}

func (c *Client) Close() {
	if c != nil && c.cache != nil {
		_ = c.cache.Close()
	}
}

func (c *Client) Authenticate(ctx context.Context, token, permissionKey string, attrs map[string][]string) (int64, error) {
	token = strings.TrimSpace(token)
	permissionKey = strings.TrimSpace(permissionKey)
	if token == "" {
		return 0, &APIError{Code: CodeTokenInvalid, Message: "令牌非法"}
	}
	if !c.Configured() {
		return 0, &APIError{Code: CodeUnavailable, Message: "IAM 服务未配置"}
	}
	if permissionKey == "" {
		return 0, &APIError{Code: CodeInvalidRequest, Message: "权限参数错误"}
	}
	attrsValue := encodeAttrs(attrs)
	cacheKey := c.permissionCacheKey(token, permissionKey, attrsValue)
	if c.cache != nil {
		if cached, err := c.cache.Get(ctx, cacheKey).Result(); err == nil {
			if userID, parseErr := strconv.ParseInt(cached, 10, 64); parseErr == nil && userID > 0 {
				slog.Debug("IAM permission granted", "permission_key", permissionKey, "iam_user_id", userID, "cache_hit", true)
				return userID, nil
			}
		}
	}

	query := url.Values{
		"app_id":         {c.appID},
		"app_secret":     {c.secret},
		"permission_key": {permissionKey},
	}
	if attrsValue != "" {
		query.Set("attrs", attrsValue)
	}
	var output struct {
		Code    *int   `json:"code"`
		Message string `json:"message"`
		Msg     string `json:"msg"`
		Data    struct {
			UserID int64 `json:"user_id"`
		} `json:"data"`
	}
	if err := c.getJSON(ctx, c.baseURL+"/api/v2/user-action-authenticate?"+query.Encode(), token, true, &output); err != nil {
		return 0, err
	}
	if output.Code != nil && *output.Code != 0 {
		return 0, &APIError{Code: *output.Code, Message: responseMessage(output.Message, output.Msg)}
	}
	if output.Data.UserID <= 0 {
		return 0, &APIError{Code: CodeUnavailable, Message: "IAM 鉴权响应缺少用户信息"}
	}
	if c.cache != nil {
		_ = c.cache.Set(ctx, cacheKey, strconv.FormatInt(output.Data.UserID, 10), c.cacheTTL).Err()
	}
	slog.Debug("IAM permission granted", "permission_key", permissionKey, "iam_user_id", output.Data.UserID, "cache_hit", false)
	return output.Data.UserID, nil
}

func (c *Client) LookupUser(ctx context.Context, token string, expectedUserID int64) (UserCommonInfo, error) {
	if strings.TrimSpace(token) == "" {
		return UserCommonInfo{}, &APIError{Code: CodeTokenInvalid, Message: "令牌非法"}
	}
	if !c.Configured() {
		return UserCommonInfo{}, &APIError{Code: CodeUnavailable, Message: "IAM 服务未配置"}
	}
	var output struct {
		Code    *int             `json:"code"`
		Message string           `json:"message"`
		Msg     string           `json:"msg"`
		Data    []UserCommonInfo `json:"data"`
	}
	if err := c.getJSON(ctx, c.baseURL+"/api/users/common-info", token, false, &output); err != nil {
		return UserCommonInfo{}, err
	}
	if output.Code != nil && *output.Code != 0 {
		return UserCommonInfo{}, &APIError{Code: *output.Code, Message: responseMessage(output.Message, output.Msg)}
	}
	for _, value := range output.Data {
		if value.ID == expectedUserID {
			if strings.TrimSpace(value.FeishuAccountInfo.UserID) == "" {
				return UserCommonInfo{}, &APIError{Code: CodeIdentityMismatch, Message: "IAM 用户未绑定飞书账号"}
			}
			return value, nil
		}
	}
	return UserCommonInfo{}, &APIError{Code: CodeIdentityMismatch, Message: "IAM 用户信息不存在"}
}

func (c *Client) GrantedPermissions(ctx context.Context, token string, keys []string) ([]string, error) {
	granted := make([]string, 0, len(keys))
	for _, key := range keys {
		_, err := c.Authenticate(ctx, token, key, nil)
		if err == nil {
			granted = append(granted, key)
			continue
		}
		if code, ok := ErrorCode(err); ok && code == CodePermissionDenied {
			continue
		}
		return nil, err
	}
	return granted, nil
}

func (c *Client) EffectivePermissions(ctx context.Context, iamUserID *int64, feishuUserID string, force bool) (EffectivePermissionResult, error) {
	if !c.Configured() {
		return EffectivePermissionResult{}, &APIError{Code: CodeUnavailable, Message: "IAM service is not configured"}
	}
	feishuUserID = strings.TrimSpace(feishuUserID)
	if (iamUserID == nil || *iamUserID <= 0) && feishuUserID == "" {
		return EffectivePermissionResult{}, &APIError{Code: CodeInvalidRequest, Message: "IAM user identity is required"}
	}
	identityKey := feishuUserID
	requestBody := map[string]any{"feishu_user_id": feishuUserID}
	if iamUserID != nil && *iamUserID > 0 {
		identityKey = strconv.FormatInt(*iamUserID, 10)
		requestBody = map[string]any{"iam_user_id": *iamUserID}
	}
	cacheKey := c.effectivePermissionCacheKey(identityKey)
	if !force && c.cache != nil {
		if cached, err := c.cache.Get(ctx, cacheKey).Bytes(); err == nil {
			var value EffectivePermissionResult
			if json.Unmarshal(cached, &value) == nil {
				return value, nil
			}
		}
	}
	var output struct {
		Code    *int   `json:"code"`
		Message string `json:"message"`
		Msg     string `json:"msg"`
		Data    struct {
			IAMUserID       *int64          `json:"iam_user_id"`
			PermissionKeys  []string        `json:"permission_keys"`
			EffectPolicy    []string        `json:"effect_policy"`
			PolicyVersion   json.RawMessage `json:"policy_version"`
			SourceUpdatedAt *time.Time      `json:"updated_at"`
		} `json:"data"`
	}
	if err := c.postJSON(ctx, c.baseURL+"/api/v2/app-user-effective-permissions", requestBody, &output); err != nil {
		return EffectivePermissionResult{}, err
	}
	if output.Code != nil && *output.Code != 0 {
		return EffectivePermissionResult{}, &APIError{Code: *output.Code, Message: responseMessage(output.Message, output.Msg)}
	}
	keys := output.Data.PermissionKeys
	if keys == nil {
		keys = output.Data.EffectPolicy
	}
	result := EffectivePermissionResult{
		IAMUserID:       output.Data.IAMUserID,
		PermissionKeys:  keys,
		PolicyVersion:   rawString(output.Data.PolicyVersion),
		SourceUpdatedAt: output.Data.SourceUpdatedAt,
	}
	if result.IAMUserID == nil {
		result.IAMUserID = iamUserID
	}
	if c.cache != nil {
		if data, err := json.Marshal(result); err == nil {
			_ = c.cache.Set(ctx, cacheKey, data, c.cacheTTL).Err()
		}
	}
	return result, nil
}

func (c *Client) getJSON(ctx context.Context, endpoint, token string, bearer bool, output any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return &APIError{Code: CodeUnavailable, Message: "IAM 请求创建失败"}
	}
	req.Header.Set("Accept", "application/json")
	authorization := token
	if bearer {
		authorization = "Bearer " + token
	}
	req.Header.Set("Authorization", authorization)
	req.Header.Set("Cookie", (&http.Cookie{Name: "iam_user_token", Value: token}).String())
	response, err := c.http.Do(req)
	if err != nil {
		return &APIError{Code: CodeUnavailable, Message: "IAM 服务暂时不可用"}
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return &APIError{Code: CodeUnavailable, Message: "读取 IAM 响应失败"}
	}
	if response.StatusCode != http.StatusOK {
		return &APIError{Code: CodeUnavailable, Message: fmt.Sprintf("IAM 服务返回 HTTP %d", response.StatusCode)}
	}
	if err = json.Unmarshal(data, output); err != nil {
		return &APIError{Code: CodeUnavailable, Message: "IAM 返回了无效数据"}
	}
	return nil
}

func (c *Client) postJSON(ctx context.Context, endpoint string, input, output any) error {
	body, err := json.Marshal(input)
	if err != nil {
		return &APIError{Code: CodeInvalidRequest, Message: "invalid IAM permission request"}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return &APIError{Code: CodeUnavailable, Message: "failed to create IAM request"}
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-IAM-App-ID", c.appID)
	req.Header.Set("X-IAM-App-Secret", c.secret)
	response, err := c.http.Do(req)
	if err != nil {
		return &APIError{Code: CodeUnavailable, Message: "IAM service is temporarily unavailable"}
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return &APIError{Code: CodeUnavailable, Message: "failed to read IAM response"}
	}
	if response.StatusCode != http.StatusOK {
		return &APIError{Code: CodeUnavailable, Message: fmt.Sprintf("IAM service returned HTTP %d", response.StatusCode)}
	}
	if err = json.Unmarshal(data, output); err != nil {
		return &APIError{Code: CodeUnavailable, Message: "IAM returned invalid data"}
	}
	return nil
}

func (c *Client) permissionCacheKey(token, permissionKey, attrs string) string {
	sum := sha256.Sum256([]byte(c.appID + "\x00" + token + "\x00" + permissionKey + "\x00" + attrs))
	return fmt.Sprintf("iam:auth:%x", sum)
}

func (c *Client) effectivePermissionCacheKey(identity string) string {
	sum := sha256.Sum256([]byte(c.appID + "\x00" + identity))
	return fmt.Sprintf("iam:effective:%x", sum)
}

func rawString(value json.RawMessage) string {
	if len(value) == 0 || string(value) == "null" {
		return ""
	}
	var text string
	if json.Unmarshal(value, &text) == nil {
		return text
	}
	var number json.Number
	if json.Unmarshal(value, &number) == nil {
		return number.String()
	}
	return strings.Trim(string(value), `"`)
}

func encodeAttrs(attrs map[string][]string) string {
	if len(attrs) == 0 {
		return ""
	}
	keys := make([]string, 0, len(attrs))
	for key := range attrs {
		keys = append(keys, key)
	}
	sortStrings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		values := attrs[key]
		if len(values) == 0 {
			continue
		}
		parts = append(parts, key+"="+strings.Join(values, "|"))
	}
	return strings.Join(parts, ",")
}

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}

func responseMessage(message, fallback string) string {
	if strings.TrimSpace(message) != "" {
		return message
	}
	if strings.TrimSpace(fallback) != "" {
		return fallback
	}
	return "IAM 请求失败"
}
