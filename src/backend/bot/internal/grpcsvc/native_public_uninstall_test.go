package grpcsvc

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/bot/internal/store"
)

// HTTP acceptance is not used as a cleanup receipt. Independently verify the
// immutable Bot journal and ask the source-owned provider to read actual rows.
func checkPublicUninstall(t *testing.T, st *store.BotStore, bot store.BotRow, intent *store.InstallationIntent, seed roleNonzeroSeed, roleDir string) {
	t.Helper()
	ctx := t.Context()
	root, err := st.UninstallCleanup(ctx, intent.InstallationID)
	require.NoError(t, err)
	require.Equal(t, "COMPLETE", root.State)
	require.Equal(t, "space_uninstall", root.Purpose)
	effects, err := st.LifecycleEffects(ctx, root)
	require.NoError(t, err)
	require.Len(t, effects, 2, "empty whitelist has native cancellation and Role cleanup only")
	for _, effect := range effects {
		require.Equal(t, "COMPLETE", effect.State)
		if effect.Kind == "ROLE_REMOVE" {
			var response rolev1.ApplyBotInstallationRemovalResponse
			require.NoError(t, proto.Unmarshal(effect.ReceiptBytes, &response))
			require.Equal(t, "space_uninstall", response.GetReceipt().GetPurpose())
			require.EqualValues(t, 1, response.GetReceipt().GetDeletedRoleCount())
			require.EqualValues(t, 2, response.GetReceipt().GetRemovedAssignmentCount())
			checkActualRoleCleanupTarget(t, roleDir, seed, effect.EffectID.String(), response.GetReceipt().GetReceiptId(), root.OperationID.String())
		}
	}
	var state string
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT state FROM bot_installation_intents WHERE installation_id=$1`, intent.InstallationID).Scan(&state))
	require.Equal(t, "CANCELED", state)
	current, err := st.GetBotByID(ctx, bot.ID)
	require.NoError(t, err)
	require.Equal(t, "live", current.Status)
	var globals, installations int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_deletion_operations WHERE bot_id=$1`, bot.ID).Scan(&globals))
	require.Zero(t, globals)
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_space_installations WHERE bot_id=$1`, bot.ID).Scan(&installations))
	require.Zero(t, installations)
	require.Equal(t, "1", os.Getenv("BOT_GATEWAY_NATIVE_UNINSTALL"))
}
