package service

import (
	"context"
	"testing"
	"time"

	"internal-ai-agent/backend/internal/domain"
)

func TestRunHubRestrictsSubscriptionsToOwner(t *testing.T) {
	hub := newRunHub(time.Minute)
	hub.Create("run-1", "owner-1", func() {})
	if _, _, found := hub.Subscribe("run-1", "owner-2"); found {
		t.Fatal("a different user must not be able to subscribe")
	}
	if _, _, found := hub.Subscribe("run-1", "owner-1"); !found {
		t.Fatal("the owner should be able to subscribe")
	}
}

func TestRunHubCancelCancelsContextAndExpiresState(t *testing.T) {
	hub := newRunHub(10 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	hub.Create("run-1", "owner-1", cancel)
	if !hub.Cancel("run-1", "owner-1") {
		t.Fatal("expected cancel to succeed")
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("run context was not cancelled")
	}
	history, events, found := hub.Subscribe("run-1", "owner-1")
	if !found || events != nil || len(history) != 1 || history[0].Type != "error" {
		t.Fatalf("unexpected cancelled run state: %#v, %v, %v", history, events, found)
	}
	time.Sleep(30 * time.Millisecond)
	if _, _, found = hub.Subscribe("run-1", "owner-1"); found {
		t.Fatal("completed run state should expire")
	}
}

func TestRunHubIgnoresEventsAfterCompletion(t *testing.T) {
	hub := newRunHub(time.Minute)
	hub.Create("run-1", "owner-1", func() {})
	hub.Publish("run-1", domain.RunEvent{Type: "done", RunID: "run-1"})
	hub.Publish("run-1", domain.RunEvent{Type: "delta", RunID: "run-1", Delta: "late"})
	history, _, found := hub.Subscribe("run-1", "owner-1")
	if !found || len(history) != 1 {
		t.Fatalf("late event must be ignored: %#v", history)
	}
}
