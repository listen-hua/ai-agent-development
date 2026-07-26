package model

import (
	"context"
	"errors"
	"fmt"
)

type AgentCredential struct {
	Provider string
	Model    string
	APIKey   string
}

type AgentCredentialResolver interface {
	ResolveModelCredential(context.Context, string) (AgentCredential, error)
}

type AgentRoutedProvider struct {
	resolver  AgentCredentialResolver
	agentKey  string
	aliyunURL string
	fallback  Provider
}

func NewAgentRoutedProvider(resolver AgentCredentialResolver, agentKey, aliyunURL string, fallback Provider) *AgentRoutedProvider {
	return &AgentRoutedProvider{resolver: resolver, agentKey: agentKey, aliyunURL: aliyunURL, fallback: fallback}
}

func (p *AgentRoutedProvider) Generate(ctx context.Context, input GenerateRequest) (string, error) {
	provider, modelName, err := p.provider(ctx)
	if err != nil {
		return "", err
	}
	if modelName != "" && !input.PreserveModel {
		input.Model = modelName
	}
	return provider.Generate(ctx, input)
}

func (p *AgentRoutedProvider) StreamGenerate(ctx context.Context, input GenerateRequest, onDelta func(string)) (string, error) {
	provider, modelName, err := p.provider(ctx)
	if err != nil {
		return "", err
	}
	if modelName != "" && !input.PreserveModel {
		input.Model = modelName
	}
	return provider.StreamGenerate(ctx, input, onDelta)
}

func (p *AgentRoutedProvider) Embed(ctx context.Context, modelName string, values []string, dimensions int) ([][]float32, error) {
	provider, _, err := p.provider(ctx)
	if err != nil {
		return nil, err
	}
	return provider.Embed(ctx, modelName, values, dimensions)
}

func (p *AgentRoutedProvider) Rerank(ctx context.Context, modelName, query string, documents []string, topN int) ([]int, error) {
	provider, _, err := p.provider(ctx)
	if err != nil {
		return nil, err
	}
	return provider.Rerank(ctx, modelName, query, documents, topN)
}

func (p *AgentRoutedProvider) Available() bool {
	return p.resolver != nil || (p.fallback != nil && p.fallback.Available())
}

func (p *AgentRoutedProvider) provider(ctx context.Context) (Provider, string, error) {
	if p.resolver != nil {
		credential, err := p.resolver.ResolveModelCredential(ctx, p.agentKey)
		if err == nil {
			if credential.APIKey == "" {
				return nil, "", errors.New("agent API key is empty")
			}
			switch credential.Provider {
			case "aliyun":
				return NewAliyun(credential.APIKey, p.aliyunURL), credential.Model, nil
			default:
				return nil, "", fmt.Errorf("text provider %s is not implemented", credential.Provider)
			}
		}
	}
	if p.fallback != nil {
		return p.fallback, "", nil
	}
	return nil, "", errors.New("no model provider is available")
}
