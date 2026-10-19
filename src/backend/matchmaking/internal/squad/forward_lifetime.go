package squad

import (
	"context"
	"github.com/google/uuid"
)

// ForwardLifetime registers one LIVE coordinator attempt with the root's
// genuine fresh-admission owner. It does not replace source or SQL admission.
// workID is a fresh runtime identity, not a provider/business operation ID.
// Successful release must remain held until durable outcome handling returns.
// The context governs acquisition only. After successful entry, cancellation or
// deadline expiry must not unregister work; only the explicit release may do so.
// Failed admission retains any uncertainty in the owning implementation.
type ForwardLifetime interface {
	EnterForward(context.Context, uuid.UUID, [32]byte) (release func(), err error)
}

// forwardAttempt owns one local registration across every positive boundary
// and all durable outcome handling. Cleanup paths never call enter.
type forwardAttempt struct {
	worker  *MatchSquadProviderWorker
	release func()
}

func (a *forwardAttempt) enter(ctx context.Context, requestHash [32]byte) error {
	if a.release != nil {
		return nil
	}
	if a.worker.ForwardLifetime == nil {
		if a.worker.Store.NativeAdmission != nil {
			return ErrPartyProvisionUnknown
		}
		return nil
	}
	if requestHash == [32]byte{} {
		return ErrPartyProvisionUnknown
	}
	release, err := a.worker.ForwardLifetime.EnterForward(ctx, uuid.New(), requestHash)
	if err != nil || release == nil {
		// The adapter owns unresolved registration on failure. Never disclose its
		// private error or turn failure into cancellation/absence evidence.
		return ErrPartyProvisionUnknown
	}
	a.release = release
	return nil
}

func (a *forwardAttempt) finish() {
	if a.release != nil {
		release := a.release
		a.release = nil
		release()
	}
}
