package nativeapp

import (
	"bytes"
	"context"
	"sort"
	"sync/atomic"
	w "voice/backend/pkg/mmadmissionwire"
)

// workloadControl is a private control-plane receiver, never an admission issuer.
type workloadControl struct {
	descriptor       w.DescriptorV1
	peer             interface{ CheckControllerPeer(w.KernelPeer) error }
	gate             *forwardGate
	prepared, launch w.Hash
	opened           atomic.Bool
	unresolved       func(context.Context) ([]w.ID, error)
}

func (c *workloadControl) handle(ctx context.Context, peer w.KernelPeer, request w.WorkloadRequestV1) (w.WorkloadResponseV1, error) {
	if c == nil || ctx == nil || ctx.Err() != nil || c.peer == nil || c.peer.CheckControllerPeer(peer) != nil || request.Validate() != nil || c.gate == nil || c.unresolved == nil || c.prepared == (w.Hash{}) || c.launch == (w.Hash{}) || request.ExpectedBinding != c.descriptor.Binding || request.ReplicaID != c.descriptor.ReplicaID || request.ProcessLaunchID != c.descriptor.ProcessLaunchID {
		return w.WorkloadResponseV1{}, errUnavailable
	}
	if request.Method == "CLOSE_REPLICA" {
		if err := c.gate.closeAndDrain(ctx); err != nil {
			return w.WorkloadResponseV1{}, errUnavailable
		}
	}
	// Include backoff, claimed and possibly issued operations. A failed/overflowed
	// census is unavailable, never an empty or truncated close acknowledgement.
	unresolved, err := c.unresolved(ctx)
	if err != nil || len(unresolved) > 1024 {
		return w.WorkloadResponseV1{}, errUnavailable
	}
	unresolved = append([]w.ID{}, unresolved...)
	sort.Slice(unresolved, func(i, j int) bool { return bytes.Compare(unresolved[i][:], unresolved[j][:]) < 0 })
	bindingHash, err := w.BindingHash(c.descriptor.Binding)
	if err != nil {
		return w.WorkloadResponseV1{}, errUnavailable
	}
	state := w.Armed
	if c.opened.Load() {
		state = w.Open
	}
	c.gate.mu.Lock()
	// Once closure begins, status never advertises OPEN. A pending drain stays
	// unavailable instead of claiming a CLOSED replica with live registrations.
	closed, pending := c.gate.closed, len(c.gate.active)
	c.gate.mu.Unlock()
	if closed {
		if pending > 0 {
			return w.WorkloadResponseV1{}, errUnavailable
		}
		state = w.Closed
	}
	response := w.WorkloadResponseV1{ProtocolVersion: 1, Nonce: request.Nonce, ReplicaID: request.ReplicaID, ProcessLaunchID: request.ProcessLaunchID, TransitionID: request.TransitionID, BindingHash: bindingHash, State: state, InFlightWorkIDs: c.gate.inFlight(), UnresolvedProviderOperationIDs: unresolved, PreparedContractsHash: c.prepared, ObservedLaunchHash: c.launch}
	if response.Validate() != nil {
		return w.WorkloadResponseV1{}, errUnavailable
	}
	return response, nil
}
