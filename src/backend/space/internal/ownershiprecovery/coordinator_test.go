package ownershiprecovery

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/space/internal/store"
)

func TestCoordinator_ExecutePersistsProtocolTwoHappyPath(t *testing.T) {
	binding := store.OwnershipBinding{
		ProtocolVersion: 2,
		SpaceID:         uuid.New(), AccountID: uuid.New(), ActorProfileID: uuid.New(),
		NewOwnerProfileID: uuid.New(), OperationID: uuid.New(), SessionEpoch: 1,
		ProofDigest: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}
	deps := newHappyPathDependencies(t, binding)

	result, err := NewCoordinator(Dependencies{Store: deps, Auth: deps, Role: deps}).Execute(context.Background(), binding, "opaque-proof")

	require.NoError(t, err)
	require.Equal(t, "completed", result.State)
	require.Equal(t, []string{"reserve", "consume_started", "consume", "confirm", "prepare", "prepared", "commit", "finalize", "complete"}, deps.calls)
}

func TestCoordinator_AmbiguousConsumeUsesReceiptLookupWithoutAnotherConsume(t *testing.T) {
	binding := store.OwnershipBinding{ProtocolVersion: 2, SpaceID: uuid.New(), AccountID: uuid.New(), ActorProfileID: uuid.New(), NewOwnerProfileID: uuid.New(), OperationID: uuid.New(), SessionEpoch: 1, ProofDigest: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}
	deps := newHappyPathDependencies(t, binding)
	deps.consumeErr = ErrAmbiguousConsume

	result, err := NewCoordinator(Dependencies{Store: deps, Auth: deps, Role: deps}).Execute(context.Background(), binding, "opaque-proof")

	require.NoError(t, err)
	require.Equal(t, "completed", result.State)
	require.Equal(t, []string{"reserve", "consume_started", "consume", "lookup", "confirm", "prepare", "prepared", "commit", "finalize", "complete"}, deps.calls)
}

func TestCoordinator_RecoverReservedSelectsAbortWithoutProofConsume(t *testing.T) {
	binding := store.OwnershipBinding{ProtocolVersion: 2, SpaceID: uuid.New(), AccountID: uuid.New(), ActorProfileID: uuid.New(), NewOwnerProfileID: uuid.New(), OperationID: uuid.New(), SessionEpoch: 1, ProofDigest: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}
	deps := newHappyPathDependencies(t, binding)
	result, err := NewCoordinator(Dependencies{Store: deps, Auth: deps, Role: deps}).Recover(context.Background(), &store.OwnershipJournal{Binding: binding, State: "reserved"})
	require.NoError(t, err)
	require.Equal(t, "aborted", result.State)
	require.Equal(t, []string{"abort_decided", "abort", "abort_complete"}, deps.calls)
}

type happyPathDependencies struct {
	binding    store.OwnershipBinding
	calls      []string
	consumeErr error
}

func newHappyPathDependencies(t *testing.T, binding store.OwnershipBinding) *happyPathDependencies {
	t.Helper()
	return &happyPathDependencies{binding: binding}
}
func (d *happyPathDependencies) ReserveOwnership(context.Context, store.OwnershipBinding) (*store.OwnershipJournal, error) {
	d.calls = append(d.calls, "reserve")
	return &store.OwnershipJournal{Binding: d.binding, State: "reserved"}, nil
}
func (d *happyPathDependencies) MarkOwnershipConsumeStarted(context.Context, store.OwnershipBinding) (*store.OwnershipJournal, error) {
	d.calls = append(d.calls, "consume_started")
	return &store.OwnershipJournal{Binding: d.binding, State: "consume_started"}, nil
}
func (d *happyPathDependencies) LoadOwnership(context.Context, uuid.UUID) (*store.OwnershipJournal, error) {
	panic("not reached")
}
func (d *happyPathDependencies) ConfirmOwnershipProof(_ context.Context, _ store.OwnershipBinding, _ store.OwnershipAuthReceipt) (*store.OwnershipJournal, error) {
	d.calls = append(d.calls, "confirm")
	return &store.OwnershipJournal{Binding: d.binding, State: "proof_confirmed"}, nil
}
func (d *happyPathDependencies) MarkOwnershipPrepared(_ context.Context, _ store.OwnershipBinding, _ *rolev1.OwnershipTransferReceipt) (*store.OwnershipJournal, error) {
	d.calls = append(d.calls, "prepared")
	return &store.OwnershipJournal{Binding: d.binding, State: "prepared"}, nil
}
func (d *happyPathDependencies) DecideOwnershipCommit(context.Context, store.OwnershipBinding) (*store.OwnershipJournal, error) {
	d.calls = append(d.calls, "commit")
	return &store.OwnershipJournal{Binding: d.binding, State: "commit_decided"}, nil
}
func (d *happyPathDependencies) DecideOwnershipAbort(context.Context, store.OwnershipBinding) (*store.OwnershipJournal, error) {
	d.calls = append(d.calls, "abort_decided")
	return &store.OwnershipJournal{Binding: d.binding, State: "abort_decided"}, nil
}
func (d *happyPathDependencies) CompleteOwnershipCommit(_ context.Context, _ store.OwnershipBinding, _ *rolev1.OwnershipTransferReceipt) (*store.OwnershipJournal, error) {
	d.calls = append(d.calls, "complete")
	return &store.OwnershipJournal{Binding: d.binding, State: "completed"}, nil
}
func (d *happyPathDependencies) CompleteOwnershipAbort(context.Context, store.OwnershipBinding, *rolev1.OwnershipTransferReceipt) (*store.OwnershipJournal, error) {
	d.calls = append(d.calls, "abort_complete")
	return &store.OwnershipJournal{Binding: d.binding, State: "aborted"}, nil
}
func (d *happyPathDependencies) Consume(context.Context, store.OwnershipBinding, string) (store.OwnershipAuthReceipt, error) {
	d.calls = append(d.calls, "consume")
	if d.consumeErr != nil {
		return store.OwnershipAuthReceipt{}, d.consumeErr
	}
	return d.receipt(), nil
}
func (d *happyPathDependencies) Lookup(context.Context, store.OwnershipBinding) (store.OwnershipAuthReceipt, error) {
	d.calls = append(d.calls, "lookup")
	return d.receipt(), nil
}
func (d *happyPathDependencies) receipt() store.OwnershipAuthReceipt {
	return store.OwnershipAuthReceipt{ReceiptID: uuid.New(), AccountID: d.binding.AccountID, ProfileID: d.binding.ActorProfileID, SpaceID: d.binding.SpaceID, NewOwnerProfileID: d.binding.NewOwnerProfileID, OperationID: d.binding.OperationID, SessionEpoch: d.binding.SessionEpoch, ConsumedAt: time.Now(), VerifiedFactors: []string{"password"}}
}
func (d *happyPathDependencies) Prepare(context.Context, store.OwnershipBinding) (*rolev1.OwnershipTransferReceipt, error) {
	d.calls = append(d.calls, "prepare")
	return &rolev1.OwnershipTransferReceipt{}, nil
}
func (d *happyPathDependencies) Finalize(context.Context, store.OwnershipBinding) (*rolev1.OwnershipTransferReceipt, error) {
	d.calls = append(d.calls, "finalize")
	return &rolev1.OwnershipTransferReceipt{}, nil
}
func (d *happyPathDependencies) Abort(context.Context, store.OwnershipBinding) (*rolev1.OwnershipTransferReceipt, error) {
	d.calls = append(d.calls, "abort")
	return &rolev1.OwnershipTransferReceipt{}, nil
}
