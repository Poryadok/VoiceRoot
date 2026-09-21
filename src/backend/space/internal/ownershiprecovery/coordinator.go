// Package ownershiprecovery contains the private, durable protocol-2 ownership
// convergence loop. It deliberately has no public RPC dependency: activation
// of the public transfer surface remains a separate vertical.
package ownershiprecovery

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/space/internal/store"
)

var ErrCoordinatorNotConfigured = errors.New("ownership recovery coordinator not configured")

// ErrAmbiguousConsume marks a request whose remote outcome was not observed.
// Recovery may only use Auth's receipt lookup; it must not submit another proof.
var ErrAmbiguousConsume = errors.New("ownership proof consume outcome is ambiguous")

type Store interface {
	ReserveOwnership(context.Context, store.OwnershipBinding) (*store.OwnershipJournal, error)
	MarkOwnershipConsumeStarted(context.Context, store.OwnershipBinding) (*store.OwnershipJournal, error)
	LoadOwnership(context.Context, uuid.UUID) (*store.OwnershipJournal, error)
	ConfirmOwnershipProof(context.Context, store.OwnershipBinding, store.OwnershipAuthReceipt) (*store.OwnershipJournal, error)
	MarkOwnershipPrepared(context.Context, store.OwnershipBinding, *rolev1.OwnershipTransferReceipt) (*store.OwnershipJournal, error)
	DecideOwnershipCommit(context.Context, store.OwnershipBinding) (*store.OwnershipJournal, error)
	DecideOwnershipAbort(context.Context, store.OwnershipBinding) (*store.OwnershipJournal, error)
	CompleteOwnershipCommit(context.Context, store.OwnershipBinding, *rolev1.OwnershipTransferReceipt) (*store.OwnershipJournal, error)
	CompleteOwnershipAbort(context.Context, store.OwnershipBinding, *rolev1.OwnershipTransferReceipt) (*store.OwnershipJournal, error)
}

type Auth interface {
	Consume(context.Context, store.OwnershipBinding, string) (store.OwnershipAuthReceipt, error)
	Lookup(context.Context, store.OwnershipBinding) (store.OwnershipAuthReceipt, error)
}

type Role interface {
	Prepare(context.Context, store.OwnershipBinding) (*rolev1.OwnershipTransferReceipt, error)
	Finalize(context.Context, store.OwnershipBinding) (*rolev1.OwnershipTransferReceipt, error)
	Abort(context.Context, store.OwnershipBinding) (*rolev1.OwnershipTransferReceipt, error)
}

type Dependencies struct {
	Store Store
	Auth  Auth
	Role  Role
}
type Coordinator struct{ dependencies Dependencies }

func NewCoordinator(dependencies Dependencies) *Coordinator {
	return &Coordinator{dependencies: dependencies}
}

// Execute reserves a new operation or resumes the exact saved operation. A
// completed replay returns durable evidence before any current-owner checks.
func (c *Coordinator) Execute(ctx context.Context, binding store.OwnershipBinding, proof string) (*store.OwnershipJournal, error) {
	if c == nil || c.dependencies.Store == nil || c.dependencies.Auth == nil || c.dependencies.Role == nil {
		return nil, ErrCoordinatorNotConfigured
	}
	journal, err := c.dependencies.Store.ReserveOwnership(ctx, binding)
	if err != nil {
		return nil, err
	}
	if journal.State == "reserved" {
		journal, err = c.dependencies.Store.MarkOwnershipConsumeStarted(ctx, binding)
		if err != nil {
			return nil, err
		}
	}
	return c.resume(ctx, binding, proof, journal)
}

// Recover resumes only durable evidence. A reservation without a persisted Auth
// receipt has no plaintext proof by design, so it selects the durable abort
// branch rather than attempting a new consume.
func (c *Coordinator) Recover(ctx context.Context, journal *store.OwnershipJournal) (*store.OwnershipJournal, error) {
	if c == nil || c.dependencies.Store == nil || c.dependencies.Auth == nil || c.dependencies.Role == nil {
		return nil, ErrCoordinatorNotConfigured
	}
	if journal == nil {
		return nil, errors.New("ownership recovery received nil journal")
	}
	return c.resume(ctx, journal.Binding, "", journal)
}

func (c *Coordinator) resume(ctx context.Context, binding store.OwnershipBinding, proof string, journal *store.OwnershipJournal) (*store.OwnershipJournal, error) {
	if journal == nil {
		return nil, errors.New("ownership recovery received nil journal")
	}
	var err error
	switch journal.State {
	case "completed", "aborted":
		return journal, nil
	case "reserved":
		return c.abort(ctx, binding, journal, nil)
	case "consume_started":
		if proof == "" {
			receipt, lookupErr := c.dependencies.Auth.Lookup(ctx, binding)
			if lookupErr != nil {
				return c.abort(ctx, binding, journal, lookupErr)
			}
			journal, err = c.dependencies.Store.ConfirmOwnershipProof(ctx, binding, receipt)
			break
		}
		receipt, err := c.dependencies.Auth.Consume(ctx, binding, proof)
		if errors.Is(err, ErrAmbiguousConsume) {
			receipt, err = c.dependencies.Auth.Lookup(ctx, binding)
		}
		if err != nil {
			return c.abort(ctx, binding, journal, err)
		}
		journal, err = c.dependencies.Store.ConfirmOwnershipProof(ctx, binding, receipt)
	case "proof_confirmed":
		var receipt *rolev1.OwnershipTransferReceipt
		receipt, err = c.dependencies.Role.Prepare(ctx, binding)
		if err != nil {
			return c.abort(ctx, binding, journal, err)
		}
		journal, err = c.dependencies.Store.MarkOwnershipPrepared(ctx, binding, receipt)
	case "prepared":
		journal, err = c.dependencies.Store.DecideOwnershipCommit(ctx, binding)
	case "commit_decided":
		var receipt *rolev1.OwnershipTransferReceipt
		receipt, err = c.dependencies.Role.Finalize(ctx, binding)
		if err == nil {
			journal, err = c.dependencies.Store.CompleteOwnershipCommit(ctx, binding, receipt)
		}
	case "abort_decided":
		return c.abort(ctx, binding, journal, nil)
	default:
		return nil, fmt.Errorf("unsupported ownership journal state %q", journal.State)
	}
	if err != nil {
		return nil, err
	}
	return c.resume(ctx, binding, proof, journal)
}

func (c *Coordinator) abort(ctx context.Context, binding store.OwnershipBinding, journal *store.OwnershipJournal, cause error) (*store.OwnershipJournal, error) {
	var err error
	if journal.State != "abort_decided" {
		journal, err = c.dependencies.Store.DecideOwnershipAbort(ctx, binding)
		if err != nil {
			return nil, errors.Join(cause, err)
		}
	}
	receipt, err := c.dependencies.Role.Abort(ctx, binding)
	if err != nil {
		return nil, errors.Join(cause, err)
	}
	journal, err = c.dependencies.Store.CompleteOwnershipAbort(ctx, binding, receipt)
	if err != nil {
		return nil, errors.Join(cause, err)
	}
	return journal, cause
}
