package parser

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

type Tika struct {
	URL    string
	Client *http.Client
}

func New(url string) *Tika {
	return &Tika{URL: strings.TrimRight(url, "/"), Client: &http.Client{Timeout: 2 * time.Minute}}
}

func (t *Tika) Extract(ctx context.Context, data []byte, mime string) (string, error) {
	if t.URL != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, t.URL+"/tika", bytes.NewReader(data))
		if err == nil {
			req.Header.Set("Content-Type", mime)
			req.Header.Set("Accept", "text/plain")
			if resp, callErr := t.Client.Do(req); callErr == nil {
				defer resp.Body.Close()
				if resp.StatusCode >= 200 && resp.StatusCode < 300 {
					body, _ := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
					value := strings.TrimSpace(string(body))
					if value != "" {
						return value, nil
					}
				}
			}
		}
	}
	if utf8.Valid(data) && (strings.HasPrefix(mime, "text/") || mime == "application/json" || mime == "application/octet-stream") {
		return string(data), nil
	}
	return "", errors.New("document parser unavailable or file requires Tika/OCR")
}
