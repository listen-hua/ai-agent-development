package model

import (
	"context"
	"fmt"
	"strings"
)

type Mock struct{}

func (Mock) Available() bool { return true }
func (Mock) Generate(_ context.Context, input GenerateRequest) (string, error) {
	contextText := ""
	question := ""
	for _, m := range input.Messages {
		if m.Role == "system" && strings.Contains(m.Content, "资料") {
			contextText = m.Content
		}
		if m.Role == "user" {
			question = m.Content
		}
	}
	if contextText == "" {
		return "现有制度资料中未找到可靠依据。请联系行政同事确认。", nil
	}
	return fmt.Sprintf("根据当前已发布的公司制度，关于“%s”的规定请以引用原文为准。[1]\n\n这是本地演示模式生成的回答；配置阿里云百炼密钥后将基于命中片段生成完整答案。", question), nil
}
func (m Mock) StreamGenerate(ctx context.Context, input GenerateRequest, onDelta func(string)) (string, error) {
	answer, err := m.Generate(ctx, input)
	if err != nil {
		return "", err
	}
	if onDelta != nil && answer != "" {
		onDelta(answer)
	}
	return answer, nil
}
func (Mock) Embed(_ context.Context, _ string, values []string, dimensions int) ([][]float32, error) {
	out := make([][]float32, len(values))
	for i := range out {
		out[i] = make([]float32, dimensions)
	}
	return out, nil
}
func (Mock) Rerank(_ context.Context, _ string, _ string, docs []string, topN int) ([]int, error) {
	if topN > len(docs) {
		topN = len(docs)
	}
	out := make([]int, topN)
	for i := range out {
		out[i] = i
	}
	return out, nil
}
