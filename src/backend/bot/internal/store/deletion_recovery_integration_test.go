package store_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/bot/internal/store"
)

func TestBotDeletionRecoveryFencesStaleClaimAndFreezesRequestsAndReceipts(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL recovery journal")
	}
	st := startBotStore(t)
	ctx := t.Context()
	owner := uuid.New()
	bot, _, err := st.CreateBot(ctx, owner, "Recover", "", `[]`, uuid.New())
	require.NoError(t, err)
	_, err = st.PrepareInstallation(ctx, bot.ID, uuid.New(), uuid.New(), nil)
	require.NoError(t, err)
	operation, err := st.BeginBotDeletion(ctx, bot.ID, owner)
	require.NoError(t, err)
	first, err := st.ClaimBotDeletion(ctx, operation.OperationID)
	require.NoError(t, err)
	_, err = st.ClaimBotDeletion(ctx, operation.OperationID)
	require.ErrorIs(t, err, store.ErrDeletionClaimLost)
	effects, err := st.BotDeletionEffects(ctx, operation.OperationID)
	require.NoError(t, err)
	require.Len(t, effects, 2)
	effect := effects[0]
	request := []byte("exact persisted request bytes")
	require.NoError(t, st.SealBotDeletionRequest(ctx, first, effect.EffectID, request))
	require.ErrorIs(t, st.SealBotDeletionRequest(ctx, first, effect.EffectID, []byte("changed request")), store.ErrInstallationConflict)
	require.ErrorIs(t, st.CompleteBotDeletion(ctx, first), store.ErrDeletionPending)
	_, err = st.Pool.Exec(ctx, `UPDATE bot_deletion_operations SET claimed_until=now()-interval '1 second' WHERE operation_id=$1`, operation.OperationID)
	require.NoError(t, err)
	second, err := st.ClaimBotDeletion(ctx, operation.OperationID)
	require.NoError(t, err)
	require.Greater(t, second.ClaimGeneration, first.ClaimGeneration)
	require.ErrorIs(t, st.SaveBotDeletionReceipt(ctx, first, effect.EffectID, request, []byte("stale receipt")), store.ErrDeletionClaimLost)
	require.NoError(t, st.SaveBotDeletionReceipt(ctx, second, effect.EffectID, request, []byte("exact receipt")))
	require.NoError(t, st.SaveBotDeletionReceipt(ctx, second, effect.EffectID, request, []byte("exact receipt")))
	require.ErrorIs(t, st.SaveBotDeletionReceipt(ctx, second, effect.EffectID, request, []byte("changed receipt")), store.ErrInstallationConflict)
	require.ErrorIs(t, st.SaveBotDeletionReceipt(ctx, second, effects[1].EffectID, request, []byte("unsealed receipt")), store.ErrInstallationConflict)
	// Recovery reuses the first frozen request/receipt; no mutable source row is
	// needed after atomic revoke removed the local installation.
	replay, err := st.BotDeletionEffects(ctx, operation.OperationID)
	require.NoError(t, err)
	require.Equal(t, request, replay[0].RequestBytes)
	require.Equal(t, []byte("exact receipt"), replay[0].ReceiptBytes)
	require.NoError(t, st.SealBotDeletionRequest(ctx, second, effects[1].EffectID, request))
	require.NoError(t, st.SaveBotDeletionReceipt(ctx, second, effects[1].EffectID, request, []byte("other receipt")))
	// The original scoped journal assertions above remain unchanged. Its
	// completed per-Space effects no longer stand in for independent global
	// Space/Role terminal receipts, including a root with no remaining targets.
	require.ErrorIs(t, st.CompleteBotDeletion(ctx, second), store.ErrDeletionPending)
	terminals, err := st.GlobalTerminalEffects(ctx, second)
	require.NoError(t, err)
	require.Len(t, terminals, 2)
	for _, terminal := range terminals {
		require.Equal(t, "PENDING", terminal.State)
		require.Nil(t, terminal.ReceiptBytes)
	}
	_, err = st.ClaimBotDeletion(ctx, operation.OperationID)
	require.ErrorIs(t, err, store.ErrDeletionClaimLost)
	var state string
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT state FROM bot_deletion_operations WHERE operation_id=$1`, operation.OperationID).Scan(&state))
	require.Equal(t, "RUNNING", state)
}
