package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/ids"
	iamintegration "internal-ai-agent/backend/internal/integration/iam"
	"internal-ai-agent/backend/internal/store"
)

var ErrPermissionDenied = errors.New("permission denied")
var ErrLastPermissionAdministrator = errors.New("at least one effective user_manage administrator is required")

type PermissionResolver struct {
	repo store.Repository
	iam  *iamintegration.Client
	now  func() time.Time
}

func NewPermissionResolver(repo store.Repository, iamClient *iamintegration.Client) *PermissionResolver {
	return &PermissionResolver{repo: repo, iam: iamClient, now: time.Now}
}

func (r *PermissionResolver) Resolve(ctx context.Context, user domain.User, iamToken string) (domain.User, error) {
	return r.resolve(ctx, user, iamToken, false)
}

func (r *PermissionResolver) Refresh(ctx context.Context, user domain.User) (domain.User, error) {
	return r.resolve(ctx, user, "", true)
}

func (r *PermissionResolver) ResolveStored(ctx context.Context, user domain.User) (domain.User, error) {
	policy, err := r.repo.GetLocalPermissionPolicy(ctx, user.ID)
	if errors.Is(err, store.ErrNotFound) {
		policy = domain.LocalPermissionPolicy{UserID: user.ID, AllowKeys: []domain.PermissionKey{}, DenyKeys: []domain.PermissionKey{}}
	} else if err != nil {
		return domain.User{}, err
	}
	snapshot, snapshotErr := r.repo.GetIAMPermissionSnapshot(ctx, user.ID)
	if errors.Is(snapshotErr, store.ErrNotFound) {
		snapshot = domain.IAMPermissionSnapshot{UserID: user.ID, PermissionKeys: []domain.PermissionKey{}}
	} else if snapshotErr != nil {
		return domain.User{}, snapshotErr
	}
	return attachResolvedPermissions(user, policy, snapshot.PermissionKeys, snapshot.SyncedAt, snapshot.LastError), nil
}

func (r *PermissionResolver) resolve(ctx context.Context, user domain.User, iamToken string, force bool) (domain.User, error) {
	policy, err := r.repo.GetLocalPermissionPolicy(ctx, user.ID)
	if errors.Is(err, store.ErrNotFound) {
		policy = domain.LocalPermissionPolicy{
			UserID: user.ID, AllowKeys: []domain.PermissionKey{}, DenyKeys: []domain.PermissionKey{},
		}
	} else if err != nil {
		return domain.User{}, err
	}

	iamKeys, syncedAt, iamErr := r.resolveIAM(ctx, user, iamToken, force)
	synced := time.Time{}
	if syncedAt != nil {
		synced = *syncedAt
	}
	user = attachResolvedPermissions(user, policy, iamKeys, synced, "")
	if iamErr != nil {
		user.PermissionError = iamErr.Error()
	}
	return user, nil
}

func (r *PermissionResolver) resolveIAM(ctx context.Context, user domain.User, iamToken string, force bool) ([]domain.PermissionKey, *time.Time, error) {
	if r.iam == nil || !r.iam.Configured() {
		return []domain.PermissionKey{}, nil, nil
	}
	now := r.now()
	var (
		keys          []domain.PermissionKey
		policyVersion string
		sourceUpdated *time.Time
		iamUserID     = user.IAMUserID
		err           error
	)
	var result iamintegration.EffectivePermissionResult
	result, err = r.iam.EffectivePermissions(ctx, user.IAMUserID, user.FeishuUserID, force)
	if err == nil {
		keys = validPermissionKeys(result.PermissionKeys)
		policyVersion = result.PolicyVersion
		sourceUpdated = result.SourceUpdatedAt
		if result.IAMUserID != nil {
			iamUserID = result.IAMUserID
		}
	} else if strings.TrimSpace(iamToken) != "" {
		if code, ok := iamintegration.ErrorCode(err); ok && code == iamintegration.CodeUnavailable {
			var granted []string
			granted, err = r.iam.GrantedPermissions(ctx, iamToken, permissionKeyStrings(domain.AllPermissionKeys))
			keys = validPermissionKeys(granted)
		}
	}
	if err == nil {
		snapshot := domain.IAMPermissionSnapshot{
			UserID: user.ID, IAMUserID: iamUserID, PermissionKeys: keys,
			PolicyVersion: policyVersion, SourceUpdatedAt: sourceUpdated, SyncedAt: now,
		}
		if saveErr := r.repo.SaveIAMPermissionSnapshot(ctx, snapshot); saveErr != nil {
			return nil, nil, saveErr
		}
		return keys, &now, nil
	}

	if code, ok := iamintegration.ErrorCode(err); ok && code == iamintegration.CodePermissionDenied {
		snapshot := domain.IAMPermissionSnapshot{
			UserID: user.ID, IAMUserID: iamUserID, PermissionKeys: []domain.PermissionKey{}, SyncedAt: now,
		}
		if saveErr := r.repo.SaveIAMPermissionSnapshot(ctx, snapshot); saveErr != nil {
			return nil, nil, saveErr
		}
		r.appendPermissionAudit(ctx, user, "identity.iam_permissions.empty", map[string]any{"iam_user_id": iamUserID})
		return []domain.PermissionKey{}, &now, nil
	}

	snapshot, snapshotErr := r.repo.GetIAMPermissionSnapshot(ctx, user.ID)
	if snapshotErr == nil {
		snapshot.LastError = err.Error()
		_ = r.repo.SaveIAMPermissionSnapshot(ctx, snapshot)
		age := now.Sub(snapshot.SyncedAt)
		switch {
		case age <= 30*time.Second:
			keys = snapshot.PermissionKeys
		case age <= 5*time.Minute && containsPermission(snapshot.PermissionKeys, domain.PermissionAgentUse):
			keys = []domain.PermissionKey{domain.PermissionAgentUse}
		default:
			keys = []domain.PermissionKey{}
		}
		syncedAt := snapshot.SyncedAt
		r.appendPermissionAudit(ctx, user, "identity.iam_permissions.degraded", map[string]any{
			"error": err.Error(), "snapshot_age_seconds": int64(age.Seconds()), "fallback_permissions": keys,
		})
		return keys, &syncedAt, err
	}
	if !errors.Is(snapshotErr, store.ErrNotFound) {
		return nil, nil, snapshotErr
	}
	_ = r.repo.SaveIAMPermissionSnapshot(ctx, domain.IAMPermissionSnapshot{
		UserID: user.ID, IAMUserID: iamUserID, PermissionKeys: []domain.PermissionKey{},
		SyncedAt: now, LastError: err.Error(),
	})
	r.appendPermissionAudit(ctx, user, "identity.iam_permissions.unavailable", map[string]any{"error": err.Error()})
	return []domain.PermissionKey{}, nil, err
}

func (r *PermissionResolver) ListUsers(ctx context.Context) ([]domain.User, error) {
	users, err := r.repo.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.User, 0, len(users))
	for _, user := range users {
		resolved, resolveErr := r.ResolveStored(ctx, user)
		if resolveErr != nil {
			return nil, resolveErr
		}
		out = append(out, resolved)
	}
	return out, nil
}

func attachResolvedPermissions(user domain.User, policy domain.LocalPermissionPolicy, iamKeys []domain.PermissionKey, syncedAt time.Time, permissionError string) domain.User {
	allowSet := permissionSet(policy.AllowKeys)
	denySet := permissionSet(policy.DenyKeys)
	iamSet := permissionSet(iamKeys)
	user.Permissions = applyPermissionPolicy(iamKeys, policy.AllowKeys, policy.DenyKeys)
	user.PermissionSources = domain.PermissionSources{
		IAM:        orderedPermissionKeys(iamSet),
		LocalAllow: orderedPermissionKeys(allowSet),
		LocalDeny:  orderedPermissionKeys(denySet),
	}
	user.LocalPermissionPolicy = &policy
	if !syncedAt.IsZero() {
		user.PermissionSyncedAt = &syncedAt
	}
	user.PermissionError = permissionError
	return user
}

func (r *PermissionResolver) UpdateLocalPolicy(ctx context.Context, actor, target domain.User, allowKeys, denyKeys []domain.PermissionKey, version int64, reason string) (domain.User, error) {
	if !actor.HasPermission(domain.PermissionUserManage) {
		return domain.User{}, ErrPermissionDenied
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return domain.User{}, fmt.Errorf("permission change reason is required")
	}
	normalizedAllow, err := domain.NormalizePermissionKeys(allowKeys)
	if err != nil {
		return domain.User{}, err
	}
	normalizedDeny, err := domain.NormalizePermissionKeys(denyKeys)
	if err != nil {
		return domain.User{}, err
	}
	allowSet := permissionSet(normalizedAllow)
	for _, key := range normalizedDeny {
		if allowSet[key] {
			return domain.User{}, fmt.Errorf("permission %s cannot be both allowed and denied", key)
		}
	}
	current, err := r.Resolve(ctx, target, "")
	if err != nil {
		return domain.User{}, err
	}
	candidate := applyPermissionPolicy(current.PermissionSources.IAM, normalizedAllow, normalizedDeny)
	currentAdministrator := current.HasPermission(domain.PermissionAgentUse) && current.HasPermission(domain.PermissionUserManage)
	candidateAdministrator := containsPermission(candidate, domain.PermissionAgentUse) && containsPermission(candidate, domain.PermissionUserManage)
	if currentAdministrator && !candidateAdministrator {
		hasOtherAdministrator, checkErr := r.hasOtherUserManager(ctx, target.ID)
		if checkErr != nil {
			return domain.User{}, checkErr
		}
		if !hasOtherAdministrator {
			return domain.User{}, ErrLastPermissionAdministrator
		}
	}
	beforeAllow := append([]domain.PermissionKey(nil), current.PermissionSources.LocalAllow...)
	beforeDeny := append([]domain.PermissionKey(nil), current.PermissionSources.LocalDeny...)
	policy, err := r.repo.UpdateLocalPermissionPolicy(ctx, domain.LocalPermissionPolicy{
		UserID: target.ID, AllowKeys: normalizedAllow, DenyKeys: normalizedDeny,
		UpdatedBy: actor.ID, Reason: reason,
	}, version)
	if err != nil {
		return domain.User{}, err
	}
	updated, err := r.Resolve(ctx, target, "")
	if err != nil {
		return domain.User{}, err
	}
	_ = r.repo.AppendAudit(ctx, domain.AuditEvent{
		ID: ids.New("aud"), ActorID: actor.ID, ActorName: actor.Name,
		Action: "identity.permissions.update", ResourceType: "user", ResourceID: target.ID,
		Metadata: map[string]any{
			"before_allow": beforeAllow, "before_deny": beforeDeny,
			"after_allow": policy.AllowKeys, "after_deny": policy.DenyKeys,
			"reason": reason, "version": policy.Version,
		},
		CreatedAt: r.now(),
	})
	return updated, nil
}

func (r *PermissionResolver) EnsureLocalAllow(ctx context.Context, actorID, userID string, keys []domain.PermissionKey, reason string) error {
	policy, err := r.repo.GetLocalPermissionPolicy(ctx, userID)
	if errors.Is(err, store.ErrNotFound) {
		policy = domain.LocalPermissionPolicy{UserID: userID}
	} else if err != nil {
		return err
	}
	set := permissionSet(policy.AllowKeys)
	denySet := permissionSet(policy.DenyKeys)
	for _, key := range keys {
		set[key] = true
		delete(denySet, key)
	}
	_, err = r.repo.UpdateLocalPermissionPolicy(ctx, domain.LocalPermissionPolicy{
		UserID: userID, AllowKeys: orderedPermissionKeys(set), DenyKeys: orderedPermissionKeys(denySet),
		UpdatedBy: actorID, Reason: reason,
	}, policy.Version)
	return err
}

func (r *PermissionResolver) appendPermissionAudit(ctx context.Context, user domain.User, action string, metadata map[string]any) {
	_ = r.repo.AppendAudit(ctx, domain.AuditEvent{
		ID: ids.New("aud"), ActorID: user.ID, ActorName: user.Name,
		Action: action, ResourceType: "user_permission", ResourceID: user.ID,
		Metadata: metadata, CreatedAt: r.now(),
	})
}

func (r *PermissionResolver) hasOtherUserManager(ctx context.Context, excludedUserID string) (bool, error) {
	users, err := r.repo.ListUsers(ctx)
	if err != nil {
		return false, err
	}
	for _, user := range users {
		if user.ID == excludedUserID || user.Status == "disabled" {
			continue
		}
		resolved, resolveErr := r.Resolve(ctx, user, "")
		if resolveErr != nil {
			return false, resolveErr
		}
		if resolved.HasPermission(domain.PermissionAgentUse) && resolved.HasPermission(domain.PermissionUserManage) {
			return true, nil
		}
	}
	return false, nil
}

func applyPermissionPolicy(iamKeys, allowKeys, denyKeys []domain.PermissionKey) []domain.PermissionKey {
	iamSet := permissionSet(iamKeys)
	allowSet := permissionSet(allowKeys)
	denySet := permissionSet(denyKeys)
	result := make([]domain.PermissionKey, 0, len(domain.AllPermissionKeys))
	for _, key := range domain.AllPermissionKeys {
		if (iamSet[key] || allowSet[key]) && !denySet[key] {
			result = append(result, key)
		}
	}
	return result
}

func permissionSet(keys []domain.PermissionKey) map[domain.PermissionKey]bool {
	out := make(map[domain.PermissionKey]bool, len(keys))
	for _, key := range keys {
		if domain.ValidPermissionKey(key) {
			out[key] = true
		}
	}
	return out
}

func orderedPermissionKeys(set map[domain.PermissionKey]bool) []domain.PermissionKey {
	out := make([]domain.PermissionKey, 0, len(set))
	for _, key := range domain.AllPermissionKeys {
		if set[key] {
			out = append(out, key)
		}
	}
	return out
}

func validPermissionKeys(keys []string) []domain.PermissionKey {
	out := make([]domain.PermissionKey, 0, len(keys))
	seen := map[domain.PermissionKey]bool{}
	for _, value := range keys {
		key := domain.PermissionKey(strings.TrimSpace(value))
		if domain.ValidPermissionKey(key) && !seen[key] {
			seen[key] = true
			out = append(out, key)
		}
	}
	return out
}

func permissionKeyStrings(keys []domain.PermissionKey) []string {
	out := make([]string, len(keys))
	for i, key := range keys {
		out[i] = string(key)
	}
	return out
}

func containsPermission(keys []domain.PermissionKey, expected domain.PermissionKey) bool {
	for _, key := range keys {
		if key == expected {
			return true
		}
	}
	return false
}
