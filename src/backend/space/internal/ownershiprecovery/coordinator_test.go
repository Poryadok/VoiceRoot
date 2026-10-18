package ownershiprecovery

import (
	"context"
	"errors"
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

func TestCoordinator_RecoverConsumeStartedLooksUpReceiptWithoutConsumingProof(t *testing.T) {
	binding := store.OwnershipBinding{ProtocolVersion: 2, SpaceID: uuid.New(), AccountID: uuid.New(), ActorProfileID: uuid.New(), NewOwnerProfileID: uuid.New(), OperationID: uuid.New(), SessionEpoch: 1, ProofDigest: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}
	deps := newHappyPathDependencies(t, binding)
	deps.journal.State = "consume_started"
	result, err := NewCoordinator(Dependencies{Store: deps, Auth: deps, Role: deps}).Recover(context.Background(), &store.OwnershipJournal{Binding: binding, State: "consume_started"})
	require.NoError(t, err)
	require.Equal(t, "completed", result.State)
	require.Equal(t, []string{"lookup", "confirm", "prepare", "prepared", "commit", "finalize", "complete"}, deps.calls)
}

func TestCoordinator_RecoverUnconfiguredFailsClosedWithoutPanic(t *testing.T) {
	binding := store.OwnershipBinding{ProtocolVersion: 2, SpaceID: uuid.New(), AccountID: uuid.New(), ActorProfileID: uuid.New(), NewOwnerProfileID: uuid.New(), OperationID: uuid.New(), SessionEpoch: 1, ProofDigest: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}
	for _, coordinator := range []*Coordinator{nil, NewCoordinator(Dependencies{})} {
		result, err := coordinator.Recover(context.Background(), &store.OwnershipJournal{Binding: binding, State: "consume_started"})
		require.Nil(t, result)
		require.ErrorIs(t, err, ErrCoordinatorNotConfigured)
	}
}

func TestCoordinator_RecoverUsesCurrentDecisionInsteadOfScannerSnapshot(t *testing.T) {
	for _, tc := range []struct {
		name, stale, current, result string
		calls                        []string
	}{
		{"commit_after_reserved_scan", "reserved", "commit_decided", "completed", []string{"finalize", "complete"}},
		{"abort_after_proof_scan", "proof_confirmed", "abort_decided", "aborted", []string{"abort", "abort_complete"}},
		{"already_completed", "proof_confirmed", "completed", "completed", nil},
		{"already_aborted", "consume_started", "aborted", "aborted", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			binding := recoveryBinding()
			deps := newHappyPathDependencies(t, binding)
			deps.journal.State = tc.current
			result, err := NewCoordinator(Dependencies{Store: deps, Auth: deps, Role: deps}).Recover(context.Background(), &store.OwnershipJournal{Binding: binding, State: tc.stale})
			require.NoError(t, err)
			require.Equal(t, tc.result, result.State)
			require.Equal(t, tc.calls, deps.calls)
			require.Positive(t, deps.loads)
		})
	}
}

func TestCoordinator_RecoverCurrentReadFailureHasNoRemoteEffects(t *testing.T) {
	binding := recoveryBinding()
	deps := newHappyPathDependencies(t, binding)
	deps.loadErr = errors.New("journal unavailable")
	result, err := NewCoordinator(Dependencies{Store: deps, Auth: deps, Role: deps}).Recover(context.Background(), &store.OwnershipJournal{Binding: binding, State: "commit_decided"})
	require.Nil(t, result)
	require.ErrorIs(t, err, deps.loadErr)
	require.Empty(t, deps.calls)
}

func TestCoordinator_RecoverChangedBindingHasNoRemoteEffects(t *testing.T) {
	binding := recoveryBinding()
	deps := newHappyPathDependencies(t, binding)
	deps.journal.Binding.NewOwnerProfileID = uuid.New()
	result, err := NewCoordinator(Dependencies{Store: deps, Auth: deps, Role: deps}).Recover(context.Background(), &store.OwnershipJournal{Binding: binding, State: "commit_decided"})
	require.Nil(t, result)
	require.ErrorIs(t, err, store.ErrOwnershipConflict)
	require.Empty(t, deps.calls)
}

func TestCoordinator_RecoverMissingCurrentJournalHasNoRemoteEffects(t *testing.T) {
	binding := recoveryBinding()
	deps := newHappyPathDependencies(t, binding)
	deps.journal = nil
	result, err := NewCoordinator(Dependencies{Store: deps, Auth: deps, Role: deps}).Recover(context.Background(), &store.OwnershipJournal{Binding: binding, State: "commit_decided"})
	require.Nil(t, result)
	require.ErrorIs(t, err, store.ErrOwnershipMissing)
	require.Empty(t, deps.calls)
}

func TestCoordinator_AbortCompletedByAnotherWorkerDoesNotRepeatRoleAbort(t *testing.T) {
	binding := recoveryBinding()
	deps := newHappyPathDependencies(t, binding)
	deps.afterAbortDecision = func() { deps.journal.State = "aborted" }
	result, err := NewCoordinator(Dependencies{Store: deps, Auth: deps, Role: deps}).Recover(context.Background(), deps.journal)
	require.NoError(t, err)
	require.Equal(t, "aborted", result.State)
	require.Equal(t, []string{"abort_decided"}, deps.calls)
}

func recoveryBinding() store.OwnershipBinding {
	return store.OwnershipBinding{ProtocolVersion: 2, SpaceID: uuid.New(), AccountID: uuid.New(), ActorProfileID: uuid.New(), NewOwnerProfileID: uuid.New(), OperationID: uuid.New(), SessionEpoch: 1, ProofDigest: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}
}

type happyPathDependencies struct {
	binding            store.OwnershipBinding
	calls              []string
	consumeErr         error
	journal            *store.OwnershipJournal
	loadErr            error
	loads              int
	afterAbortDecision func()
}

func newHappyPathDependencies(t *testing.T, binding store.OwnershipBinding) *happyPathDependencies {
	t.Helper()
	return &happyPathDependencies{binding: binding, journal: &store.OwnershipJournal{Binding: binding, State: "reserved"}}
}
func (d *happyPathDependencies) saved(state string) (*store.OwnershipJournal, error) {
	d.journal = &store.OwnershipJournal{Binding: d.binding, State: state}
	return d.journal, nil
}
func (d *happyPathDependencies) ReserveOwnership(context.Context, store.OwnershipBinding) (*store.OwnershipJournal, error) {
	d.calls = append(d.calls, "reserve")
	return d.saved("reserved")
}
func (d *happyPathDependencies) MarkOwnershipConsumeStarted(context.Context, store.OwnershipBinding) (*store.OwnershipJournal, error) {
	d.calls = append(d.calls, "consume_started")
	return d.saved("consume_started")
}
func (d *happyPathDependencies) LoadOwnership(context.Context, uuid.UUID) (*store.OwnershipJournal, error) {
	d.loads++
	return d.journal, d.loadErr
}
func (d *happyPathDependencies) ConfirmOwnershipProof(_ context.Context, _ store.OwnershipBinding, _ store.OwnershipAuthReceipt) (*store.OwnershipJournal, error) {
	d.calls = append(d.calls, "confirm")
	return d.saved("proof_confirmed")
}
func (d *happyPathDependencies) MarkOwnershipPrepared(_ context.Context, _ store.OwnershipBinding, _ *rolev1.OwnershipTransferReceipt) (*store.OwnershipJournal, error) {
	d.calls = append(d.calls, "prepared")
	return d.saved("prepared")
}
func (d *happyPathDependencies) DecideOwnershipCommit(context.Context, store.OwnershipBinding) (*store.OwnershipJournal, error) {
	d.calls = append(d.calls, "commit")
	return d.saved("commit_decided")
}
func (d *happyPathDependencies) DecideOwnershipAbort(context.Context, store.OwnershipBinding) (*store.OwnershipJournal, error) {
	d.calls = append(d.calls, "abort_decided")
	journal, err := d.saved("abort_decided")
	if d.afterAbortDecision != nil {
		d.afterAbortDecision()
	}
	return journal, err
}
func (d *happyPathDependencies) CompleteOwnershipCommit(_ context.Context, _ store.OwnershipBinding, _ *rolev1.OwnershipTransferReceipt) (*store.OwnershipJournal, error) {
	d.calls = append(d.calls, "complete")
	return d.saved("completed")
}
func (d *happyPathDependencies) CompleteOwnershipAbort(context.Context, store.OwnershipBinding, *rolev1.OwnershipTransferReceipt) (*store.OwnershipJournal, error) {
	d.calls = append(d.calls, "abort_complete")
	return d.saved("aborted")
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
