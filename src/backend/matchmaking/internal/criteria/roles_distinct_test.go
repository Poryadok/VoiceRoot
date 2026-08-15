package criteria_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"voice/backend/matchmaking/internal/criteria"
	"voice/backend/matchmaking/internal/config"
)

func TestCompatible_AllowsDistinctRolesForStackMode(t *testing.T) {
	t.Parallel()
	mode := config.Mode{
		Name:          "5v5 Ranked",
		Slots:         10,
		RolesRequired: true,
		Ranks:         []config.Rank{{Name: "Herald", Value: 0}},
	}
	a := criteria.SearchCriteria{Region: "eu", Self: criteria.SelfCriteria{Role: "Carry", Rank: "Herald"}}
	b := criteria.SearchCriteria{Region: "eu", Self: criteria.SelfCriteria{Role: "Mid", Rank: "Herald"}}
	require.True(t, criteria.Compatible(a, b, mode))
	require.False(t, criteria.Compatible(a, a, mode))
}

func TestRolesDistinct_RejectsDuplicateRoles(t *testing.T) {
	t.Parallel()
	mode := config.Mode{RolesRequired: true}
	group := []criteria.SearchCriteria{
		{Self: criteria.SelfCriteria{Role: "Carry"}},
		{Self: criteria.SelfCriteria{Role: "Mid"}},
		{Self: criteria.SelfCriteria{Role: "Carry"}},
	}
	require.False(t, criteria.RolesDistinct(group, mode))
}
