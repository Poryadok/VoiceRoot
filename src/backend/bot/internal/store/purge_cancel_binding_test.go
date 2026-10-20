package store_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	botv1 "voice.app/voice/bot/v1"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/principal"
)

func TestPurgeFirstNativeCancelIdentitySurvivesLateAcceptedRoot(t *testing.T) {
	if testing.Short() {
		t.Skip("real BotPG first-frozen cancellation identity")
	}
	for _, purpose := range []string{"global_delete", "space_uninstall"} {
		t.Run(purpose, func(t *testing.T) {
			st := startBotLifecycleStore(t)
			ctx := t.Context()
			owner, actor, space := uuid.New(), uuid.New(), uuid.New()
			bot, _, err := st.CreateBot(ctx, owner, "P3 first cancel", "", `[]`, actor)
			require.NoError(t, err)
			i, err := st.PrepareInstallation(ctx, bot.ID, space, actor, nil)
			require.NoError(t, err)
			require.NoError(t, st.CompleteInstallation(ctx, i, nil))
			purge := preparePurgeHandoffDecision(t, st, space)
			_, err = st.PurgeSpace(ctx, purge)
			require.ErrorIs(t, err, store.ErrDeletionPending)
			claim, err := st.ClaimNextBotPurgeHandoff(ctx)
			require.NoError(t, err)
			plans, err := claim.NativeDrainRequests(bot.ID)
			require.NoError(t, err)
			require.Len(t, plans, 1)
			first := uuid.MustParse(plans[0].NativeCancellation.InstallationIntent.OperationId)
			var before []byte
			require.NoError(t, st.Pool.QueryRow(ctx, `SELECT source_bytes FROM bot_space_purge_sources WHERE space_id=$1 AND bot_id=$2`, space, bot.ID).Scan(&before))
			var request proto.Message
			method := botv1.BotService_DeleteBot_FullMethodName
			if purpose == "global_delete" {
				request = &botv1.DeleteBotRequest{BotId: bot.ID.String()}
			} else {
				request = &botv1.UninstallBotFromSpaceRequest{BotId: bot.ID.String(), SpaceId: space.String()}
				method = botv1.BotService_UninstallBotFromSpace_FullMethodName
			}
			body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(request)
			require.NoError(t, err)
			hash, err := principal.RequestHash(request)
			require.NoError(t, err)
			admission := store.DeletionAdmission{AccountID: owner, ProfileID: actor, SessionEpoch: 9, AccountType: "regular", Method: method, RequestID: uuid.NewString(), RequestHash: hash, RequestBytes: body, ExpiresAt: time.Now().Add(time.Minute)}
			if purpose == "global_delete" {
				root, e := st.BeginBotDeletionWithAdmission(ctx, bot.ID, owner, admission)
				require.NoError(t, e)
				effects, e := st.LifecycleEffects(ctx, root)
				require.NoError(t, e)
				require.NotEmpty(t, effects)
				require.Equal(t, first, effects[0].Target.CancelOperationID)
			} else {
				cancel, e := st.BeginInstallationCancellationWithAdmission(ctx, bot.ID, space, actor, admission)
				require.NoError(t, e)
				require.Equal(t, first, cancel.CancelOperationID)
			}
			var saved uuid.UUID
			require.NoError(t, st.Pool.QueryRow(ctx, `SELECT cancel_operation_id FROM bot_installation_intents WHERE installation_id=$1`, i.InstallationID).Scan(&saved))
			require.Equal(t, first, saved)
			var after []byte
			require.NoError(t, st.Pool.QueryRow(ctx, `SELECT source_bytes FROM bot_space_purge_sources WHERE space_id=$1 AND bot_id=$2`, space, bot.ID).Scan(&after))
			require.Equal(t, before, after, "accepted root links to original P3 identity without rewriting its immutable snapshot")
		})
	}
}

func TestPurgeDivergentCommittedNativeCancelIsNeverRewritten(t *testing.T) {
	if testing.Short() {
		t.Skip("real divergent immutable native cancellation boundary")
	}
	st := startBotLifecycleStore(t)
	ctx := t.Context()
	owner, actor, space := uuid.New(), uuid.New(), uuid.New()
	bot, _, e := st.CreateBot(ctx, owner, "Divergent native identity", "", `[]`, actor)
	require.NoError(t, e)
	i, e := st.PrepareInstallation(ctx, bot.ID, space, actor, nil)
	require.NoError(t, e)
	require.NoError(t, st.CompleteInstallation(ctx, i, nil))
	p := preparePurgeHandoffDecision(t, st, space)
	_, e = st.PurgeSpace(ctx, p)
	require.ErrorIs(t, e, store.ErrDeletionPending)
	var before []byte
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT source_bytes FROM bot_space_purge_sources WHERE space_id=$1 AND bot_id=$2`, space, bot.ID).Scan(&before))
	other := uuid.New()
	_, e = st.Pool.Exec(ctx, `UPDATE bot_installation_intents SET state='CANCELING',cancel_operation_id=$2 WHERE installation_id=$1`, i.InstallationID, other)
	require.NoError(t, e)
	_, e = st.BeginInstallationCancellation(ctx, bot.ID, space, actor)
	require.ErrorIs(t, e, store.ErrInstallationConflict)
	q := &botv1.DeleteBotRequest{BotId: bot.ID.String()}
	raw, e := (proto.MarshalOptions{Deterministic: true}).Marshal(q)
	require.NoError(t, e)
	hash, e := principal.RequestHash(q)
	require.NoError(t, e)
	_, e = st.BeginBotDeletionWithAdmission(ctx, bot.ID, owner, store.DeletionAdmission{AccountID: owner, ProfileID: actor, SessionEpoch: 9, AccountType: "regular", Method: botv1.BotService_DeleteBot_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash, RequestBytes: raw, ExpiresAt: time.Now().Add(time.Minute)})
	require.ErrorIs(t, e, store.ErrInstallationConflict)
	var saved uuid.UUID
	var after []byte
	var roots int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT cancel_operation_id FROM bot_installation_intents WHERE installation_id=$1`, i.InstallationID).Scan(&saved))
	require.Equal(t, other, saved)
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT source_bytes FROM bot_space_purge_sources WHERE space_id=$1 AND bot_id=$2`, space, bot.ID).Scan(&after))
	require.Equal(t, before, after)
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_deletion_operations WHERE bot_id=$1`, bot.ID).Scan(&roots))
	require.Zero(t, roots)
}
