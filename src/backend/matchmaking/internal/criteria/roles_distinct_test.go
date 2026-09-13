package criteria_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"voice/backend/matchmaking/internal/config"
	"voice/backend/matchmaking/internal/criteria"
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

func TestRolesDistinct_RequiresDiversityAcrossFull10SlotLobby(t *testing.T) {
	t.Parallel()
	mode := config.Mode{Slots: 10, RolesRequired: true}
	group := make([]criteria.SearchCriteria, mode.Slots)
	for i := range group {
		group[i] = criteria.SearchCriteria{
			Self: criteria.SelfCriteria{Role: string(rune('A' + i))},
		}
	}

	require.True(t, criteria.RolesDistinct(group, mode))

	group[len(group)-1].Self.Role = group[0].Self.Role
	require.False(t, criteria.RolesDistinct(group, mode))
}
