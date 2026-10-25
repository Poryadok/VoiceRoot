package storylfpadmission

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
)

func origin(t *testing.T, n byte) Origin {
	t.Helper()
	o, err := NewOrigin([16]byte{1}, [16]byte{2}, [16]byte{n}, [32]byte{4})
	if err != nil {
		t.Fatal(err)
	}
	return o
}

type permit struct {
	o        Origin
	releases atomic.Int32
}

func (p *permit) Origin() Origin { return p.o }
func (p *permit) Release()       { p.releases.Add(1) }

type admissionFunc func(context.Context) (Permit, error)

func (f admissionFunc) Enter(c context.Context) (Permit, error) { return f(c) }

type peerFunc func(context.Context) (Origin, error)

func (f peerFunc) ResolveAuthenticatedPeer(c context.Context) (Origin, error) { return f(c) }

func TestDrainIncludesTransactionCompletion(t *testing.T) {
	o := origin(t, 3)
	upstream := &permit{o: o}
	b := NewBarrier(o, admissionFunc(func(context.Context) (Permit, error) { return upstream, nil }))
	p, err := b.Enter(context.Background())
	if err != nil {
		t.Fatalf("current admitted transaction rejected: %v", err)
	}
	closed, cancel := context.WithCancel(context.Background())
	cancel()
	if err := b.CloseAndDrain(closed); !errors.Is(err, context.Canceled) {
		t.Fatalf("drain crossed uncommitted transaction: %v", err)
	}
	if _, err := b.Enter(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("new transaction admitted after close: %v", err)
	}
	p.Release()
	p.Release()
	if err := b.CloseAndDrain(context.Background()); err != nil {
		t.Fatal(err)
	}
	if upstream.releases.Load() != 1 {
		t.Fatalf("underlying permit released %d times", upstream.releases.Load())
	}
}

func TestCloseWhileAuthorityLookupInFlight(t *testing.T) {
	o := origin(t, 3)
	upstream := &permit{o: o}
	started := make(chan struct{})
	resume := make(chan struct{})
	b := NewBarrier(o, admissionFunc(func(context.Context) (Permit, error) { close(started); <-resume; return upstream, nil }))
	result := make(chan error, 1)
	go func() { _, err := b.Enter(context.Background()); result <- err }()
	<-started
	if err := b.CloseAndDrain(context.Background()); err != nil {
		t.Fatal(err)
	}
	close(resume)
	if err := <-result; !errors.Is(err, ErrUnavailable) {
		t.Fatalf("stale lookup crossed close: %v", err)
	}
	if upstream.releases.Load() != 1 {
		t.Fatal("stale lookup permit leaked")
	}
}

func TestCanceledDrainCannotReopen(t *testing.T) {
	o := origin(t, 3)
	b := NewBarrier(o, admissionFunc(func(context.Context) (Permit, error) { return &permit{o: o}, nil }))
	p, err := b.Enter(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := b.CloseAndDrain(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected unresolved canceled drain: %v", err)
	}
	if _, err := b.Enter(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("canceled drain reopened: %v", err)
	}
	p.Release()
	if err := b.CloseAndDrain(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestMissingAndMismatchedAuthority(t *testing.T) {
	o := origin(t, 3)
	for _, b := range []*Barrier{nil, {}, NewBarrier(o, nil), NewBarrier(Origin{}, nil)} {
		if _, err := b.Enter(context.Background()); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("missing authority admitted: %v", err)
		}
	}
	p := &permit{o: origin(t, 8)}
	b := NewBarrier(o, admissionFunc(func(context.Context) (Permit, error) { return p, nil }))
	if _, err := b.Enter(context.Background()); !errors.Is(err, ErrIneligible) {
		t.Fatalf("foreign cohort admitted: %v", err)
	}
	if p.releases.Load() != 1 {
		t.Fatal("foreign permit leaked")
	}
}

func TestVerifiedPeerRetainsActualOrigin(t *testing.T) {
	o := origin(t, 3)
	p, err := ResolvePeer(context.Background(), o, peerFunc(func(context.Context) (Origin, error) { return o, nil }))
	if err != nil || !p.Valid() || p.Origin() != o {
		t.Fatalf("verified peer provenance lost: %v", err)
	}
	for _, tc := range []struct {
		name string
		a    AuthenticatedPeerAuthority
		want error
	}{
		{"missing", nil, ErrUnavailable},
		{"unknown", peerFunc(func(context.Context) (Origin, error) { return Origin{}, nil }), ErrUnavailable},
		{"retired", peerFunc(func(context.Context) (Origin, error) { return origin(t, 8), nil }), ErrIneligible},
		{"known-retired", peerFunc(func(context.Context) (Origin, error) {
			return Origin{}, fmt.Errorf("private identity: %w", ErrIneligible)
		}), ErrIneligible},
		{"unavailable", peerFunc(func(context.Context) (Origin, error) { return Origin{}, errors.New("private endpoint detail") }), ErrUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := ResolvePeer(context.Background(), o, tc.a)
			if !errors.Is(err, tc.want) || p.Valid() {
				t.Fatalf("invalid peer accepted: %+v %v", p, err)
			}
			if err.Error() == "private endpoint detail" {
				t.Fatal("private failure escaped")
			}
		})
	}
}

func TestAuthorityErrorsPreserveOnlySafeClassification(t *testing.T) {
	o := origin(t, 3)
	for _, tc := range []struct {
		name             string
		cause, errorWant error
	}{
		{"retired", fmt.Errorf("private identity: %w", ErrIneligible), ErrIneligible},
		{"unknown", errors.New("private topology"), ErrUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &permit{o: o}
			b := NewBarrier(o, admissionFunc(func(context.Context) (Permit, error) { return p, tc.cause }))
			if _, err := b.Enter(context.Background()); err != tc.errorWant {
				t.Fatalf("wrong public classification: %v", err)
			}
			if p.releases.Load() != 1 {
				t.Fatal("failed authority permit leaked")
			}
		})
	}
}

func TestOriginIsImmutableAndRedacted(t *testing.T) {
	o := origin(t, 3)
	env := o.Environment()
	env[0] = 9
	if o.Environment()[0] != 1 {
		t.Fatal("origin mutated through accessor")
	}
	if fmt.Sprintf("%v/%#v", o, o) != "LFP origin/LFP origin" {
		t.Fatal("private origin formatted")
	}
	encoded, err := json.Marshal(o) //nolint:staticcheck // Intentionally prove the opaque origin cannot serialize private fields.
	if err != nil || string(encoded) != "{}" {
		t.Fatal("private origin serialized")
	}
	if _, err := NewOrigin([16]byte{}, [16]byte{2}, [16]byte{3}, [32]byte{4}); !errors.Is(err, ErrUnavailable) {
		t.Fatal("empty environment admitted")
	}
}
