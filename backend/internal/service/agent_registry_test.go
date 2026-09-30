package service

import (
	"context"
	"strings"
	"testing"

	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/store"
)

func TestAgentRegistryKeepsIndependentCredentials(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory(domain.AgentConfig{})
	registry, err := NewAgentRegistry(repo, "a-long-enough-session-secret", "aliyun-key", "", "qwen-plus", "gemini-3.1-flash-image")
	if err != nil {
		t.Fatal(err)
	}
	if err = registry.EnsureDefaults(ctx); err != nil {
		t.Fatal(err)
	}
	values, err := registry.List(ctx, true)
	if err != nil || len(values) != 2 {
		t.Fatalf("unexpected default agents: %#v, %v", values, err)
	}
	admin, _ := repo.GetUser(ctx, store.DemoAdminID)
	var image domain.AgentProfile
	for _, value := range values {
		if value.AgentKey == "administrative_assistant" && !value.HasAPIKey {
			t.Fatal("expected administrative assistant to use the environment key")
		}
		if value.AgentKey == "image_generator" {
			image = value
		}
	}
	updated, err := registry.Save(ctx, admin, image.ID, AgentProfileInput{AgentKey: image.AgentKey, Name: image.Name, Description: image.Description, Kind: image.Kind, Provider: image.Provider, Model: image.Model, APIKey: "gemini-database-key", Enabled: true, Settings: image.Settings})
	if err != nil {
		t.Fatal(err)
	}
	if !updated.HasAPIKey || updated.EncryptedAPIKey != "" || updated.APIKeyHint == "gemini-database-key" {
		t.Fatalf("secret leaked or was not marked configured: %#v", updated)
	}
	_, secret, err := registry.Credential(ctx, "image_generator")
	if err != nil || secret != "gemini-database-key" {
		t.Fatalf("unexpected resolved credential: %q, %v", secret, err)
	}
}

func TestAgentRegistryRepairsEncodingDamagedDefaultLabels(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory(domain.AgentConfig{})
	registry, err := NewAgentRegistry(repo, "a-long-enough-session-secret", "aliyun-key", "", "qwen-plus", "gemini-3.1-flash-image")
	if err != nil {
		t.Fatal(err)
	}
	if err = registry.EnsureDefaults(ctx); err != nil {
		t.Fatal(err)
	}
	image, err := repo.GetAgentProfileByKey(ctx, "image_generator")
	if err != nil {
		t.Fatal(err)
	}
	image.Name = "AI " + strings.Repeat("?", 2)
	image.Description = strings.Repeat("?", 6) + " Gemini " + strings.Repeat("?", 7)
	image.Model = "custom-image-model"
	image.CredentialSource = "database"
	image.EncryptedAPIKey = "encrypted-secret"
	if err = repo.UpsertAgentProfile(ctx, image); err != nil {
		t.Fatal(err)
	}

	if err = registry.EnsureDefaults(ctx); err != nil {
		t.Fatal(err)
	}
	repaired, err := repo.GetAgentProfileByKey(ctx, "image_generator")
	if err != nil {
		t.Fatal(err)
	}
	if repaired.Name != "AI 生图" || repaired.Description != "在画布中使用 Gemini 生成和编辑图片" {
		t.Fatalf("default labels were not repaired: %#v", repaired)
	}
	if repaired.Model != "custom-image-model" || repaired.EncryptedAPIKey != "encrypted-secret" || repaired.CredentialSource != "database" {
		t.Fatalf("repair overwrote administrator configuration: %#v", repaired)
	}
}

func TestAgentRegistryHidesImageProfilesWhenFeatureDisabled(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory(domain.AgentConfig{})
	enabled, err := NewAgentRegistry(repo, "a-long-enough-session-secret", "aliyun-key", "gemini-key", "qwen-plus", "gemini-image", true)
	if err != nil {
		t.Fatal(err)
	}
	if err = enabled.EnsureDefaults(ctx); err != nil {
		t.Fatal(err)
	}

	disabled, err := NewAgentRegistry(repo, "a-long-enough-session-secret", "aliyun-key", "gemini-key", "qwen-plus", "gemini-image", false)
	if err != nil {
		t.Fatal(err)
	}
	profiles, err := disabled.List(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, profile := range profiles {
		if profile.Kind == "image" || profile.AgentKey == "image_generator" {
			t.Fatalf("disabled image profile was exposed: %#v", profile)
		}
	}
	admin, _ := repo.GetUser(ctx, store.DemoAdminID)
	_, err = disabled.Save(ctx, admin, "", AgentProfileInput{AgentKey: "another_image", Name: "图片", Kind: "image", Provider: "custom", Model: "image-model", Enabled: true})
	if err == nil {
		t.Fatal("expected image profile creation to be rejected while feature is disabled")
	}
}
