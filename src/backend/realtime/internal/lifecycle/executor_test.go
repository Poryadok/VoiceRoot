package lifecycle

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"sync"
	"testing"
)

type memoryAdmissions struct {
	mu        sync.Mutex
	pending   int
	settleErr error
}

func (m *memoryAdmissions) Begin(context.Context, uuid.UUID, uuid.UUID) (uuid.UUID, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pending++
	return uuid.New(), nil
}
func (m *memoryAdmissions) Settle(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.settleErr != nil {
		return m.settleErr
	}
	m.pending--
	return nil
}

func TestScopeCannotEnqueueAfterCompletion(t *testing.T) {
	db := &memoryAdmissions{}
	executor := NewExecutor(db, uuid.New())
	var saved *Scope
	sent := 0
	if err := executor.Run(context.Background(), uuid.New(), func(s *Scope) {
		saved = s
		if err := s.Enqueue(func() { sent++ }); err != nil {
			t.Fatal(err)
		}
	}); err != nil {
		t.Fatal(err)
	}
	if err := saved.Enqueue(func() { sent++ }); !errors.Is(err, ErrDenied) {
		t.Fatalf("late enqueue accepted: %v", err)
	}
	if sent != 1 || db.pending != 0 {
		t.Fatalf("sent=%d pending=%d", sent, db.pending)
	}
}

func TestClosingWaitsForAllSynchronousSendsAndSettlementFailureStaysPending(t *testing.T) {
	db := &memoryAdmissions{settleErr: errors.New("database unavailable")}
	executor := NewExecutor(db, uuid.New())
	entered, release, finished := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		finished <- executor.Run(context.Background(), uuid.New(), func(s *Scope) { _ = s.Enqueue(func() { close(entered); <-release }) })
	}()
	<-entered
	if executor.TryClose() {
		t.Fatal("closed while sink may still execute")
	}
	close(release)
	if err := <-finished; err == nil {
		t.Fatal("failed settlement reported success")
	}
	if executor.TryClose() {
		t.Fatal("unknown durable settlement attested closed")
	}
	db.mu.Lock()
	db.settleErr = nil
	db.mu.Unlock()
	if err := executor.RetrySettlements(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !executor.TryClose() {
		t.Fatal("true drain did not close")
	}
	if err := executor.Run(context.Background(), uuid.New(), func(*Scope) { t.Fatal("closed executor invoked sink") }); !errors.Is(err, ErrDenied) {
		t.Fatal(err)
	}
}
