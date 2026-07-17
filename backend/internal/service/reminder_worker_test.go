package service

import (
	"context"
	"testing"
	"time"

	"internal-ai-agent/backend/internal/domain"
)

type reminderSenderStub struct{ sent int }

func (s *reminderSenderStub) Configured() bool { return true }
func (s *reminderSenderStub) SendReminder(context.Context, string, string, string, string, string) (string, error) {
	s.sent++
	return "om_test", nil
}
func (s *reminderSenderStub) SendReminderConfirmation(context.Context, string, string, string, string, string) (string, error) {
	return "om_confirm", nil
}
func (s *reminderSenderStub) SendText(context.Context, string, string, string, string) (string, error) {
	return "om_text", nil
}

func TestReminderDispatcherSendsOccurrenceOnce(t *testing.T) {
	location, _ := time.LoadLocation("Asia/Shanghai")
	createdAt := time.Date(2026, 7, 17, 9, 0, 0, 0, location)
	service, repo, user := newReminderTestService(t, createdAt)
	onceAt := time.Date(2026, 7, 17, 10, 0, 0, 0, location)
	action, err := service.CreateAction(context.Background(), user, "create", "", "写周报", &domain.ReminderSchedule{Type: domain.ReminderOnce, Timezone: "Asia/Shanghai", OnceAt: &onceAt}, "h5", "")
	if err != nil {
		t.Fatal(err)
	}
	_, reminder, err := service.Confirm(context.Background(), user, action.ID)
	if err != nil {
		t.Fatal(err)
	}
	sender := &reminderSenderStub{}
	dispatcher := NewReminderDispatcher(repo, service, sender, "", 30*time.Minute)
	dispatcher.now = func() time.Time { return onceAt.Add(5 * time.Minute) }
	if err = dispatcher.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = dispatcher.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if sender.sent != 1 {
		t.Fatalf("expected one delivery, got %d", sender.sent)
	}
	updated, err := repo.GetReminder(context.Background(), reminder.ID)
	if err != nil || updated.Status != "completed" {
		t.Fatalf("unexpected reminder state: %+v, %v", updated, err)
	}
	deliveries, err := repo.ListReminderDeliveries(context.Background(), reminder.ID, 10)
	if err != nil || len(deliveries) != 1 || deliveries[0].Status != "sent" {
		t.Fatalf("unexpected deliveries: %+v, %v", deliveries, err)
	}
}

func TestReminderDispatcherSkipsBeyondGracePeriod(t *testing.T) {
	location, _ := time.LoadLocation("Asia/Shanghai")
	createdAt := time.Date(2026, 7, 17, 9, 0, 0, 0, location)
	service, repo, user := newReminderTestService(t, createdAt)
	onceAt := time.Date(2026, 7, 17, 10, 0, 0, 0, location)
	action, _ := service.CreateAction(context.Background(), user, "create", "", "写周报", &domain.ReminderSchedule{Type: domain.ReminderOnce, Timezone: "Asia/Shanghai", OnceAt: &onceAt}, "h5", "")
	_, reminder, _ := service.Confirm(context.Background(), user, action.ID)
	sender := &reminderSenderStub{}
	dispatcher := NewReminderDispatcher(repo, service, sender, "", 30*time.Minute)
	dispatcher.now = func() time.Time { return onceAt.Add(31 * time.Minute) }
	if err := dispatcher.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if sender.sent != 0 {
		t.Fatalf("overdue reminder should not be sent")
	}
	deliveries, _ := repo.ListReminderDeliveries(context.Background(), reminder.ID, 10)
	if len(deliveries) != 1 || deliveries[0].Status != "missed" {
		t.Fatalf("unexpected deliveries: %+v", deliveries)
	}
}
