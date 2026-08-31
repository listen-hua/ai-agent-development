package imageproxy

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"regexp"
	"sort"
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
	body := strings.ToLower(e.Body)
	// Some relays incorrectly return HTTP 500 for a deterministic multipart
	// parsing failure. Replaying the exact same body cannot recover and may
	// create duplicate billable attempts.
	if strings.Contains(body, "multipart: nextpart: bufio: buffer full") ||
		strings.Contains(body, `"code":"invalid_request"`) ||
		strings.Contains(body, `"code":"model_not_found"`) ||
		strings.Contains(body, "no available channel for model") {
		return false
	}
	return e.StatusCode == http.StatusTooManyRequests || e.StatusCode >= 500
}

type Image struct {
	Data []byte
	MIME string
}

type ReferenceImage struct {
	FileName string
	MIME     string
	Data     []byte
	Width    int
	Height   int
}

type RemoteModel struct {
	ID                     string   `json:"id"`
	SupportedEndpointTypes []string `json:"supported_endpoint_types"`
}

// GenerateUniversalEdit uses the multipart image editing contract exposed by
// relays such as XGAPI. Unlike the OpenAI GPT Image 2 contract, this endpoint
// accepts aspect_ratio and quality directly.
func (c *Client) GenerateUniversalEdit(ctx context.Context, model, prompt, aspectRatio, imageSize string, references []ReferenceImage) (Image, error) {
	if len(references) > 5 {
		return Image{}, errors.New("universal image edit supports at most five reference images")
	}
	quality := strings.ToUpper(strings.TrimSpace(imageSize))
	if quality != "1K" && quality != "2K" && quality != "4K" {
		return Image{}, fmt.Errorf("unsupported universal image edit quality %q", imageSize)
	}
	ratio := strings.TrimSpace(aspectRatio)
	if ratio == "original" {
		ratio = ""
	}
	body, contentType, err := universalImageEditBody(model, prompt, ratio, quality, references)
	if err != nil {
		return Image{}, err
	}
	var response map[string]any
	if err = c.requestMultipartWithTimeout(ctx, "/images/edits", body, contentType, &response, imageTierTimeout(imageSize)); err != nil {
		return Image{}, err
	}
	values := collectImageValues(response)
	if len(values) == 0 {
		return Image{}, errors.New("relay response did not contain an image")
	}
	return c.decodeImage(ctx, values[0])
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

func (c *Client) Models(ctx context.Context) ([]RemoteModel, error) {
	var response struct {
		Data []RemoteModel `json:"data"`
	}
	if err := c.request(ctx, http.MethodGet, "/models", nil, &response); err != nil {
		return nil, err
	}
	result := make([]RemoteModel, 0, len(response.Data))
	for _, value := range response.Data {
		if id := strings.TrimSpace(value.ID); id != "" {
			value.ID = id
			value.SupportedEndpointTypes = normalizedEndpointTypes(value.SupportedEndpointTypes)
			result = append(result, value)
		}
	}
	return result, nil
}

func normalizedEndpointTypes(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func (c *Client) GenerateChat(ctx context.Context, model, prompt, aspectRatio, imageSize string, references []string) (Image, error) {
	input := chatGenerationInput(model, prompt, aspectRatio, imageSize, references)
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

func chatGenerationInput(model, prompt, aspectRatio, imageSize string, references []string) map[string]any {
	content := []map[string]any{{"type": "text", "text": prompt}}
	for _, reference := range references {
		content = append(content, map[string]any{"type": "image_url", "image_url": map[string]string{"url": reference}})
	}
	imageConfig := map[string]string{"image_size": imageSize}
	if aspectRatio != "original" {
		imageConfig["aspect_ratio"] = aspectRatio
	}
	return map[string]any{
		"model":      model,
		"messages":   []map[string]any{{"role": "user", "content": content}},
		"modalities": []string{"image"},
		"n":          1,
		"extra_body": map[string]any{
			"google": map[string]any{
				"image_config": imageConfig,
			},
		},
	}
}

func (c *Client) GenerateImage(ctx context.Context, model, prompt, aspectRatio, imageSize string) (Image, error) {
	input := map[string]any{
		"model":           model,
		"prompt":          prompt,
		"n":               1,
		"response_format": "b64_json",
		"size":            pixelSize(aspectRatio, imageSize),
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

// GenerateGPTImage2 uses the GPT Image 2 compatible Images endpoints. GPT
// Image 2 accepts concrete WIDTHxHEIGHT values in size; the UI's 1K/2K/4K
// tiers must not be sent through Gemini's image_size/quality extensions.
func (c *Client) GenerateGPTImage2(ctx context.Context, model, prompt, aspectRatio, imageSize string, references []ReferenceImage) (Image, error) {
	size, err := GPTImagePixelSize(aspectRatio, imageSize, references)
	if err != nil {
		return Image{}, err
	}
	minimumTimeout := imageTierTimeout(imageSize)
	var response map[string]any
	if len(references) == 0 {
		input := gptImage2GenerationInput(model, prompt, size)
		if err := c.requestWithTimeout(ctx, http.MethodPost, "/images/generations", input, &response, minimumTimeout); err != nil {
			return Image{}, err
		}
	} else {
		if len(references) > 5 {
			return Image{}, errors.New("GPT Image 2 supports at most five reference images")
		}
		body, contentType, err := gptImage2EditBody(model, prompt, size, references)
		if err != nil {
			return Image{}, err
		}
		if err := c.requestMultipartWithTimeout(ctx, "/images/edits", body, contentType, &response, minimumTimeout); err != nil {
			return Image{}, err
		}
	}
	values := collectImageValues(response)
	if len(values) == 0 {
		return Image{}, errors.New("relay response did not contain an image")
	}
	return c.decodeImage(ctx, values[0])
}

func imageTierTimeout(tier string) time.Duration {
	switch strings.ToUpper(strings.TrimSpace(tier)) {
	case "2K":
		return 240 * time.Second
	case "4K":
		return 360 * time.Second
	default:
		return 120 * time.Second
	}
}

func gptImage2GenerationInput(model, prompt, size string) map[string]any {
	return map[string]any{
		"model": model, "prompt": prompt, "n": 1, "size": size, "response_format": "b64_json",
	}
}

func gptImage2EditBody(model, prompt, size string, references []ReferenceImage) (*bytes.Buffer, string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("model", model); err != nil {
		return nil, "", err
	}
	if err := writer.WriteField("prompt", prompt); err != nil {
		return nil, "", err
	}
	for index, reference := range references {
		part, err := writer.CreateFormFile("image", relayReferenceFilename(index, reference))
		if err != nil {
			return nil, "", err
		}
		if _, err = part.Write(reference.Data); err != nil {
			return nil, "", err
		}
	}
	for _, field := range []struct{ name, value string }{
		{"size", size}, {"n", "1"}, {"response_format", "b64_json"},
	} {
		if err := writer.WriteField(field.name, field.value); err != nil {
			return nil, "", err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return &body, writer.FormDataContentType(), nil
}

// GPTImagePixelSize maps the product's semantic tiers to sizes accepted by
// GPT Image 2. Every result is within the model's 3840px edge and 8,294,400
// pixel limits and both dimensions are multiples of 16.
func GPTImagePixelSize(aspectRatio, tier string, references []ReferenceImage) (string, error) {
	tier = strings.ToUpper(strings.TrimSpace(tier))
	if tier != "1K" && tier != "2K" && tier != "4K" {
		return "", fmt.Errorf("unsupported GPT Image 2 resolution %q", tier)
	}
	ratio := strings.TrimSpace(aspectRatio)
	if ratio == "original" {
		if len(references) == 0 || references[0].Width <= 0 || references[0].Height <= 0 {
			return "", errors.New("GPT Image 2 original ratio requires a valid reference image")
		}
		return fitOriginalGPTImageSize(references[0].Width, references[0].Height, tier)
	}
	sizes := map[string]map[string]string{
		"1K": {"1:1": "1024x1024", "16:9": "1280x720", "9:16": "720x1280", "4:3": "1152x864", "3:4": "864x1152"},
		"2K": {"1:1": "2048x2048", "16:9": "2048x1152", "9:16": "1152x2048", "4:3": "2048x1536", "3:4": "1536x2048"},
		"4K": {"1:1": "2880x2880", "16:9": "3840x2160", "9:16": "2160x3840", "4:3": "3264x2448", "3:4": "2448x3264"},
	}
	if size := sizes[tier][ratio]; size != "" {
		return size, nil
	}
	return "", fmt.Errorf("unsupported GPT Image 2 aspect ratio %q", aspectRatio)
}

func fitOriginalGPTImageSize(sourceWidth, sourceHeight int, tier string) (string, error) {
	ratio := float64(sourceWidth) / float64(sourceHeight)
	if ratio > 3 || ratio < 1.0/3.0 {
		return "", errors.New("参考图比例超出 GPT Image 2 支持的 1:3 至 3:1 范围，请选择固定画面比例")
	}
	pixelBudget := map[string]float64{"1K": 1024 * 1024, "2K": 2048 * 2048, "4K": 8294400}[tier]
	scale := math.Sqrt(pixelBudget / float64(sourceWidth*sourceHeight))
	if edgeScale := 3840.0 / float64(max(sourceWidth, sourceHeight)); scale > edgeScale {
		scale = edgeScale
	}
	width := int(math.Floor(float64(sourceWidth)*scale/16)) * 16
	height := int(math.Floor(float64(sourceHeight)*scale/16)) * 16
	width, height = max(width, 16), max(height, 16)
	if width > height*3 {
		width = height * 3
	} else if height > width*3 {
		height = width * 3
	}
	if width*height < 655360 {
		return "", errors.New("参考图比例无法生成符合 GPT Image 2 最低像素要求的图片")
	}
	return fmt.Sprintf("%dx%d", width, height), nil
}

func universalImageEditBody(model, prompt, aspectRatio, quality string, references []ReferenceImage) (*bytes.Buffer, string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	// Keep the part order identical to XGAPI's documented Go example. Its
	// upstream multipart parser is stricter than Go's standard reader and has
	// returned "NextPart: bufio: buffer full" for otherwise valid forms when
	// the generic map-based writer placed all fields before the binary part.
	if err := writer.WriteField("model", model); err != nil {
		return nil, "", err
	}
	if err := writer.WriteField("prompt", prompt); err != nil {
		return nil, "", err
	}
	for index, reference := range references {
		// The relay does not need the user's original filename. A short generated
		// filename keeps Content-Disposition bounded and prevents control
		// characters or legacy filenames from corrupting the multipart header.
		part, err := writer.CreateFormFile("image", relayReferenceFilename(index, reference))
		if err != nil {
			return nil, "", err
		}
		if _, err = part.Write(reference.Data); err != nil {
			return nil, "", err
		}
	}
	for _, field := range []struct{ name, value string }{
		{"aspect_ratio", aspectRatio},
		{"quality", quality},
		{"response_format", "b64_json"},
	} {
		if err := writer.WriteField(field.name, field.value); err != nil {
			return nil, "", err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return &body, writer.FormDataContentType(), nil
}

func relayReferenceFilename(index int, reference ReferenceImage) string {
	mimeType := strings.ToLower(strings.TrimSpace(reference.MIME))
	if mimeType == "" {
		mimeType = http.DetectContentType(reference.Data)
	}
	extension := ".png"
	switch mimeType {
	case "image/jpeg":
		extension = ".jpg"
	case "image/webp":
		extension = ".webp"
	}
	return fmt.Sprintf("reference-%d%s", index+1, extension)
}

func multipartImageBody(fields map[string]string, references []ReferenceImage) (*bytes.Buffer, string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			return nil, "", err
		}
	}
	for index, reference := range references {
		name := strings.TrimSpace(reference.FileName)
		if name == "" {
			name = fmt.Sprintf("reference-%d.png", index+1)
		}
		name = strings.NewReplacer("\\", "_", "\"", "_").Replace(name)
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="image"; filename="%s"`, name))
		mimeType := strings.TrimSpace(reference.MIME)
		if mimeType == "" {
			mimeType = http.DetectContentType(reference.Data)
		}
		header.Set("Content-Type", mimeType)
		part, err := writer.CreatePart(header)
		if err != nil {
			return nil, "", err
		}
		if _, err = part.Write(reference.Data); err != nil {
			return nil, "", err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return &body, writer.FormDataContentType(), nil
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
	return c.requestWithTimeout(ctx, method, path, input, output, 0)
}

func (c *Client) requestWithTimeout(ctx context.Context, method, path string, input, output any, minimumTimeout time.Duration) error {
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
	return c.doRequest(request, output, minimumTimeout)
}

func (c *Client) requestMultipartWithTimeout(ctx context.Context, path string, body io.Reader, contentType string, output any, minimumTimeout time.Duration) error {
	base, _ := url.Parse(c.baseURL)
	if err := validatePublicHost(ctx, base.Hostname()); err != nil {
		return fmt.Errorf("relay base host is not public: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, body)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+c.apiKey)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", contentType)
	return c.doRequest(request, output, minimumTimeout)
}

func (c *Client) doRequest(request *http.Request, output any, minimumTimeout time.Duration) error {
	httpClient := c.http
	if minimumTimeout > httpClient.Timeout {
		clone := *httpClient
		clone.Timeout = minimumTimeout
		httpClient = &clone
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, maxImageBytes*2))
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
			keys := make([]string, 0, len(current))
			for key := range current {
				keys = append(keys, key)
			}
			sort.Slice(keys, func(i, j int) bool {
				priority := func(key string) int {
					switch key {
					case "b64_json":
						return 0
					case "url":
						return 1
					case "data":
						return 2
					case "choices":
						return 3
					default:
						return 4
					}
				}
				left, right := priority(keys[i]), priority(keys[j])
				if left != right {
					return left < right
				}
				return keys[i] < keys[j]
			})
			for _, key := range keys {
				child := current[key]
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
	width, height := ExpectedDimensions(aspectRatio, tier)
	return fmt.Sprintf("%dx%d", width, height)
}

func ExpectedDimensions(aspectRatio, tier string) (int, int) {
	base := map[string][2]int{
		"1:1": {1024, 1024}, "16:9": {1344, 768}, "9:16": {768, 1344},
		"4:3": {1152, 864}, "3:4": {864, 1152},
	}[aspectRatio]
	if base == [2]int{} {
		base = [2]int{1024, 1024}
	}
	scale := 1
	if tier == "2K" {
		scale = 2
	} else if tier == "4K" {
		scale = 4
	}
	return base[0] * scale, base[1] * scale
}
