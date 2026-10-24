// Package storylfpadmission defines the shared Story/MM LFP admission boundary.
// It does not install a controller, authenticate a transport, or fence credentials.
package storylfpadmission

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrUnavailable = errors.New("LFP admission unavailable")
	ErrIneligible  = errors.New("LFP origin ineligible")
)

// Origin is immutable provenance, stored alongside each owning journal row.
// IDs are UUID bytes; Binding is the externally admitted Story DB binding hash.
type Origin struct {
	environment, installation, incarnation [16]byte
	binding                                [32]byte
}

// NewOrigin validates data only. It never authenticates or admits that data.
func NewOrigin(environment, installation, incarnation [16]byte, binding [32]byte) (Origin, error) {
	o := Origin{environment, installation, incarnation, binding}
	if !o.Valid() {
		return Origin{}, ErrUnavailable
	}
	return o, nil
}

func (o Origin) Valid() bool {
	return o.environment != [16]byte{} && o.installation != [16]byte{} && o.incarnation != [16]byte{} && o.binding != [32]byte{}
}
func (o Origin) Environment() [16]byte     { return o.environment }
func (o Origin) Installation() [16]byte    { return o.installation }
func (o Origin) Incarnation() [16]byte     { return o.incarnation }
func (o Origin) DatabaseBinding() [32]byte { return o.binding }
func (o Origin) String() string            { return "LFP origin" }
func (o Origin) GoString() string          { return "LFP origin" }

// Permit must remain held until the owning database transaction has completed,
// including commit. Release is idempotent and must release all underlying guards.
type Permit interface {
	Origin() Origin
	Release()
}

// Admission is implemented only by the installation's bound runtime factory.
// A JSON descriptor, environment variable or database marker is not this factory.
// Enter must fail closed on missing/partial authority and return current origin.
type Admission interface {
	Enter(context.Context) (Permit, error)
}

// AuthenticatedPeerAuthority belongs to the protected transport composition.
// It resolves the actual authenticated endpoint and credential against the
// controller-admitted trust manifest; it must not read a caller cohort claim.
type AuthenticatedPeerAuthority interface {
	ResolveAuthenticatedPeer(context.Context) (Origin, error)
}

// VerifiedPeer accompanies the exact v1 validation response. Zero is invalid.
// It must be retained rather than discarded when returning the response body.
type VerifiedPeer struct{ origin Origin }

func (p VerifiedPeer) Origin() Origin { return p.origin }
func (p VerifiedPeer) Valid() bool    { return p.origin.Valid() }

func ResolvePeer(ctx context.Context, expected Origin, authority AuthenticatedPeerAuthority) (VerifiedPeer, error) {
	if err := ctx.Err(); err != nil {
		return VerifiedPeer{}, err
	}
	if !expected.Valid() || authority == nil {
		return VerifiedPeer{}, ErrUnavailable
	}
	o, err := authority.ResolveAuthenticatedPeer(ctx)
	if err != nil || !o.Valid() {
		return VerifiedPeer{}, publicAuthorityError(err)
	}
	if err := ctx.Err(); err != nil {
		return VerifiedPeer{}, err
	}
	if o != expected {
		return VerifiedPeer{}, ErrIneligible
	}
	return VerifiedPeer{origin: o}, nil
}

// Barrier adds a process-local, permanently closing transaction drain to an
// already bound admission factory. It is not a distributed restore fence.
type Barrier struct {
	mu        sync.Mutex
	expected  Origin
	authority Admission
	closed    bool
	inflight  int
	drained   chan struct{}
}

func NewBarrier(expected Origin, authority Admission) *Barrier {
	return &Barrier{expected: expected, authority: authority, drained: make(chan struct{})}
}

func (b *Barrier) Enter(ctx context.Context) (Permit, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if b == nil {
		return nil, ErrUnavailable
	}
	b.mu.Lock()
	if b.closed || !b.expected.Valid() || b.authority == nil {
		b.mu.Unlock()
		return nil, ErrUnavailable
	}
	b.mu.Unlock()
	// Do not hold the local mutex during authority resolution. A concurrent
	// close wins before this lookup can become a transaction permit.
	p, err := b.authority.Enter(ctx)
	if err != nil || p == nil {
		if p != nil {
			p.Release()
		}
		return nil, publicAuthorityError(err)
	}
	b.mu.Lock()
	if b.closed || ctx.Err() != nil {
		b.mu.Unlock()
		p.Release()
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return nil, ErrUnavailable
	}
	o := p.Origin()
	if !o.Valid() || o != b.expected {
		b.mu.Unlock()
		p.Release()
		if !o.Valid() {
			return nil, ErrUnavailable
		}
		return nil, ErrIneligible
	}
	b.inflight++
	b.mu.Unlock()
	return &barrierPermit{owner: b, underlying: p, origin: o}, nil
}

func publicAuthorityError(err error) error {
	if errors.Is(err, ErrIneligible) {
		return ErrIneligible
	}
	return ErrUnavailable
}

// CloseAndDrain permanently rejects new permits. An interrupted drain remains
// closed and may be resumed; reopening requires a new, independently bound root.
// Authority lookups are not admitted work: a late lookup is released and denied.
// The upstream controller must independently drain/fence its own operations.
func (b *Barrier) CloseAndDrain(ctx context.Context) error {
	if b == nil {
		return ErrUnavailable
	}
	b.mu.Lock()
	if b.drained == nil {
		b.drained = make(chan struct{})
	}
	if !b.closed {
		b.closed = true
		if b.inflight == 0 {
			close(b.drained)
		}
	}
	drained := b.drained
	b.mu.Unlock()
	select {
	case <-drained:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type barrierPermit struct {
	owner      *Barrier
	underlying Permit
	origin     Origin
	once       sync.Once
}

func (p *barrierPermit) Origin() Origin { return p.origin }
func (p *barrierPermit) Release() {
	p.once.Do(func() {
		// Drain completes only after the underlying transaction guard releases.
		p.underlying.Release()
		p.owner.mu.Lock()
		defer p.owner.mu.Unlock()
		p.owner.inflight--
		if p.owner.closed && p.owner.inflight == 0 {
			close(p.owner.drained)
		}
	})
}
