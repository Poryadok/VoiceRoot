package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonv1 "voice.app/voice/common/v1"
	notificationv1 "voice.app/voice/notification/v1"
	"voice/backend/notification/internal/store"
)

func TestNotificationSpaceLifecycle_FreezeRestoreAndPurgeSettings(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startNotificationPostgresForTest(t, ctx)
	applyNotificationMigration(t, ctx, pool)

	s := &store.SettingsStore{Pool: pool}
	spaceID, profileID, operationID := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, s.UpsertSettings(ctx, store.NotificationSettings{ProfileID: profileID, ScopeType: "space", ScopeID: &spaceID, Enabled: false}))
	require.NoError(t, s.UpsertSettings(ctx, store.NotificationSettings{ProfileID: profileID, ScopeType: "global", Enabled: false}))
	page := notificationManifestFixture(spaceID, operationID, 1, nil)
	manifest := page.Manifest
	_, err := s.ImportSpacePurgeManifestPage(ctx, &notificationv1.ImportSpacePurgeManifestPageRequest{ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: operationID.String(), ScheduleGeneration: 1, Page: page, SealsManifest: true})
	require.NoError(t, err)
	freeze := &commonv1.SpaceLifecycleFenceRequest{ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: operationID.String(), Generation: 1, DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: manifest}
	freezeReceipt, err := s.ApplySpaceLifecycleFence(ctx, freeze)
	require.NoError(t, err)
	require.Equal(t, commonv1.ParticipantId_PARTICIPANT_ID_NOTIFICATION, freezeReceipt.GetParticipantId())
	replayedFreeze, err := s.ApplySpaceLifecycleFence(ctx, freeze)
	require.NoError(t, err)
	require.True(t, proto.Equal(freezeReceipt, replayedFreeze))
	changedFreeze := proto.Clone(freeze).(*commonv1.SpaceLifecycleFenceRequest)
	changedFreeze.Manifest.ItemCount = 1
	_, err = s.ApplySpaceLifecycleFence(ctx, changedFreeze)
	require.Error(t, err, "same operation identity with changed request bytes must conflict")
	err = s.UpsertSettings(ctx, store.NotificationSettings{ProfileID: profileID, ScopeType: "space", ScopeID: &spaceID, Enabled: true})
	require.ErrorIs(t, err, store.ErrSpaceLifecycleNotLive)

	restore := proto.Clone(freeze).(*commonv1.SpaceLifecycleFenceRequest)
	restore.Generation = 2
	restore.DesiredState = commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE
	_, err = s.ApplySpaceLifecycleFence(ctx, restore)
	require.NoError(t, err)
	require.NoError(t, s.UpsertSettings(ctx, store.NotificationSettings{ProfileID: profileID, ScopeType: "space", ScopeID: &spaceID, Enabled: true}))

	newOperationID := uuid.New()
	page = notificationManifestFixture(spaceID, newOperationID, 4, nil)
	manifest = page.Manifest
	_, err = s.ImportSpacePurgeManifestPage(ctx, &notificationv1.ImportSpacePurgeManifestPageRequest{ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: newOperationID.String(), ScheduleGeneration: 4, Page: page, SealsManifest: true})
	require.NoError(t, err)
	newFreeze := proto.Clone(freeze).(*commonv1.SpaceLifecycleFenceRequest)
	newFreeze.DeletionOperationId = newOperationID.String()
	newFreeze.Generation = 4
	newFreeze.Manifest = manifest
	_, err = s.ApplySpaceLifecycleFence(ctx, newFreeze)
	require.NoError(t, err)
	purgeFence := proto.Clone(newFreeze).(*commonv1.SpaceLifecycleFenceRequest)
	purgeFence.Generation = 5
	purgeFence.DesiredState = commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED
	_, err = s.ApplySpaceLifecycleFence(ctx, purgeFence)
	require.NoError(t, err)
	purgeRequest := &commonv1.SpacePurgeRequest{ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: newOperationID.String(), Generation: 5, PurgeDecidedAt: timestamppb.New(time.Now().UTC().Truncate(time.Microsecond)), ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_NOTIFICATION, Manifest: manifest}
	purgeReceipt, err := s.PurgeSpace(ctx, purgeRequest)
	require.NoError(t, err)
	require.Equal(t, commonv1.PurgeReceiptState_PURGE_RECEIPT_STATE_COMPLETED, purgeReceipt.GetState())
	replayedPurge, err := s.PurgeSpace(ctx, purgeRequest)
	require.NoError(t, err)
	require.Equal(t, purgeReceipt.GetReceiptId(), replayedPurge.GetReceiptId(), "exact purge retry must return its durable receipt")
	_, err = s.GetSettings(ctx, profileID, "space", &spaceID)
	require.ErrorIs(t, err, store.ErrSpaceLifecycleNotLive)
	global, err := s.GetSettings(ctx, profileID, "global", nil)
	require.NoError(t, err)
	require.False(t, global.Enabled)
	var scopedCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM notification_settings WHERE profile_id=$1 AND scope_type='space' AND scope_id=$2`, profileID, spaceID).Scan(&scopedCount))
	require.Zero(t, scopedCount)
	_, err = pool.Exec(ctx, `UPDATE notification_space_lifecycle_fence_receipts SET retain_until=clock_timestamp()-interval '1 second' WHERE space_id=$1`, spaceID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE notification_space_lifecycle_purge_receipts SET retain_until=clock_timestamp()-interval '1 second' WHERE space_id=$1`, spaceID)
	require.NoError(t, err)
	deleted, err := s.DeleteExpiredSpaceLifecycleReceipts(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, deleted, int64(1))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM notification_space_lifecycle_fences WHERE space_id=$1 AND state='PURGED' AND manifest_id='' AND manifest_item_count=0`, spaceID).Scan(&scopedCount))
	require.Equal(t, 1, scopedCount, "expiry keeps only the permanent minimal Space tombstone")
	_, err = s.PurgeSpace(ctx, purgeRequest)
	require.ErrorIs(t, err, store.ErrSpaceLifecycleNotLive, "an expired receipt must not allow a purge replay to recreate state")
}
