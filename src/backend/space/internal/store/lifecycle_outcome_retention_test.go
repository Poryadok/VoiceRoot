package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	spacev1 "voice.app/voice/space/v1"
)

func TestLifecycleOutcomeRetention_RestoredSpaceDropsExpiredPrivateOutcomes(t *testing.T) {
	f := newLifecycleRestoreFixture(t)
	ctx := context.Background()
	for _, name := range []string{"000014_ownership_outbox_delivery.up.sql", "000015_audit_ledger.up.sql", "000022_lifecycle_runtime.up.sql", "000023_lifecycle_evidence_retention.up.sql"} {
		raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db", name))
		require.NoError(t, err)
		_, err = f.st.Pool.Exec(ctx, string(raw))
		require.NoError(t, err)
	}
	f.admit(t, len(lifecycleTestParticipants))
	_, _, err := f.st.CompleteLifecycleRestore(ctx, f.space)
	require.NoError(t, err)
	count := func() int {
		var result int
		require.NoError(t, f.st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_lifecycle_operations WHERE space_id=$1`, f.space).Scan(&result))
		return result
	}
	require.NoError(t, f.st.CleanupLifecycleEvidence(ctx))
	require.Equal(t, 2, count(), "unexpired completed outcomes remain replayable")
	_, err = f.st.Pool.Exec(ctx, `UPDATE space_lifecycle_operations SET completed_at=clock_timestamp()-interval '30 days' WHERE space_id=$1`, f.space)
	require.NoError(t, err)
	require.NoError(t, f.st.CleanupLifecycleEvidence(ctx))
	require.Zero(t, count(), "restored outcome/proof bytes cannot wait indefinitely for PURGED")
	active, err := f.st.LoadLifecycle(ctx, f.space)
	require.NoError(t, err)
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_LIVE, active.Phase())
	require.Equal(t, uint64(2), active.Generation())
	require.NotEmpty(t, readyLifecycleEvents(t, f.store), "outcome expiry cannot remove an undelivered event")
	response, err := f.st.ReplayLifecycleRestoreOutcome(ctx, f.account, f.actor, 7, f.restore)
	require.NoError(t, err)
	require.Nil(t, response, "missing expired private bytes are never a successful replay")
}
