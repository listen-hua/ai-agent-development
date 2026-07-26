package model

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Aliyun struct {
	apiKey, baseURL string
	client          *http.Client
}

func NewAliyun(apiKey, baseURL string) *Aliyun {
	return &Aliyun{apiKey: apiKey, baseURL: strings.TrimRight(baseURL, "/"), client: &http.Client{Timeout: 90 * time.Second}}
}
func (a *Aliyun) Available() bool { return a.apiKey != "" }

func (a *Aliyun) Generate(ctx context.Context, input GenerateRequest) (string, error) {
	var output struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	payload := map[string]any{"model": input.Model, "messages": input.Messages, "temperature": input.Temperature, "enable_search": false}
	if input.JSONMode {
		payload["response_format"] = map[string]string{"type": "json_object"}
		payload["enable_thinking"] = false
	}
	if input.MaxTokens > 0 {
		payload["max_tokens"] = input.MaxTokens
	}
	err := a.call(ctx, "/chat/completions", payload, &output)
	if err != nil {
		return "", err
	}
	if len(output.Choices) == 0 {
		return "", errors.New("aliyun returned no choices")
	}
	return output.Choices[0].Message.Content, nil
}

func (a *Aliyun) StreamGenerate(ctx context.Context, input GenerateRequest, onDelta func(string)) (string, error) {
	if !a.Available() {
		return "", errors.New("DASHSCOPE_API_KEY is not configured")
	}
	payload := map[string]any{
		"model":          input.Model,
		"messages":       input.Messages,
		"temperature":    input.Temperature,
		"max_tokens":     input.MaxTokens,
		"enable_search":  false,
		"stream":         true,
		"stream_options": map[string]bool{"include_usage": true},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		limited, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("aliyun request failed (%d): %s", resp.StatusCode, string(limited))
	}

	var answer strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 16<<10), 1<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
			Error *struct {
				Message string `json:"message"`
				Code    string `json:"code"`
			} `json:"error"`
		}
		if err = json.Unmarshal([]byte(data), &chunk); err != nil {
			return answer.String(), fmt.Errorf("decode aliyun stream chunk: %w", err)
		}
		if chunk.Error != nil {
			return answer.String(), fmt.Errorf("aliyun stream failed (%s): %s", chunk.Error.Code, chunk.Error.Message)
		}
		if len(chunk.Choices) == 0 || chunk.Choices[0].Delta.Content == "" {
			continue
		}
		delta := chunk.Choices[0].Delta.Content
		answer.WriteString(delta)
		if onDelta != nil {
			onDelta(delta)
		}
	}
	if err = scanner.Err(); err != nil {
		return answer.String(), fmt.Errorf("read aliyun stream: %w", err)
	}
	if answer.Len() == 0 {
		return "", errors.New("aliyun returned no streamed content")
	}
	return answer.String(), nil
}

func (a *Aliyun) Embed(ctx context.Context, modelName string, values []string, dimensions int) ([][]float32, error) {
	var output struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
	}
	err := a.call(ctx, "/embeddings", map[string]any{"model": modelName, "input": values, "dimensions": dimensions, "encoding_format": "float"}, &output)
	if err != nil {
		return nil, err
	}
	result := make([][]float32, len(values))
	for _, item := range output.Data {
		if item.Index >= 0 && item.Index < len(result) {
			result[item.Index] = item.Embedding
		}
	}
	return result, nil
}

func (a *Aliyun) Rerank(ctx context.Context, modelName, query string, documents []string, topN int) ([]int, error) {
	var output struct {
		Results []struct {
			Index          int     `json:"index"`
			RelevanceScore float64 `json:"relevance_score"`
		} `json:"results"`
	}
	err := a.call(ctx, "/reranks", map[string]any{"model": modelName, "query": query, "documents": documents, "top_n": topN}, &output)
	if err != nil {
		return nil, err
	}
	result := make([]int, 0, len(output.Results))
	for _, item := range output.Results {
		result = append(result, item.Index)
	}
	return result, nil
}

func (a *Aliyun) call(ctx context.Context, path string, input, output any) error {
	if !a.Available() {
		return errors.New("DASHSCOPE_API_KEY is not configured")
	}
	body, err := json.Marshal(input)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		limited, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("aliyun request failed (%d): %s", resp.StatusCode, string(limited))
	}
	return json.NewDecoder(resp.Body).Decode(output)
}
