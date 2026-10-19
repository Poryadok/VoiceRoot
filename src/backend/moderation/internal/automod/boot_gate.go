package automod

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"sync"
)

var ErrBootClosed = errors.New("moderation current-boot privacy reconciliation is incomplete")
var ErrBootReceipt = errors.New("invalid or conflicting current-boot phase receipt")

type BootCheck func(context.Context) error
type BootPhase int

const (
	BootLocalIssuer BootPhase = iota
	BootAuthority
	BootTerminalImported
	BootPrivateRepair
	BootFiniteCensus
	BootEffectsReady
)

// BootReceipt references a coordinator-verified local phase receipt. The gate
// enforces ordering/identity, not network authentication or receipt validation.
type BootReceipt struct {
	BootID     uuid.UUID
	Generation uint64
	Phase      BootPhase
	ProofID    uuid.UUID
}

type BootGate struct {
	mu         sync.RWMutex
	bootID     uuid.UUID
	generation uint64
	phase      BootPhase
	proofs     [6]uuid.UUID
	closed     bool
}

func NewBootGate() *BootGate { return &BootGate{bootID: uuid.New(), phase: -1} }
func (g *BootGate) Identity() (uuid.UUID, uint64) {
	if g == nil {
		return uuid.Nil, 0
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.bootID, g.generation
}
func (g *BootGate) Advance(receipt BootReceipt) error {
	if g == nil {
		return ErrBootClosed
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed || g.bootID == uuid.Nil {
		return ErrBootClosed
	}
	if receipt.BootID != g.bootID || receipt.Generation != g.generation || receipt.Phase < BootLocalIssuer || receipt.Phase > BootEffectsReady || receipt.ProofID == uuid.Nil {
		return ErrBootReceipt
	}
	if receipt.Phase <= g.phase {
		if g.proofs[receipt.Phase] == receipt.ProofID {
			return nil
		}
		return ErrBootReceipt
	}
	if receipt.Phase != g.phase+1 {
		return ErrBootReceipt
	}
	g.proofs[receipt.Phase] = receipt.ProofID
	g.phase = receipt.Phase
	return nil
}

func (g *BootGate) AcquireEffects(ctx context.Context) (BootCheck, error) {
	return g.acquire(ctx, BootEffectsReady)
}
func (g *BootGate) AcquirePrivateRepair(ctx context.Context) (BootCheck, error) {
	return g.acquire(ctx, BootTerminalImported)
}
func (g *BootGate) acquire(ctx context.Context, minimum BootPhase) (BootCheck, error) {
	if g == nil || ctx == nil {
		return nil, ErrBootClosed
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.closed || g.bootID == uuid.Nil || g.phase < minimum {
		return nil, ErrBootClosed
	}
	id, generation := g.bootID, g.generation
	return func(ctx context.Context) error {
		if ctx == nil {
			return ErrBootClosed
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		g.mu.RLock()
		defer g.mu.RUnlock()
		if g.closed || g.bootID != id || g.generation != generation || g.phase < minimum {
			return ErrBootClosed
		}
		return nil
	}, nil
}

// Close permanently invalidates this boot's captured checks. Recovery constructs
// a new gate/boot identity and must execute every phase again; no ready reload.
func (g *BootGate) Close() {
	if g == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.closed {
		g.closed = true
		g.generation++
	}
}
