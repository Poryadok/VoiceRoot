package criteria_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"voice/backend/matchmaking/internal/config"
	"voice/backend/matchmaking/internal/criteria"
)

func TestCompatible_AllowsSameAndDistinctRolesForStackMode(t *testing.T) {
	t.Parallel()
	mode := config.Mode{
		Name:          "5v5 Ranked",
		Slots:         10,
		RolesRequired: true,
		Roles: []config.Role{
			{Name: "Carry"},
			{Name: "Mid"},
		},
		Ranks: []config.Rank{{Name: "Herald", Value: 0}},
	}
	a := criteria.SearchCriteria{Region: "eu", Self: criteria.SelfCriteria{Role: "Carry", Rank: "Herald"}}
	b := criteria.SearchCriteria{Region: "eu", Self: criteria.SelfCriteria{Role: "Mid", Rank: "Herald"}}
	require.True(t, criteria.Compatible(a, b, mode))
	require.True(t, criteria.Compatible(a, a, mode))
	require.False(t, criteria.Compatible(a, criteria.SearchCriteria{Region: "eu", Self: criteria.SelfCriteria{Role: "Jungler", Rank: "Herald"}}, mode))
}
