// Package spacegatetest supplies an explicit healthy gate contract for unit
// fixtures using memory CallStore. Redis loss/CAS proof uses the real store.
package spacegatetest

import (
	"context"
	"github.com/google/uuid"
	"sync"
	"voice/backend/voice/internal/store"
)

type Gates struct {
	mu      sync.Mutex
	entries map[string]store.SpaceProjectionTicket
	live    map[string]bool
}

func New() *Gates {
	return &Gates{entries: map[string]store.SpaceProjectionTicket{}, live: map[string]bool{}}
}
func (g *Gates) EnsureSpaceLifecycle(ctx context.Context, space string, load store.SpaceLifecycleLoader) error {
	g.mu.Lock()
	_, ok := g.entries[space]
	g.mu.Unlock()
	if ok {
		return nil
	}
	generation, live, err := load(ctx, space)
	if err != nil {
		return err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, ok := g.entries[space]; !ok {
		g.entries[space] = store.SpaceProjectionTicket{Incarnation: uuid.NewString(), Generation: generation, SpaceID: space}
		g.live[space] = live
	}
	return nil
}
func (g *Gates) ReadReadySpaceGate(_ context.Context, space string) (store.SpaceProjectionTicket, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	t, ok := g.entries[space]
	if !ok || !g.live[space] {
		return store.SpaceProjectionTicket{}, store.ErrSpaceMediaStaleGrant
	}
	return t, nil
}

func (g *Gates) EnsureAdmissionDispatch(ctx context.Context, operation string, load store.SpaceAdmissionGateLoader) (store.SpaceProjectionTicket, error) {
	epoch, live, err := load(ctx, operation)
	if err != nil {
		return store.SpaceProjectionTicket{}, err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	key := "admission:" + operation
	t, ok := g.entries[key]
	if !ok || t.AdmissionOwnerEpoch != epoch || t.AdmissionLive != live {
		t = store.SpaceProjectionTicket{OperationID: operation, AdmissionIncarnation: uuid.NewString(), AdmissionOwnerEpoch: epoch, AdmissionLive: live}
		g.entries[key] = t
	}
	return t, nil
}
