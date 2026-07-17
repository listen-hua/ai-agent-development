package service

import (
	"context"
	"testing"
	"time"

	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/model"
	"internal-ai-agent/backend/internal/store"
)

func newReminderTestService(t *testing.T, now time.Time) (*Reminder, *store.Memory, domain.User) {
	t.Helper()
	repo := store.NewMemory(domain.AgentConfig{})
	user, err := repo.GetUser(context.Background(), store.DemoEmployeeID)
	if err != nil {
		t.Fatal(err)
	}
	service := NewReminder(repo, model.Mock{}, "qwen-flash", "Asia/Shanghai", 100)
	service.now = func() time.Time { return now }
	return service, repo, user
}

func TestDeterministicReminderParsesWorkdayTypo(t *testing.T) {
	location, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Date(2026, 7, 17, 10, 0, 0, 0, location)
	parsed, ok := deterministicReminder("在工作日提醒我早上11点50点外卖", now, location)
	if !ok || parsed.Intent != "create" {
		t.Fatalf("unexpected parse result: %+v", parsed)
	}
	if parsed.Content != "点外卖" || parsed.Schedule.Type != domain.ReminderWorkday || parsed.Schedule.LocalTime != "11:50" {
		t.Fatalf("unexpected reminder: %+v", parsed)
	}
}

func TestDeterministicReminderRejectsPastExplicitToday(t *testing.T) {
	location, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Date(2026, 7, 17, 16, 0, 0, 0, location)
	parsed, ok := deterministicReminder("今天下午3点提醒我要写周报", now, location)
	if !ok || parsed.Clarification == "" {
		t.Fatalf("expected clarification: %+v", parsed)
	}
}

func TestReminderConfirmationIsIdempotent(t *testing.T) {
	location, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Date(2026, 7, 17, 10, 0, 0, 0, location)
	service, _, user := newReminderTestService(t, now)
	onceAt := now.Add(2 * time.Hour)
	schedule := domain.ReminderSchedule{Type: domain.ReminderOnce, Timezone: "Asia/Shanghai", OnceAt: &onceAt}
	action, err := service.CreateAction(context.Background(), user, "create", "", "写周报", &schedule, "h5", "")
	if err != nil {
		t.Fatal(err)
	}
	_, first, err := service.Confirm(context.Background(), user, action.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, second, err := service.Confirm(context.Background(), user, action.ID)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatalf("duplicate confirmation created different reminders: %s %s", first.ID, second.ID)
	}
}

func TestCompanyWorkdayOverrideAffectsNextOccurrence(t *testing.T) {
	location, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, location) // Friday
	service, repo, user := newReminderTestService(t, now)
	if err := repo.UpsertWorkdayOverride(context.Background(), domain.WorkdayOverride{Date: "2026-07-18", IsWorkday: true, UpdatedBy: user.ID, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	next, err := service.NextOccurrence(context.Background(), domain.ReminderSchedule{Type: domain.ReminderWorkday, Timezone: "Asia/Shanghai", LocalTime: "11:50"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if next.Format("2006-01-02 15:04") != "2026-07-18 11:50" {
		t.Fatalf("unexpected next occurrence: %s", next)
	}
}
