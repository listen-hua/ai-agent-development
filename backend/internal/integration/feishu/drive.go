package feishu

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

const maxDriveFileSize = 64 << 20

type DriveItem struct {
	Token string `json:"token"`
	// RemoteToken is the stable identity used by the connected source. For
	// Wiki sources this is the node token while Token is the underlying object
	// token required by the document API.
	RemoteToken  string            `json:"-"`
	Name         string            `json:"name"`
	Type         string            `json:"type"`
	ParentToken  string            `json:"parent_token"`
	URL          string            `json:"url"`
	ModifiedTime string            `json:"modified_time"`
	ShortcutInfo DriveShortcutInfo `json:"shortcut_info"`
}

type DriveShortcutInfo struct {
	TargetToken string `json:"target_token"`
	TargetType  string `json:"target_type"`
}

type DriveFile struct {
	Data     []byte
	MimeType string
	Name     string
}

type APIError struct {
	HTTPStatus int
	Code       int
	Message    string
}

func (e *APIError) Error() string {
	if e.HTTPStatus != 0 {
		return fmt.Sprintf("feishu HTTP %d: %s", e.HTTPStatus, e.Message)
	}
	return fmt.Sprintf("feishu API %d: %s", e.Code, e.Message)
}

func IsAccessLoss(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.HTTPStatus == http.StatusForbidden || apiErr.HTTPStatus == http.StatusNotFound ||
			apiErr.Code == 1061003 || apiErr.Code == 1061004 || apiErr.Code == 1061007 ||
			apiErr.Code == 1062501 || apiErr.Code == 1062502 || apiErr.Code == 91203 || apiErr.Code == 91204
	}
	return false
}

func ExtractFolderToken(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("飞书文件夹链接或 Token 不能为空")
	}
	if !strings.Contains(value, "://") {
		if strings.ContainsAny(value, "/?#") {
			return "", errors.New("飞书文件夹 Token 格式无效")
		}
		return value, nil
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", errors.New("飞书文件夹链接格式无效")
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for i := 0; i+2 < len(parts); i++ {
		if parts[i] == "drive" && parts[i+1] == "folder" && parts[i+2] != "" {
			return parts[i+2], nil
		}
	}
	return "", errors.New("请输入包含 /drive/folder/ 的飞书云空间文件夹链接或文件夹 Token")
}

func (c *Client) ListDriveFolder(ctx context.Context, folderToken string) ([]DriveItem, error) {
	if !c.Configured() {
		return nil, errors.New("feishu is not configured")
	}
	token, err := c.getTenantToken(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]DriveItem, 0)
	pageToken := ""
	for page := 0; page < 200; page++ {
		query := url.Values{}
		query.Set("folder_token", folderToken)
		query.Set("page_size", "200")
		if pageToken != "" {
			query.Set("page_token", pageToken)
		}
		var output struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
			Data struct {
				Files         []DriveItem `json:"files"`
				NextPageToken string      `json:"next_page_token"`
				HasMore       bool        `json:"has_more"`
			} `json:"data"`
		}
		endpoint := c.baseURL + "/open-apis/drive/v1/files?" + query.Encode()
		if err = c.getJSON(ctx, endpoint, token, &output); err != nil {
			return nil, err
		}
		if output.Code != 0 {
			return nil, &APIError{Code: output.Code, Message: output.Msg}
		}
		items = append(items, output.Data.Files...)
		if len(items) > 10000 {
			return nil, errors.New("Feishu source contains more than 10,000 items")
		}
		pageToken = output.Data.NextPageToken
		if !output.Data.HasMore || pageToken == "" {
			return items, nil
		}
	}
	return nil, errors.New("Feishu drive pagination exceeded the safety limit")
}

func (c *Client) FetchDriveItem(ctx context.Context, item DriveItem) (DriveFile, error) {
	item = resolveShortcut(item)
	switch item.Type {
	case "file":
		data, contentType, err := c.download(ctx, "/open-apis/drive/v1/files/"+url.PathEscape(item.Token)+"/download")
		if err != nil {
			return DriveFile{}, err
		}
		if contentType == "" || contentType == "application/octet-stream" {
			contentType = mimeFromName(item.Name)
		}
		return DriveFile{Data: data, MimeType: contentType, Name: item.Name}, nil
	case "docx":
		if err := c.waitSpecialRate(ctx); err != nil {
			return DriveFile{}, err
		}
		token, err := c.getTenantToken(ctx)
		if err != nil {
			return DriveFile{}, err
		}
		var output struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
			Data struct {
				Content string `json:"content"`
			} `json:"data"`
		}
		endpoint := c.baseURL + "/open-apis/docx/v1/documents/" + url.PathEscape(item.Token) + "/raw_content"
		if err = c.getJSON(ctx, endpoint, token, &output); err != nil {
			return DriveFile{}, err
		}
		if output.Code != 0 {
			return DriveFile{}, &APIError{Code: output.Code, Message: output.Msg}
		}
		return DriveFile{Data: []byte(output.Data.Content), MimeType: "text/plain; charset=utf-8", Name: item.Name + ".txt"}, nil
	case "doc":
		return c.exportDriveItem(ctx, item, "docx")
	case "sheet", "bitable":
		return c.exportDriveItem(ctx, item, "xlsx")
	default:
		return DriveFile{}, fmt.Errorf("unsupported Feishu document type %q", item.Type)
	}
}

func resolveShortcut(item DriveItem) DriveItem {
	if item.Type == "shortcut" && item.ShortcutInfo.TargetToken != "" {
		item.Token = item.ShortcutInfo.TargetToken
		item.Type = item.ShortcutInfo.TargetType
	}
	return item
}

func (c *Client) exportDriveItem(ctx context.Context, item DriveItem, extension string) (DriveFile, error) {
	if err := c.waitSpecialRate(ctx); err != nil {
		return DriveFile{}, err
	}
	token, err := c.getTenantToken(ctx)
	if err != nil {
		return DriveFile{}, err
	}
	var created struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Ticket string `json:"ticket"`
		} `json:"data"`
	}
	payload := map[string]string{"file_extension": extension, "token": item.Token, "type": item.Type}
	if err = c.postJSON(ctx, c.baseURL+"/open-apis/drive/v1/export_tasks", payload, token, &created); err != nil {
		return DriveFile{}, err
	}
	if created.Code != 0 || created.Data.Ticket == "" {
		return DriveFile{}, &APIError{Code: created.Code, Message: created.Msg}
	}

	pollCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		var status struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
			Data struct {
				Result struct {
					FileToken   string `json:"file_token"`
					JobStatus   int    `json:"job_status"`
					JobErrorMsg string `json:"job_error_msg"`
				} `json:"result"`
			} `json:"data"`
		}
		query := url.Values{"token": []string{item.Token}}
		endpoint := c.baseURL + "/open-apis/drive/v1/export_tasks/" + url.PathEscape(created.Data.Ticket) + "?" + query.Encode()
		if err = c.getJSON(pollCtx, endpoint, token, &status); err != nil {
			return DriveFile{}, err
		}
		if status.Code != 0 {
			return DriveFile{}, &APIError{Code: status.Code, Message: status.Msg}
		}
		if status.Data.Result.FileToken != "" {
			data, contentType, downloadErr := c.download(pollCtx, "/open-apis/drive/v1/export_tasks/file/"+url.PathEscape(status.Data.Result.FileToken)+"/download")
			if downloadErr != nil {
				return DriveFile{}, downloadErr
			}
			name := strings.TrimSuffix(item.Name, filepath.Ext(item.Name)) + "." + extension
			if contentType == "" || contentType == "application/octet-stream" {
				contentType = mimeFromName(name)
			}
			return DriveFile{Data: data, MimeType: contentType, Name: name}, nil
		}
		if status.Data.Result.JobErrorMsg != "" && !strings.EqualFold(status.Data.Result.JobErrorMsg, "success") {
			return DriveFile{}, errors.New(status.Data.Result.JobErrorMsg)
		}
		select {
		case <-pollCtx.Done():
			return DriveFile{}, errors.New("Feishu document export timed out")
		case <-ticker.C:
		}
	}
}

func (c *Client) download(ctx context.Context, path string) ([]byte, string, error) {
	if err := c.waitSpecialRate(ctx); err != nil {
		return nil, "", err
	}
	token, err := c.getTenantToken(ctx)
	if err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		limited, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, "", &APIError{HTTPStatus: resp.StatusCode, Message: strings.TrimSpace(string(limited))}
	}
	if resp.ContentLength > maxDriveFileSize {
		return nil, "", errors.New("Feishu file exceeds the 64 MB import limit")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxDriveFileSize+1))
	if err != nil {
		return nil, "", err
	}
	if len(data) > maxDriveFileSize {
		return nil, "", errors.New("Feishu file exceeds the 64 MB import limit")
	}
	contentType := strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])
	return data, contentType, nil
}

func (c *Client) waitSpecialRate(ctx context.Context) error {
	c.specialMu.Lock()
	defer c.specialMu.Unlock()
	wait := 210*time.Millisecond - time.Since(c.specialLast)
	if wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}
	c.specialLast = time.Now()
	return nil
}

func mimeFromName(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".pdf":
		return "application/pdf"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case ".md":
		return "text/markdown"
	case ".txt":
		return "text/plain"
	default:
		return "application/octet-stream"
	}
}
