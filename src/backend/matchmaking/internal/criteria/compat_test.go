package criteria

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompatible_ExactRegionAndOverlappingRanks(t *testing.T) {
	t.Parallel()
	mode := testMode()
	a := SearchCriteria{
		Region: "eu",
		Self:   SelfCriteria{Role: "Carry", Rank: "Herald"},
		Sought: SoughtCriteria{RankMin: "Herald", RankMax: "Guardian"},
	}
	b := SearchCriteria{
		Region: "eu",
		Self:   SelfCriteria{Role: "Carry", Rank: "Guardian"},
		Sought: SoughtCriteria{RankMin: "Herald", RankMax: "Ancient"},
	}
	require.True(t, Compatible(a, b, mode))
}

func TestCompatible_RejectsDifferentRegion(t *testing.T) {
	t.Parallel()
	mode := testMode()
	a := SearchCriteria{Region: "eu", Self: SelfCriteria{Role: "Carry", Rank: "Herald"}}
	b := SearchCriteria{Region: "cis", Self: SelfCriteria{Role: "Carry", Rank: "Herald"}}
	require.False(t, Compatible(a, b, mode))
}

func TestCompatible_RejectsDifferentRoles(t *testing.T) {
	t.Parallel()
	mode := testMode()
	a := SearchCriteria{Region: "eu", Self: SelfCriteria{Role: "Carry", Rank: "Herald"}}
	b := SearchCriteria{Region: "eu", Self: SelfCriteria{Role: "Mid", Rank: "Herald"}}
	require.False(t, Compatible(a, b, mode))
}

func TestCompatible_RejectsNonOverlappingRankRanges(t *testing.T) {
	t.Parallel()
	mode := testMode()
	a := SearchCriteria{
		Region: "eu",
		Self:   SelfCriteria{Role: "Carry", Rank: "Herald"},
		Sought: SoughtCriteria{RankMin: "Herald", RankMax: "Herald"},
	}
	b := SearchCriteria{
		Region: "eu",
		Self:   SelfCriteria{Role: "Carry", Rank: "Ancient"},
		Sought: SoughtCriteria{RankMin: "Ancient", RankMax: "Ancient"},
	}
	require.False(t, Compatible(a, b, mode))
}
