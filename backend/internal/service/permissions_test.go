package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"internal-ai-agent/backend/internal/domain"
	iamintegration "internal-ai-agent/backend/internal/integration/iam"
	"internal-ai-agent/backend/internal/store"
)

func TestPermissionResolverMergesIAMAllowAndLocalDeny(t *testing.T) {
	iamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/app-user-effective-permissions" {
			t.Fatalf("unexpected IAM path: %s", r.URL.Path)
		}
		if r.Header.Get("X-IAM-App-ID") != "shimmer" || r.Header.Get("X-IAM-App-Secret") != "secret" {
			t.Fatal("missing service credentials")
		}
		_, _ = w.Write([]byte(`{"code":0,"data":{"iam_user_id":42,"permission_keys":["agent_use","notification_manage"],"policy_version":7}}`))
	}))
	defer iamServer.Close()

	repo := store.NewMemory(domain.AgentConfig{})
	iamID := int64(42)
	user, err := repo.UpsertIAMUser(context.Background(), domain.User{
		FeishuOpenID: "ou_permissions", FeishuUserID: "fs_42", IAMUserID: &iamID,
		Name: "Permission User", Status: "active",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.UpdateLocalPermissionPolicy(context.Background(), domain.LocalPermissionPolicy{
		UserID:    user.ID,
		AllowKeys: []domain.PermissionKey{domain.PermissionKnowledgeManage},
		DenyKeys:  []domain.PermissionKey{domain.PermissionNotificationManage},
		Reason:    "test",
	}, 0)
	if err != nil {
		t.Fatal(err)
	}

	client := iamintegration.New(iamServer.URL, "shimmer", "secret", "", 30*time.Second, time.Second)
	resolved, err := NewPermissionResolver(repo, client).Resolve(context.Background(), user, "")
	if err != nil {
		t.Fatal(err)
	}
	if !resolved.HasPermission(domain.PermissionAgentUse) || !resolved.HasPermission(domain.PermissionKnowledgeManage) {
		t.Fatalf("expected IAM and local allow permissions, got %v", resolved.Permissions)
	}
	if resolved.HasPermission(domain.PermissionNotificationManage) {
		t.Fatalf("local deny must override IAM, got %v", resolved.Permissions)
	}
}

func TestPermissionResolverUsesOnlyAgentUseFromOlderOutageSnapshot(t *testing.T) {
	unavailable := false
	iamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if unavailable {
			http.Error(w, "down", http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(`{"code":0,"data":{"permission_keys":["agent_use","user_manage"]}}`))
	}))
	defer iamServer.Close()

	repo := store.NewMemory(domain.AgentConfig{})
	user, err := repo.UpsertUser(context.Background(), domain.User{
		FeishuOpenID: "ou_outage", FeishuUserID: "fs_outage", Name: "Outage User", Status: "active",
	})
	if err != nil {
		t.Fatal(err)
	}
	client := iamintegration.New(iamServer.URL, "shimmer", "secret", "", time.Millisecond, time.Second)
	resolver := NewPermissionResolver(repo, client)
	now := time.Now()
	resolver.now = func() time.Time { return now }
	if _, err = resolver.Resolve(context.Background(), user, ""); err != nil {
		t.Fatal(err)
	}

	unavailable = true
	now = now.Add(31 * time.Second)
	resolved, err := resolver.Refresh(context.Background(), user)
	if err != nil {
		t.Fatal(err)
	}
	if !resolved.HasPermission(domain.PermissionAgentUse) || resolved.HasPermission(domain.PermissionUserManage) {
		t.Fatalf("older outage snapshot must retain only agent_use, got %v", resolved.Permissions)
	}
	if resolved.PermissionError == "" {
		t.Fatal("expected the IAM outage to be visible in the resolved user")
	}
}

func TestPermissionResolverProtectsLastUsableUserManager(t *testing.T) {
	repo := store.NewMemory(domain.AgentConfig{})
	resolver := NewPermissionResolver(repo, nil)
	admin, err := repo.GetUser(context.Background(), store.DemoAdminID)
	if err != nil {
		t.Fatal(err)
	}
	admin, err = resolver.Resolve(context.Background(), admin, "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = resolver.UpdateLocalPolicy(
		context.Background(), admin, admin,
		[]domain.PermissionKey{domain.PermissionUserManage},
		nil,
		admin.LocalPermissionPolicy.Version,
		"remove own access",
	)
	if !errors.Is(err, ErrLastPermissionAdministrator) {
		t.Fatalf("expected last administrator protection, got %v", err)
	}
}
