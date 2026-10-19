package spacelifecycle

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	commonv1 "voice.app/voice/common/v1"
)

// Removal/absence and the original Voice expiry do not revoke a provider-refreshed
// bearer. A callback acknowledging local effects is not a retirement proof.
func TestEjectOnlyCannotCompleteFreezeOrAuthorizeLive(t *testing.T) {
	ctx := context.Background()
	pool := startSpaceLifecyclePostgres(t, ctx)
	s := NewPostgresStore(pool)
	space, operation := uuid.NewString(), uuid.NewString()
	manifest := &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: make([]byte, 32)}
	frozen := fenceRequest(space, operation, 1, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, manifest)
	ejects := 0
	ejectOnly := func(context.Context, *commonv1.SpaceLifecycleFenceRequest) error { ejects++; return nil }
	for range 2 {
		receipt, err := s.ApplyFence(ctx, frozen, ejectOnly)
		require.Error(t, err, "even repeated successful ejects cannot prove refreshed credentials retired")
		require.Nil(t, receipt)
	}
	require.Equal(t, 2, ejects, "pending exact retry continues local drain")
	var receipt []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT receipt_bytes FROM voice_space_lifecycle_fence_receipts WHERE space_id=$1 AND generation=1`, space).Scan(&receipt))
	require.Empty(t, receipt)
	live := fenceRequest(space, operation, 2, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE, manifest)
	_, err := s.ApplyFence(ctx, live, ejectOnly)
	require.Error(t, err, "pending FROZEN must never authorize LIVE")
	require.ErrorIs(t, s.CheckAdmission(ctx, space), ErrSpaceFrozen)
}
