package store_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	botcommonv1 "voice.app/voice/common/v1"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/integrationtest"
)

func startBotLifecycleStore(t *testing.T) *store.BotStore {
	t.Helper()
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "botdb", "")
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations", "bot_db")
	for _, name := range []string{
		"000001_init.up.sql", "000002_bot_presence.up.sql", "000003_message_delivery_outbox.up.sql",
		"000004_slash_interaction_outbox.up.sql", "000005_space_lifecycle.up.sql",
	} {
		sql, err := os.ReadFile(filepath.Join(root, name))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(sql))
		require.NoError(t, err)
	}
	return &store.BotStore{Pool: pool}
}

func TestSpaceLifecycleFenceReplayAndPurgeOnlySpaceOwnedBotData(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st := startBotLifecycleStore(t)
	spaceID, operationID, profileID := uuid.New(), uuid.New(), uuid.New()
	bot, _, err := st.CreateBot(ctx, uuid.New(), "Lifecycle Bot", "", `["TEXT_CHAT_READ_HISTORY"]`, profileID)
	require.NoError(t, err)
	chatID := uuid.New()
	_, err = st.InstallInSpace(ctx, bot.ID, spaceID, profileID, []uuid.UUID{chatID})
	require.NoError(t, err)
	deliveryID := uuid.New()
	_, err = st.Pool.Exec(ctx, `INSERT INTO bot_message_deliveries
(id,bot_id,message_id,chat_id,payload,status) VALUES($1,$2,$3,$4,'{"content":"space-private"}'::jsonb,'pending')`,
		deliveryID, bot.ID, uuid.New(), chatID)
	require.NoError(t, err)
	manifest := &botcommonv1.ManifestBinding{ManifestId: operationID.String(), ManifestSha256: make([]byte, 32), ItemCount: 1}
	frozen := &botcommonv1.SpaceLifecycleFenceRequest{
		ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: operationID.String(), Generation: 1,
		DesiredState: botcommonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: manifest,
	}
	frozenReceipt, err := st.ApplySpaceLifecycleFence(ctx, frozen)
	require.NoError(t, err)
	require.Equal(t, botcommonv1.ParticipantId_PARTICIPANT_ID_BOT, frozenReceipt.GetParticipantId())
	replay, err := st.ApplySpaceLifecycleFence(ctx, proto.Clone(frozen).(*botcommonv1.SpaceLifecycleFenceRequest))
	require.NoError(t, err)
	require.True(t, proto.Equal(frozenReceipt, replay))
	var deliveryStatus, payload string
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT status,payload::text FROM bot_message_deliveries WHERE id=$1`, deliveryID).Scan(&deliveryStatus, &payload))
	require.Equal(t, "canceled", deliveryStatus)
	require.NotContains(t, payload, "space-private")

	purgeFence := proto.Clone(frozen).(*botcommonv1.SpaceLifecycleFenceRequest)
	purgeFence.Generation = 2
	purgeFence.DesiredState = botcommonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED
	_, err = st.ApplySpaceLifecycleFence(ctx, purgeFence)
	require.NoError(t, err)
	purge := &botcommonv1.SpacePurgeRequest{
		ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: operationID.String(), Generation: 2,
		ParticipantId: botcommonv1.ParticipantId_PARTICIPANT_ID_BOT, PurgeDecidedAt: timestamppb.New(time.Now().UTC()), Manifest: manifest,
	}
	purgeReceipt, err := st.PurgeSpace(ctx, purge)
	require.NoError(t, err)
	require.Equal(t, botcommonv1.PurgeReceiptState_PURGE_RECEIPT_STATE_COMPLETED, purgeReceipt.GetState())
	purgeReplay, err := st.PurgeSpace(ctx, proto.Clone(purge).(*botcommonv1.SpacePurgeRequest))
	require.NoError(t, err)
	require.True(t, proto.Equal(purgeReceipt, purgeReplay))
	for _, table := range []string{"bot_space_installations", "bot_chat_whitelist"} {
		var count int
		require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM `+table+` WHERE space_id=$1`, spaceID).Scan(&count))
		require.Zero(t, count, table)
	}
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT status,payload::text FROM bot_message_deliveries WHERE id=$1`, deliveryID).Scan(&deliveryStatus, &payload))
	require.Equal(t, "canceled", deliveryStatus)
	require.NotContains(t, payload, "space-private")
}

func TestSpaceLifecycleFenceRejectsChangedReplayAndOutOfOrderRestore(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st := startBotLifecycleStore(t)
	spaceID, operationID := uuid.New(), uuid.New()
	request := &botcommonv1.SpaceLifecycleFenceRequest{
		ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: operationID.String(), Generation: 1,
		DesiredState: botcommonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN,
		Manifest:     &botcommonv1.ManifestBinding{ManifestId: operationID.String(), ManifestSha256: make([]byte, 32)},
	}
	_, err := st.ApplySpaceLifecycleFence(ctx, request)
	require.NoError(t, err)
	changed := proto.Clone(request).(*botcommonv1.SpaceLifecycleFenceRequest)
	changed.Manifest.ItemCount++
	_, err = st.ApplySpaceLifecycleFence(ctx, changed)
	require.ErrorIs(t, err, store.ErrSpaceLifecycleConflict)
	outOfOrder := proto.Clone(request).(*botcommonv1.SpaceLifecycleFenceRequest)
	outOfOrder.DesiredState = botcommonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE
	outOfOrder.Generation = 3
	_, err = st.ApplySpaceLifecycleFence(ctx, outOfOrder)
	require.ErrorIs(t, err, store.ErrSpaceLifecycleConflict)
}
