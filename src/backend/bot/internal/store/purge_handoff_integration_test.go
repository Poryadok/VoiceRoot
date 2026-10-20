package store_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	botv1 "voice.app/voice/bot/v1"
	commonv1 "voice.app/voice/common/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/principal"
)

func TestPurgePreservesPendingUninstallAsExplicitBoundedHandoff(t *testing.T) {
	if testing.Short() {
		t.Skip("real Bot PostgreSQL P3 source capture")
	}
	ctx := t.Context()
	st := startBotLifecycleStore(t)
	owner, actor, space, operation := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	bot, _, err := st.CreateBot(ctx, owner, "Pending P3 source", "", `[]`, actor)
	require.NoError(t, err)
	intent, err := st.PrepareInstallation(ctx, bot.ID, space, actor, nil)
	require.NoError(t, err)
	require.NoError(t, st.CompleteInstallation(ctx, intent, nil))
	uninstall := &botv1.UninstallBotFromSpaceRequest{BotId: bot.ID.String(), SpaceId: space.String()}
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(uninstall)
	require.NoError(t, err)
	hash, err := principal.RequestHash(uninstall)
	require.NoError(t, err)
	canceled, err := st.BeginInstallationCancellationWithAdmission(ctx, bot.ID, space, actor, store.DeletionAdmission{AccountID: owner, ProfileID: actor, SessionEpoch: 9, AccountType: "regular", Method: botv1.BotService_UninstallBotFromSpace_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash, RequestBytes: body, ExpiresAt: time.Now().Add(time.Minute)})
	require.NoError(t, err)
	accepted, err := st.UninstallCleanup(ctx, intent.InstallationID)
	require.NoError(t, err)
	effects, err := st.LifecycleEffects(ctx, accepted)
	require.NoError(t, err)
	require.Len(t, effects, 2)
	manifest := &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: make([]byte, 32)}
	fence := &commonv1.SpaceLifecycleFenceRequest{ProtocolVersion: 1, SpaceId: space.String(), DeletionOperationId: operation.String(), Generation: 1, DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: manifest}
	_, err = st.ApplySpaceLifecycleFence(ctx, fence)
	require.NoError(t, err)
	fence.Generation = 2
	fence.DesiredState = commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED
	_, err = st.ApplySpaceLifecycleFence(ctx, fence)
	require.NoError(t, err)
	purge := &commonv1.SpacePurgeRequest{ProtocolVersion: 1, SpaceId: space.String(), DeletionOperationId: operation.String(), Generation: 2, ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_BOT, PurgeDecidedAt: timestamppb.Now(), Manifest: manifest}
	receipt, err := st.PurgeSpace(ctx, purge)
	require.ErrorIs(t, err, store.ErrDeletionPending, "accepted unfinished work must become durable recoverable handoff, not FK failure or completed purge")
	require.Nil(t, receipt)
	var state string
	var requestBytes, snapshotBytes []byte
	var sources int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT state,request_bytes FROM bot_space_purge_handoffs WHERE space_id=$1`, space).Scan(&state, &requestBytes))
	require.Equal(t, "PENDING", state)
	require.NotEmpty(t, requestBytes)
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT source_bytes FROM bot_space_purge_sources WHERE space_id=$1 AND bot_id=$2`, space, bot.ID).Scan(&snapshotBytes))
	require.NotEmpty(t, snapshotBytes)
	var snapshot store.BotPurgeSource
	require.NoError(t, json.Unmarshal(snapshotBytes, &snapshot))
	require.Equal(t, intent.InstallationID, snapshot.Intent.InstallationID)
	require.Equal(t, intent.OperationID, snapshot.Intent.OperationID)
	require.Equal(t, canceled.CancelOperationID, snapshot.NativeCancelOperationID)
	require.Len(t, snapshot.Retained, 1)
	require.Equal(t, accepted.DecisionBytes, snapshot.Retained[0].DecisionBytes)
	require.Equal(t, effects[0].Target.ScopeBytes, snapshot.Retained[0].TargetBytes)
	require.Len(t, snapshot.Retained[0].Effects, 2)
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_space_purge_sources WHERE space_id=$1`, space).Scan(&sources))
	require.Equal(t, 1, sources)
	after, err := st.UninstallCleanup(ctx, intent.InstallationID)
	require.NoError(t, err)
	require.Equal(t, accepted.OperationID, after.OperationID)
	require.Equal(t, accepted.DecisionBytes, after.DecisionBytes)
	afterEffects, err := st.LifecycleEffects(ctx, after)
	require.NoError(t, err)
	require.Equal(t, effects, afterEffects)
	var intentState string
	var cancel uuid.UUID
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT state,cancel_operation_id FROM bot_installation_intents WHERE installation_id=$1`, intent.InstallationID).Scan(&intentState, &cancel))
	require.Equal(t, "CANCELING", intentState)
	require.Equal(t, canceled.CancelOperationID, cancel)
	var purgeReceipts int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_space_lifecycle_purge_receipts WHERE space_id=$1`, space).Scan(&purgeReceipts))
	require.Zero(t, purgeReceipts)
	_, err = st.PurgeSpace(ctx, proto.Clone(purge).(*commonv1.SpacePurgeRequest))
	require.ErrorIs(t, err, store.ErrDeletionPending)
	var replay []byte
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT source_bytes FROM bot_space_purge_sources WHERE space_id=$1 AND bot_id=$2`, space, bot.ID).Scan(&replay))
	require.Equal(t, snapshotBytes, replay, "retry owns exact captured sources")
}

func TestPurgeCapturesCanceledPendingNativeTuple(t *testing.T) {
	if testing.Short() {
		t.Skip("real accepted cancellation source survives local CANCELED state")
	}
	ctx := t.Context()
	st := startBotLifecycleStore(t)
	owner, actor, space := uuid.New(), uuid.New(), uuid.New()
	bot, _, err := st.CreateBot(ctx, owner, "Canceled native P3 source", "", `[]`, actor)
	require.NoError(t, err)
	intent, err := st.PrepareInstallation(ctx, bot.ID, space, actor, nil)
	require.NoError(t, err)
	require.NoError(t, st.CompleteInstallation(ctx, intent, nil))
	q := &botv1.UninstallBotFromSpaceRequest{BotId: bot.ID.String(), SpaceId: space.String()}
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(q)
	require.NoError(t, err)
	hash, err := principal.RequestHash(q)
	require.NoError(t, err)
	cancel, err := st.BeginInstallationCancellationWithAdmission(ctx, bot.ID, space, actor, store.DeletionAdmission{AccountID: owner, ProfileID: actor, SessionEpoch: 9, AccountType: "regular", Method: botv1.BotService_UninstallBotFromSpace_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash, RequestBytes: body, ExpiresAt: time.Now().Add(time.Minute)})
	require.NoError(t, err)
	require.NoError(t, st.CompleteInstallationCancellation(ctx, cancel))
	purge := preparePurgeHandoffDecision(t, st, space)
	_, err = st.PurgeSpace(ctx, purge)
	require.ErrorIs(t, err, store.ErrDeletionPending)
	var raw []byte
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT source_bytes FROM bot_space_purge_sources WHERE space_id=$1 AND bot_id=$2`, space, bot.ID).Scan(&raw))
	// Read the frozen wire-neutral source bytes: no later join to the disposable
	// live intent may allocate or reconstruct this accepted native obligation.
	var source struct {
		NativeDrains []struct {
			OperationID   uuid.UUID `json:"operation_id"`
			RequestBytes  []byte    `json:"request_bytes"`
			RequestSHA256 []byte    `json:"request_sha256"`
		} `json:"native_drains"`
	}
	require.NoError(t, json.Unmarshal(raw, &source))
	require.Len(t, source.NativeDrains, 1, "the accepted unfinished target remains eligible after its native ACK/local CANCELED")
	drain := source.NativeDrains[0]
	require.NotEqual(t, uuid.Nil, drain.OperationID)
	require.NotEqual(t, cancel.CancelOperationID, drain.OperationID)
	digest := sha256.Sum256(drain.RequestBytes)
	require.Equal(t, digest[:], drain.RequestSHA256)
	var native spacev1.RemoveBotMemberRequest
	require.NoError(t, proto.Unmarshal(drain.RequestBytes, &native))
	require.Equal(t, intent.OperationID.String(), native.InstallOperationId)
	require.Equal(t, space.String(), native.SpaceId)
	require.Equal(t, actor.String(), native.ProfileId)
	require.Equal(t, &spacev1.BotInstallationIntent{OperationId: cancel.CancelOperationID.String(), BotId: bot.ID.String(), InstallationId: intent.InstallationID.String(), SpaceId: space.String(), ActorProfileId: actor.String(), Generation: uint64(intent.Generation)}, native.InstallationIntent)
	_, err = st.PurgeSpace(ctx, proto.Clone(purge).(*commonv1.SpacePurgeRequest))
	require.ErrorIs(t, err, store.ErrDeletionPending)
	var replay []byte
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT source_bytes FROM bot_space_purge_sources WHERE space_id=$1 AND bot_id=$2`, space, bot.ID).Scan(&replay))
	require.Equal(t, raw, replay)
}

func TestCompletedUninstallJournalOutlivesItsPurgedProjection(t *testing.T) {
	if testing.Short() {
		t.Skip("real Bot PostgreSQL relational journal retention")
	}
	ctx := t.Context()
	st := startBotLifecycleStore(t)
	owner, actor, space, operation := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	bot, _, err := st.CreateBot(ctx, owner, "Completed retained journal", "", `[]`, actor)
	require.NoError(t, err)
	intent, err := st.PrepareInstallation(ctx, bot.ID, space, actor, nil)
	require.NoError(t, err)
	require.NoError(t, st.CompleteInstallation(ctx, intent, nil))
	uninstall := &botv1.UninstallBotFromSpaceRequest{BotId: bot.ID.String(), SpaceId: space.String()}
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(uninstall)
	require.NoError(t, err)
	hash, err := principal.RequestHash(uninstall)
	require.NoError(t, err)
	cancel, err := st.BeginInstallationCancellationWithAdmission(ctx, bot.ID, space, actor, store.DeletionAdmission{AccountID: owner, ProfileID: actor, SessionEpoch: 9, AccountType: "regular", Method: botv1.BotService_UninstallBotFromSpace_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash, RequestBytes: body, ExpiresAt: time.Now().Add(time.Minute)})
	require.NoError(t, err)
	root, err := st.UninstallCleanup(ctx, intent.InstallationID)
	require.NoError(t, err)
	require.NoError(t, st.CompleteInstallationCancellation(ctx, cancel))
	// This storage-only completed-row fixture does not claim remote cleanup. The
	// existing actual ordinary-uninstall transport tests prove that earlier stage.
	// Here only the accepted journal/projection lifetime relation is under test.
	requestFixture, receiptFixture := []byte("storage request"), []byte("storage receipt")
	requestDigest, receiptDigest := sha256.Sum256(requestFixture), sha256.Sum256(receiptFixture)
	_, err = st.Pool.Exec(ctx, `UPDATE bot_uninstall_effects e SET state='COMPLETE',request_bytes=$2,request_sha256=$3,receipt_bytes=$4,receipt_sha256=$5 FROM bot_uninstall_installations t WHERE e.target_id=t.target_id AND t.operation_id=$1`, root.OperationID, requestFixture, requestDigest[:], receiptFixture, receiptDigest[:])
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `UPDATE bot_uninstall_operations SET state='COMPLETE',completed_at=clock_timestamp() WHERE operation_id=$1`, root.OperationID)
	require.NoError(t, err)
	before, err := st.LifecycleEffects(ctx, &store.BotDeletion{Purpose: "space_uninstall", OperationID: root.OperationID})
	require.NoError(t, err)
	manifest := &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: make([]byte, 32)}
	fence := &commonv1.SpaceLifecycleFenceRequest{ProtocolVersion: 1, SpaceId: space.String(), DeletionOperationId: operation.String(), Generation: 1, DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: manifest}
	_, err = st.ApplySpaceLifecycleFence(ctx, fence)
	require.NoError(t, err)
	fence.Generation = 2
	fence.DesiredState = commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED
	_, err = st.ApplySpaceLifecycleFence(ctx, fence)
	require.NoError(t, err)
	purge := &commonv1.SpacePurgeRequest{ProtocolVersion: 1, SpaceId: space.String(), DeletionOperationId: operation.String(), Generation: 2, ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_BOT, PurgeDecidedAt: timestamppb.Now(), Manifest: manifest}
	receipt, err := st.PurgeSpace(ctx, purge)
	require.NoError(t, err)
	require.Equal(t, commonv1.PurgeReceiptState_PURGE_RECEIPT_STATE_COMPLETED, receipt.State)
	after, err := st.UninstallCleanup(ctx, intent.InstallationID)
	require.NoError(t, err)
	require.Equal(t, root.DecisionBytes, after.DecisionBytes)
	require.Equal(t, "COMPLETE", after.State)
	retained, err := st.LifecycleEffects(ctx, after)
	require.NoError(t, err)
	require.Equal(t, before, retained)
	var intents int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_installation_intents WHERE installation_id=$1`, intent.InstallationID).Scan(&intents))
	require.Zero(t, intents)
	var state string
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT state FROM bot_space_purge_handoffs WHERE space_id=$1`, space).Scan(&state))
	require.Equal(t, "COMPLETE", state)
	// Isolate the second DOWN invariant without granting a production compactor:
	// temporarily remove only this storage-test header in a transaction which
	// must roll back. The accepted journal still cannot regain its old live FK.
	tx, err := st.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(context.Background()) }()
	_, err = tx.Exec(ctx, `DELETE FROM bot_space_purge_handoffs WHERE space_id=$1`, space)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, purgeHandoffMigration(t, "down"))
	require.ErrorContains(t, err, "accepted uninstall journal outlives its installation projection")
	require.NoError(t, tx.Rollback(ctx))
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT state FROM bot_space_purge_handoffs WHERE space_id=$1`, space).Scan(&state))
	require.Equal(t, "COMPLETE", state)
}
