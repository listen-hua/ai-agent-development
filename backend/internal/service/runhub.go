package service

import (
	"context"
	"sync"
	"time"

	"internal-ai-agent/backend/internal/domain"
)

const defaultRunRetention = 10 * time.Minute

type runState struct {
	ownerID     string
	cancel      context.CancelFunc
	events      []domain.RunEvent
	subscribers []chan domain.RunEvent
	done        bool
}

type RunHub struct {
	mu        sync.Mutex
	runs      map[string]*runState
	retention time.Duration
}

func NewRunHub() *RunHub { return newRunHub(defaultRunRetention) }

func newRunHub(retention time.Duration) *RunHub {
	return &RunHub{runs: map[string]*runState{}, retention: retention}
}

func (h *RunHub) Create(id, ownerID string, cancel context.CancelFunc) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.runs[id] = &runState{ownerID: ownerID, cancel: cancel}
}

func (h *RunHub) Publish(id string, event domain.RunEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	state, ok := h.runs[id]
	if !ok || state.done {
		return
	}
	state.events = append(state.events, event)
	for _, ch := range state.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
	if event.Type == "done" || event.Type == "error" {
		h.finishLocked(id, state)
	}
}

func (h *RunHub) Subscribe(id, ownerID string) ([]domain.RunEvent, <-chan domain.RunEvent, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	state, ok := h.runs[id]
	if !ok || state.ownerID != ownerID {
		return nil, nil, false
	}
	history := append([]domain.RunEvent(nil), state.events...)
	if state.done {
		return history, nil, true
	}
	ch := make(chan domain.RunEvent, 256)
	state.subscribers = append(state.subscribers, ch)
	return history, ch, true
}

func (h *RunHub) Cancel(id, ownerID string) bool {
	h.mu.Lock()
	state, ok := h.runs[id]
	if !ok || state.ownerID != ownerID {
		h.mu.Unlock()
		return false
	}
	if state.done {
		h.mu.Unlock()
		return true
	}
	cancel := state.cancel
	event := domain.RunEvent{
		Type:      "error",
		RunID:     id,
		Error:     "生成已停止",
		Metadata:  map[string]string{"reason": "cancelled"},
		CreatedAt: time.Now(),
	}
	state.events = append(state.events, event)
	for _, ch := range state.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
	h.finishLocked(id, state)
	h.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return true
}

func (h *RunHub) finishLocked(id string, state *runState) {
	state.done = true
	state.cancel = nil
	for _, ch := range state.subscribers {
		close(ch)
	}
	state.subscribers = nil
	time.AfterFunc(h.retention, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if h.runs[id] == state {
			delete(h.runs, id)
		}
	})
}
