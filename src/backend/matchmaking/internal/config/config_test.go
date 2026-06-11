package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParse_ValidDotaConfig(t *testing.T) {
	t.Parallel()
	raw := `{
	  "genre": "MOBA",
	  "platforms": ["pc"],
	  "regions": ["eu", "cis"],
	  "modes": [{
	    "name": "5v5 Ranked",
	    "slots": 10,
	    "party_size_min": 1,
	    "party_size_max": 5,
	    "roles_required": true,
	    "rank_required": true,
	    "roles": [
	      {"name": "Carry", "required": true},
	      {"name": "Mid", "required": false},
	      {"name": "Support", "required": false}
	    ],
	    "ranks": [
	      {"name": "Herald", "value": 0},
	      {"name": "Guardian", "value": 770}
	    ]
	  }]
	}`
	cfg, err := Parse(raw)
	require.NoError(t, err)
	require.Equal(t, "MOBA", cfg.Genre)
	require.Len(t, cfg.Modes[0].Roles, 3)
	require.Equal(t, "Carry", cfg.Modes[0].Roles[0].Name)
}

func TestParse_EmptyModesRejected(t *testing.T) {
	t.Parallel()
	_, err := Parse(`{"regions":["eu"],"modes":[]}`)
	require.ErrorIs(t, err, ErrNoModes)
}

func TestParse_MissingRegionsRejected(t *testing.T) {
	t.Parallel()
	_, err := Parse(`{"regions":[],"modes":[{"name":"Solo","slots":1,"party_size_min":1,"party_size_max":1}]}`)
	require.ErrorIs(t, err, ErrNoRegions)
}

func TestParse_DuplicateRoleNamesRejected(t *testing.T) {
	t.Parallel()
	raw := `{
	  "regions": ["eu"],
	  "modes": [{
	    "name": "5v5",
	    "slots": 10,
	    "party_size_min": 1,
	    "party_size_max": 5,
	    "roles": [
	      {"name": "Carry", "required": true},
	      {"name": "Carry", "required": false}
	    ]
	  }]
	}`
	_, err := Parse(raw)
	require.ErrorIs(t, err, ErrDuplicateName)
}

func TestParse_NonMonotonicRanksRejected(t *testing.T) {
	t.Parallel()
	raw := `{
	  "regions": ["eu"],
	  "modes": [{
	    "name": "Ranked",
	    "slots": 10,
	    "party_size_min": 1,
	    "party_size_max": 5,
	    "ranks": [
	      {"name": "Gold", "value": 2000},
	      {"name": "Silver", "value": 1000}
	    ]
	  }]
	}`
	_, err := Parse(raw)
	require.Error(t, err)
	require.Contains(t, err.Error(), "non-decreasing")
}

func TestMustMarshal_RoundTrip(t *testing.T) {
	t.Parallel()
	cfg := GameConfig{
		Regions: []string{"eu"},
		Modes: []Mode{{
			Name: "Solo", Slots: 1, PartySizeMin: 1, PartySizeMax: 1,
			Roles: []Role{{Name: "Flex", Required: true}},
			Ranks: []Rank{{Name: "Bronze", Value: 0}},
		}},
	}
	raw := MustMarshal(cfg)
	parsed, err := Parse(raw)
	require.NoError(t, err)
	require.Equal(t, cfg.Modes[0].Name, parsed.Modes[0].Name)
}

func TestParse_EmptyStringRejected(t *testing.T) {
	t.Parallel()
	_, err := Parse("   ")
	require.ErrorIs(t, err, ErrEmptyConfig)
}

func TestParse_RolesRequiredWithoutRolesRejected(t *testing.T) {
	t.Parallel()
	raw := `{
	  "regions": ["eu"],
	  "modes": [{
	    "name": "Ranked",
	    "slots": 10,
	    "party_size_min": 1,
	    "party_size_max": 5,
	    "roles_required": true
	  }]
	}`
	_, err := Parse(raw)
	require.Error(t, err)
}
