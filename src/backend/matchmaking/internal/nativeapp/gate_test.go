package nativeapp

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	w "voice/backend/pkg/mmadmissionwire"
)

// Explicit local observation fixture. Production constructs readbackObserver
// only from the independently authenticated private-FD wire client.
type gateObservationFixture struct {
	binding w.Binding
	stamp   w.AdmissionStampV1
	err     error
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (f *gateObservationFixture) observeFresh(ctx context.Context, _ uuid.UUID, _ [32]byte) (w.Binding, w.AdmissionStampV1, error) {
	if f.started != nil {
		f.once.Do(func() { close(f.started) })
		select {
		case <-f.release:
		case <-ctx.Done():
			return w.Binding{}, w.AdmissionStampV1{}, ctx.Err()
		}
	}
	return f.binding, f.stamp, f.err
}

func gateFixture(t *testing.T) (w.Binding, w.AdmissionStampV1, *gateObservationFixture) {
	t.Helper()
	b := w.Binding{ProtocolVersion: 1, EnvironmentID: w.ID(uuid.New()), InstallationID: w.ID(uuid.New()), AuthorityIncarnation: w.ID(uuid.New()), TransitionID: w.ID(uuid.New()), AdmissionGeneration: 1, CredentialCohortID: w.ID(uuid.New()), ExpectedLoginRoleOID: 42, ExpectedLoginRoleName: "mm_fixture"}
	b.DatabaseBindingHash, b.SchemaContractHash, b.SourceFactoryManifestHash, b.FreshRouteManifestHash, b.AdmissionReceiptHash, b.LaunchManifestHash = w.Hash{1}, w.Hash{2}, w.Hash{3}, w.Hash{4}, w.Hash{5}, w.Hash{6}
	h, err := w.BindingHash(b)
	require.NoError(t, err)
	s := w.AdmissionStampV1{Version: 1, BindingHash: h, DatabaseLineageID: w.ID(uuid.New()), ContinuityHistoryRootHash: w.Hash{7}}
	return b, s, &gateObservationFixture{binding: b, stamp: s}
}

func TestForwardGateCloseWaitsAcrossObservationAndDispatchReceipt(t *testing.T) {
	b, s, observer := gateFixture(t)
	observer.started, observer.release = make(chan struct{}), make(chan struct{})
	g := newForwardGate(b, s, observer)
	id := uuid.New()
	type entry struct {
		release func()
		err     error
	}
	entered := make(chan entry, 1)
	go func() { release, err := g.enter(context.Background(), id, [32]byte{1}); entered <- entry{release, err} }()
	select {
	case <-observer.started:
	case <-time.After(time.Second):
		t.Fatal("forward entry never requested fresh admission")
	}
	require.Equal(t, []w.ID{w.ID(id)}, g.inFlight())
	closed := make(chan error, 1)
	go func() { closed <- g.closeAndDrain(context.Background()) }()
	// The in-flight registration precedes the network read. A close cannot
	// report drained in the gap before the authorized SQL/remote dispatch.
	select {
	case <-closed:
		t.Fatal("close bypassed pending fresh observation")
	case <-time.After(30 * time.Millisecond):
	}
	close(observer.release)
	var active entry
	select {
	case active = <-entered:
	case <-time.After(time.Second):
		t.Fatal("entry stalled")
	}
	require.NoError(t, active.err)
	select {
	case <-closed:
		t.Fatal("close bypassed remote receipt persistence")
	case <-time.After(30 * time.Millisecond):
	}
	active.release()
	active.release()
	require.NoError(t, <-closed)
	require.Empty(t, g.inFlight())
	_, err := g.enter(context.Background(), uuid.New(), [32]byte{1})
	require.ErrorIs(t, err, errUnavailable)
}

func TestForwardGateUnknownOrChangedStampCannotDispatch(t *testing.T) {
	for _, kind := range []string{"unknown", "binding", "history", "absent"} {
		t.Run(kind, func(t *testing.T) {
			b, s, o := gateFixture(t)
			var observer admissionObserver = o
			switch kind {
			case "unknown":
				o.err = errors.New("private observation unavailable")
			case "binding":
				o.binding.AdmissionGeneration++
			case "history":
				o.stamp.ContinuityHistoryRootHash = w.Hash{99}
			case "absent":
				observer = nil
			}
			g := newForwardGate(b, s, observer)
			release, err := g.enter(context.Background(), uuid.New(), [32]byte{1})
			require.ErrorIs(t, err, errUnavailable)
			require.Nil(t, release)
			require.Empty(t, g.inFlight())
			require.NoError(t, g.closeAndDrain(context.Background()))
		})
	}
}

func TestForwardGateTimedOutCloseNeverReopens(t *testing.T) {
	b, s, o := gateFixture(t)
	g := newForwardGate(b, s, o)
	release, err := g.enter(context.Background(), uuid.New(), [32]byte{1})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	require.Error(t, g.closeAndDrain(ctx))
	_, err = g.enter(context.Background(), uuid.New(), [32]byte{1})
	require.ErrorIs(t, err, errUnavailable)
	release()
	require.NoError(t, g.closeAndDrain(context.Background()))
}

func TestForwardGateCanceledAttemptRetainsRegistrationThroughDurableHandling(t *testing.T) {
	b, s, observer := gateFixture(t)
	g := newForwardGate(b, s, observer)
	acquire, cancel := context.WithCancel(context.Background())
	id := uuid.New()
	release, err := g.EnterForward(acquire, id, [32]byte{1})
	require.NoError(t, err)
	cancel()
	require.Equal(t, []w.ID{w.ID(id)}, g.inFlight(), "attempt cancellation is not a durable outcome or release")
	drain, done := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer done()
	require.Error(t, g.closeAndDrain(drain), "close must wait for retry/claim/receipt persistence after attempt cancellation")
	release()
	require.NoError(t, g.closeAndDrain(context.Background()))
}
