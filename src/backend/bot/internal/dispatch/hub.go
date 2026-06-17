package dispatch

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"voice/backend/bot/internal/store"
)

// AutocompleteChoice is one autocomplete suggestion completed by a polling bot.
type AutocompleteChoice struct {
	Name  string
	Value string
}

const DefaultTimeout = 3 * time.Second

// Hub tracks in-flight slash interactions awaiting bot responses.
type Hub struct {
	mu       sync.Mutex
	pending  map[string]chan store.InteractionReply
	deferred map[string]struct{}

	acCounter           uint64
	autocompletePending map[string]string
	autocompleteResults map[string][]AutocompleteChoice
}

func NewHub() *Hub {
	return &Hub{
		pending:             make(map[string]chan store.InteractionReply),
		deferred:            make(map[string]struct{}),
		autocompletePending: make(map[string]string),
		autocompleteResults: make(map[string][]AutocompleteChoice),
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

func autocompleteCacheKey(botID, chatID, command, option, focused string) string {
	return botID + "|" + chatID + "|" + command + "|" + option + "|" + focused
}

// NextAutocompleteRequestID returns the next polling autocomplete request id.
func (h *Hub) NextAutocompleteRequestID() string {
	n := atomic.AddUint64(&h.acCounter, 1)
	return fmt.Sprintf("ac-req-%d", n)
}

// RegisterAutocomplete tracks a pending autocomplete request.
func (h *Hub) RegisterAutocomplete(requestID, cacheKey string) {
	h.mu.Lock()
	h.autocompletePending[requestID] = cacheKey
	h.mu.Unlock()
}

// CompleteAutocomplete stores choices for a polling autocomplete request.
func (h *Hub) CompleteAutocomplete(requestID string, choices []AutocompleteChoice) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	cacheKey, ok := h.autocompletePending[requestID]
	if !ok {
		return false
	}
	delete(h.autocompletePending, requestID)
	h.autocompleteResults[cacheKey] = choices
	return true
}

// GetAutocompleteChoices returns cached choices for a prior autocomplete request.
func (h *Hub) GetAutocompleteChoices(cacheKey string) ([]AutocompleteChoice, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	choices, ok := h.autocompleteResults[cacheKey]
	return choices, ok
}

// AutocompleteCacheKey builds the cache key for an autocomplete request.
func AutocompleteCacheKey(botID, chatID, command, option, focused string) string {
	return autocompleteCacheKey(botID, chatID, command, option, focused)
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
