package service

import (
	"internal-ai-agent/backend/internal/domain"
	"sync"
)

type runState struct {
	events      []domain.RunEvent
	subscribers []chan domain.RunEvent
	done        bool
}
type RunHub struct {
	mu   sync.Mutex
	runs map[string]*runState
}

func NewRunHub() *RunHub           { return &RunHub{runs: map[string]*runState{}} }
func (h *RunHub) Create(id string) { h.mu.Lock(); defer h.mu.Unlock(); h.runs[id] = &runState{} }
func (h *RunHub) Publish(id string, event domain.RunEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	state, ok := h.runs[id]
	if !ok {
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
		state.done = true
		for _, ch := range state.subscribers {
			close(ch)
		}
		state.subscribers = nil
	}
}
func (h *RunHub) Subscribe(id string) ([]domain.RunEvent, <-chan domain.RunEvent, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	state, ok := h.runs[id]
	if !ok {
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
