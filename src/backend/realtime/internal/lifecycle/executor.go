package lifecycle

import (
	"context"
	"github.com/google/uuid"
	"sync"
	"time"
)

type AdmissionStore interface {
	Begin(context.Context, uuid.UUID, uuid.UUID) (uuid.UUID, error)
	Settle(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error
}

// Executor counts execution scopes before admission begins. Its irreversible
// closing flag rejects new scopes; a failed Begin/commit is retained as unknown
// until process termination proof, never treated as drained by a timeout.
type Executor struct {
	mu          sync.Mutex
	store       AdmissionStore
	incarnation uuid.UUID
	closing     bool
	active      int
	unknown     bool
	pending     map[uuid.UUID]uuid.UUID
}

func NewExecutor(store AdmissionStore, incarnation uuid.UUID) *Executor {
	return &Executor{store: store, incarnation: incarnation, pending: make(map[uuid.UUID]uuid.UUID)}
}

// Scope owns the only synchronous sink entry for one admitted finite fanout.
// A captured pointer cannot enqueue after Run returns, including child goroutines.
type Scope struct {
	mu     sync.Mutex
	closed bool
}

func (s *Scope) Enqueue(send func()) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrDenied
	}
	send()
	return nil
}

func (e *Executor) Run(ctx context.Context, chat uuid.UUID, fanout func(*Scope)) error {
	e.mu.Lock()
	if e.closing {
		e.mu.Unlock()
		return ErrDenied
	}
	e.active++
	e.mu.Unlock()
	defer func() { e.mu.Lock(); e.active--; e.mu.Unlock() }()
	ticket, err := e.store.Begin(ctx, e.incarnation, chat)
	if err != nil {
		// A known denial has no commit ambiguity. Other failures may have committed
		// durable work and cannot permit a cooperative no-future-work attestation.
		if err != ErrDenied {
			e.mu.Lock()
			e.unknown = true
			e.mu.Unlock()
		}
		return err
	}
	scope := &Scope{}
	defer func() { scope.mu.Lock(); scope.closed = true; scope.mu.Unlock() }()
	fanout(scope)
	scope.mu.Lock()
	scope.closed = true
	scope.mu.Unlock()
	// Cancellation of the source event cannot cancel completion before attempting
	// it. The short bounded attempt does not expire durable pending evidence.
	completion, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err = e.store.Settle(completion, e.incarnation, chat, ticket); err != nil {
		e.mu.Lock()
		e.pending[ticket] = chat
		e.mu.Unlock()
		return err
	}
	return nil
}

// TryClose is local quiescence only, not a durable process closure receipt.
// Once called it prevents new scopes even when work still needs to drain.
func (e *Executor) TryClose() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.closing = true
	return e.active == 0 && !e.unknown && len(e.pending) == 0
}

// RetrySettlements retries only terminal metadata; it never replays a payload.
func (e *Executor) RetrySettlements(ctx context.Context) error {
	e.mu.Lock()
	saved := make(map[uuid.UUID]uuid.UUID, len(e.pending))
	for ticket, chat := range e.pending {
		saved[ticket] = chat
	}
	e.mu.Unlock()
	for ticket, chat := range saved {
		if err := e.store.Settle(ctx, e.incarnation, chat, ticket); err != nil {
			return err
		}
		e.mu.Lock()
		delete(e.pending, ticket)
		e.mu.Unlock()
	}
	return nil
}
