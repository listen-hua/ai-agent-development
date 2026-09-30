package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/ids"
	"internal-ai-agent/backend/internal/model"
	"internal-ai-agent/backend/internal/security"
	"internal-ai-agent/backend/internal/store"
)

var agentKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{2,49}$`)

type AgentProfileInput struct {
	AgentKey    string
	Name        string
	Description string
	Kind        string
	Provider    string
	Model       string
	APIKey      string
	Enabled     bool
	Settings    map[string]any
}

type AgentRegistry struct {
	repo               store.Repository
	secrets            *security.SecretBox
	environmentAPIKeys map[string]string
	defaultAliyunModel string
	defaultGeminiModel string
	imageAgentEnabled  bool
}

func NewAgentRegistry(repo store.Repository, encryptionSecret, aliyunAPIKey, geminiAPIKey, aliyunModel, geminiModel string, imageAgentEnabled ...bool) (*AgentRegistry, error) {
	secrets, err := security.NewSecretBox(encryptionSecret)
	if err != nil {
		return nil, err
	}
	imageEnabled := true
	if len(imageAgentEnabled) > 0 {
		imageEnabled = imageAgentEnabled[0]
	}
	return &AgentRegistry{
		repo:               repo,
		secrets:            secrets,
		environmentAPIKeys: map[string]string{"aliyun": aliyunAPIKey, "gemini": geminiAPIKey},
		defaultAliyunModel: aliyunModel,
		defaultGeminiModel: geminiModel,
		imageAgentEnabled:  imageEnabled,
	}, nil
}

func (a *AgentRegistry) EnsureDefaults(ctx context.Context) error {
	now := time.Now()
	defaults := []domain.AgentProfile{
		{ID: "00000000-0000-4000-8000-000000000101", AgentKey: "administrative_assistant", Name: "行政助手", Description: "基于公司制度知识库回答行政问题", Kind: "chat", Provider: "aliyun", Model: a.defaultAliyunModel, Enabled: true, CredentialSource: "environment", Settings: map[string]any{}, CreatedAt: now, UpdatedAt: now},
	}
	if a.imageAgentEnabled {
		defaults = append(defaults, domain.AgentProfile{ID: "00000000-0000-4000-8000-000000000102", AgentKey: "image_generator", Name: "AI 生图", Description: "在画布中使用 Gemini 生成和编辑图片", Kind: "image", Provider: "gemini", Model: a.defaultGeminiModel, Enabled: true, CredentialSource: "environment", Settings: map[string]any{"aspect_ratio": "1:1", "image_size": "1K"}, CreatedAt: now, UpdatedAt: now})
	}
	for _, value := range defaults {
		existing, err := a.repo.GetAgentProfileByKey(ctx, value.AgentKey)
		if err == nil {
			changed := false
			if looksLikeEncodingDamage(existing.Name) {
				existing.Name = value.Name
				changed = true
			}
			if looksLikeEncodingDamage(existing.Description) {
				existing.Description = value.Description
				changed = true
			}
			if changed {
				existing.UpdatedAt = now
				if err = a.repo.UpsertAgentProfile(ctx, existing); err != nil {
					return err
				}
			}
			continue
		}
		if !errors.Is(err, store.ErrNotFound) {
			return err
		}
		if err = a.repo.UpsertAgentProfile(ctx, value); err != nil {
			return err
		}
	}
	return nil
}

func looksLikeEncodingDamage(value string) bool {
	if strings.ContainsRune(value, '\uFFFD') {
		return true
	}
	questionMarks := strings.Count(value, "?")
	if questionMarks < 2 {
		return false
	}
	nonSpaceRunes := 0
	for _, current := range value {
		if !strings.ContainsRune(" \t\r\n", current) {
			nonSpaceRunes++
		}
	}
	return questionMarks*2 >= nonSpaceRunes
}

func (a *AgentRegistry) List(ctx context.Context, includeDisabled bool) ([]domain.AgentProfile, error) {
	values, err := a.repo.ListAgentProfiles(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]domain.AgentProfile, 0, len(values))
	for _, value := range values {
		if !a.imageAgentEnabled && (value.Kind == "image" || value.AgentKey == "image_generator") {
			continue
		}
		if !includeDisabled && !value.Enabled {
			continue
		}
		decorated := a.decorate(value)
		if !includeDisabled {
			decorated.APIKeyHint = ""
		}
		result = append(result, decorated)
	}
	return result, nil
}

func (a *AgentRegistry) Save(ctx context.Context, actor domain.User, id string, input AgentProfileInput) (domain.AgentProfile, error) {
	input.AgentKey = strings.TrimSpace(strings.ToLower(input.AgentKey))
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.Kind = strings.TrimSpace(strings.ToLower(input.Kind))
	input.Provider = strings.TrimSpace(strings.ToLower(input.Provider))
	input.Model = strings.TrimSpace(input.Model)
	input.APIKey = strings.TrimSpace(input.APIKey)
	if !a.imageAgentEnabled && (input.Kind == "image" || input.AgentKey == "image_generator") {
		return domain.AgentProfile{}, errors.New("image agents are disabled")
	}
	if err := validateAgentProfile(input); err != nil {
		return domain.AgentProfile{}, err
	}
	now := time.Now()
	value := domain.AgentProfile{ID: id, AgentKey: input.AgentKey, Name: input.Name, Description: input.Description, Kind: input.Kind, Provider: input.Provider, Model: input.Model, Enabled: input.Enabled, Settings: input.Settings, UpdatedBy: actor.ID, UpdatedAt: now}
	if value.Settings == nil {
		value.Settings = map[string]any{}
	}
	if id == "" {
		value.ID = ids.New("agent")
		value.CreatedBy = actor.ID
		value.CreatedAt = now
		value.CredentialSource = "environment"
	} else {
		existing, err := a.repo.GetAgentProfile(ctx, id)
		if err != nil {
			return value, err
		}
		value.CreatedBy = existing.CreatedBy
		value.CreatedAt = existing.CreatedAt
		value.CredentialSource = existing.CredentialSource
		value.EncryptedAPIKey = existing.EncryptedAPIKey
		value.APIKeyHint = existing.APIKeyHint
	}
	if input.APIKey != "" {
		encrypted, err := a.secrets.Encrypt(input.APIKey)
		if err != nil {
			return value, err
		}
		value.CredentialSource = "database"
		value.EncryptedAPIKey = encrypted
		value.APIKeyHint = apiKeyHint(input.APIKey)
	}
	if err := a.repo.UpsertAgentProfile(ctx, value); err != nil {
		return value, err
	}
	action := "agent.profile.create"
	if id != "" {
		action = "agent.profile.update"
	}
	_ = a.repo.AppendAudit(ctx, domain.AuditEvent{ID: ids.New("aud"), ActorID: actor.ID, ActorName: actor.Name, Action: action, ResourceType: "agent_profile", ResourceID: value.ID, Metadata: map[string]any{"agent_key": value.AgentKey, "provider": value.Provider, "model": value.Model, "api_key_updated": input.APIKey != ""}, CreatedAt: now})
	return a.decorate(value), nil
}

func (a *AgentRegistry) Credential(ctx context.Context, agentKey string) (domain.AgentProfile, string, error) {
	if !a.imageAgentEnabled && agentKey == "image_generator" {
		return domain.AgentProfile{}, "", errors.New("image agent is disabled")
	}
	value, err := a.repo.GetAgentProfileByKey(ctx, agentKey)
	if err != nil {
		return value, "", err
	}
	if !value.Enabled {
		return value, "", errors.New("agent is disabled")
	}
	if value.CredentialSource == "database" {
		secret, decryptErr := a.secrets.Decrypt(value.EncryptedAPIKey)
		return a.decorate(value), secret, decryptErr
	}
	secret := a.environmentAPIKeys[value.Provider]
	if secret == "" {
		return a.decorate(value), "", fmt.Errorf("API key is not configured for %s", value.Provider)
	}
	return a.decorate(value), secret, nil
}

func (a *AgentRegistry) ResolveModelCredential(ctx context.Context, agentKey string) (model.AgentCredential, error) {
	profile, secret, err := a.Credential(ctx, agentKey)
	if err != nil {
		return model.AgentCredential{}, err
	}
	return model.AgentCredential{Provider: profile.Provider, Model: profile.Model, APIKey: secret}, nil
}

func (a *AgentRegistry) decorate(value domain.AgentProfile) domain.AgentProfile {
	value.HasAPIKey = value.EncryptedAPIKey != ""
	if value.CredentialSource == "environment" {
		if secret := a.environmentAPIKeys[value.Provider]; secret != "" {
			value.HasAPIKey = true
			value.APIKeyHint = apiKeyHint(secret)
		} else {
			value.APIKeyHint = ""
		}
	}
	value.EncryptedAPIKey = ""
	return value
}

func validateAgentProfile(input AgentProfileInput) error {
	if !agentKeyPattern.MatchString(input.AgentKey) {
		return errors.New("agent_key must contain 3-50 lowercase letters, numbers or underscores")
	}
	if input.Name == "" || len([]rune(input.Name)) > 50 {
		return errors.New("agent name is required and cannot exceed 50 characters")
	}
	if len([]rune(input.Description)) > 200 {
		return errors.New("agent description cannot exceed 200 characters")
	}
	if input.Kind != "chat" && input.Kind != "image" {
		return errors.New("agent kind must be chat or image")
	}
	if input.Provider == "" || input.Model == "" {
		return errors.New("provider and model are required")
	}
	return nil
}

func apiKeyHint(value string) string {
	if len(value) <= 4 {
		return "••••"
	}
	return "••••" + value[len(value)-4:]
}
