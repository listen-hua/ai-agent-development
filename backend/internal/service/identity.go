package service

import (
	"context"
	"errors"
	"time"

	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/ids"
	"internal-ai-agent/backend/internal/store"
)

// EnsureBootstrapSuperAdmins reconciles configured bootstrap identities at
// process startup. This also upgrades users with an existing session, so
// granting the first administrator does not depend on another OAuth exchange.
func EnsureBootstrapSuperAdmins(ctx context.Context, repo store.Repository, openIDs []string) (int, error) {
	granted := 0
	for _, openID := range openIDs {
		user, err := repo.GetUserByOpenID(ctx, openID)
		if errors.Is(err, store.ErrNotFound) {
			// A user who has never signed in will be granted the role by the login flow.
			continue
		}
		if err != nil {
			return granted, err
		}
		policy, policyErr := repo.GetLocalPermissionPolicy(ctx, user.ID)
		if policyErr != nil && !errors.Is(policyErr, store.ErrNotFound) {
			return granted, policyErr
		}
		if containsAllPermissions(policy.AllowKeys) {
			continue
		}
		resolver := NewPermissionResolver(repo, nil)
		if err = resolver.EnsureLocalAllow(ctx, user.ID, user.ID, domain.AllPermissionKeys, "Bootstrap super administrator"); err != nil {
			return granted, err
		}
		granted++
		_ = repo.AppendAudit(ctx, domain.AuditEvent{
			ID: ids.New("aud"), ActorID: user.ID, ActorName: user.Name,
			Action: "identity.bootstrap_super_admin", ResourceType: "user", ResourceID: user.ID,
			Metadata: map[string]any{"source": "deployment_config"}, CreatedAt: time.Now(),
		})
	}
	return granted, nil
}

func containsAllPermissions(keys []domain.PermissionKey) bool {
	if len(keys) < len(domain.AllPermissionKeys) {
		return false
	}
	set := permissionSet(keys)
	for _, key := range domain.AllPermissionKeys {
		if !set[key] {
			return false
		}
	}
	return true
}
