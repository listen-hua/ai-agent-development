package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/ids"
	"internal-ai-agent/backend/internal/integration/feishu"
	"internal-ai-agent/backend/internal/store"
)

type Directory struct {
	repo   store.Repository
	feishu *feishu.Client
}

func NewDirectory(repo store.Repository, client *feishu.Client) *Directory {
	return &Directory{repo: repo, feishu: client}
}

func (d *Directory) Enrich(ctx context.Context, base domain.User) (domain.User, error) {
	profile, err := d.feishu.GetContactUser(ctx, base.FeishuOpenID)
	if err != nil {
		return base, err
	}
	now := time.Now()
	base.DepartmentIDs = profile.DepartmentIDs
	base.JobTitle = profile.JobTitle
	base.JobLevelID = profile.JobLevelID
	base.JobFamilyID = profile.JobFamilyID
	base.EmployeeType = profile.EmployeeType
	base.Status = profile.Status
	base.OrganizationSyncedAt = &now
	if profile.Name != "" {
		base.Name = profile.Name
	}
	if profile.AvatarURL != "" {
		base.AvatarURL = profile.AvatarURL
	}
	return base, nil
}

func (d *Directory) SyncUser(ctx context.Context, openID string) (domain.User, error) {
	base, err := d.repo.GetUserByOpenID(ctx, openID)
	if errors.Is(err, store.ErrNotFound) {
		base = domain.User{FeishuOpenID: openID, Roles: []domain.Role{domain.RoleEmployee}, Status: "active"}
	} else if err != nil {
		return domain.User{}, err
	}
	base, err = d.Enrich(ctx, base)
	if err != nil {
		return domain.User{}, err
	}
	return d.repo.UpsertUser(ctx, base)
}

func (d *Directory) SyncKnownUsers(ctx context.Context, actor domain.User) (int, int) {
	users, err := d.repo.ListUsers(ctx)
	if err != nil {
		return 0, 1
	}
	succeeded, failed := 0, 0
	for _, user := range users {
		if user.FeishuOpenID == "" || len(user.FeishuOpenID) >= 8 && user.FeishuOpenID[:8] == "ou_demo_" {
			continue
		}
		if _, err = d.SyncUser(ctx, user.FeishuOpenID); err != nil {
			failed++
			slog.Warn("feishu directory user sync failed", "open_id", user.FeishuOpenID, "error", err)
			continue
		}
		succeeded++
	}
	_ = d.repo.AppendAudit(ctx, domain.AuditEvent{ID: ids.New("aud"), ActorID: actor.ID, ActorName: actor.Name, Action: "directory.sync", ResourceType: "user", ResourceID: "known_users", Metadata: map[string]any{"succeeded": succeeded, "failed": failed}, CreatedAt: time.Now()})
	return succeeded, failed
}

func (d *Directory) HandleContactChange(ctx context.Context, event feishu.ContactChangeEvent) error {
	marked := event.EventID != "" && d.repo.MarkEventProcessed(ctx, event.EventID)
	if event.EventID != "" && !marked {
		return nil
	}
	var err error
	if event.Change == "deleted" {
		err = d.repo.UpdateUserStatus(ctx, event.OpenID, "inactive")
		if errors.Is(err, store.ErrNotFound) {
			return nil
		}
	} else {
		_, err = d.SyncUser(ctx, event.OpenID)
	}
	if err != nil && marked {
		d.repo.ForgetProcessedEvent(ctx, event.EventID)
	}
	return err
}
