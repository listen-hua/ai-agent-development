package service

import (
	"context"
	"testing"

	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/store"
)

func TestEnsureBootstrapSuperAdminsUpgradesExistingUser(t *testing.T) {
	repo := store.NewMemory(domain.AgentConfig{})
	user, err := repo.UpsertUser(context.Background(), domain.User{FeishuOpenID: "ou_real_user", Name: "正式用户", Roles: []domain.Role{domain.RoleEmployee}})
	if err != nil {
		t.Fatal(err)
	}
	granted, err := EnsureBootstrapSuperAdmins(context.Background(), repo, []string{user.FeishuOpenID})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := repo.GetUser(context.Background(), user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if granted != 1 || !updated.HasRole(domain.RoleSuperAdmin) {
		t.Fatalf("expected existing user to become super admin, granted=%d roles=%v", granted, updated.Roles)
	}
}

func TestEnsureBootstrapSuperAdminsSkipsUnknownUser(t *testing.T) {
	repo := store.NewMemory(domain.AgentConfig{})
	granted, err := EnsureBootstrapSuperAdmins(context.Background(), repo, []string{"ou_not_signed_in"})
	if err != nil || granted != 0 {
		t.Fatalf("expected unknown bootstrap identity to wait for login, granted=%d error=%v", granted, err)
	}
}
