package store_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	botv1 "voice.app/voice/bot/v1"
	commonv1 "voice.app/voice/common/v1"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
)

// This oracle is executed against the immutable OLD 9088 Store implementation
// plus only the official015 SQL and this test, not the new app pending gate.
func TestOldPurgeWriterCannotCommitWithoutPreparedHandoff(t *testing.T) {
	if os.Getenv("VOICE_BOT_OLD_PURGE_WRITER") != "1" {
		t.Skip("requires immutable old9088 implementation plus exact015 SQL")
	}
	if testing.Short() {
		t.Skip("real old-writer Bot PostgreSQL migration compatibility")
	}
	ctx := t.Context()
	database := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join("..", "..", "..", "migrations", "bot_db"))
	database.Run(true, "up")
	st := &store.BotStore{Pool: database.Pool}
	owner, actor, space, operation := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	bot, _, err := st.CreateBot(ctx, owner, "Old purge writer", "", `[]`, actor)
	require.NoError(t, err)
	intent, err := st.PrepareInstallation(ctx, bot.ID, space, actor, nil)
	require.NoError(t, err)
	require.NoError(t, st.CompleteInstallation(ctx, intent, nil))
	q := &botv1.UninstallBotFromSpaceRequest{BotId: bot.ID.String(), SpaceId: space.String()}
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(q)
	require.NoError(t, err)
	hash, err := principal.RequestHash(q)
	require.NoError(t, err)
	_, err = st.BeginInstallationCancellationWithAdmission(ctx, bot.ID, space, actor, store.DeletionAdmission{AccountID: owner, ProfileID: actor, SessionEpoch: 9, AccountType: "regular", Method: botv1.BotService_UninstallBotFromSpace_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash, RequestBytes: body, ExpiresAt: time.Now().Add(time.Minute)})
	require.NoError(t, err)
	root, err := st.UninstallCleanup(ctx, intent.InstallationID)
	require.NoError(t, err)
	manifest := &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: make([]byte, 32)}
	fence := &commonv1.SpaceLifecycleFenceRequest{ProtocolVersion: 1, SpaceId: space.String(), DeletionOperationId: operation.String(), Generation: 1, DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: manifest}
	_, err = st.ApplySpaceLifecycleFence(ctx, fence)
	require.NoError(t, err)
	fence.Generation = 2
	fence.DesiredState = commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED
	_, err = st.ApplySpaceLifecycleFence(ctx, fence)
	require.NoError(t, err)
	receipt, err := st.PurgeSpace(ctx, &commonv1.SpacePurgeRequest{ProtocolVersion: 1, SpaceId: space.String(), DeletionOperationId: operation.String(), Generation: 2, ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_BOT, PurgeDecidedAt: timestamppb.Now(), Manifest: manifest})
	require.Error(t, err, "old writer cannot ACK/erase accepted obligations after FK decoupling")
	require.Nil(t, receipt)
	var pgerr *pgconn.PgError
	require.ErrorAs(t, err, &pgerr)
	require.Equal(t, "P0001", pgerr.Code)
	require.Contains(t, pgerr.Message, "prepared and covered handoff")
	var intentState, headState string
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT state FROM bot_installation_intents WHERE installation_id=$1`, intent.InstallationID).Scan(&intentState))
	require.Equal(t, "CANCELING", intentState)
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT desired_state FROM bot_space_lifecycle_heads WHERE space_id=$1`, space).Scan(&headState))
	require.Equal(t, "PURGE_DECIDED", headState)
	after, err := st.UninstallCleanup(ctx, intent.InstallationID)
	require.NoError(t, err)
	require.Equal(t, root.DecisionBytes, after.DecisionBytes)
	require.Equal(t, "PENDING", after.State)
	for _, table := range []string{"bot_space_purge_handoffs", "bot_space_lifecycle_purge_receipts"} {
		var n int
		require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM `+table+` WHERE space_id=$1`, space).Scan(&n))
		require.Zero(t, n)
	}
}
