package feishu

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type UserInfo struct {
	OpenID    string `json:"open_id"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
	TenantKey string `json:"tenant_key"`
}
type ContactUser struct {
	OpenID        string
	UserID        string
	Name          string
	AvatarURL     string
	DepartmentIDs []string
	JobTitle      string
	JobLevelID    string
	JobFamilyID   string
	EmployeeType  int
	Status        string
}
type ContactDepartment struct {
	OpenDepartmentID   string
	Name               string
	ParentDepartmentID string
	Order              string
	MemberCount        int
}
type ChatInfo struct {
	ChatID    string
	Name      string
	AvatarURL string
}
type CardImage struct {
	ImageKey string
	Alt      string
}
type Client struct {
	appID, appSecret, redirectURI string
	baseURL                       string
	http                          *http.Client
	mu                            sync.Mutex
	tenantToken                   string
	tenantExpires                 time.Time
	appToken                      string
	appExpires                    time.Time
	specialMu                     sync.Mutex
	specialLast                   time.Time
}

func New(appID, appSecret, redirectURI string) *Client {
	return &Client{appID: appID, appSecret: appSecret, redirectURI: redirectURI, baseURL: "https://open.feishu.cn", http: &http.Client{Timeout: 20 * time.Second}}
}
func (c *Client) Configured() bool { return c.appID != "" && c.appSecret != "" }

func (c *Client) ExchangeCode(ctx context.Context, code string) (UserInfo, error) {
	if !c.Configured() {
		return UserInfo{}, errors.New("feishu is not configured")
	}
	var tokenResp struct {
		Code             int    `json:"code"`
		Message          string `json:"message"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
		AccessToken      string `json:"access_token"`
	}
	err := c.postJSON(ctx, c.baseURL+"/open-apis/authen/v2/oauth/token", map[string]any{"grant_type": "authorization_code", "client_id": c.appID, "client_secret": c.appSecret, "code": code}, "", &tokenResp)
	if err != nil {
		return UserInfo{}, err
	}
	if tokenResp.AccessToken == "" {
		detail := tokenResp.ErrorDescription
		if detail == "" {
			detail = tokenResp.Message
		}
		if detail == "" {
			detail = tokenResp.Error
		}
		return UserInfo{}, fmt.Errorf("feishu oauth failed: %s", detail)
	}
	return c.getUserInfo(ctx, tokenResp.AccessToken)
}

func (c *Client) ExchangeLegacyCode(ctx context.Context, code string) (UserInfo, error) {
	if !c.Configured() {
		return UserInfo{}, errors.New("feishu is not configured")
	}
	appToken, err := c.getAppToken(ctx)
	if err != nil {
		return UserInfo{}, err
	}
	var output struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	if err = c.postJSON(ctx, c.baseURL+"/open-apis/authen/v1/access_token", map[string]string{"grant_type": "authorization_code", "code": code}, appToken, &output); err != nil {
		return UserInfo{}, err
	}
	if output.Code != 0 || output.Data.AccessToken == "" {
		return UserInfo{}, fmt.Errorf("feishu legacy oauth failed: %s", output.Msg)
	}
	return c.getUserInfo(ctx, output.Data.AccessToken)
}

func (c *Client) getUserInfo(ctx context.Context, accessToken string) (UserInfo, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/open-apis/authen/v1/user_info", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := c.http.Do(req)
	if err != nil {
		return UserInfo{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		limited, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return UserInfo{}, fmt.Errorf("feishu HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(limited)))
	}
	var wrapper struct {
		Code int      `json:"code"`
		Msg  string   `json:"msg"`
		Data UserInfo `json:"data"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return UserInfo{}, err
	}
	if wrapper.Code != 0 {
		return UserInfo{}, errors.New(wrapper.Msg)
	}
	return wrapper.Data, nil
}

func (c *Client) GetContactUser(ctx context.Context, openID string) (ContactUser, error) {
	return c.getContactUser(ctx, openID, "open_id")
}

func (c *Client) GetContactUserByUserID(ctx context.Context, userID string) (ContactUser, error) {
	return c.getContactUser(ctx, userID, "user_id")
}

func (c *Client) getContactUser(ctx context.Context, identifier, userIDType string) (ContactUser, error) {
	if !c.Configured() {
		return ContactUser{}, errors.New("feishu is not configured")
	}
	token, err := c.getTenantToken(ctx)
	if err != nil {
		return ContactUser{}, err
	}
	endpoint := c.baseURL + "/open-apis/contact/v3/users/" + url.PathEscape(identifier) + "?user_id_type=" + url.QueryEscape(userIDType) + "&department_id_type=open_department_id"
	var output struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			User struct {
				OpenID        string   `json:"open_id"`
				UserID        string   `json:"user_id"`
				Name          string   `json:"name"`
				DepartmentIDs []string `json:"department_ids"`
				JobTitle      string   `json:"job_title"`
				JobLevelID    string   `json:"job_level_id"`
				JobFamilyID   string   `json:"job_family_id"`
				EmployeeType  int      `json:"employee_type"`
				Avatar        struct {
					Avatar72 string `json:"avatar_72"`
				} `json:"avatar"`
				Status struct {
					IsActivated bool `json:"is_activated"`
					IsFrozen    bool `json:"is_frozen"`
					IsResigned  bool `json:"is_resigned"`
					IsExited    bool `json:"is_exited"`
				} `json:"status"`
			} `json:"user"`
		} `json:"data"`
	}
	if err = c.getJSON(ctx, endpoint, token, &output); err != nil {
		return ContactUser{}, err
	}
	if output.Code != 0 {
		return ContactUser{}, fmt.Errorf("feishu contact user failed: %s", output.Msg)
	}
	value := output.Data.User
	status := "active"
	if value.Status.IsFrozen {
		status = "frozen"
	}
	if value.Status.IsResigned || value.Status.IsExited {
		status = "inactive"
	}
	return ContactUser{OpenID: value.OpenID, UserID: value.UserID, Name: value.Name, AvatarURL: value.Avatar.Avatar72, DepartmentIDs: value.DepartmentIDs, JobTitle: value.JobTitle, JobLevelID: value.JobLevelID, JobFamilyID: value.JobFamilyID, EmployeeType: value.EmployeeType, Status: status}, nil
}

func (c *Client) getAppToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.appToken != "" && time.Now().Before(c.appExpires) {
		return c.appToken, nil
	}
	var output struct {
		Code           int    `json:"code"`
		Msg            string `json:"msg"`
		AppAccessToken string `json:"app_access_token"`
		Expire         int    `json:"expire"`
	}
	if err := c.postJSON(ctx, c.baseURL+"/open-apis/auth/v3/app_access_token/internal", map[string]string{"app_id": c.appID, "app_secret": c.appSecret}, "", &output); err != nil {
		return "", err
	}
	if output.Code != 0 || output.AppAccessToken == "" {
		return "", fmt.Errorf("feishu app token failed: %s", output.Msg)
	}
	c.appToken = output.AppAccessToken
	c.appExpires = tokenExpiry(output.Expire)
	return c.appToken, nil
}

func (c *Client) SendText(ctx context.Context, receiveType, receiveID, text, idempotencyKey string) (string, error) {
	content, _ := json.Marshal(map[string]string{"text": text})
	return c.send(ctx, receiveType, map[string]any{"receive_id": receiveID, "msg_type": "text", "content": string(content), "uuid": idempotencyKey})
}
func (c *Client) SendCard(ctx context.Context, receiveType, receiveID, title, content, idempotencyKey string) (string, error) {
	content = normalizeCardMarkdown(content)
	card := map[string]any{"header": map[string]any{"template": "blue", "title": map[string]string{"tag": "plain_text", "content": title}}, "elements": []any{map[string]string{"tag": "markdown", "content": content}}}
	return c.sendCard(ctx, receiveType, receiveID, card, idempotencyKey)
}
func (c *Client) SendRichCard(ctx context.Context, receiveType, receiveID, title, markdown string, images []CardImage, idempotencyKey string) (string, error) {
	markdown = normalizeCardMarkdown(markdown)
	elements := []any{map[string]string{"tag": "markdown", "content": markdown}}
	for _, image := range images {
		if image.ImageKey == "" {
			continue
		}
		alt := strings.TrimSpace(image.Alt)
		if alt == "" {
			alt = "通知图片"
		}
		elements = append(elements, map[string]any{"tag": "img", "img_key": image.ImageKey, "alt": map[string]string{"tag": "plain_text", "content": alt}})
	}
	card := map[string]any{"header": map[string]any{"template": "blue", "title": map[string]string{"tag": "plain_text", "content": title}}, "elements": elements}
	return c.sendCard(ctx, receiveType, receiveID, card, idempotencyKey)
}

func (c *Client) ListChats(ctx context.Context) ([]ChatInfo, error) {
	if !c.Configured() {
		return nil, errors.New("feishu is not configured")
	}
	token, err := c.getTenantToken(ctx)
	if err != nil {
		return nil, err
	}
	values := []ChatInfo{}
	pageToken := ""
	for page := 0; page < 20; page++ {
		endpoint := c.baseURL + "/open-apis/im/v1/chats?page_size=100"
		if pageToken != "" {
			endpoint += "&page_token=" + url.QueryEscape(pageToken)
		}
		var output struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
			Data struct {
				Items []struct {
					ChatID   string `json:"chat_id"`
					Name     string `json:"name"`
					Avatar   string `json:"avatar"`
					ChatMode string `json:"chat_mode"`
				} `json:"items"`
				HasMore   bool   `json:"has_more"`
				PageToken string `json:"page_token"`
			} `json:"data"`
		}
		if err = c.getJSON(ctx, endpoint, token, &output); err != nil {
			return nil, err
		}
		if output.Code != 0 {
			return nil, &APIError{Code: output.Code, Message: output.Msg}
		}
		for _, item := range output.Data.Items {
			if item.ChatID == "" || item.ChatMode == "p2p" {
				continue
			}
			name := strings.TrimSpace(item.Name)
			if name == "" {
				name = "未命名群聊"
			}
			values = append(values, ChatInfo{ChatID: item.ChatID, Name: name, AvatarURL: item.Avatar})
		}
		if !output.Data.HasMore || output.Data.PageToken == "" {
			break
		}
		pageToken = output.Data.PageToken
	}
	return values, nil
}

func (c *Client) UploadMessageImage(ctx context.Context, filename string, data []byte) (string, error) {
	if !c.Configured() {
		return "", errors.New("feishu is not configured")
	}
	token, err := c.getTenantToken(ctx)
	if err != nil {
		return "", err
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err = writer.WriteField("image_type", "message"); err != nil {
		return "", err
	}
	part, err := writer.CreateFormFile("image", filename)
	if err != nil {
		return "", err
	}
	if _, err = part.Write(data); err != nil {
		return "", err
	}
	if err = writer.Close(); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/open-apis/im/v1/images", &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		limited, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", &APIError{HTTPStatus: resp.StatusCode, Message: strings.TrimSpace(string(limited))}
	}
	var output struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			ImageKey string `json:"image_key"`
		} `json:"data"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&output); err != nil {
		return "", err
	}
	if output.Code != 0 || output.Data.ImageKey == "" {
		return "", &APIError{Code: output.Code, Message: output.Msg}
	}
	return output.Data.ImageKey, nil
}
func (c *Client) SendReminderConfirmation(ctx context.Context, openID, title, content, actionID, idempotencyKey string) (string, error) {
	card := map[string]any{
		"header": map[string]any{"template": "blue", "title": map[string]string{"tag": "plain_text", "content": title}},
		"elements": []any{
			map[string]any{"tag": "div", "text": map[string]string{"tag": "plain_text", "content": content}},
			map[string]any{"tag": "action", "actions": []any{
				map[string]any{"tag": "button", "type": "primary", "name": "reminder_confirm", "text": map[string]string{"tag": "plain_text", "content": "确认"}, "value": map[string]string{"reminder_action_id": actionID}},
				map[string]any{"tag": "button", "name": "reminder_cancel", "text": map[string]string{"tag": "plain_text", "content": "取消"}, "value": map[string]string{"reminder_action_id": actionID}},
			}},
		},
	}
	return c.sendCard(ctx, "open_id", openID, card, idempotencyKey)
}
func (c *Client) SendReminder(ctx context.Context, openID, content, scheduleText, appLink, idempotencyKey string) (string, error) {
	elements := []any{
		map[string]any{"tag": "div", "text": map[string]string{"tag": "plain_text", "content": content}},
		map[string]any{"tag": "note", "elements": []any{map[string]string{"tag": "plain_text", "content": scheduleText}}},
	}
	if appLink != "" {
		elements = append(elements, map[string]any{"tag": "action", "actions": []any{map[string]any{"tag": "button", "type": "primary", "text": map[string]string{"tag": "plain_text", "content": "管理我的提醒"}, "url": appLink}}})
	}
	card := map[string]any{"header": map[string]any{"template": "blue", "title": map[string]string{"tag": "plain_text", "content": "提醒"}}, "elements": elements}
	return c.sendCard(ctx, "open_id", openID, card, idempotencyKey)
}
func (c *Client) sendCard(ctx context.Context, receiveType, receiveID string, card map[string]any, idempotencyKey string) (string, error) {
	encoded, _ := json.Marshal(card)
	return c.send(ctx, receiveType, map[string]any{"receive_id": receiveID, "msg_type": "interactive", "content": string(encoded), "uuid": idempotencyKey})
}
func (c *Client) send(ctx context.Context, receiveType string, payload map[string]any) (string, error) {
	if value, ok := payload["uuid"].(string); ok {
		payload["uuid"] = normalizeMessageUUID(value)
	}
	token, err := c.getTenantToken(ctx)
	if err != nil {
		return "", err
	}
	endpoint := c.baseURL + "/open-apis/im/v1/messages?receive_id_type=" + url.QueryEscape(receiveType)
	var output struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			MessageID string `json:"message_id"`
		} `json:"data"`
	}
	if err = c.postJSON(ctx, endpoint, payload, token, &output); err != nil {
		return "", err
	}
	if output.Code != 0 {
		return "", &APIError{Code: output.Code, Message: output.Msg}
	}
	return output.Data.MessageID, nil
}

func normalizeMessageUUID(value string) string {
	if len(value) <= 50 {
		return value
	}
	digest := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", digest)[:50]
}

func normalizeCardMarkdown(value string) string {
	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	for index, line := range lines {
		level, title := cardMarkdownHeading(line)
		if level == 0 {
			continue
		}
		marker := "• "
		if level == 1 {
			marker = "▌ "
		}
		lines[index] = "**" + marker + title + "**"
	}
	return strings.Join(lines, "\n")
}

func cardMarkdownHeading(line string) (int, string) {
	for level := 1; level <= 6; level++ {
		prefix := strings.Repeat("#", level) + " "
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		title := strings.TrimSpace(strings.TrimPrefix(line, prefix))
		if title != "" {
			return level, title
		}
		return 0, ""
	}
	return 0, ""
}

func (c *Client) getTenantToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.tenantToken != "" && time.Now().Before(c.tenantExpires) {
		return c.tenantToken, nil
	}
	var output struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int    `json:"expire"`
	}
	if err := c.postJSON(ctx, c.baseURL+"/open-apis/auth/v3/tenant_access_token/internal/", map[string]string{"app_id": c.appID, "app_secret": c.appSecret}, "", &output); err != nil {
		return "", err
	}
	if output.Code != 0 {
		return "", errors.New(output.Msg)
	}
	c.tenantToken = output.TenantAccessToken
	c.tenantExpires = tokenExpiry(output.Expire)
	return c.tenantToken, nil
}

func tokenExpiry(seconds int) time.Time {
	if seconds <= 120 {
		return time.Now().Add(time.Duration(seconds) * time.Second)
	}
	return time.Now().Add(time.Duration(seconds-120) * time.Second)
}
func (c *Client) postJSON(ctx context.Context, endpoint string, input any, token string, output any) error {
	body, _ := json.Marshal(input)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		limited, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return &APIError{HTTPStatus: resp.StatusCode, Message: strings.TrimSpace(string(limited))}
	}
	return json.NewDecoder(resp.Body).Decode(output)
}

func (c *Client) getJSON(ctx context.Context, endpoint, token string, output any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		limited, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return &APIError{HTTPStatus: resp.StatusCode, Message: strings.TrimSpace(string(limited))}
	}
	return json.NewDecoder(resp.Body).Decode(output)
}
