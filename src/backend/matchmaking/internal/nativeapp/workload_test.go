package nativeapp

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
	w "voice/backend/pkg/mmadmissionwire"
)

// Explicit kernel-peer fixture. Serving supplies only the private-FD client.
type workloadPeerFixture struct{ expected w.KernelPeer }

func (p workloadPeerFixture) CheckControllerPeer(actual w.KernelPeer) error {
	if actual != p.expected {
		return errors.New("wrong peer")
	}
	return nil
}

func workloadFixture(t *testing.T) (*workloadControl, w.KernelPeer, w.WorkloadRequestV1) {
	t.Helper()
	b, s, o := gateFixture(t)
	peer := w.KernelPeer{PID: 42, UID: 0, GID: 0}
	c := &workloadControl{descriptor: w.DescriptorV1{Binding: b, ReplicaID: w.ID(uuid.New()), ProcessLaunchID: w.ID(uuid.New())}, peer: workloadPeerFixture{peer}, gate: newForwardGate(b, s, o), prepared: w.Hash{8}, launch: w.Hash{9}, unresolved: func(context.Context) ([]w.ID, error) { return []w.ID{}, nil }}
	request := w.WorkloadRequestV1{ProtocolVersion: 1, Method: "READ_REPLICA_STATUS", Nonce: w.Hash{1}, ReplicaID: c.descriptor.ReplicaID, ProcessLaunchID: c.descriptor.ProcessLaunchID, TransitionID: b.TransitionID, ExpectedBinding: b}
	return c, peer, request
}

func TestWorkloadCloseWaitsThenReportsRetainedProviderObligation(t *testing.T) {
	c, peer, request := workloadFixture(t)
	id := uuid.New()
	release, err := c.gate.enter(context.Background(), id, [32]byte{1})
	require.NoError(t, err)
	obligation := w.ID(uuid.New())
	c.unresolved = func(context.Context) ([]w.ID, error) { return []w.ID{obligation}, nil }
	response, err := c.handle(context.Background(), peer, request)
	require.NoError(t, err)
	require.Equal(t, w.Armed, response.State)
	require.Equal(t, []w.ID{w.ID(id)}, response.InFlightWorkIDs)
	request.Method = "CLOSE_REPLICA"
	done := make(chan error, 1)
	go func() {
		r, e := c.handle(context.Background(), peer, request)
		if e == nil {
			e = r.Validate()
			if r.State != w.Closed || len(r.UnresolvedProviderOperationIDs) != 1 {
				e = errors.New("close lost obligation")
			}
		}
		done <- e
	}()
	select {
	case <-done:
		t.Fatal("close bypassed durable receipt lifetime")
	case <-time.After(30 * time.Millisecond):
	}
	release()
	require.NoError(t, <-done)
	_, err = c.gate.enter(context.Background(), uuid.New(), [32]byte{1})
	require.Error(t, err)
}

func TestWorkloadRejectsWrongPeerOrRequestBeforeInventory(t *testing.T) {
	for _, kind := range []string{"peer", "replica", "launch", "binding", "method", "nonce", "missing-prepared"} {
		t.Run(kind, func(t *testing.T) {
			c, peer, r := workloadFixture(t)
			calls := 0
			c.unresolved = func(context.Context) ([]w.ID, error) { calls++; return nil, nil }
			switch kind {
			case "peer":
				peer.PID++
			case "replica":
				r.ReplicaID = w.ID(uuid.New())
			case "launch":
				r.ProcessLaunchID = w.ID(uuid.New())
			case "binding":
				r.ExpectedBinding.AdmissionGeneration++
			case "method":
				r.Method = "OPEN_REPLICA"
			case "nonce":
				r.Nonce = w.Hash{}
			case "missing-prepared":
				c.prepared = w.Hash{}
			}
			_, err := c.handle(context.Background(), peer, r)
			require.Error(t, err)
			require.Zero(t, calls)
		})
	}
}

func TestWorkloadInventoryFailureNeverReportsDrainedAndCloseNeverReopens(t *testing.T) {
	c, peer, r := workloadFixture(t)
	r.Method = "CLOSE_REPLICA"
	c.unresolved = func(context.Context) ([]w.ID, error) { return nil, errors.New("database unavailable") }
	_, err := c.handle(context.Background(), peer, r)
	require.Error(t, err)
	_, err = c.gate.enter(context.Background(), uuid.New(), [32]byte{1})
	require.Error(t, err)
	c.unresolved = func(context.Context) ([]w.ID, error) { return []w.ID{}, nil }
	response, err := c.handle(context.Background(), peer, r)
	require.NoError(t, err)
	require.Equal(t, w.Closed, response.State)
}
