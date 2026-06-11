package criteria

import (
	"testing"

	"github.com/stretchr/testify/require"

	"voice/backend/matchmaking/internal/config"
)

func testMode() config.Mode {
	return config.Mode{
		Name:          "5v5 Ranked",
		Slots:         10,
		PartySizeMin:  1,
		PartySizeMax:  5,
		RolesRequired: true,
		RankRequired:  true,
		Roles: []config.Role{
			{Name: "Carry", Required: true},
			{Name: "Mid", Required: false},
		},
		Ranks: []config.Rank{
			{Name: "Herald", Value: 0},
			{Name: "Guardian", Value: 770},
			{Name: "Ancient", Value: 3850},
		},
	}
}

func testGameCfg() config.GameConfig {
	return config.GameConfig{
		Regions: []string{"eu", "cis"},
		Modes:   []config.Mode{testMode()},
	}
}

func TestParse_ValidCriteria(t *testing.T) {
	t.Parallel()
	raw := `{"region":"cis","self":{"role":"Carry","rank":"Herald"},"sought":{"rank_min":"Herald","rank_max":"Guardian"}}`
	c, err := Parse(raw)
	require.NoError(t, err)
	require.Equal(t, "cis", c.Region)
	require.Equal(t, "Carry", c.Self.Role)
	require.Equal(t, "Herald", c.Self.Rank)
}

func TestValidate_RejectsInvalidRegion(t *testing.T) {
	t.Parallel()
	c := SearchCriteria{Region: "na", Self: SelfCriteria{Role: "Carry", Rank: "Herald"}}
	_, err := Validate(c, testGameCfg(), "5v5 Ranked", 1)
	require.ErrorIs(t, err, ErrInvalidRegion)
}

func TestValidate_RejectsMissingRoleWhenRequired(t *testing.T) {
	t.Parallel()
	c := SearchCriteria{Region: "eu", Self: SelfCriteria{Rank: "Herald"}}
	_, err := Validate(c, testGameCfg(), "5v5 Ranked", 1)
	require.ErrorIs(t, err, ErrRoleRequired)
}

func TestValidate_RejectsPartySizeBelowMin(t *testing.T) {
	t.Parallel()
	cfg := testGameCfg()
	cfg.Modes[0].PartySizeMin = 2
	c := SearchCriteria{Region: "eu", Self: SelfCriteria{Role: "Carry", Rank: "Herald"}}
	_, err := Validate(c, cfg, "5v5 Ranked", 1)
	require.ErrorIs(t, err, ErrInvalidPartySize)
}

func TestValidate_ModeNotFound(t *testing.T) {
	t.Parallel()
	c := SearchCriteria{Region: "eu", Self: SelfCriteria{Role: "Carry", Rank: "Herald"}}
	_, err := Validate(c, testGameCfg(), "missing-mode", 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestCompatible_SkipsRoleWhenNotRequired(t *testing.T) {
	t.Parallel()
	mode := testMode()
	mode.RolesRequired = false
	mode.RankRequired = false
	a := SearchCriteria{Region: "eu", Self: SelfCriteria{Role: "Carry"}}
	b := SearchCriteria{Region: "eu", Self: SelfCriteria{Role: "Mid"}}
	require.True(t, Compatible(a, b, mode))
}

func TestParse_RejectsEmpty(t *testing.T) {
	t.Parallel()
	_, err := Parse("  ")
	require.ErrorIs(t, err, ErrEmptyCriteria)
}

func TestValidate_RejectsInvalidSoughtRange(t *testing.T) {
	t.Parallel()
	c := SearchCriteria{
		Region: "eu",
		Self:   SelfCriteria{Role: "Carry", Rank: "Ancient"},
		Sought: SoughtCriteria{RankMin: "Ancient", RankMax: "Herald"},
	}
	_, err := Validate(c, testGameCfg(), "5v5 Ranked", 1)
	require.ErrorIs(t, err, ErrInvalidSought)
}

func TestValidate_AcceptsValidSoloCriteria(t *testing.T) {
	t.Parallel()
	c := SearchCriteria{
		Region: "eu",
		Self:   SelfCriteria{Role: "Carry", Rank: "Herald"},
		Sought: SoughtCriteria{RankMin: "Herald", RankMax: "Guardian"},
	}
	mode, err := Validate(c, testGameCfg(), "5v5 Ranked", 1)
	require.NoError(t, err)
	require.Equal(t, "5v5 Ranked", mode.Name)
}
