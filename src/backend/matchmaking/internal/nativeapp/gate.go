// Package nativeapp is the fixed native Matchmaking executable composition.
package nativeapp

import (
	"bytes"
	"context"
	"errors"
	"github.com/google/uuid"
	"sort"
	"sync"
	w "voice/backend/pkg/mmadmissionwire"
)

var errUnavailable = errors.New("matchmaking native runtime unavailable")

type admissionObserver interface {
	observeFresh(context.Context, uuid.UUID, [32]byte) (w.Binding, w.AdmissionStampV1, error)
}

type readbackObserver struct{ client w.Client }

func (o readbackObserver) observeFresh(ctx context.Context, id uuid.UUID, hash [32]byte) (w.Binding, w.AdmissionStampV1, error) {
	observation, err := o.client.ObserveFresh(ctx, w.ID(id), w.Hash(hash))
	return observation.Binding(), observation.Stamp(), err
}

// A registration spans fresh external observation, the local transition,
// provider dispatch, classification and durable receipt persistence. It is
// deliberately independent of SQL locks. Closing is irreversible even when the
// caller's drain deadline expires; the controller must retain pending work.
type forwardGate struct {
	mu       sync.Mutex
	closed   bool
	binding  w.Binding
	stamp    w.AdmissionStampV1
	observer admissionObserver
	active   map[uuid.UUID]struct{}
	drained  chan struct{}
}

func newForwardGate(binding w.Binding, stamp w.AdmissionStampV1, observer admissionObserver) *forwardGate {
	return &forwardGate{binding: binding, stamp: stamp, observer: observer, active: make(map[uuid.UUID]struct{}), drained: make(chan struct{})}
}

// EnterForward implements the provider coordinator's lifetime seam. The work
// identifier is a process registration, never a provider business operation.
func (g *forwardGate) EnterForward(ctx context.Context, workID uuid.UUID, requestHash [32]byte) (func(), error) {
	return g.enter(ctx, workID, requestHash)
}

// beginClose is non-blocking so a critical inbox conflict can stop new forward
// work without waiting for its own consumer callback/ACK to finish.
func (g *forwardGate) beginClose() {
	if g == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.closed = true
	if len(g.active) == 0 {
		select {
		case <-g.drained:
		default:
			close(g.drained)
		}
	}
}

func (g *forwardGate) enter(ctx context.Context, id uuid.UUID, hash [32]byte) (func(), error) {
	if g == nil || ctx == nil || ctx.Err() != nil || id == uuid.Nil || hash == [32]byte{} {
		return nil, errUnavailable
	}
	g.mu.Lock()
	_, duplicate := g.active[id]
	if g.closed || duplicate || g.observer == nil || g.binding.Validate() != nil || g.stamp.Validate() != nil || len(g.active) >= 1024 {
		g.mu.Unlock()
		return nil, errUnavailable
	}
	g.active[id] = struct{}{}
	g.mu.Unlock()
	var once sync.Once
	release := func() {
		once.Do(func() {
			g.mu.Lock()
			defer g.mu.Unlock()
			delete(g.active, id)
			if g.closed && len(g.active) == 0 {
				select {
				case <-g.drained:
				default:
					close(g.drained)
				}
			}
		})
	}
	actual, stamp, err := g.observer.observeFresh(ctx, id, hash)
	if err != nil || ctx.Err() != nil || actual != g.binding || stamp != g.stamp {
		release()
		return nil, errUnavailable
	}
	return release, nil
}

func (g *forwardGate) closeAndDrain(ctx context.Context) error {
	if g == nil || ctx == nil {
		return errUnavailable
	}
	g.beginClose()
	drained := g.drained
	select {
	case <-drained:
		return nil
	case <-ctx.Done():
		return errUnavailable
	}
}

func (g *forwardGate) inFlight() []w.ID {
	if g == nil {
		return nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make([]w.ID, 0, len(g.active))
	for id := range g.active {
		out = append(out, w.ID(id))
	}
	sort.Slice(out, func(i, j int) bool { return bytes.Compare(out[i][:], out[j][:]) < 0 })
	return out
}
