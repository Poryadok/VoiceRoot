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
}

func NewHub() *Hub {
	return &Hub{pending: make(map[string]chan store.InteractionReply)}
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
		delete(h.pending, token)
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
