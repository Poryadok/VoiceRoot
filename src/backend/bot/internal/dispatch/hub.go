package dispatch

import (
	"fmt"
	"github.com/google/uuid"
	"strings"
	"sync"
	"time"

	"voice/backend/bot/internal/store"
	"voice/backend/pkg/runtimeconfig"
)

// AutocompleteChoice is one autocomplete suggestion completed by a polling bot.
type AutocompleteChoice struct {
	Name  string
	Value string
}

// DefaultTimeout is the webhook delivery wait used by the dispatch hub.
func DefaultTimeout() time.Duration {
	return runtimeconfig.DurationFromEnv("BOT_WEBHOOK_TIMEOUT", 3*time.Second)
}

// Hub tracks in-flight slash interactions awaiting bot responses.
type Hub struct {
	mu       sync.Mutex
	pending  map[string]chan store.InteractionReply
	deferred map[string]struct{}
	bindings map[string]store.Session
	floors   map[uuid.UUID]int64

	autocompletePending  map[string]string
	autocompleteResults  map[string][]AutocompleteChoice
	autocompleteBindings map[string]store.Session
	cacheBindings        map[string]store.Session
}

func NewHub() *Hub {
	return &Hub{
		pending:              make(map[string]chan store.InteractionReply),
		deferred:             make(map[string]struct{}),
		bindings:             make(map[string]store.Session),
		floors:               make(map[uuid.UUID]int64),
		autocompletePending:  make(map[string]string),
		autocompleteResults:  make(map[string][]AutocompleteChoice),
		autocompleteBindings: make(map[string]store.Session),
		cacheBindings:        make(map[string]store.Session),
	}
}

func (h *Hub) Register(token string, session store.Session) chan store.InteractionReply {
	ch := make(chan store.InteractionReply, 1)
	h.mu.Lock()
	if session.Epoch < 1 || session.BotID == uuid.Nil || session.Epoch < h.floors[session.BotID] {
		ch <- store.InteractionReply{Err: store.ErrSessionRevoked}
	} else {
		h.pending[token] = ch
		h.bindings[token] = session
	}
	h.mu.Unlock()
	return ch
}

func (h *Hub) Complete(token string, session store.Session, reply store.InteractionReply) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	ch, ok := h.pending[token]
	ok = ok && h.bindings[token] == session && session.Epoch >= h.floors[session.BotID]
	if !ok {
		return false
	}
	if _, alreadyNotified := h.deferred[token]; alreadyNotified {
		return false
	}
	// Nonblocking notification is serialized with the epoch floor. A full
	// channel does not consume or mutate the pending record.
	select {
	case ch <- reply:
	default:
		return false
	}
	if reply.Deferred {
		h.deferred[token] = struct{}{}
	} else {
		delete(h.pending, token)
		delete(h.bindings, token)
	}
	return true
}

func (h *Hub) Cancel(token string) {
	h.mu.Lock()
	delete(h.pending, token)
	delete(h.deferred, token)
	delete(h.bindings, token)
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
	delete(h.bindings, token)
	h.mu.Unlock()
}

// RegisterDeferred marks a token as deferred without a live waiter (restart recovery).
func (h *Hub) RegisterDeferred(token string, session store.Session) {
	h.mu.Lock()
	if session.Epoch < 1 || session.BotID == uuid.Nil || session.Epoch < h.floors[session.BotID] {
		h.mu.Unlock()
		return
	}
	if h.bindings[token] == session && h.pending[token] != nil {
		h.deferred[token] = struct{}{}
		h.mu.Unlock()
		return
	}
	h.pending[token] = make(chan store.InteractionReply, 1)
	h.deferred[token] = struct{}{}
	h.bindings[token] = session
	h.mu.Unlock()
}

func autocompleteCacheKey(botID, chatID, command, option, focused string) string {
	return botID + "|" + chatID + "|" + command + "|" + option + "|" + focused
}

// NextAutocompleteRequestID returns the next polling autocomplete request id.
func (h *Hub) NextAutocompleteRequestID() string { return uuid.NewString() }

// RegisterAutocomplete tracks a pending autocomplete request.
func (h *Hub) RegisterAutocomplete(requestID, cacheKey string, session store.Session) {
	h.mu.Lock()
	if session.Epoch > 0 && session.BotID != uuid.Nil && session.Epoch >= h.floors[session.BotID] {
		h.autocompletePending[requestID] = cacheKey
		h.autocompleteBindings[requestID] = session
	}
	h.mu.Unlock()
}

func (h *Hub) CancelAutocomplete(requestID string) {
	h.mu.Lock()
	delete(h.autocompletePending, requestID)
	delete(h.autocompleteBindings, requestID)
	h.mu.Unlock()
}

// CompleteAutocomplete stores choices only for the bot that owns the request.
func (h *Hub) CompleteAutocomplete(requestID, botID string, session store.Session, choices []AutocompleteChoice) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	cacheKey, ok := h.autocompletePending[requestID]
	if !ok || !strings.HasPrefix(cacheKey, botID+"|") || session.BotID.String() != botID || h.autocompleteBindings[requestID] != session || session.Epoch < h.floors[session.BotID] {
		return false
	}
	delete(h.autocompletePending, requestID)
	delete(h.autocompleteBindings, requestID)
	h.autocompleteResults[cacheKey] = append([]AutocompleteChoice(nil), choices...)
	h.cacheBindings[cacheKey] = session
	return true
}

// GetAutocompleteChoices returns cached choices for a prior autocomplete request.
func (h *Hub) GetAutocompleteChoices(cacheKey string, session store.Session) ([]AutocompleteChoice, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	choices, ok := h.autocompleteResults[cacheKey]
	return append([]AutocompleteChoice(nil), choices...), ok && h.cacheBindings[cacheKey] == session && session.Epoch >= h.floors[session.BotID]
}

// AutocompleteCacheKey builds the cache key for an autocomplete request.
func AutocompleteCacheKey(botID, chatID, command, option, focused string, epoch int64) string {
	return autocompleteCacheKey(botID, chatID, command, option, focused) + fmt.Sprintf("|%d", epoch)
}

// PurgeBot only removes older entries for this bot. The floor prevents a paused
// rehydrator from resurrecting them after cleanup. Notifications never block.
func (h *Hub) PurgeBot(bot uuid.UUID, epoch int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if epoch <= h.floors[bot] {
		return
	}
	h.floors[bot] = epoch
	for token, binding := range h.bindings {
		if binding.BotID != bot || binding.Epoch >= epoch {
			continue
		}
		if ch := h.pending[token]; ch != nil {
			select {
			case ch <- store.InteractionReply{Err: store.ErrSessionRevoked}:
			default:
			}
		}
		delete(h.pending, token)
		delete(h.deferred, token)
		delete(h.bindings, token)
	}
	for id, binding := range h.autocompleteBindings {
		if binding.BotID == bot && binding.Epoch < epoch {
			delete(h.autocompleteBindings, id)
			delete(h.autocompletePending, id)
		}
	}
	for key, binding := range h.cacheBindings {
		if binding.BotID == bot && binding.Epoch < epoch {
			delete(h.cacheBindings, key)
			delete(h.autocompleteResults, key)
		}
	}
}

func (h *Hub) Wait(ch chan store.InteractionReply, timeout time.Duration) (store.InteractionReply, bool) {
	if timeout <= 0 {
		timeout = DefaultTimeout()
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
