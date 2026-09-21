package profileprojection

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerationRoutePromoteAndRollbackAreAtomic(t *testing.T) {
	t.Parallel()

	route := GenerationRoute{Active: 1}
	require.NoError(t, route.Promote(2, map[uint64]GenerationState{
		1: GenerationReady,
		2: GenerationReady,
	}))
	require.Equal(t, uint64(2), route.Active)
	require.Equal(t, uint64(1), route.Rollback)

	require.NoError(t, route.RollbackToPrevious())
	require.Equal(t, uint64(1), route.Active)
	require.Equal(t, uint64(2), route.Rollback)
}

func TestGenerationRouteRejectsInvalidOrUnreadyPromotion(t *testing.T) {
	t.Parallel()

	route := GenerationRoute{Active: 1}
	require.Error(t, route.Promote(0, nil))
	require.Error(t, route.Promote(2, map[uint64]GenerationState{2: GenerationBuilding}))
	require.Equal(t, uint64(1), route.Active)
	require.Zero(t, route.Rollback)
}
