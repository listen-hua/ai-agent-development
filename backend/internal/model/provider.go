package model

import "context"

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GenerateRequest struct {
	Model       string
	Messages    []Message
	Temperature float64
	MaxTokens   int
	JSONMode    bool
}

type Provider interface {
	Generate(context.Context, GenerateRequest) (string, error)
	StreamGenerate(context.Context, GenerateRequest, func(string)) (string, error)
	Embed(context.Context, string, []string, int) ([][]float32, error)
	Rerank(context.Context, string, string, []string, int) ([]int, error)
	Available() bool
}
