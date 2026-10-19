package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	commonv1 "voice.app/voice/common/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
)

type botPurgeHandoffReader interface {
	GetBotPurgeHandoff(context.Context, uuid.UUID, uuid.UUID, uint64, *commonv1.ManifestBinding) (*BotPurgeHandoff, error)
}

func TestBotPurgeHandoffDoesNotReconstructErasedOriginals(t *testing.T) {
	if testing.Short() {
		t.Skip("requires private PostgreSQL18 real terminal retention clock")
	}
	admin, serving, event := nativeTerminalMaintenanceFixtureHook(t, nativeTerminalClockImage(t), nil, func(admin *SpaceStore) {
		setNativeTerminalDBTime(t, admin, time.Now().UTC().Add(-time.Second).Truncate(time.Second))
	})
	ctx := context.Background()
	space, operation := uuid.MustParse(event.SpaceID), uuid.MustParse(event.DeletionOperationID)
	get := func() (*BotPurgeHandoff, error) {
		return serving.GetBotPurgeHandoff(ctx, space, operation, event.Generation, lifecycleManifestFixture())
	}
	before, err := get()
	require.NoError(t, err)
	require.NotNil(t, before.Role)
	require.NotNil(t, before.Messaging)
	var deadline, expiry time.Time
	require.NoError(t, admin.Pool.QueryRow(ctx, `SELECT a.completed_at+interval '30 days',t.retain_until AT TIME ZONE 'UTC' FROM public.space_lifecycle_aggregates a JOIN public.space_deletion_tombstones t USING(space_id) WHERE a.space_id=$1`, space).Scan(&deadline, &expiry))
	setNativeTerminalDBTime(t, admin, deadline.Add(-time.Second))
	var participantOwnDeadlinePassed bool
	require.NoError(t, admin.Pool.QueryRow(ctx, `SELECT completed_at+interval '30 days'<clock_timestamp() FROM public.space_lifecycle_participants WHERE space_id=$1 AND participant_id=$2 AND request_kind='PURGE'`, space, int16(commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING)).Scan(&participantOwnDeadlinePassed))
	require.True(t, participantOwnDeadlinePassed, "coordinator copy remains available beyond the participant's own completion+30d")
	retained, err := get()
	require.NoError(t, err)
	require.Equal(t, before.Messaging.ReceiptBytes, retained.Messaging.ReceiptBytes)
	require.Equal(t, before.Role.ReceiptBytes, retained.Role.ReceiptBytes)
	setNativeTerminalDBTime(t, admin, deadline)
	require.NoError(t, serving.CleanupLifecycleEvidence(ctx))
	erased, err := get()
	if err == nil {
		require.Nil(t, erased.Role)
		require.Nil(t, erased.Messaging)
	} else {
		require.Nil(t, erased)
	}
	var absent bool
	require.NoError(t, admin.Pool.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM public.space_lifecycle_participants WHERE space_id=$1) AND EXISTS(SELECT 1 FROM native_membership_guard.spaces WHERE space_id=$1 AND terminal_maintenance_phase='COMPACT')`, space).Scan(&absent))
	require.True(t, absent)
	setNativeTerminalDBTime(t, admin, expiry)
	require.NoError(t, serving.CleanupLifecycleEvidence(ctx))
	erased, err = get()
	require.Error(t, err)
	require.Nil(t, erased, "expired aggregate cannot mint receipts from the permanent fence")
}

func TestBotPurgeHandoffBeforeAggregateCompletion(t *testing.T) {
	if testing.Short() {
		t.Skip("requires private PostgreSQL18/current47")
	}
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixtureWithImage(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"), "postgres:18-alpine")
	db.Run(true, "goto", "34")
	admin := &SpaceStore{Pool: db.Pool}
	f := newLifecycleOutcomeFixtureWithStore(t, admin, len(lifecycleTestParticipants))
	_, _, err := admin.CompleteLifecycleSchedule(ctx, f.space)
	require.NoError(t, err)
	_, err = admin.Pool.Exec(ctx, `UPDATE public.space_lifecycle_aggregates SET scheduled_at=scheduled_at-interval '8 days',purge_after=purge_after-interval '8 days' WHERE space_id=$1`, f.space)
	require.NoError(t, err)
	a, err := admin.DecideLifecycleRecovery(ctx, f.space, 2)
	require.NoError(t, err)
	op := uuid.MustParse(f.request.OperationId)
	recordLifecycleFenceBarrier(t, a, f.space, op, 2, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED)
	require.NoError(t, a.BeginPurging())
	require.NoError(t, a.RecordRoleRetirementReceipt(lifecycleRoleRetirementReceipt(t, f.space, op, 2, a.Snapshot().PurgeDecidedAt)))
	require.NoError(t, a.RecordPurgeReceipt(lifecyclePurgeReceipt(t, f.space, op, commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING, 2, a.Snapshot().PurgeDecidedAt)))
	require.NoError(t, admin.PersistLifecycle(ctx, a))
	db.Run(true, "goto", "46")
	_, err = admin.TransitionNativeMembershipCatalog46(ctx)
	require.NoError(t, err)
	require.NoError(t, admin.InitializeNativeMembership(ctx, f.space))
	db.Run(true, "up", "1")
	_, err = admin.TransitionNativeMembershipCatalog47(ctx)
	require.NoError(t, err)
	serving := &SpaceStore{Pool: nativeReadinessLogin(t, ctx, admin.Pool)}
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGING, a.Phase())
	require.Nil(t, a.Snapshot().PurgeReceipts[commonv1.ParticipantId_PARTICIPANT_ID_BOT])
	reader, ok := any(serving).(botPurgeHandoffReader)
	require.True(t, ok, "accepted participant originals need a Bot-only relay before Bot/aggregate completion")
	got, err := reader.GetBotPurgeHandoff(ctx, f.space, op, 2, a.Snapshot().Manifest)
	require.NoError(t, err)
	require.NotNil(t, got.Role)
	require.NotNil(t, got.Messaging)
	require.True(t, proto.Equal(a.Snapshot().RoleReceipt, got.Role.Receipt))
	require.True(t, proto.Equal(a.Snapshot().PurgeReceipts[commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING], got.Messaging.Receipt))
	t.Run("exact original domains and isolated copies", func(t *testing.T) {
		roleRequest := lifecycleHash("voice.role.v1.RetireSpaceRequest", got.Role.RequestBytes)
		roleReceipt := lifecycleHash("voice.role.v1.RetireSpaceReceipt", got.Role.ReceiptBytes)
		messageRequest := lifecycleHash("voice.messaging.v1.PurgeSpaceRequest", got.Messaging.RequestBytes)
		messageReceipt := lifecycleHash("voice.common.v1.SpacePurgeReceipt", got.Messaging.ReceiptBytes)
		require.Equal(t, roleRequest[:], got.Role.RequestSHA256)
		require.Equal(t, roleReceipt[:], got.Role.ReceiptSHA256)
		require.Equal(t, messageRequest[:], got.Messaging.RequestSHA256)
		require.Equal(t, messageReceipt[:], got.Messaging.ReceiptSHA256)
		inner, err := lifecycleSingleBytesField(got.Messaging.RequestBytes, 1)
		require.NoError(t, err)
		var request commonv1.SpacePurgeRequest
		require.NoError(t, proto.Unmarshal(inner, &request))
		require.Equal(t, commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING, request.ParticipantId)
		got.Role.RequestBytes[0] ^= 1
		got.Messaging.Receipt.SpaceId = uuid.NewString()
		again, err := reader.GetBotPurgeHandoff(ctx, f.space, op, 2, a.Snapshot().Manifest)
		require.NoError(t, err)
		require.Equal(t, roleRequest[:], again.Role.RequestSHA256)
		require.True(t, proto.Equal(a.Snapshot().PurgeReceipts[commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING], again.Messaging.Receipt))
	})
	t.Run("wrong immutable binding", func(t *testing.T) {
		_, err := reader.GetBotPurgeHandoff(ctx, f.space, uuid.New(), 2, a.Snapshot().Manifest)
		require.ErrorIs(t, err, ErrLifecycleConflict)
		_, err = reader.GetBotPurgeHandoff(ctx, f.space, op, 1, a.Snapshot().Manifest)
		require.ErrorIs(t, err, ErrLifecycleConflict)
		changed := proto.Clone(a.Snapshot().Manifest).(*commonv1.ManifestBinding)
		changed.ItemCount++
		_, err = reader.GetBotPurgeHandoff(ctx, f.space, op, 2, changed)
		require.ErrorIs(t, err, ErrLifecycleConflict)
	})
	var noBotOrEvent bool
	require.NoError(t, admin.Pool.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM public.space_lifecycle_participants WHERE space_id=$1 AND participant_id=$2 AND request_kind='PURGE' AND progress='COMPLETE') AND NOT EXISTS(SELECT 1 FROM public.space_lifecycle_outbox WHERE space_id=$1 AND event_type='space.deleted' AND state IN ('READY','DELIVERED'))`, f.space, int16(commonv1.ParticipantId_PARTICIPANT_ID_BOT)).Scan(&noBotOrEvent))
	require.True(t, noBotOrEvent, "read relay must neither complete Bot nor publish aggregate completion")
}
