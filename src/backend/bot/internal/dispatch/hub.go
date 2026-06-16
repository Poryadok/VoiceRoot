package dispatch

import (
	"sync"
	"time"

	"voice/backend/bot/internal/store"
)

const DefaultTimeout = 3 * time.Second

// Hub tracks in-flight slash interactions awaiting bot responses.
type Hub struct {
	mu       sync.Mutex
	pending  map[string]chan store.InteractionReply
	deferred map[string]struct{}
}

func NewHub() *Hub {
	return &Hub{
		pending:  make(map[string]chan store.InteractionReply),
		deferred: make(map[string]struct{}),
	}
}

func (h *Hub) Register(token string) chan store.InteractionReply {
	ch := make(chan store.InteractionReply, 1)
	h.mu.Lock()
	h.pending[token] = ch
	h.mu.Unlock()
	return ch
}

func (h *Hub) Complete(token string, reply store.InteractionReply) bool {
	h.mu.Lock()
	ch, ok := h.pending[token]
	if ok {
		if reply.Deferred {
			h.deferred[token] = struct{}{}
		} else {
			delete(h.pending, token)
			delete(h.deferred, token)
		}
	}
	h.mu.Unlock()
	if !ok {
		return false
	}
	ch <- reply
	return true
}

func (h *Hub) Cancel(token string) {
	h.mu.Lock()
	delete(h.pending, token)
	delete(h.deferred, token)
	h.mu.Unlock()
}

// IsPending reports whether a token is still registered (including deferred follow-up).
func (h *Hub) IsPending(token string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	_, ok := h.pending[token]
	return ok
}

// IsDeferred reports whether the interaction was deferred and awaits async completion.
func (h *Hub) IsDeferred(token string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	_, ok := h.deferred[token]
	return ok
}

// FinishDeferred removes a deferred token after async follow-up completes.
func (h *Hub) FinishDeferred(token string) {
	h.mu.Lock()
	delete(h.pending, token)
	delete(h.deferred, token)
	h.mu.Unlock()
}

// RegisterDeferred marks a token as deferred without a live waiter (restart recovery).
func (h *Hub) RegisterDeferred(token string) {
	h.mu.Lock()
	h.pending[token] = make(chan store.InteractionReply, 1)
	h.deferred[token] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) Wait(ch chan store.InteractionReply, timeout time.Duration) (store.InteractionReply, bool) {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	select {
	case reply := <-ch:
		return reply, true
	case <-time.After(timeout):
		return store.InteractionReply{Err: ErrTimeout}, false
	}
}

// ErrTimeout is returned when a bot does not respond in time.
var ErrTimeout = &TimeoutError{}

type TimeoutError struct{}

func (e *TimeoutError) Error() string { return "bot interaction timeout" }
