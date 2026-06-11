package profile_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"voice/backend/matchmaking/internal/config"
	"voice/backend/matchmaking/internal/profile"
)

func sampleConfig() config.GameConfig {
	return config.GameConfig{
		Regions: []string{"eu", "cis"},
		Modes: []config.Mode{{
			Name: "5v5",
			Roles: []config.Role{
				{Name: "Carry", Required: true},
				{Name: "Support", Required: false},
			},
			Ranks: []config.Rank{
				{Name: "Herald", Value: 0},
				{Name: "Ancient", Value: 3850},
			},
		}},
	}
}

func TestValidateEntry_RequiresRegion(t *testing.T) {
	t.Parallel()
	err := profile.ValidateEntry(sampleConfig(), profile.EntryInput{})
	require.ErrorIs(t, err, profile.ErrRegionRequired)
}

func TestValidateEntry_RejectsUnknownRegion(t *testing.T) {
	t.Parallel()
	err := profile.ValidateEntry(sampleConfig(), profile.EntryInput{Region: "na"})
	require.ErrorIs(t, err, profile.ErrInvalidRegion)
}

func TestValidateEntry_AcceptsValidRoleAndRank(t *testing.T) {
	t.Parallel()
	role := "Carry"
	rank := "Herald"
	err := profile.ValidateEntry(sampleConfig(), profile.EntryInput{
		Region: "eu",
		Role:   &role,
		Rank:   &rank,
	})
	require.NoError(t, err)
}

func TestValidateEntry_RejectsUnknownRole(t *testing.T) {
	t.Parallel()
	role := "Jungle"
	err := profile.ValidateEntry(sampleConfig(), profile.EntryInput{
		Region: "eu",
		Role:   &role,
	})
	require.ErrorIs(t, err, profile.ErrInvalidRole)
}

func TestValidateEntry_RejectsUnknownRank(t *testing.T) {
	t.Parallel()
	rank := "Radiant"
	err := profile.ValidateEntry(sampleConfig(), profile.EntryInput{
		Region: "eu",
		Rank:   &rank,
	})
	require.ErrorIs(t, err, profile.ErrInvalidRank)
}
