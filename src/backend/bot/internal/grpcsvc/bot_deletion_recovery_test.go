package grpcsvc_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	botv1 "voice.app/voice/bot/v1"
	grpcsvc "voice/backend/bot/internal/grpcsvc"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
)

func TestDeleteBotRetainsRecoveryAndDoesNotAcknowledgeUnavailableCleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL deletion boundary")
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "bot_delete_recovery", "")
	_, err := pool.Exec(ctx, migrationSQL(t))
	require.NoError(t, err)
	st := &store.BotStore{Pool: pool}
	owner, installer := uuid.New(), uuid.New()
	bot, token, err := st.CreateBot(ctx, owner, "Deleting", "", `[]`, uuid.New())
	require.NoError(t, err)
	liveSpace, pendingSpace, orphanSpace, chat, disabledChat := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	installed, err := st.PrepareInstallation(ctx, bot.ID, liveSpace, installer, []uuid.UUID{chat})
	require.NoError(t, err)
	require.NoError(t, st.CompleteInstallation(ctx, installed, []uuid.UUID{chat}))
	pending, err := st.PrepareInstallation(ctx, bot.ID, pendingSpace, installer, nil)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO bot_chat_whitelist(bot_id,chat_id,space_id,enabled,added_by_profile_id) VALUES($1,$2,$3,false,$4)`, bot.ID, disabledChat, orphanSpace, installer)
	require.NoError(t, err)
	other, _, err := st.CreateBot(ctx, owner, "Unrelated", "", `[]`, uuid.New())
	require.NoError(t, err)
	svc := grpcsvc.NewBotGRPC(st, nil)
	request := &botv1.DeleteBotRequest{BotId: bot.ID.String()}
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	// This is a handler persistence component fixture. Real network verification
	// is covered separately by the mTLS and cross-process HTTP tests.
	ctx = principal.WithVerified(ctx, principal.Principal{Kind: "delegated_user", Issuer: "gateway", Subject: owner.String(), AccountID: owner.String(), ProfileID: installer.String(), SessionEpoch: 1, AccountType: "regular", Audience: "bot", RPC: botv1.BotService_DeleteBot_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash, ExpiresAt: time.Now().Add(time.Minute)})
	_, err = svc.DeleteBot(ctx, request)
	require.Equal(t, codes.Unavailable, status.Code(err), "DeleteBot must not claim completed cleanup without participant acknowledgements")
	_, err = st.GetBotByTokenHash(ctx, store.HashToken(token))
	require.ErrorIs(t, err, store.ErrNotFound)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM bot_deletion_operations WHERE bot_id=$1 AND state<>'COMPLETE'`, bot.ID).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM bot_deletion_installations WHERE bot_id=$1`, bot.ID).Scan(&count))
	require.Equal(t, 3, count, "capture installed, admitted pending and orphan whitelist Space")
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM bot_deletion_installations WHERE bot_id=$1 AND installation_id=ANY($2)`, bot.ID, []uuid.UUID{installed.InstallationID, pending.InstallationID}).Scan(&count))
	require.Equal(t, 2, count)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM bot_space_installations WHERE bot_id=$1`, bot.ID).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM bot_chat_whitelist WHERE bot_id=$1`, bot.ID).Scan(&count))
	require.Zero(t, count)
	_, err = st.PrepareInstallation(ctx, bot.ID, uuid.New(), installer, nil)
	require.ErrorIs(t, err, store.ErrInstallationConflict)
	unchanged, err := st.GetBotByID(ctx, other.ID)
	require.NoError(t, err)
	require.Equal(t, "live", unchanged.Status)
}
