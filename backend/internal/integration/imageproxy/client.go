package imageproxy

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const maxImageBytes = 64 << 20

type Client struct {
	baseURL      string
	apiKey       string
	allowedHosts []string
	http         *http.Client
}

type APIError struct {
	StatusCode int
	Body       string
}

type HostNotAllowedError struct {
	Resource string
	Host     string
}

func (e *HostNotAllowedError) Error() string {
	resource := strings.TrimSpace(e.Resource)
	if resource == "" {
		resource = "URL"
	}
	host := strings.ToLower(strings.TrimSpace(e.Host))
	if host == "" {
		host = "<empty>"
	}
	return fmt.Sprintf("relay %s host %q is not allowed", resource, host)
}

func (e *APIError) Error() string {
	return fmt.Sprintf("image relay HTTP %d: %s", e.StatusCode, e.Body)
}

func (e *APIError) Retryable() bool {
	return e.StatusCode == http.StatusTooManyRequests || e.StatusCode >= 500
}

type Image struct {
	Data []byte
	MIME string
}

func New(baseURL, apiKey string, timeout time.Duration, allowedHosts []string) (*Client, error) {
	parsed, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" {
		return nil, errors.New("relay base URL must be a valid HTTPS URL")
	}
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	if len(allowedHosts) == 0 {
		allowedHosts = []string{parsed.Hostname()}
	}
	client := &Client{
		baseURL:      strings.TrimRight(baseURL, "/"),
		apiKey:       apiKey,
		allowedHosts: allowedHosts,
	}
	client.http = &http.Client{
		Timeout: timeout,
		CheckRedirect: func(request *http.Request, _ []*http.Request) error {
			if request.URL.Scheme != "https" {
				return errors.New("relay redirect URL must use HTTPS")
			}
			if !client.hostAllowed(request.URL.Hostname()) {
				return &HostNotAllowedError{Resource: "redirect URL", Host: request.URL.Hostname()}
			}
			return validatePublicHost(request.Context(), request.URL.Hostname())
		},
	}
	return client, nil
}

func (c *Client) Test(ctx context.Context) error {
	_, err := c.Models(ctx)
	return err
}

func (c *Client) Models(ctx context.Context) ([]string, error) {
	var response struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := c.request(ctx, http.MethodGet, "/models", nil, &response); err != nil {
		return nil, err
	}
	result := make([]string, 0, len(response.Data))
	for _, value := range response.Data {
		if id := strings.TrimSpace(value.ID); id != "" {
			result = append(result, id)
		}
	}
	return result, nil
}

func (c *Client) GenerateChat(ctx context.Context, model, prompt, aspectRatio, imageSize string, references []string) (Image, error) {
	content := []map[string]any{{"type": "text", "text": prompt}}
	for _, reference := range references {
		content = append(content, map[string]any{"type": "image_url", "image_url": map[string]string{"url": reference}})
	}
	input := map[string]any{
		"model":      model,
		"messages":   []map[string]any{{"role": "user", "content": content}},
		"modalities": []string{"image"},
		"n":          1,
		"extra_body": map[string]any{
			"aspect_ratio": aspectRatio,
			"image_size":   imageSize,
			"google": map[string]any{
				"generation_config": map[string]any{
					"responseModalities": []string{"IMAGE"},
					"candidateCount":     1,
					"imageConfig":        map[string]string{"aspectRatio": aspectRatio, "imageSize": imageSize},
				},
			},
		},
	}
	var response map[string]any
	if err := c.request(ctx, http.MethodPost, "/chat/completions", input, &response); err != nil {
		return Image{}, err
	}
	values := collectImageValues(response)
	if len(values) == 0 {
		return Image{}, errors.New("relay response did not contain an image")
	}
	return c.decodeImage(ctx, values[0])
}

func (c *Client) GenerateImage(ctx context.Context, model, prompt, aspectRatio, imageSize string) (Image, error) {
	input := map[string]any{
		"model":           model,
		"prompt":          prompt,
		"n":               1,
		"response_format": "b64_json",
		"size":            pixelSize(aspectRatio, imageSize),
		"extra_body":      map[string]string{"aspect_ratio": aspectRatio, "image_size": imageSize},
	}
	var response map[string]any
	if err := c.request(ctx, http.MethodPost, "/images/generations", input, &response); err != nil {
		return Image{}, err
	}
	values := collectImageValues(response)
	if len(values) == 0 {
		return Image{}, errors.New("relay response did not contain an image")
	}
	return c.decodeImage(ctx, values[0])
}

func (c *Client) ReversePrompt(ctx context.Context, model string, references []string) (string, error) {
	if len(references) == 0 {
		return "", errors.New("at least one reference image is required")
	}
	content := []map[string]any{{
		"type": "text",
		"text": "请分析这些参考图，输出一段可直接用于文生图模型的中文描述。只输出描述本身，包含主体、构图、光线、材质、色彩和风格，不要解释。",
	}}
	for _, reference := range references {
		content = append(content, map[string]any{"type": "image_url", "image_url": map[string]string{"url": reference}})
	}
	input := map[string]any{
		"model":    model,
		"messages": []map[string]any{{"role": "user", "content": content}},
	}
	var response struct {
		Choices []struct {
			Message struct {
				Content any `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := c.request(ctx, http.MethodPost, "/chat/completions", input, &response); err != nil {
		return "", err
	}
	if len(response.Choices) == 0 {
		return "", errors.New("relay response did not contain a description")
	}
	switch value := response.Choices[0].Message.Content.(type) {
	case string:
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value), nil
		}
	case []any:
		for _, block := range value {
			entry, _ := block.(map[string]any)
			if entry["type"] == "text" {
				if text, _ := entry["text"].(string); strings.TrimSpace(text) != "" {
					return strings.TrimSpace(text), nil
				}
			}
		}
	}
	return "", errors.New("relay response did not contain a text description")
}

func (c *Client) request(ctx context.Context, method, path string, input, output any) error {
	base, _ := url.Parse(c.baseURL)
	if err := validatePublicHost(ctx, base.Hostname()); err != nil {
		return fmt.Errorf("relay base host is not public: %w", err)
	}
	var body io.Reader
	if input != nil {
		data, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+c.apiKey)
	request.Header.Set("Accept", "application/json")
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 32<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return &APIError{StatusCode: response.StatusCode, Body: strings.TrimSpace(string(data))}
	}
	if output != nil && len(data) > 0 {
		if err = json.Unmarshal(data, output); err != nil {
			return fmt.Errorf("decode relay response: %w", err)
		}
	}
	return nil
}

var markdownImagePattern = regexp.MustCompile(`!\[[^\]]*\]\((https://[^)\s]+|data:image/[^)\s]+)\)`)

func collectImageValues(response map[string]any) []string {
	result := []string{}
	var walk func(any)
	walk = func(value any) {
		switch current := value.(type) {
		case map[string]any:
			for key, child := range current {
				switch key {
				case "b64_json":
					if text, ok := child.(string); ok && text != "" {
						result = append(result, "data:image/png;base64,"+text)
						continue
					}
				case "url":
					if text, ok := child.(string); ok && (strings.HasPrefix(text, "https://") || strings.HasPrefix(text, "data:image/")) {
						result = append(result, text)
						continue
					}
				}
				walk(child)
			}
		case []any:
			for _, child := range current {
				walk(child)
			}
		case string:
			for _, match := range markdownImagePattern.FindAllStringSubmatch(current, -1) {
				result = append(result, match[1])
			}
		}
	}
	walk(response)
	return result
}

func (c *Client) decodeImage(ctx context.Context, value string) (Image, error) {
	if strings.HasPrefix(value, "data:image/") {
		header, encoded, ok := strings.Cut(value, ",")
		if !ok || !strings.Contains(header, ";base64") {
			return Image{}, errors.New("invalid image data URL")
		}
		data, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return Image{}, err
		}
		return validateImage(data)
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Hostname() == "" {
		return Image{}, errors.New("relay returned an invalid image URL")
	}
	if parsed.Scheme != "https" {
		return Image{}, errors.New("relay image URL must use HTTPS")
	}
	if !c.hostAllowed(parsed.Hostname()) {
		return Image{}, &HostNotAllowedError{Resource: "image URL", Host: parsed.Hostname()}
	}
	if err = validatePublicHost(ctx, parsed.Hostname()); err != nil {
		return Image{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return Image{}, err
	}
	response, err := c.http.Do(request)
	if err != nil {
		return Image{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return Image{}, fmt.Errorf("download generated image: HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxImageBytes+1))
	if err != nil {
		return Image{}, err
	}
	return validateImage(data)
}

func (c *Client) hostAllowed(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	for _, allowed := range c.allowedHosts {
		allowed = strings.ToLower(strings.TrimSpace(strings.TrimSuffix(allowed, ".")))
		if host == allowed || strings.HasSuffix(host, "."+allowed) {
			return true
		}
	}
	return false
}

func validatePublicHost(ctx context.Context, host string) error {
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return err
	}
	if len(addresses) == 0 {
		return errors.New("image host did not resolve")
	}
	for _, address := range addresses {
		ip := address.IP
		if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
			return errors.New("relay image URL resolves to a private network")
		}
	}
	return nil
}

func validateImage(data []byte) (Image, error) {
	if len(data) == 0 {
		return Image{}, errors.New("generated image is empty")
	}
	if len(data) > maxImageBytes {
		return Image{}, fmt.Errorf("generated image exceeds %d MB", maxImageBytes>>20)
	}
	mime := http.DetectContentType(data)
	switch mime {
	case "image/jpeg", "image/png", "image/webp":
		return Image{Data: data, MIME: mime}, nil
	default:
		return Image{}, fmt.Errorf("unsupported generated image MIME type: %s", mime)
	}
}

func pixelSize(aspectRatio, tier string) string {
	base := map[string][2]int{
		"1:1": {1024, 1024}, "16:9": {1344, 768}, "9:16": {768, 1344},
		"4:3": {1152, 896}, "3:4": {896, 1152},
	}[aspectRatio]
	scale := 1
	if tier == "2K" {
		scale = 2
	} else if tier == "4K" {
		scale = 4
	}
	return fmt.Sprintf("%dx%d", base[0]*scale, base[1]*scale)
}
