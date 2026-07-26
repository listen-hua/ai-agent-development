package service

import "sync"

// conversationTurnTicket is reserved synchronously when a request is accepted.
// This makes execution order match request arrival order even when the goroutines
// that process the requests are scheduled in a different order.
type conversationTurnTicket struct {
	previous <-chan struct{}
	current  chan struct{}
	key      string
	owner    *conversationLocks
	once     sync.Once
}

func (t *conversationTurnTicket) Wait() {
	if t.previous != nil {
		<-t.previous
	}
}

func (t *conversationTurnTicket) Done() {
	t.once.Do(func() {
		close(t.current)
		t.owner.release(t.key, t.current)
	})
}

type conversationLockEntry struct {
	tail chan struct{}
	refs int
}

type conversationLocks struct {
	mu    sync.Mutex
	items map[string]*conversationLockEntry
}

func newConversationLocks() *conversationLocks {
	return &conversationLocks{items: map[string]*conversationLockEntry{}}
}

func (l *conversationLocks) Reserve(key string) *conversationTurnTicket {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := l.items[key]
	if entry == nil {
		entry = &conversationLockEntry{}
		l.items[key] = entry
	}
	current := make(chan struct{})
	ticket := &conversationTurnTicket{previous: entry.tail, current: current, key: key, owner: l}
	entry.tail = current
	entry.refs++
	return ticket
}

func (l *conversationLocks) release(key string, current chan struct{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := l.items[key]
	if entry == nil {
		return
	}
	entry.refs--
	if entry.refs == 0 && entry.tail == current {
		delete(l.items, key)
	}
}
