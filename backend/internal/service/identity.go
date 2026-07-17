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
		if user.HasRole(domain.RoleSuperAdmin) {
			continue
		}
		roles := append(append([]domain.Role{}, user.Roles...), domain.RoleSuperAdmin)
		updated, err := repo.UpdateUserRoles(ctx, user.ID, roles)
		if err != nil {
			return granted, err
		}
		granted++
		_ = repo.AppendAudit(ctx, domain.AuditEvent{
			ID: ids.New("aud"), ActorID: updated.ID, ActorName: updated.Name,
			Action: "identity.bootstrap_super_admin", ResourceType: "user", ResourceID: updated.ID,
			Metadata: map[string]any{"source": "deployment_config"}, CreatedAt: time.Now(),
		})
	}
	return granted, nil
}
