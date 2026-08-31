package pixian

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
	"time"
)

const DefaultBaseURL = "https://api.pixian.ai/api/v2"

type Client struct {
	baseURL    string
	apiID      string
	apiSecret  string
	httpClient *http.Client
}

type Account struct {
	CreditPack string  `json:"creditPack"`
	State      string  `json:"state"`
	UseBefore  string  `json:"useBefore"`
	Credits    float64 `json:"credits"`
}

type Result struct {
	Data              []byte
	CreditsCharged    float64
	CreditsCalculated float64
	InputSize         string
	ResultSize        string
}

type APIError struct {
	Status  int
	Code    int
	Message string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("Pixian API %d: %s", e.Status, e.Message)
	}
	return fmt.Sprintf("Pixian API HTTP %d", e.Status)
}

func (e *APIError) Retryable() bool { return e.Status == http.StatusTooManyRequests || e.Status >= 500 }

func New(apiID, apiSecret string, timeout time.Duration) *Client {
	return NewWithBaseURL(DefaultBaseURL, apiID, apiSecret, timeout)
}

func NewWithBaseURL(baseURL, apiID, apiSecret string, timeout time.Duration) *Client {
	if timeout < 180*time.Second {
		timeout = 180 * time.Second
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), apiID: apiID, apiSecret: apiSecret, httpClient: &http.Client{Timeout: timeout}}
}

func (c *Client) Account(ctx context.Context) (Account, error) {
	var value Account
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/account", nil)
	if err != nil {
		return value, err
	}
	req.SetBasicAuth(c.apiID, c.apiSecret)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return value, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return value, decodeError(resp)
	}
	return value, json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&value)
}

func (c *Client) RemoveBackground(ctx context.Context, fileName, mimeType string, data []byte, test bool, maxPixels int) (Result, error) {
	var result Result
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="image"; filename="%s"`, strings.ReplaceAll(fileName, `"`, "")))
	header.Set("Content-Type", mimeType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return result, err
	}
	if _, err = part.Write(data); err != nil {
		return result, err
	}
	_ = writer.WriteField("output.format", "png")
	_ = writer.WriteField("max_pixels", strconv.Itoa(maxPixels))
	if test {
		_ = writer.WriteField("test", "true")
	}
	if err = writer.Close(); err != nil {
		return result, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/remove-background", body)
	if err != nil {
		return result, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "image/png")
	req.SetBasicAuth(c.apiID, c.apiSecret)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return result, decodeError(resp)
	}
	if contentType := strings.ToLower(resp.Header.Get("Content-Type")); !strings.HasPrefix(contentType, "image/png") {
		return result, fmt.Errorf("Pixian returned unsupported content type %q", contentType)
	}
	result.Data, err = io.ReadAll(io.LimitReader(resp.Body, (50<<20)+1))
	if err != nil {
		return result, err
	}
	if len(result.Data) > 50<<20 {
		return Result{}, fmt.Errorf("Pixian result exceeds 50 MB")
	}
	result.CreditsCharged, _ = strconv.ParseFloat(resp.Header.Get("X-Credits-Charged"), 64)
	result.CreditsCalculated, _ = strconv.ParseFloat(resp.Header.Get("X-Credits-Calculated"), 64)
	result.InputSize = resp.Header.Get("X-Input-Size")
	result.ResultSize = resp.Header.Get("X-Result-Size")
	return result, nil
}

func decodeError(resp *http.Response) error {
	var payload struct {
		Error struct {
			Status  int    `json:"status"`
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload)
	status := payload.Error.Status
	if status == 0 {
		status = resp.StatusCode
	}
	return &APIError{Status: status, Code: payload.Error.Code, Message: strings.TrimSpace(payload.Error.Message)}
}
