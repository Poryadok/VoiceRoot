package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	commonv1 "voice.app/voice/common/v1"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/spacecore"
)

var lifecycleTestParticipants = []commonv1.ParticipantId{
	commonv1.ParticipantId_PARTICIPANT_ID_ROLE,
	commonv1.ParticipantId_PARTICIPANT_ID_CHAT,
	commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING,
	commonv1.ParticipantId_PARTICIPANT_ID_FILE,
	commonv1.ParticipantId_PARTICIPANT_ID_VOICE,
	commonv1.ParticipantId_PARTICIPANT_ID_MATCHMAKING,
	commonv1.ParticipantId_PARTICIPANT_ID_SEARCH,
	commonv1.ParticipantId_PARTICIPANT_ID_SUBSCRIPTION,
	commonv1.ParticipantId_PARTICIPANT_ID_BOT,
	commonv1.ParticipantId_PARTICIPANT_ID_NOTIFICATION,
}

var lifecycleTestRequestPackages = map[commonv1.ParticipantId]string{
	commonv1.ParticipantId_PARTICIPANT_ID_ROLE:         "voice.role.v1",
	commonv1.ParticipantId_PARTICIPANT_ID_CHAT:         "voice.chat.v1",
	commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING:    "voice.messaging.v1",
	commonv1.ParticipantId_PARTICIPANT_ID_FILE:         "voice.file.v1",
	commonv1.ParticipantId_PARTICIPANT_ID_VOICE:        "voice.calls.v1",
	commonv1.ParticipantId_PARTICIPANT_ID_MATCHMAKING:  "voice.matchmaking.v1",
	commonv1.ParticipantId_PARTICIPANT_ID_SEARCH:       "voice.search.v1",
	commonv1.ParticipantId_PARTICIPANT_ID_SUBSCRIPTION: "voice.subscription.v1",
	commonv1.ParticipantId_PARTICIPANT_ID_BOT:          "voice.bot.v1",
	commonv1.ParticipantId_PARTICIPANT_ID_NOTIFICATION: "voice.notification.v1",
}

type durableLifecycleStore interface {
	ReserveLifecycleSchedule(context.Context, uuid.UUID, uuid.UUID, int64, *spacev1.DeleteSpaceRequest) (*spacecore.LifecycleAggregate, error)
	RecordLifecycleDeletionProofReceipt(context.Context, uuid.UUID, uuid.UUID, []byte, [sha256.Size]byte) (*spacecore.LifecycleAggregate, error)
	PersistLifecycle(context.Context, *spacecore.LifecycleAggregate) error
	LoadLifecycle(context.Context, uuid.UUID) (*spacecore.LifecycleAggregate, error)
	CompleteLifecycleSchedule(context.Context, uuid.UUID) (*spacecore.LifecycleAggregate, spacecore.LifecycleOutboxRecord, error)
	DecideLifecycleRecovery(context.Context, uuid.UUID, uint64) (*spacecore.LifecycleAggregate, error)
	CompleteLifecyclePurge(context.Context, *spacecore.LifecycleAggregate, []byte, []byte, string) error
	ReadReadyLifecycleOutbox(context.Context, int, func(spacecore.LifecycleOutboxRecord) error) error
}

func requireDurableLifecycleStore(t *testing.T, st *SpaceStore) durableLifecycleStore {
	t.Helper()
	store, ok := any(st).(durableLifecycleStore)
	require.True(t, ok, "SpaceStore must expose the typed durable lifecycle persistence contract")
	return store
}

func TestLifecyclePersistence_TypedStoreSurfaceIsRequired(t *testing.T) {
	_, ok := any(&SpaceStore{}).(durableLifecycleStore)
	require.True(t, ok, "missing typed lifecycle reservation/receipt/persistence/clock/purge/outbox store contract")
}

func lifecycleMigrationPath(t *testing.T, direction string) string {
	t.Helper()
	pattern := filepath.Join(repoRoot(t), "src", "backend", "migrations", "space_db", "000013_*.sql")
	paths, err := filepath.Glob(pattern)
	require.NoError(t, err)
	filtered := paths[:0]
	for _, path := range paths {
		if filepath.Ext(strings.TrimSuffix(path, ".sql")) == "."+direction {
			filtered = append(filtered, path)
		}
	}
	require.Len(t, filtered, 1, "R23 owns exactly one additive Space migration 000013 %s file", direction)
	return filtered[0]
}

func lifecycleStoreFixture(t *testing.T) *SpaceStore {
	t.Helper()
	if testing.Short() {
		t.Skip("requires PostgreSQL lifecycle migration")
	}
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	raw, err := os.ReadFile(lifecycleMigrationPath(t, "up"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(raw))
	require.NoError(t, err)
	return &SpaceStore{Pool: pool}
}

func lifecycleManifestFixture() *commonv1.ManifestBinding {
	return &commonv1.ManifestBinding{
		ManifestId:     "33333333-3333-4333-8333-333333333333",
		ManifestSha256: bytes.Repeat([]byte{0x5a}, sha256.Size),
		ItemCount:      23,
	}
}

func lifecycleAggregateFixture(t *testing.T, st *SpaceStore) (*spacecore.LifecycleAggregate, uuid.UUID, uuid.UUID) {
	t.Helper()
	space, err := st.CreateSpace(context.Background(), uuid.New(), "R23 durable lifecycle", "", "private")
	require.NoError(t, err)
	operationID := uuid.New()
	aggregate, err := spacecore.NewLifecycleAggregate(space.ID.String(), operationID.String())
	require.NoError(t, err)
	require.NoError(t, aggregate.BeginSchedule(1))
	require.NoError(t, aggregate.BeginFreeze(lifecycleManifestFixture()))
	return aggregate, space.ID, operationID
}

func lifecycleDomainHash(fqn string, deterministicBytes []byte) [sha256.Size]byte {
	payload := make([]byte, 0, len(fqn)+1+len(deterministicBytes))
	payload = append(payload, fqn...)
	payload = append(payload, 0)
	payload = append(payload, deterministicBytes...)
	return sha256.Sum256(payload)
}

func lifecycleFenceRequestEvidence(t *testing.T, spaceID, operationID uuid.UUID, participant commonv1.ParticipantId, generation uint64, state commonv1.LifecycleFenceState, manifest *commonv1.ManifestBinding) ([]byte, [sha256.Size]byte) {
	t.Helper()
	request := &commonv1.SpaceLifecycleFenceRequest{
		ProtocolVersion:     1,
		SpaceId:             spaceID.String(),
		DeletionOperationId: operationID.String(),
		Generation:          generation,
		DesiredState:        state,
		Manifest:            proto.Clone(manifest).(*commonv1.ManifestBinding),
	}
	inner, err := proto.MarshalOptions{Deterministic: true}.Marshal(request)
	require.NoError(t, err)
	wrapper := protowire.AppendTag(nil, 1, protowire.BytesType)
	wrapper = protowire.AppendBytes(wrapper, inner)
	fqn := lifecycleTestRequestPackages[participant] + ".ApplySpaceLifecycleFenceRequest"
	return wrapper, lifecycleDomainHash(fqn, wrapper)
}

func lifecycleFenceReceiptFixture(t *testing.T, spaceID, operationID uuid.UUID, participant commonv1.ParticipantId, generation uint64, state commonv1.LifecycleFenceState, manifest *commonv1.ManifestBinding) *commonv1.SpaceLifecycleFenceReceipt {
	t.Helper()
	_, requestHash := lifecycleFenceRequestEvidence(t, spaceID, operationID, participant, generation, state, manifest)
	return &commonv1.SpaceLifecycleFenceReceipt{
		ProtocolVersion:     1,
		ReceiptId:           fmt.Sprintf("receipt-%d-%d-%d", participant, state, generation),
		SpaceId:             spaceID.String(),
		DeletionOperationId: operationID.String(),
		Generation:          generation,
		ParticipantId:       participant,
		AppliedState:        state,
		RequestSha256:       append([]byte(nil), requestHash[:]...),
		ManifestSha256:      append([]byte(nil), manifest.GetManifestSha256()...),
		AppliedAt:           timestamppb.New(time.Date(2026, time.September, 12, 9, 0, 0, int(participant), time.UTC)),
	}
}

func recordLifecycleFenceBarrier(t *testing.T, aggregate *spacecore.LifecycleAggregate, spaceID, operationID uuid.UUID, generation uint64, state commonv1.LifecycleFenceState) {
	t.Helper()
	for _, participant := range lifecycleTestParticipants {
		receipt := lifecycleFenceReceiptFixture(t, spaceID, operationID, participant, generation, state, lifecycleManifestFixture())
		require.NoError(t, aggregate.RecordFenceReceipt(receipt))
	}
}

func readyLifecycleEvents(t *testing.T, store durableLifecycleStore) []spacecore.LifecycleOutboxRecord {
	t.Helper()
	var events []spacecore.LifecycleOutboxRecord
	err := store.ReadReadyLifecycleOutbox(context.Background(), 100, func(event spacecore.LifecycleOutboxRecord) error {
		events = append(events, event)
		return nil
	})
	require.NoError(t, err)
	return events
}

func completeStoredSchedule(t *testing.T, st *SpaceStore) (durableLifecycleStore, *spacecore.LifecycleAggregate, uuid.UUID, uuid.UUID, spacecore.LifecycleOutboxRecord) {
	t.Helper()
	store := requireDurableLifecycleStore(t, st)
	aggregate, spaceID, operationID := lifecycleAggregateFixture(t, st)
	recordLifecycleFenceBarrier(t, aggregate, spaceID, operationID, 1, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN)
	require.NoError(t, store.PersistLifecycle(context.Background(), aggregate))
	completed, event, err := store.CompleteLifecycleSchedule(context.Background(), spaceID)
	require.NoError(t, err)
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_SCHEDULED, completed.Phase())
	require.Equal(t, event.OccurredAt.Add(7*24*time.Hour), completed.PurgeAfter())
	return store, completed, spaceID, operationID, event
}

func lifecycleScheduleRequest(spaceID, operationID uuid.UUID) *spacev1.DeleteSpaceRequest {
	return &spacev1.DeleteSpaceRequest{
		SpaceId:          spaceID.String(),
		ConfirmationName: "R23 durable lifecycle",
		Proof:            "opaque-proof-must-never-be-persisted",
		OperationId:      operationID.String(),
	}
}

func lifecycleScheduleRequestEvidence(t *testing.T, request *spacev1.DeleteSpaceRequest) ([sha256.Size]byte, [sha256.Size]byte, [sha256.Size]byte) {
	t.Helper()
	requestBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(request)
	require.NoError(t, err)
	requestHash := lifecycleDomainHash("voice.space.v1.DeleteSpaceRequest", requestBytes)
	confirmationHash := sha256.Sum256([]byte(request.GetConfirmationName()))
	proofDigest := sha256.Sum256([]byte(request.GetProof()))
	return requestHash, confirmationHash, proofDigest
}

func lifecycleAuthProofBindingHash(accountID, actorProfileID, spaceID, operationID uuid.UUID, sessionEpoch int64, confirmationHash, proofDigest [sha256.Size]byte) [sha256.Size]byte {
	return lifecycleAuthProofBindingHashForFactors(accountID, actorProfileID, spaceID, operationID, sessionEpoch, confirmationHash, proofDigest, []uint64{1})
}

func lifecycleAuthProofBindingHashForFactors(accountID, actorProfileID, spaceID, operationID uuid.UUID, sessionEpoch int64, confirmationHash, proofDigest [sha256.Size]byte, factors []uint64) [sha256.Size]byte {
	binding := protowire.AppendTag(nil, 1, protowire.VarintType)
	binding = protowire.AppendVarint(binding, 1)
	binding = protowire.AppendTag(binding, 2, protowire.BytesType)
	binding = protowire.AppendString(binding, accountID.String())
	binding = protowire.AppendTag(binding, 3, protowire.BytesType)
	binding = protowire.AppendString(binding, actorProfileID.String())
	binding = protowire.AppendTag(binding, 4, protowire.VarintType)
	binding = protowire.AppendVarint(binding, uint64(sessionEpoch))
	binding = protowire.AppendTag(binding, 5, protowire.BytesType)
	binding = protowire.AppendString(binding, spaceID.String())
	binding = protowire.AppendTag(binding, 6, protowire.BytesType)
	binding = protowire.AppendString(binding, operationID.String())
	binding = protowire.AppendTag(binding, 7, protowire.BytesType)
	binding = protowire.AppendBytes(binding, confirmationHash[:])
	binding = protowire.AppendTag(binding, 8, protowire.BytesType)
	binding = protowire.AppendBytes(binding, proofDigest[:])
	binding = protowire.AppendTag(binding, 9, protowire.BytesType)
	binding = protowire.AppendBytes(binding, lifecyclePackedVerifiedFactors(factors))
	binding = protowire.AppendTag(binding, 10, protowire.VarintType)
	binding = protowire.AppendVarint(binding, 1) // PROOF_PURPOSE_SPACE_DELETE
	return lifecycleDomainHash("voice.auth.v1.SpaceDeletionProofBinding", binding)
}

// The Space module intentionally has no Auth generated-code dependency. These
// literal typed-wire builders freeze deterministic Auth messages without adding
// production module coupling merely for a RED fixture.
func lifecycleAuthReceiptEvidence(t *testing.T, accountID, actorProfileID, spaceID, operationID uuid.UUID, sessionEpoch int64, confirmationHash, proofDigest [sha256.Size]byte) ([]byte, [sha256.Size]byte) {
	return lifecycleAuthReceiptEvidenceForFactors(t, accountID, actorProfileID, spaceID, operationID, sessionEpoch, confirmationHash, proofDigest, []uint64{1}, []uint64{1})
}

func lifecycleAuthReceiptEvidenceForFactors(t *testing.T, accountID, actorProfileID, spaceID, operationID uuid.UUID, sessionEpoch int64, confirmationHash, proofDigest [sha256.Size]byte, bindingFactors, receiptFactors []uint64) ([]byte, [sha256.Size]byte) {
	t.Helper()
	bindingHash := lifecycleAuthProofBindingHashForFactors(accountID, actorProfileID, spaceID, operationID, sessionEpoch, confirmationHash, proofDigest, bindingFactors)
	receipt := protowire.AppendTag(nil, 1, protowire.VarintType)
	receipt = protowire.AppendVarint(receipt, 1)
	receipt = protowire.AppendTag(receipt, 2, protowire.BytesType)
	receipt = protowire.AppendString(receipt, uuid.NewString())
	receipt = protowire.AppendTag(receipt, 3, protowire.BytesType)
	receipt = protowire.AppendString(receipt, operationID.String())
	receipt = protowire.AppendTag(receipt, 4, protowire.BytesType)
	receipt = protowire.AppendBytes(receipt, bindingHash[:])
	receipt = protowire.AppendTag(receipt, 5, protowire.BytesType)
	receipt = protowire.AppendBytes(receipt, confirmationHash[:])
	timestampBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(timestamppb.New(time.Date(2026, time.September, 12, 10, 0, 0, 0, time.UTC)))
	require.NoError(t, err)
	receipt = protowire.AppendTag(receipt, 6, protowire.BytesType)
	receipt = protowire.AppendBytes(receipt, timestampBytes)
	receipt = protowire.AppendTag(receipt, 7, protowire.BytesType)
	receipt = protowire.AppendBytes(receipt, lifecyclePackedVerifiedFactors(receiptFactors))
	wrapper := protowire.AppendTag(nil, 1, protowire.BytesType)
	wrapper = protowire.AppendBytes(wrapper, receipt)
	return wrapper, lifecycleDomainHash("voice.auth.v1.ConsumeSpaceDeletionProofResponse", wrapper)
}

func lifecyclePackedVerifiedFactors(factors []uint64) []byte {
	var packed []byte
	for _, factor := range factors {
		packed = protowire.AppendVarint(packed, factor)
	}
	return packed
}

func reserveLifecycleOperationFixture(t *testing.T, st *SpaceStore) (durableLifecycleStore, *spacecore.LifecycleAggregate, uuid.UUID, uuid.UUID, uuid.UUID, *spacev1.DeleteSpaceRequest, []byte, [sha256.Size]byte) {
	t.Helper()
	store := requireDurableLifecycleStore(t, st)
	accountID, actorProfileID := uuid.New(), uuid.New()
	space, err := st.CreateSpace(context.Background(), actorProfileID, "R23 durable lifecycle", "", "private")
	require.NoError(t, err)
	operationID := uuid.New()
	request := lifecycleScheduleRequest(space.ID, operationID)
	aggregate, err := store.ReserveLifecycleSchedule(context.Background(), accountID, actorProfileID, 7, request)
	require.NoError(t, err)
	_, confirmationHash, proofDigest := lifecycleScheduleRequestEvidence(t, request)
	receiptBytes, receiptHash := lifecycleAuthReceiptEvidence(t, accountID, actorProfileID, space.ID, operationID, 7, confirmationHash, proofDigest)
	return store, aggregate, accountID, actorProfileID, space.ID, request, receiptBytes, receiptHash
}

func TestLifecyclePersistence_PreAuthOperationAndReceiptSurviveRestartAndRejectTamper(t *testing.T) {
	st := lifecycleStoreFixture(t)
	store, aggregate, accountID, actorProfileID, spaceID, request, receiptBytes, receiptHash := reserveLifecycleOperationFixture(t, st)
	operationID := uuid.MustParse(request.GetOperationId())
	requestHash, confirmationHash, proofDigest := lifecycleScheduleRequestEvidence(t, request)
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_SCHEDULE_PENDING, aggregate.Phase())
	require.Equal(t, uint64(1), aggregate.Generation())

	var gotAccountID, gotActorID uuid.UUID
	var gotEpoch int64
	var gotRequestHash, gotConfirmationHash, gotProofDigest, gotReceiptBytes, gotReceiptHash []byte
	var state string
	require.NoError(t, st.Pool.QueryRow(context.Background(), `SELECT account_id,actor_profile_id,session_epoch,request_sha256,confirmation_name_sha256,proof_digest_sha256,auth_receipt_bytes,auth_receipt_sha256,upper(state)
		FROM space_lifecycle_operations WHERE actor_profile_id=$1 AND operation_id=$2`, actorProfileID, operationID).Scan(
		&gotAccountID, &gotActorID, &gotEpoch, &gotRequestHash, &gotConfirmationHash, &gotProofDigest, &gotReceiptBytes, &gotReceiptHash, &state))
	require.Equal(t, accountID, gotAccountID)
	require.Equal(t, actorProfileID, gotActorID)
	require.Equal(t, int64(7), gotEpoch)
	require.Equal(t, requestHash[:], gotRequestHash)
	require.Equal(t, confirmationHash[:], gotConfirmationHash)
	require.Equal(t, proofDigest[:], gotProofDigest)
	require.Nil(t, gotReceiptBytes)
	require.Nil(t, gotReceiptHash)
	require.Equal(t, "SCHEDULE_PENDING", state)

	aggregate, err := store.RecordLifecycleDeletionProofReceipt(context.Background(), actorProfileID, operationID, receiptBytes, receiptHash)
	require.NoError(t, err)
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_SCHEDULE_PENDING, aggregate.Phase())
	require.NoError(t, st.Pool.QueryRow(context.Background(), `SELECT auth_receipt_bytes,auth_receipt_sha256 FROM space_lifecycle_operations WHERE actor_profile_id=$1 AND operation_id=$2`, actorProfileID, operationID).Scan(&gotReceiptBytes, &gotReceiptHash))
	require.Equal(t, receiptBytes, gotReceiptBytes)
	require.Equal(t, receiptHash[:], gotReceiptHash)

	restartedPool, err := pgxpool.NewWithConfig(context.Background(), st.Pool.Config())
	require.NoError(t, err)
	defer restartedPool.Close()
	restarted := requireDurableLifecycleStore(t, &SpaceStore{Pool: restartedPool})
	loaded, err := restarted.LoadLifecycle(context.Background(), spaceID)
	require.NoError(t, err)
	require.Equal(t, aggregate.Phase(), loaded.Phase())
	require.Equal(t, aggregate.Generation(), loaded.Generation())
	bindingHash := lifecycleAuthProofBindingHash(accountID, actorProfileID, spaceID, operationID, 7, confirmationHash, proofDigest)
	badBindingHash := bytes.Repeat([]byte{0x55}, sha256.Size)
	require.Equal(t, 1, bytes.Count(receiptBytes, bindingHash[:]))
	badBindingReceipt := bytes.Replace(receiptBytes, bindingHash[:], badBindingHash, 1)
	badBindingReceiptHash := lifecycleDomainHash("voice.auth.v1.ConsumeSpaceDeletionProofResponse", badBindingReceipt)

	for _, mutation := range []struct {
		name, sql string
		args      []any
	}{
		{"request hash", `UPDATE space_lifecycle_operations SET request_sha256=decode(repeat('11',32),'hex') WHERE operation_id=$1`, nil},
		{"confirmation hash", `UPDATE space_lifecycle_operations SET confirmation_name_sha256=decode(repeat('22',32),'hex') WHERE operation_id=$1`, nil},
		{"proof digest", `UPDATE space_lifecycle_operations SET proof_digest_sha256=decode(repeat('33',32),'hex') WHERE operation_id=$1`, nil},
		{"receipt bytes", `UPDATE space_lifecycle_operations SET auth_receipt_bytes=auth_receipt_bytes || decode('00','hex') WHERE operation_id=$1`, nil},
		{"receipt hash", `UPDATE space_lifecycle_operations SET auth_receipt_sha256=decode(repeat('44',32),'hex') WHERE operation_id=$1`, nil},
		{"binding hash", `UPDATE space_lifecycle_operations SET auth_receipt_bytes=$2,auth_receipt_sha256=$3 WHERE operation_id=$1`, []any{badBindingReceipt, badBindingReceiptHash[:]}},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			tx, txErr := st.Pool.Begin(context.Background())
			require.NoError(t, txErr)
			defer func() { _ = tx.Rollback(context.Background()) }()
			_, txErr = tx.Exec(context.Background(), `SET LOCAL session_replication_role='replica'`)
			require.NoError(t, txErr)
			_, txErr = tx.Exec(context.Background(), mutation.sql, append([]any{operationID}, mutation.args...)...)
			require.NoError(t, txErr)
			corruptStore := requireDurableLifecycleStore(t, &SpaceStore{Pool: st.Pool, tx: tx})
			corrupt, loadErr := corruptStore.LoadLifecycle(context.Background(), spaceID)
			require.Error(t, loadErr)
			require.Nil(t, corrupt)
		})
	}
}

func TestLifecyclePersistence_AuthReceiptFactorsAreCanonicalAndBound(t *testing.T) {
	st := lifecycleStoreFixture(t)
	store := requireDurableLifecycleStore(t, st)

	reserve := func(t *testing.T) (uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID, int64, [sha256.Size]byte, [sha256.Size]byte) {
		t.Helper()
		accountID, actorProfileID := uuid.New(), uuid.New()
		space, err := st.CreateSpace(context.Background(), actorProfileID, "R23 durable lifecycle", "", "private")
		require.NoError(t, err)
		operationID := uuid.New()
		request := lifecycleScheduleRequest(space.ID, operationID)
		_, err = store.ReserveLifecycleSchedule(context.Background(), accountID, actorProfileID, 7, request)
		require.NoError(t, err)
		_, confirmationHash, proofDigest := lifecycleScheduleRequestEvidence(t, request)
		return accountID, actorProfileID, space.ID, operationID, 7, confirmationHash, proofDigest
	}

	for _, test := range []struct {
		name    string
		factors []uint64
	}{
		{name: "password", factors: []uint64{1}},
		{name: "password and TOTP", factors: []uint64{1, 2}},
		{name: "password and backup code", factors: []uint64{1, 3}},
	} {
		t.Run(test.name, func(t *testing.T) {
			accountID, actorProfileID, spaceID, operationID, epoch, confirmationHash, proofDigest := reserve(t)
			receiptBytes, receiptHash := lifecycleAuthReceiptEvidenceForFactors(t, accountID, actorProfileID, spaceID, operationID, epoch, confirmationHash, proofDigest, test.factors, test.factors)
			_, err := store.RecordLifecycleDeletionProofReceipt(context.Background(), actorProfileID, operationID, receiptBytes, receiptHash)
			require.NoError(t, err)
			loaded, err := store.LoadLifecycle(context.Background(), spaceID)
			require.NoError(t, err)
			require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_SCHEDULE_PENDING, loaded.Phase())
		})
	}

	for _, test := range []struct {
		name           string
		bindingFactors []uint64
		receiptFactors []uint64
	}{
		{name: "factor binding mismatch", bindingFactors: []uint64{1}, receiptFactors: []uint64{1, 2}},
		{name: "zero factor", bindingFactors: []uint64{0}, receiptFactors: []uint64{0}},
		{name: "unknown factor", bindingFactors: []uint64{1, 4}, receiptFactors: []uint64{1, 4}},
		{name: "duplicate factor", bindingFactors: []uint64{1, 1}, receiptFactors: []uint64{1, 1}},
		{name: "noncanonical order", bindingFactors: []uint64{2, 1}, receiptFactors: []uint64{2, 1}},
		{name: "TOTP without password", bindingFactors: []uint64{2}, receiptFactors: []uint64{2}},
		{name: "backup code without password", bindingFactors: []uint64{3}, receiptFactors: []uint64{3}},
		{name: "two second factors", bindingFactors: []uint64{1, 2, 3}, receiptFactors: []uint64{1, 2, 3}},
	} {
		t.Run(test.name, func(t *testing.T) {
			accountID, actorProfileID, spaceID, operationID, epoch, confirmationHash, proofDigest := reserve(t)
			receiptBytes, receiptHash := lifecycleAuthReceiptEvidenceForFactors(t, accountID, actorProfileID, spaceID, operationID, epoch, confirmationHash, proofDigest, test.bindingFactors, test.receiptFactors)
			aggregate, err := store.RecordLifecycleDeletionProofReceipt(context.Background(), actorProfileID, operationID, receiptBytes, receiptHash)
			require.Error(t, err)
			require.Nil(t, aggregate)
		})
	}
}

func TestLifecyclePersistence_ReloadsExactManifestAndTenParticipantEvidence(t *testing.T) {
	st := lifecycleStoreFixture(t)
	store := requireDurableLifecycleStore(t, st)
	aggregate, spaceID, operationID := lifecycleAggregateFixture(t, st)
	recordLifecycleFenceBarrier(t, aggregate, spaceID, operationID, 1, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN)
	require.NoError(t, store.PersistLifecycle(context.Background(), aggregate))

	rows, err := st.Pool.Query(context.Background(), `SELECT participant_id,request_bytes,request_sha256,receipt_bytes,receipt_sha256,manifest_id,manifest_sha256,manifest_item_count
		FROM space_lifecycle_participants WHERE space_id=$1 AND deletion_operation_id=$2 AND generation=1 ORDER BY participant_id`, spaceID, operationID)
	require.NoError(t, err)
	defer rows.Close()
	seen := 0
	for rows.Next() {
		var participant int32
		var requestBytes, requestHash, receiptBytes, receiptHash, manifestHash []byte
		var manifestID uuid.UUID
		var manifestCount int64
		require.NoError(t, rows.Scan(&participant, &requestBytes, &requestHash, &receiptBytes, &receiptHash, &manifestID, &manifestHash, &manifestCount))
		participantID := commonv1.ParticipantId(participant)
		expectedRequest, expectedRequestHash := lifecycleFenceRequestEvidence(t, spaceID, operationID, participantID, 1, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, lifecycleManifestFixture())
		expectedReceipt := lifecycleFenceReceiptFixture(t, spaceID, operationID, participantID, 1, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, lifecycleManifestFixture())
		expectedReceiptBytes, marshalErr := proto.MarshalOptions{Deterministic: true}.Marshal(expectedReceipt)
		require.NoError(t, marshalErr)
		expectedReceiptHash := lifecycleDomainHash("voice.common.v1.SpaceLifecycleFenceReceipt", expectedReceiptBytes)
		require.Equal(t, expectedRequest, requestBytes)
		require.Equal(t, expectedRequestHash[:], requestHash)
		require.Equal(t, expectedReceiptBytes, receiptBytes)
		require.Equal(t, expectedReceiptHash[:], receiptHash)
		require.Equal(t, uuid.MustParse(lifecycleManifestFixture().GetManifestId()), manifestID)
		require.Equal(t, lifecycleManifestFixture().GetManifestSha256(), manifestHash)
		require.Equal(t, int64(lifecycleManifestFixture().GetItemCount()), manifestCount)
		seen++
	}
	require.NoError(t, rows.Err())
	require.Equal(t, len(lifecycleTestParticipants), seen)

	restartedPool, err := pgxpool.NewWithConfig(context.Background(), st.Pool.Config())
	require.NoError(t, err)
	defer restartedPool.Close()
	restarted := requireDurableLifecycleStore(t, &SpaceStore{Pool: restartedPool})
	loaded, err := restarted.LoadLifecycle(context.Background(), spaceID)
	require.NoError(t, err)
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_FREEZE_PENDING, loaded.Phase())
	require.Equal(t, uint64(1), loaded.Generation())
	_, err = loaded.CompleteSchedule(time.Now().UTC())
	require.NoError(t, err, "hydration must restore every accepted receipt, not only the aggregate phase")
}

func TestLifecyclePersistence_ReloadRejectsExactEvidenceOrManifestTamper(t *testing.T) {
	for _, mutation := range []struct {
		name string
		sql  string
	}{
		{name: "request bytes", sql: `UPDATE space_lifecycle_participants SET request_bytes=request_bytes || decode('00','hex') WHERE participant_id=2`},
		{name: "request hash", sql: `UPDATE space_lifecycle_participants SET request_sha256=decode(repeat('5b',32),'hex') WHERE participant_id=2`},
		{name: "receipt bytes", sql: `UPDATE space_lifecycle_participants SET receipt_bytes=receipt_bytes || decode('00','hex') WHERE participant_id=2`},
		{name: "receipt hash", sql: `UPDATE space_lifecycle_participants SET receipt_sha256=decode(repeat('6c',32),'hex') WHERE participant_id=2`},
		{name: "manifest hash", sql: `UPDATE space_lifecycle_participants SET manifest_sha256=decode(repeat('7d',32),'hex') WHERE participant_id=2`},
		{name: "manifest ID", sql: `UPDATE space_lifecycle_participants SET manifest_id='44444444-4444-4444-8444-444444444444' WHERE participant_id=2`},
		{name: "manifest binding", sql: `UPDATE space_lifecycle_participants SET manifest_item_count=manifest_item_count+1 WHERE participant_id=2`},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			st := lifecycleStoreFixture(t)
			store := requireDurableLifecycleStore(t, st)
			aggregate, spaceID, operationID := lifecycleAggregateFixture(t, st)
			recordLifecycleFenceBarrier(t, aggregate, spaceID, operationID, 1, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN)
			require.NoError(t, store.PersistLifecycle(context.Background(), aggregate))
			_, err := st.Pool.Exec(context.Background(), `SET session_replication_role='replica'; `+mutation.sql+`; SET session_replication_role='origin'`)
			require.NoError(t, err, "fixture must model durable corruption even when normal immutability triggers exist")
			loaded, loadErr := store.LoadLifecycle(context.Background(), spaceID)
			require.Error(t, loadErr)
			require.Nil(t, loaded)
		})
	}
}

func TestLifecyclePersistence_SchedulePhaseAndReadyOutboxCommitAtomicallyWithStableReplay(t *testing.T) {
	st := lifecycleStoreFixture(t)
	store := requireDurableLifecycleStore(t, st)
	aggregate, spaceID, operationID := lifecycleAggregateFixture(t, st)
	recordLifecycleFenceBarrier(t, aggregate, spaceID, operationID, 1, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN)
	require.NoError(t, store.PersistLifecycle(context.Background(), aggregate))

	var blockedID uuid.UUID
	var blockedState string
	var blockedOccurredAt *time.Time
	require.NoError(t, st.Pool.QueryRow(context.Background(), `SELECT event_id,upper(state),occurred_at FROM space_lifecycle_outbox WHERE space_id=$1 AND event_type='space.deletion_scheduled'`, spaceID).Scan(&blockedID, &blockedState, &blockedOccurredAt))
	require.Equal(t, "BLOCKED", blockedState)
	require.Nil(t, blockedOccurredAt)
	require.Empty(t, readyLifecycleEvents(t, store))

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	gate := int64(73023001)
	blocker, err := st.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback(context.Background()) }()
	_, err = blocker.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, gate)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, fmt.Sprintf(`CREATE FUNCTION r23_pause_schedule() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF upper(NEW.phase)='SCHEDULED' THEN PERFORM pg_advisory_xact_lock(%d); END IF; RETURN NEW; END $$; CREATE TRIGGER r23_pause_schedule BEFORE UPDATE ON space_lifecycle_aggregates FOR EACH ROW EXECUTE FUNCTION r23_pause_schedule()`, gate))
	require.NoError(t, err)

	workerPool := r22SecondSpacePool(t, ctx, st.Pool, "r23_atomic_schedule")
	worker := requireDurableLifecycleStore(t, &SpaceStore{Pool: workerPool})
	type completion struct {
		aggregate *spacecore.LifecycleAggregate
		event     spacecore.LifecycleOutboxRecord
		err       error
	}
	result := make(chan completion, 1)
	go func() {
		completed, event, callErr := worker.CompleteLifecycleSchedule(ctx, spaceID)
		result <- completion{aggregate: completed, event: event, err: callErr}
	}()
	r22WaitForSpaceLock(t, ctx, st.Pool, "r23_atomic_schedule")
	before, err := store.LoadLifecycle(ctx, spaceID)
	require.NoError(t, err)
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_FREEZE_PENDING, before.Phase())
	require.Empty(t, readyLifecycleEvents(t, store))
	require.NoError(t, blocker.Commit(ctx))
	completed := <-result
	require.NoError(t, completed.err)
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_SCHEDULED, completed.aggregate.Phase())
	require.Equal(t, blockedID.String(), completed.event.EventID)
	require.False(t, completed.event.OccurredAt.IsZero())

	ready := readyLifecycleEvents(t, store)
	require.Len(t, ready, 1)
	require.Equal(t, completed.event, ready[0])
	restartedPool, err := pgxpool.NewWithConfig(context.Background(), st.Pool.Config())
	require.NoError(t, err)
	defer restartedPool.Close()
	restarted := requireDurableLifecycleStore(t, &SpaceStore{Pool: restartedPool})
	_, replay, err := restarted.CompleteLifecycleSchedule(context.Background(), spaceID)
	require.NoError(t, err)
	require.Equal(t, completed.event, replay, "restart/replay must not regenerate event_id or occurred_at")
	require.Equal(t, operationID.String(), replay.DeletionOperationID)
}

func TestLifecyclePersistence_RejectsStaleExpectedGenerationAndStateCASWrite(t *testing.T) {
	st := lifecycleStoreFixture(t)
	store := requireDurableLifecycleStore(t, st)
	aggregate, spaceID, operationID := lifecycleAggregateFixture(t, st)
	recordLifecycleFenceBarrier(t, aggregate, spaceID, operationID, 1, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN)
	require.NoError(t, store.PersistLifecycle(context.Background(), aggregate))
	stale, err := store.LoadLifecycle(context.Background(), spaceID)
	require.NoError(t, err)
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_FREEZE_PENDING, stale.Phase())
	require.Equal(t, uint64(1), stale.Generation())

	_, committedEvent, err := store.CompleteLifecycleSchedule(context.Background(), spaceID)
	require.NoError(t, err)
	err = store.PersistLifecycle(context.Background(), stale)
	require.Error(t, err, "a stale expected phase/generation snapshot must not overwrite the committed terminal barrier")
	current, err := store.LoadLifecycle(context.Background(), spaceID)
	require.NoError(t, err)
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_SCHEDULED, current.Phase())
	require.Equal(t, uint64(1), current.Generation())
	require.Equal(t, []spacecore.LifecycleOutboxRecord{committedEvent}, readyLifecycleEvents(t, store))
}

func TestLifecyclePersistence_RestoreAndDeleteOutboxStayBlockedUntilTheirTerminalBarriers(t *testing.T) {
	t.Run("restore", func(t *testing.T) {
		st := lifecycleStoreFixture(t)
		store, _, spaceID, operationID, scheduleEvent := completeStoredSchedule(t, st)
		decided, err := store.DecideLifecycleRecovery(context.Background(), spaceID, 2)
		require.NoError(t, err)
		require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_RESTORE_DECIDED, decided.Phase())
		require.Equal(t, []string{"space.deletion_scheduled"}, lifecycleEventTypes(readyLifecycleEvents(t, store)))
		recordLifecycleFenceBarrier(t, decided, spaceID, operationID, 2, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE)
		require.NoError(t, store.PersistLifecycle(context.Background(), decided))
		require.Equal(t, []string{"space.deletion_scheduled"}, lifecycleEventTypes(readyLifecycleEvents(t, store)))
		restoredEvent, err := decided.CompleteRestore(scheduleEvent.OccurredAt.Add(time.Hour))
		require.NoError(t, err)
		require.NoError(t, store.PersistLifecycle(context.Background(), decided))
		require.Equal(t, []string{"space.deletion_scheduled", "space.restored"}, lifecycleEventTypes(readyLifecycleEvents(t, store)))
		loaded, err := store.LoadLifecycle(context.Background(), spaceID)
		require.NoError(t, err)
		replay, err := loaded.CompleteRestore(restoredEvent.OccurredAt.Add(time.Hour))
		require.NoError(t, err)
		require.Equal(t, restoredEvent, replay)
	})

	t.Run("delete", func(t *testing.T) {
		st := lifecycleStoreFixture(t)
		store, _, spaceID, operationID, _ := completeStoredSchedule(t, st)
		_, err := st.Pool.Exec(context.Background(), `UPDATE space_lifecycle_aggregates SET purge_after=clock_timestamp() WHERE space_id=$1`, spaceID)
		require.NoError(t, err)
		decided, err := store.DecideLifecycleRecovery(context.Background(), spaceID, 2)
		require.NoError(t, err)
		require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGE_DECIDED, decided.Phase())
		require.Equal(t, []string{"space.deletion_scheduled"}, lifecycleEventTypes(readyLifecycleEvents(t, store)))
		recordLifecycleFenceBarrier(t, decided, spaceID, operationID, 2, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED)
		require.NoError(t, decided.BeginPurging())
		require.NoError(t, store.PersistLifecycle(context.Background(), decided))
		require.Equal(t, []string{"space.deletion_scheduled"}, lifecycleEventTypes(readyLifecycleEvents(t, store)))

		var purgeDecidedAt time.Time
		require.NoError(t, st.Pool.QueryRow(context.Background(), `SELECT purge_decided_at FROM space_lifecycle_aggregates WHERE space_id=$1`, spaceID).Scan(&purgeDecidedAt))
		require.NoError(t, decided.RecordRoleRetirementReceipt(lifecycleRoleRetirementReceipt(t, spaceID, operationID, 2, purgeDecidedAt)))
		for _, participant := range lifecycleTestParticipants[1:] {
			require.NoError(t, decided.RecordPurgeReceipt(lifecyclePurgeReceipt(t, spaceID, operationID, participant, 2, purgeDecidedAt)))
		}
		require.NoError(t, store.PersistLifecycle(context.Background(), decided))
		require.Equal(t, []string{"space.deletion_scheduled"}, lifecycleEventTypes(readyLifecycleEvents(t, store)))
		require.NoError(t, decided.RecordLocalPurgeCompleted())
		deletedEvent, err := decided.CompletePurge(time.Now().UTC())
		require.NoError(t, err)
		require.NoError(t, store.CompleteLifecyclePurge(context.Background(), decided, bytes.Repeat([]byte{0xa1}, 32), bytes.Repeat([]byte{0xb2}, 32), "space-tombstone-k7"))
		require.Equal(t, []string{"space.deletion_scheduled", "space.deleted"}, lifecycleEventTypes(readyLifecycleEvents(t, store)))
		loaded, err := store.LoadLifecycle(context.Background(), spaceID)
		require.NoError(t, err)
		replay, err := loaded.CompletePurge(deletedEvent.OccurredAt.Add(time.Hour))
		require.NoError(t, err)
		require.Equal(t, deletedEvent, replay)
	})
}

func storedPurgingLifecycleFixture(t *testing.T, st *SpaceStore) (durableLifecycleStore, *spacecore.LifecycleAggregate, uuid.UUID, uuid.UUID) {
	t.Helper()
	store, _, spaceID, operationID, _ := completeStoredSchedule(t, st)
	_, err := st.Pool.Exec(context.Background(), `UPDATE space_lifecycle_aggregates
		SET scheduled_at=scheduled_at-interval '8 days', purge_after=purge_after-interval '8 days'
		WHERE space_id=$1`, spaceID)
	require.NoError(t, err)
	aggregate, err := store.DecideLifecycleRecovery(context.Background(), spaceID, 2)
	require.NoError(t, err)
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGE_DECIDED, aggregate.Phase())
	recordLifecycleFenceBarrier(t, aggregate, spaceID, operationID, 2, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED)
	require.NoError(t, aggregate.BeginPurging())
	require.NoError(t, store.PersistLifecycle(context.Background(), aggregate))
	var purgeDecidedAt time.Time
	require.NoError(t, st.Pool.QueryRow(context.Background(), `SELECT purge_decided_at FROM space_lifecycle_aggregates WHERE space_id=$1`, spaceID).Scan(&purgeDecidedAt))
	require.NoError(t, aggregate.RecordRoleRetirementReceipt(lifecycleRoleRetirementReceipt(t, spaceID, operationID, 2, purgeDecidedAt)))
	for _, participant := range lifecycleTestParticipants[1:] {
		require.NoError(t, aggregate.RecordPurgeReceipt(lifecyclePurgeReceipt(t, spaceID, operationID, participant, 2, purgeDecidedAt)))
	}
	require.NoError(t, store.PersistLifecycle(context.Background(), aggregate))
	require.NoError(t, aggregate.RecordLocalPurgeCompleted())
	return store, aggregate, spaceID, operationID
}

func TestLifecyclePersistence_TombstoneTerminalAggregateAndDeletedReadyCommitAtomically(t *testing.T) {
	st := lifecycleStoreFixture(t)
	_, aggregate, spaceID, operationID := storedPurgingLifecycleFixture(t, st)
	deletedEvent, err := aggregate.CompletePurge(time.Now().UTC())
	require.NoError(t, err)
	ownerHMAC := bytes.Repeat([]byte{0xa1}, sha256.Size)
	actorHMAC := bytes.Repeat([]byte{0xb2}, sha256.Size)
	const keyVersion = "space-tombstone-k7"

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	gate := int64(73023002)
	blocker, err := st.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback(context.Background()) }()
	_, err = blocker.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, gate)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, fmt.Sprintf(`CREATE FUNCTION r23_pause_tombstone() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_advisory_xact_lock(%d); RETURN NEW; END $$; CREATE TRIGGER r23_pause_tombstone BEFORE INSERT ON space_deletion_tombstones FOR EACH ROW EXECUTE FUNCTION r23_pause_tombstone()`, gate))
	require.NoError(t, err)
	workerPool := r22SecondSpacePool(t, ctx, st.Pool, "r23_atomic_tombstone")
	worker := requireDurableLifecycleStore(t, &SpaceStore{Pool: workerPool})
	result := make(chan error, 1)
	go func() { result <- worker.CompleteLifecyclePurge(ctx, aggregate, ownerHMAC, actorHMAC, keyVersion) }()
	r22WaitForSpaceLock(t, ctx, st.Pool, "r23_atomic_tombstone")

	var phase, deletedState string
	var tombstoneExists bool
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT upper(a.phase),
		EXISTS(SELECT 1 FROM space_deletion_tombstones t WHERE t.space_id=a.space_id),
		upper(o.state) FROM space_lifecycle_aggregates a JOIN space_lifecycle_outbox o ON o.space_id=a.space_id AND o.event_type='space.deleted' WHERE a.space_id=$1`, spaceID).Scan(&phase, &tombstoneExists, &deletedState))
	require.Equal(t, []any{"PURGING", false, "BLOCKED"}, []any{phase, tombstoneExists, deletedState})
	require.NoError(t, blocker.Commit(ctx))
	require.NoError(t, <-result)

	var gotSpaceID uuid.UUID
	var gotOwnerHMAC, gotActorHMAC []byte
	var gotKeyVersion, reason string
	var scheduledAt, purgeAfter, purgeDecidedAt, purgedAt, retainUntil time.Time
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT a.space_id,upper(a.phase),upper(o.state),t.owner_account_hmac,t.actor_account_hmac,t.key_version,t.reason,
		t.scheduled_at,t.purge_after,t.purge_decided_at,t.purged_at,t.retain_until
		FROM space_lifecycle_aggregates a JOIN space_lifecycle_outbox o ON o.space_id=a.space_id AND o.event_type='space.deleted'
		JOIN space_deletion_tombstones t ON t.space_id=a.space_id WHERE a.space_id=$1`, spaceID).Scan(
		&gotSpaceID, &phase, &deletedState, &gotOwnerHMAC, &gotActorHMAC, &gotKeyVersion, &reason,
		&scheduledAt, &purgeAfter, &purgeDecidedAt, &purgedAt, &retainUntil))
	require.Equal(t, spaceID, gotSpaceID)
	require.Equal(t, "PURGED", phase)
	require.Equal(t, "READY", deletedState)
	require.Equal(t, ownerHMAC, gotOwnerHMAC)
	require.Equal(t, actorHMAC, gotActorHMAC)
	require.Equal(t, keyVersion, gotKeyVersion)
	require.Equal(t, "OWNER_REQUESTED", reason)
	require.False(t, scheduledAt.IsZero())
	require.Equal(t, scheduledAt.Add(7*24*time.Hour), purgeAfter)
	require.False(t, purgeDecidedAt.Before(purgeAfter))
	require.Equal(t, deletedEvent.OccurredAt, purgedAt)
	require.Equal(t, purgedAt.Add(365*24*time.Hour), retainUntil)
	require.Equal(t, operationID.String(), deletedEvent.DeletionOperationID)
}

func lifecycleEventTypes(events []spacecore.LifecycleOutboxRecord) []string {
	types := make([]string, 0, len(events))
	for _, event := range events {
		types = append(types, event.EventType)
	}
	return types
}

func lifecyclePurgeRequestHash(t *testing.T, spaceID, operationID uuid.UUID, participant commonv1.ParticipantId, generation uint64, purgeDecidedAt time.Time) [sha256.Size]byte {
	t.Helper()
	request := &commonv1.SpacePurgeRequest{
		ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: operationID.String(), Generation: generation,
		PurgeDecidedAt: timestamppb.New(purgeDecidedAt), ParticipantId: participant, Manifest: lifecycleManifestFixture(),
	}
	inner, err := proto.MarshalOptions{Deterministic: true}.Marshal(request)
	require.NoError(t, err)
	wrapper := protowire.AppendTag(nil, 1, protowire.BytesType)
	wrapper = protowire.AppendBytes(wrapper, inner)
	return lifecycleDomainHash(lifecycleTestRequestPackages[participant]+".PurgeSpaceRequest", wrapper)
}

func lifecyclePurgeReceipt(t *testing.T, spaceID, operationID uuid.UUID, participant commonv1.ParticipantId, generation uint64, purgeDecidedAt time.Time) *commonv1.SpacePurgeReceipt {
	t.Helper()
	hash := lifecyclePurgeRequestHash(t, spaceID, operationID, participant, generation, purgeDecidedAt)
	return &commonv1.SpacePurgeReceipt{
		ProtocolVersion: 1, ReceiptId: fmt.Sprintf("purge-%d", participant), SpaceId: spaceID.String(), DeletionOperationId: operationID.String(), Generation: generation,
		ParticipantId: participant, State: commonv1.PurgeReceiptState_PURGE_RECEIPT_STATE_COMPLETED, RequestSha256: append([]byte(nil), hash[:]...), CompletedAt: timestamppb.Now(),
	}
}

func lifecycleRoleRetirementReceipt(t *testing.T, spaceID, operationID uuid.UUID, generation uint64, purgeDecidedAt time.Time) *rolev1.RetireSpaceReceipt {
	t.Helper()
	request := &rolev1.RetireSpaceRequest{
		ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: operationID.String(), Generation: generation,
		PurgeDecidedAt: timestamppb.New(purgeDecidedAt), Manifest: lifecycleManifestFixture(),
	}
	requestBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(request)
	require.NoError(t, err)
	hash := lifecycleDomainHash("voice.role.v1.RetireSpaceRequest", requestBytes)
	return &rolev1.RetireSpaceReceipt{
		ProtocolVersion: 1, ReceiptId: "role-retired", SpaceId: spaceID.String(), DeletionOperationId: operationID.String(), Generation: generation,
		State: rolev1.RoleRetirementState_ROLE_RETIREMENT_STATE_RETIRED, RequestSha256: append([]byte(nil), hash[:]...), ManifestSha256: lifecycleManifestFixture().GetManifestSha256(), RetiredAt: timestamppb.Now(),
	}
}

func TestLifecyclePersistence_RecoveryDecisionUsesFreshPostgresClockAfterSpaceLock(t *testing.T) {
	st := lifecycleStoreFixture(t)
	store, _, spaceID, _, _ := completeStoredSchedule(t, st)
	var purgeAfter time.Time
	require.NoError(t, st.Pool.QueryRow(context.Background(), `UPDATE space_lifecycle_aggregates SET purge_after=clock_timestamp()+interval '500 milliseconds' WHERE space_id=$1 RETURNING purge_after`, spaceID).Scan(&purgeAfter))

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	blocker, err := st.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback(context.Background()) }()
	_, err = blocker.Exec(ctx, `SELECT pg_advisory_xact_lock($1::bigint)`, spaceMutationAdvisoryKey(spaceID))
	require.NoError(t, err)
	workerPool := r22SecondSpacePool(t, ctx, st.Pool, "r23_after_lock_clock")
	worker := requireDurableLifecycleStore(t, &SpaceStore{Pool: workerPool})
	result := make(chan struct {
		aggregate *spacecore.LifecycleAggregate
		err       error
	}, 1)
	go func() {
		aggregate, callErr := worker.DecideLifecycleRecovery(ctx, spaceID, 2)
		result <- struct {
			aggregate *spacecore.LifecycleAggregate
			err       error
		}{aggregate: aggregate, err: callErr}
	}()
	r22WaitForSpaceLock(t, ctx, st.Pool, "r23_after_lock_clock")
	for {
		var reached bool
		require.NoError(t, st.Pool.QueryRow(ctx, `SELECT clock_timestamp() >= $1`, purgeAfter).Scan(&reached))
		if reached {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(10 * time.Millisecond):
		}
	}
	require.NoError(t, blocker.Commit(ctx))
	decision := <-result
	require.NoError(t, decision.err)
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGE_DECIDED, decision.aggregate.Phase(), "transaction-start now() would incorrectly restore after waiting past the deadline")

	loaded, err := store.LoadLifecycle(context.Background(), spaceID)
	require.NoError(t, err)
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGE_DECIDED, loaded.Phase())
}

func TestLifecyclePersistence_InvalidReadyReadFailsClosed(t *testing.T) {
	store := requireDurableLifecycleStore(t, &SpaceStore{})
	for _, limit := range []int{-1, 0, 101} {
		err := store.ReadReadyLifecycleOutbox(context.Background(), limit, func(spacecore.LifecycleOutboxRecord) error { return nil })
		require.Error(t, err)
	}
	err := store.ReadReadyLifecycleOutbox(context.Background(), 1, nil)
	require.Error(t, err)
	_, err = store.LoadLifecycle(context.Background(), uuid.Nil)
	require.Error(t, err)
}
