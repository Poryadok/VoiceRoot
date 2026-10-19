package store_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/bot/internal/store"
)

func TestBotDeletionRecoveryRejectsCorruptRetainedEffectBytes(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL retained effect integrity")
	}
	for _, column := range []string{"request_bytes", "receipt_bytes"} {
		t.Run(column, func(t *testing.T) {
			st := startBotStore(t)
			ctx := t.Context()
			owner := uuid.New()
			bot, _, err := st.CreateBot(ctx, owner, "Integrity", "", `[]`, uuid.New())
			require.NoError(t, err)
			_, err = st.PrepareInstallation(ctx, bot.ID, uuid.New(), uuid.New(), nil)
			require.NoError(t, err)
			root, err := st.BeginBotDeletion(ctx, bot.ID, owner)
			require.NoError(t, err)
			claim, err := st.ClaimBotDeletion(ctx, root.OperationID)
			require.NoError(t, err)
			effects, err := st.BotDeletionEffects(ctx, root.OperationID)
			require.NoError(t, err)
			effect := effects[0]
			require.NoError(t, st.SealBotDeletionRequest(ctx, claim, effect.EffectID, []byte("frozen request")))
			require.NoError(t, st.SaveBotDeletionReceipt(ctx, claim, effect.EffectID, []byte("frozen request"), []byte("validated receipt")))
			// Deliberate fixture corruption leaves the retained SHA untouched.
			_, err = st.Pool.Exec(ctx, "UPDATE bot_deletion_effects SET "+column+"=$2 WHERE effect_id=$1", effect.EffectID, []byte("corrupt retained bytes"))
			require.NoError(t, err)
			_, err = st.BotDeletionEffects(ctx, root.OperationID)
			require.ErrorIs(t, err, store.ErrInstallationConflict, "worker cannot trust COMPLETE without original frozen bytes")
		})
	}
}
