package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	commonv1 "voice.app/voice/common/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/spacecore"
)

type lifecycleScheduleOutcomeStore interface {
	ReplayLifecycleScheduleOutcome(context.Context, uuid.UUID, uuid.UUID, int64, *spacev1.DeleteSpaceRequest) (*spacev1.DeleteSpaceResponse, error)
}

func requireLifecycleScheduleOutcome(t *testing.T, st *SpaceStore) lifecycleScheduleOutcomeStore {
	t.Helper()
	outcomes, ok := any(st).(lifecycleScheduleOutcomeStore)
	require.True(t, ok, "SpaceStore must expose typed saved DELETE outcome replay independent of the mutable aggregate")
	return outcomes
}

func TestLifecycleOutcome_TypedReplaySurfaceRequired(t *testing.T) {
	requireLifecycleScheduleOutcome(t, &SpaceStore{})
}

type lifecycleOutcomeFixture struct {
	st      *SpaceStore
	store   durableLifecycleStore
	account uuid.UUID
	actor   uuid.UUID
	space   uuid.UUID
	request *spacev1.DeleteSpaceRequest
}

func newLifecycleOutcomeFixture(t *testing.T, fenceCount int) lifecycleOutcomeFixture {
	t.Helper()
	st := lifecycleStoreFixture(t)
	store, aggregate, account, actor, spaceID, request, proofBytes, proofHash := reserveLifecycleOperationFixture(t, st)
	operationID := uuid.MustParse(request.OperationId)
	_, err := store.RecordLifecycleDeletionProofReceipt(context.Background(), actor, operationID, proofBytes, proofHash)
	require.NoError(t, err)
	require.NoError(t, aggregate.BeginFreeze(lifecycleManifestFixture()))
	for _, participant := range lifecycleTestParticipants[:fenceCount] {
		require.NoError(t, aggregate.RecordFenceReceipt(lifecycleFenceReceiptFixture(t, spaceID, operationID, participant, 1,
			commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, lifecycleManifestFixture())))
	}
	require.NoError(t, store.PersistLifecycle(context.Background(), aggregate))
	return lifecycleOutcomeFixture{st, store, account, actor, spaceID, request}
}

type lifecycleOutcomeEvidence struct {
	state       string
	bytes       []byte
	hash        []byte
	completedAt *time.Time
}

func readLifecycleOutcomeEvidence(t *testing.T, f lifecycleOutcomeFixture) lifecycleOutcomeEvidence {
	t.Helper()
	var evidence lifecycleOutcomeEvidence
	require.NoError(t, f.st.Pool.QueryRow(context.Background(), `SELECT state,outcome_bytes,outcome_sha256,completed_at
		FROM space_lifecycle_operations WHERE operation_id=$1`, f.request.OperationId).
		Scan(&evidence.state, &evidence.bytes, &evidence.hash, &evidence.completedAt))
	return evidence
}

func requireLifecycleOutcomePending(t *testing.T, f lifecycleOutcomeFixture) {
	t.Helper()
	evidence := readLifecycleOutcomeEvidence(t, f)
	require.Equal(t, "SCHEDULE_PENDING", evidence.state)
	require.Nil(t, evidence.bytes)
	require.Nil(t, evidence.hash)
	require.Nil(t, evidence.completedAt)
}

func requireLifecycleOutcomeReplay(t *testing.T, f lifecycleOutcomeFixture, st *SpaceStore) {
	t.Helper()
	response, err := requireLifecycleScheduleOutcome(t, st).ReplayLifecycleScheduleOutcome(context.Background(), f.account, f.actor, 7, f.request)
	require.NoError(t, err)
	require.NotNil(t, response, "nil response is pending/missing, not the saved successful empty response")
	require.True(t, proto.Equal(&spacev1.DeleteSpaceResponse{}, response))
}

// This test uses the existing completion API so its RED phase demonstrates the
// missing database effect independently of the newly proposed replay method.
func TestLifecycleOutcome_CompletionPersistsResponseWithScheduleAndReadyEvent(t *testing.T) {
	f := newLifecycleOutcomeFixture(t, len(lifecycleTestParticipants)-1)
	ctx := context.Background()
	_, _, err := f.store.CompleteLifecycleSchedule(ctx, f.space)
	require.Error(t, err, "nine participant receipts cannot complete the operation")
	requireLifecycleOutcomePending(t, f)
	require.Empty(t, readyLifecycleEvents(t, f.store))
	last := lifecycleTestParticipants[len(lifecycleTestParticipants)-1]
	_, err = requireLifecycleTransitions(t, f.st).RecordLifecycleFenceReceipt(ctx, lifecycleFenceReceiptFixture(t, f.space,
		uuid.MustParse(f.request.OperationId), last, 1, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, lifecycleManifestFixture()))
	require.NoError(t, err)
	completed, event, err := f.store.CompleteLifecycleSchedule(ctx, f.space)
	require.NoError(t, err)
	evidence := readLifecycleOutcomeEvidence(t, f)
	require.Equal(t, "COMPLETED", evidence.state)
	require.NotNil(t, evidence.bytes, "empty protobuf response must be stored as empty bytea, not SQL NULL")
	expectedBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(&spacev1.DeleteSpaceResponse{})
	require.NoError(t, err)
	require.Equal(t, expectedBytes, evidence.bytes)
	expectedHash := lifecycleDomainHash("voice.space.v1.DeleteSpaceResponse", expectedBytes)
	require.Equal(t, expectedHash[:], evidence.hash)
	require.NotNil(t, evidence.completedAt)
	require.True(t, evidence.completedAt.Equal(event.OccurredAt))
	require.True(t, evidence.completedAt.Equal(completed.Snapshot().ScheduledAt))
	require.Equal(t, []spacecore.LifecycleOutboxRecord{event}, readyLifecycleEvents(t, f.store))
	_, repeatedEvent, err := f.store.CompleteLifecycleSchedule(ctx, f.space)
	require.NoError(t, err)
	require.Equal(t, event, repeatedEvent)
	require.Equal(t, evidence, readLifecycleOutcomeEvidence(t, f), "retry cannot reset the outcome or its retention clock")
	var operationJSON string
	require.NoError(t, f.st.Pool.QueryRow(ctx, `SELECT row_to_json(o)::text FROM space_lifecycle_operations o WHERE operation_id=$1`, f.request.OperationId).Scan(&operationJSON))
	require.NotContains(t, operationJSON, f.request.Proof)
	require.NotContains(t, operationJSON, f.request.ConfirmationName)
	require.NotContains(t, operationJSON, hex.EncodeToString([]byte(f.request.Proof)), "bytea encoding must not hide plaintext proof storage")
	require.NotContains(t, operationJSON, hex.EncodeToString([]byte(f.request.ConfirmationName)), "bytea encoding must not hide plaintext confirmation storage")
}

func TestLifecycleOutcome_OperationCompletionFailureRollsBackScheduleAndOutbox(t *testing.T) {
	f := newLifecycleOutcomeFixture(t, len(lifecycleTestParticipants))
	ctx := context.Background()
	before, err := f.store.LoadLifecycle(ctx, f.space)
	require.NoError(t, err)
	_, err = f.st.Pool.Exec(ctx, `CREATE FUNCTION reject_lifecycle_outcome() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN IF NEW.state='COMPLETED' THEN RAISE EXCEPTION 'injected outcome completion failure'; END IF; RETURN NEW; END $$;
		CREATE TRIGGER reject_lifecycle_outcome BEFORE UPDATE ON space_lifecycle_operations FOR EACH ROW EXECUTE FUNCTION reject_lifecycle_outcome()`)
	require.NoError(t, err)
	_, _, err = f.store.CompleteLifecycleSchedule(ctx, f.space)
	require.ErrorContains(t, err, "injected outcome completion failure")
	after, err := f.store.LoadLifecycle(ctx, f.space)
	require.NoError(t, err)
	require.Equal(t, before.Snapshot(), after.Snapshot(), "the aggregate must roll back with operation completion")
	requireLifecycleOutcomePending(t, f)
	require.Empty(t, readyLifecycleEvents(t, f.store))
	var state string
	require.NoError(t, f.st.Pool.QueryRow(ctx, `SELECT state FROM space_lifecycle_outbox WHERE space_id=$1 AND event_type='space.deletion_scheduled'`, f.space).Scan(&state))
	require.Equal(t, "BLOCKED", state)
	_, err = f.st.Pool.Exec(ctx, `DROP TRIGGER reject_lifecycle_outcome ON space_lifecycle_operations`)
	require.NoError(t, err)
	_, _, err = f.store.CompleteLifecycleSchedule(ctx, f.space)
	require.NoError(t, err, "failed transaction must remain resumable")
	require.Equal(t, "COMPLETED", readLifecycleOutcomeEvidence(t, f).state)
}

func TestLifecycleOutcome_MissingAndPendingAreNotSuccessfulResponses(t *testing.T) {
	st := lifecycleStoreFixture(t)
	outcomes := requireLifecycleScheduleOutcome(t, st)
	ctx := context.Background()
	missing := lifecycleScheduleRequest(uuid.New(), uuid.New())
	response, err := outcomes.ReplayLifecycleScheduleOutcome(ctx, uuid.New(), uuid.New(), 7, missing)
	require.NoError(t, err)
	require.Nil(t, response)
	store, _, account, actor, spaceID, request, proofBytes, proofHash := reserveLifecycleOperationFixture(t, st)
	_, err = st.Pool.Exec(ctx, `UPDATE space_lifecycle_operations SET created_at=clock_timestamp()-interval '60 days' WHERE operation_id=$1`, request.OperationId)
	require.NoError(t, err)
	for _, withProof := range []bool{false, true} {
		if withProof {
			_, err = store.RecordLifecycleDeletionProofReceipt(ctx, actor, uuid.MustParse(request.OperationId), proofBytes, proofHash)
			require.NoError(t, err)
		}
		response, err = outcomes.ReplayLifecycleScheduleOutcome(ctx, account, actor, 7, request)
		require.NoError(t, err, "nonterminal operation has no 30-day replay expiry")
		require.Nil(t, response, "proof receipt alone is not a completed schedule")
	}
	requireLifecycleOutcomePending(t, lifecycleOutcomeFixture{st, store, account, actor, spaceID, request})
}

func TestLifecycleOutcome_MissingAuthReceiptCannotCompleteAdmittedOperation(t *testing.T) {
	f := newLifecycleOutcomeFixture(t, len(lifecycleTestParticipants))
	ctx := context.Background()
	_, err := f.st.Pool.Exec(ctx, `UPDATE space_lifecycle_operations SET auth_receipt_bytes=NULL,auth_receipt_sha256=NULL WHERE operation_id=$1`, f.request.OperationId)
	require.NoError(t, err)
	_, _, err = f.store.CompleteLifecycleSchedule(ctx, f.space)
	require.Error(t, err, "an admitted operation without durable validated Auth evidence must fail closed")
	requireLifecycleOutcomePending(t, f)
	require.Empty(t, readyLifecycleEvents(t, f.store))
	var phase string
	require.NoError(t, f.st.Pool.QueryRow(ctx, `SELECT phase FROM space_lifecycle_aggregates WHERE space_id=$1`, f.space).Scan(&phase))
	require.Equal(t, "FREEZE_PENDING", phase)
}

func TestLifecycleOutcome_ReplaySurvivesOwnerNameChangeRestoreAndRemovedSpace(t *testing.T) {
	f := newLifecycleOutcomeFixture(t, len(lifecycleTestParticipants))
	ctx := context.Background()
	_, _, err := f.store.CompleteLifecycleSchedule(ctx, f.space)
	require.NoError(t, err)
	evidence := readLifecycleOutcomeEvidence(t, f)
	_, err = f.st.Pool.Exec(ctx, `UPDATE spaces SET name='new name',owner_profile_id=$2 WHERE id=$1`, f.space, uuid.New())
	require.NoError(t, err)
	requireLifecycleOutcomeReplay(t, f, f.st)
	_, err = f.store.DecideLifecycleRecovery(ctx, f.space, 2)
	require.NoError(t, err)
	requireLifecycleOutcomeReplay(t, f, f.st)
	_, err = f.st.Pool.Exec(ctx, `DELETE FROM space_lifecycle_aggregates WHERE space_id=$1`, f.space)
	require.NoError(t, err)
	_, err = f.st.Pool.Exec(ctx, `DELETE FROM spaces WHERE id=$1`, f.space)
	require.NoError(t, err)
	restarted := &SpaceStore{Pool: r22SecondSpacePool(t, ctx, f.st.Pool, "lifecycle_outcome_restart")}
	requireLifecycleOutcomeReplay(t, f, restarted)
	require.Equal(t, evidence, readLifecycleOutcomeEvidence(t, f))
}

func TestLifecycleOutcome_ReplayRejectsChangedPrincipalAndFullRequest(t *testing.T) {
	f := newLifecycleOutcomeFixture(t, len(lifecycleTestParticipants))
	_, _, err := f.store.CompleteLifecycleSchedule(context.Background(), f.space)
	require.NoError(t, err)
	outcomes := requireLifecycleScheduleOutcome(t, f.st)
	for _, tc := range []struct {
		name           string
		account, actor uuid.UUID
		epoch          int64
		mutate         func(*spacev1.DeleteSpaceRequest)
	}{
		{name: "account", account: uuid.New(), actor: f.actor, epoch: 7},
		{name: "actor", account: f.account, actor: uuid.New(), epoch: 7},
		{name: "epoch", account: f.account, actor: f.actor, epoch: 8},
		{name: "space", account: f.account, actor: f.actor, epoch: 7, mutate: func(r *spacev1.DeleteSpaceRequest) { r.SpaceId = uuid.NewString() }},
		{name: "confirmation", account: f.account, actor: f.actor, epoch: 7, mutate: func(r *spacev1.DeleteSpaceRequest) { r.ConfirmationName += " " }},
		{name: "proof", account: f.account, actor: f.actor, epoch: 7, mutate: func(r *spacev1.DeleteSpaceRequest) { r.Proof += "-changed" }},
		{name: "unknown field", account: f.account, actor: f.actor, epoch: 7, mutate: func(r *spacev1.DeleteSpaceRequest) {
			r.ProtoReflect().SetUnknown(protowire.AppendVarint(protowire.AppendTag(nil, 100, protowire.VarintType), 1))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := proto.Clone(f.request).(*spacev1.DeleteSpaceRequest)
			if tc.mutate != nil {
				tc.mutate(request)
			}
			response, callErr := outcomes.ReplayLifecycleScheduleOutcome(context.Background(), tc.account, tc.actor, tc.epoch, request)
			require.Error(t, callErr)
			require.Nil(t, response)
		})
	}
	requireLifecycleOutcomeReplay(t, f, f.st)
}

func TestLifecycleOutcome_ReplayRejectsCorruptBytesHashAndMalformedRehashedResponse(t *testing.T) {
	f := newLifecycleOutcomeFixture(t, len(lifecycleTestParticipants))
	_, _, err := f.store.CompleteLifecycleSchedule(context.Background(), f.space)
	require.NoError(t, err)
	outcomes := requireLifecycleScheduleOutcome(t, f.st)
	original := readLifecycleOutcomeEvidence(t, f)
	malformed := []byte{0xff}
	malformedHash := lifecycleDomainHash("voice.space.v1.DeleteSpaceResponse", malformed)
	for _, tc := range []struct {
		name        string
		bytes, hash []byte
	}{
		{"changed bytes", malformed, original.hash},
		{"changed hash", original.bytes, make([]byte, sha256.Size)},
		{"malformed with matching hash", malformed, malformedHash[:]},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, updateErr := f.st.Pool.Exec(context.Background(), `UPDATE space_lifecycle_operations SET outcome_bytes=$2,outcome_sha256=$3 WHERE operation_id=$1`, f.request.OperationId, tc.bytes, tc.hash)
			require.NoError(t, updateErr)
			response, callErr := outcomes.ReplayLifecycleScheduleOutcome(context.Background(), f.account, f.actor, 7, f.request)
			require.Error(t, callErr)
			require.Nil(t, response)
		})
	}
	_, err = f.st.Pool.Exec(context.Background(), `UPDATE space_lifecycle_operations SET outcome_bytes=$2,outcome_sha256=$3 WHERE operation_id=$1`, f.request.OperationId, original.bytes, original.hash)
	require.NoError(t, err)
	requireLifecycleOutcomeReplay(t, f, f.st)
}

func TestLifecycleOutcome_ThirtyDayExpiryUsesCompletionAndRetryDoesNotExtendIt(t *testing.T) {
	f := newLifecycleOutcomeFixture(t, len(lifecycleTestParticipants))
	ctx := context.Background()
	_, _, err := f.store.CompleteLifecycleSchedule(ctx, f.space)
	require.NoError(t, err)
	outcomes := requireLifecycleScheduleOutcome(t, f.st)
	_, err = f.st.Pool.Exec(ctx, `UPDATE space_lifecycle_operations SET created_at=clock_timestamp()-interval '60 days',completed_at=clock_timestamp()-interval '29 days' WHERE operation_id=$1`, f.request.OperationId)
	require.NoError(t, err)
	beforeRetry := readLifecycleOutcomeEvidence(t, f)
	requireLifecycleOutcomeReplay(t, f, f.st)
	require.Equal(t, beforeRetry, readLifecycleOutcomeEvidence(t, f), "read must not refresh completed_at")
	// Setting the boundary from PostgreSQL time guarantees the next read occurs
	// at or beyond 30 days without a host-clock assumption or sleep.
	_, err = f.st.Pool.Exec(ctx, `UPDATE space_lifecycle_operations SET completed_at=clock_timestamp()-interval '30 days' WHERE operation_id=$1`, f.request.OperationId)
	require.NoError(t, err)
	expired := readLifecycleOutcomeEvidence(t, f)
	for range 2 {
		response, callErr := outcomes.ReplayLifecycleScheduleOutcome(ctx, f.account, f.actor, 7, f.request)
		require.Error(t, callErr, "expired result must not be mistaken for pending or successful")
		require.Nil(t, response)
		_, callErr = f.store.ReserveLifecycleSchedule(ctx, f.account, f.actor, 7, f.request)
		require.Error(t, callErr, "reservation replay cannot bypass completed operation retention")
		require.Equal(t, expired, readLifecycleOutcomeEvidence(t, f))
	}
}
