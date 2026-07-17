package service

import (
	"context"
	"testing"
	"time"

	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/integration/feishu"
	"internal-ai-agent/backend/internal/model"
	"internal-ai-agent/backend/internal/store"
)

func TestNotificationCannotSkipApproval(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory(domain.AgentConfig{GenerationModel: "mock"})
	admin, _ := repo.GetUser(ctx, store.DemoAdminID)
	service := NewNotification(repo, feishu.New("", "", ""), model.Mock{})
	draft, err := service.Create(ctx, admin, "测试通知", "正文", domain.ACL{Scope: "all"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Send(ctx, admin, draft.ID); err == nil {
		t.Fatal("expected unapproved send to fail")
	}
	if _, err = service.Approve(ctx, admin, draft.ID); err != nil {
		t.Fatal(err)
	}
	sent, err := service.Send(ctx, admin, draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sent.Status != "sent" {
		t.Fatalf("unexpected status: %s", sent.Status)
	}
}

func TestTargetedNotificationSchedulesAfterApproval(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory(domain.AgentConfig{GenerationModel: "mock"})
	admin, _ := repo.GetUser(ctx, store.DemoAdminID)
	notifications := NewNotification(repo, feishu.New("", "", ""), model.Mock{})
	scheduledAt := time.Now().Add(time.Hour)
	draft, err := notifications.CreateTargeted(ctx, admin, "计划通知", "**富文本**正文", "user", []string{store.DemoEmployeeID}, []domain.NotificationImage{{ImageKey: "img_demo", Name: "demo.png", Alt: "示意图"}}, &scheduledAt)
	if err != nil {
		t.Fatal(err)
	}
	if draft.RecipientCount != 1 || draft.RecipientType != "user" || len(draft.Images) != 1 {
		t.Fatalf("unexpected targeted draft: %#v", draft)
	}
	approved, err := notifications.Approve(ctx, admin, draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	if approved.Status != "scheduled" {
		t.Fatalf("expected scheduled status, got %s", approved.Status)
	}
}

func TestNotificationRejectsPastSchedule(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory(domain.AgentConfig{GenerationModel: "mock"})
	admin, _ := repo.GetUser(ctx, store.DemoAdminID)
	notifications := NewNotification(repo, feishu.New("", "", ""), model.Mock{})
	past := time.Now().Add(-time.Hour)
	if _, err := notifications.CreateTargeted(ctx, admin, "过期通知", "正文", "user", []string{store.DemoEmployeeID}, nil, &past); err == nil {
		t.Fatal("expected a past schedule to be rejected")
	}
}
