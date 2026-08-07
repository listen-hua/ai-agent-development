package service

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/store"
)

func TestMassageQueueEnrollmentAndParallelCalling(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory(domain.AgentConfig{})
	admin, _ := repo.GetUser(ctx, store.DemoAdminID)
	base := time.Date(2026, 8, 10, 9, 0, 0, 0, time.Local)
	service := NewMassage(repo, repo, nil, "", 3*time.Minute, 365*24*time.Hour)
	service.now = func() time.Time { return base }
	users := []domain.User{}
	for i := 0; i < 6; i++ {
		user, err := repo.UpsertUser(ctx, domain.User{FeishuOpenID: fmt.Sprintf("ou_massage_%d", i), Name: fmt.Sprintf("员工%d", i), Status: "active"})
		if err != nil {
			t.Fatal(err)
		}
		users = append(users, user)
	}
	cycle, err := service.CreateCycle(ctx, admin, domain.MassageCycle{ServiceMonth: "2026-08", Title: "八月按摩", SignupNoticeAt: base.Add(time.Hour), SignupDeadline: base.Add(24 * time.Hour), Audience: domain.MassageAudience{Scope: "all"}, Sessions: []domain.MassageSession{{StartsAt: base.Add(48 * time.Hour), Quota: 3, ConcurrentSlots: 2}, {StartsAt: base.Add(7 * 24 * time.Hour), Quota: 3, ConcurrentSlots: 2}}})
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return base.Add(2 * time.Hour) }
	if _, err = service.Publish(ctx, admin, cycle.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.ClaimMassageDeliveries(ctx, base.Add(2*time.Hour), time.Minute, 100); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for _, user := range users {
		wg.Add(1)
		go func(u domain.User) {
			defer wg.Done()
			if _, joinErr := service.Respond(ctx, u, cycle.ID, "enroll"); joinErr != nil {
				t.Errorf("enroll: %v", joinErr)
			}
		}(user)
	}
	wg.Wait()
	me, err := repo.ListMassageMe(ctx, users[0].ID)
	if err != nil || len(me) != 1 {
		t.Fatalf("me=%v err=%v", me, err)
	}
	stats, err := repo.MassageStatistics(ctx, cycle.ID)
	if err != nil {
		t.Fatal(err)
	}
	numbers := []int{}
	for _, e := range stats.Enrollments {
		numbers = append(numbers, e.QueueNumber)
	}
	sort.Ints(numbers)
	for i, n := range numbers {
		if n != i+1 {
			t.Fatalf("queue numbers=%v", numbers)
		}
	}
	updated, _ := repo.GetMassageCycle(ctx, cycle.ID)
	session := updated.Sessions[0]
	if _, err = service.SessionAction(ctx, admin, session.ID, "start", ""); err != nil {
		t.Fatal(err)
	}
	calls, err := repo.FillMassageSession(ctx, session.ID, base.Add(48*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 {
		t.Fatalf("expected two parallel calls, got %d", len(calls))
	}
	now := base.Add(48 * time.Hour)
	due := now.Add(3 * time.Minute)
	if err = repo.MarkMassageCallSent(ctx, calls[0].ID, "om_1", now, due); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.RespondMassageCall(ctx, calls[0].ID, calls[0].UserID, "accept", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err = service.AdminCallAction(ctx, admin, calls[0].ID, "complete"); err != nil {
		t.Fatal(err)
	}
	more, err := repo.FillMassageSession(ctx, session.ID, now.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(more) != 1 {
		t.Fatalf("completion should free one slot, got %d", len(more))
	}
}

func TestMassageValidationRequiresTwoOrderedSessions(t *testing.T) {
	err := domain.ValidateMassageCycle(domain.MassageCycle{ServiceMonth: "2026-08", Title: "按摩", SignupNoticeAt: time.Now(), SignupDeadline: time.Now().Add(time.Hour), Sessions: []domain.MassageSession{{Sequence: 1, StartsAt: time.Now().Add(2 * time.Hour), Quota: 1, ConcurrentSlots: 1}}})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestMassagePublishedCycleCanUpdateAllParameters(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory(domain.AgentConfig{})
	admin, _ := repo.GetUser(ctx, store.DemoAdminID)
	now := time.Date(2026, 8, 1, 9, 0, 0, 0, time.Local)
	first, _ := repo.UpsertUser(ctx, domain.User{FeishuOpenID: "ou_first", Name: "员工甲", Status: "active"})
	second, _ := repo.UpsertUser(ctx, domain.User{FeishuOpenID: "ou_second", Name: "员工乙", Status: "active"})
	svc := NewMassage(repo, repo, nil, "", 3*time.Minute, 365*24*time.Hour)
	svc.now = func() time.Time { return now }
	cycle, err := svc.CreateCycle(ctx, admin, domain.MassageCycle{
		ServiceMonth: "2026-08", Title: "原批次", SignupNoticeAt: now.Add(time.Hour), SignupDeadline: now.Add(24 * time.Hour),
		Audience: domain.MassageAudience{Scope: "restricted", UserIDs: []string{first.ID}},
		Sessions: []domain.MassageSession{{StartsAt: now.Add(48 * time.Hour), Quota: 1, ConcurrentSlots: 1}, {StartsAt: now.Add(72 * time.Hour), Quota: 1, ConcurrentSlots: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cycle, err = svc.Publish(ctx, admin, cycle.ID); err != nil {
		t.Fatal(err)
	}
	cycle.ServiceMonth = "2026-09"
	cycle.Title = "更新后的批次"
	cycle.SignupNoticeAt = now.Add(2 * time.Hour)
	cycle.SignupDeadline = now.Add(36 * time.Hour)
	cycle.Audience = domain.MassageAudience{Scope: "restricted", UserIDs: []string{second.ID}}
	cycle.Sessions[0].StartsAt = now.Add(96 * time.Hour)
	cycle.Sessions[0].Quota = 3
	cycle.Sessions[0].ConcurrentSlots = 2
	cycle.Sessions[1].StartsAt = now.Add(120 * time.Hour)
	cycle.Sessions[1].Quota = 4
	cycle.Sessions[1].ConcurrentSlots = 3
	updated, err := svc.UpdateCycle(ctx, admin, cycle)
	if err != nil {
		t.Fatal(err)
	}
	if updated.ServiceMonth != "2026-09" || updated.Title != "更新后的批次" || updated.Sessions[0].Quota != 3 || updated.Sessions[0].ConcurrentSlots != 2 {
		t.Fatalf("all fields were not updated: %+v", updated)
	}
	firstCycles, _ := repo.ListMassageMe(ctx, first.ID)
	secondCycles, _ := repo.ListMassageMe(ctx, second.ID)
	if len(firstCycles) != 0 || len(secondCycles) != 1 {
		t.Fatalf("audience was not reconciled: first=%d second=%d", len(firstCycles), len(secondCycles))
	}
	if err = svc.DeleteCycle(ctx, admin, cycle.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.GetMassageCycle(ctx, cycle.ID); err != store.ErrNotFound {
		t.Fatalf("deleted cycle should not exist, got %v", err)
	}
}
