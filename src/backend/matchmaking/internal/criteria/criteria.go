package criteria

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"voice/backend/matchmaking/internal/config"
)

var (
	ErrEmptyCriteria   = errors.New("criteria is empty")
	ErrInvalidJSON     = errors.New("invalid criteria json")
	ErrRegionRequired  = errors.New("region is required")
	ErrInvalidRegion   = errors.New("region not in game config")
	ErrRoleRequired    = errors.New("self.role is required for this mode")
	ErrInvalidRole     = errors.New("self.role not in game mode")
	ErrRankRequired    = errors.New("self.rank is required for this mode")
	ErrInvalidRank     = errors.New("self.rank not in game mode")
	ErrInvalidSought   = errors.New("invalid sought rank range")
	ErrInvalidPartySize = errors.New("party size not allowed for mode")
)

// SelfCriteria is the player's own MM parameters.
type SelfCriteria struct {
	Role string `json:"role,omitempty"`
	Rank string `json:"rank,omitempty"`
}

// SoughtCriteria is what the player looks for in teammates.
type SoughtCriteria struct {
	RankMin string `json:"rank_min,omitempty"`
	RankMax string `json:"rank_max,omitempty"`
}

// SearchCriteria is the canonical criteria_json schema for solo v1.
type SearchCriteria struct {
	Region string         `json:"region"`
	Self   SelfCriteria   `json:"self"`
	Sought SoughtCriteria `json:"sought,omitempty"`
}

// Parse unmarshals criteria_json.
func Parse(raw string) (SearchCriteria, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return SearchCriteria{}, ErrEmptyCriteria
	}
	var c SearchCriteria
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return SearchCriteria{}, fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}
	return c, nil
}

// MustMarshal returns canonical JSON for persistence.
func MustMarshal(c SearchCriteria) string {
	b, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// Validate checks criteria against game config and party size (solo v1: partySize=1).
func Validate(c SearchCriteria, gameCfg config.GameConfig, modeName string, partySize int) (config.Mode, error) {
	modeName = strings.TrimSpace(modeName)
	var mode config.Mode
	found := false
	for _, m := range gameCfg.Modes {
		if m.Name == modeName {
			mode = m
			found = true
			break
		}
	}
	if !found {
		return config.Mode{}, fmt.Errorf("mode %q not found", modeName)
	}

	region := strings.TrimSpace(c.Region)
	if region == "" {
		return config.Mode{}, ErrRegionRequired
	}
	if !containsString(gameCfg.Regions, region) {
		return config.Mode{}, ErrInvalidRegion
	}

	if partySize < mode.PartySizeMin || partySize > mode.PartySizeMax {
		return config.Mode{}, ErrInvalidPartySize
	}

	if mode.RolesRequired {
		role := strings.TrimSpace(c.Self.Role)
		if role == "" {
			return config.Mode{}, ErrRoleRequired
		}
		if !roleInMode(mode, role) {
			return config.Mode{}, ErrInvalidRole
		}
	}

	if mode.RankRequired {
		rank := strings.TrimSpace(c.Self.Rank)
		if rank == "" {
			return config.Mode{}, ErrRankRequired
		}
		if rankValue(mode, rank) < 0 {
			return config.Mode{}, ErrInvalidRank
		}
		if err := validateSoughtRange(c.Sought, mode); err != nil {
			return config.Mode{}, err
		}
	}

	return mode, nil
}

func validateSoughtRange(sought SoughtCriteria, mode config.Mode) error {
	minName := strings.TrimSpace(sought.RankMin)
	maxName := strings.TrimSpace(sought.RankMax)
	if minName == "" && maxName == "" {
		return nil
	}
	if minName == "" || maxName == "" {
		return fmt.Errorf("%w: rank_min and rank_max must both be set", ErrInvalidSought)
	}
	minVal := rankValue(mode, minName)
	maxVal := rankValue(mode, maxName)
	if minVal < 0 || maxVal < 0 {
		return ErrInvalidSought
	}
	if minVal > maxVal {
		return fmt.Errorf("%w: rank_min value exceeds rank_max", ErrInvalidSought)
	}
	return nil
}

// Compatible reports whether two search criteria can match per docs/features/matchmaking.md.
func Compatible(a, b SearchCriteria, mode config.Mode) bool {
	if strings.TrimSpace(a.Region) != strings.TrimSpace(b.Region) {
		return false
	}
	if !rolesCompatible(a, b, mode) {
		return false
	}
	return ranksCompatible(a, b, mode)
}

func rolesCompatible(a, b SearchCriteria, mode config.Mode) bool {
	if !mode.RolesRequired {
		return true
	}
	roleA := strings.TrimSpace(a.Self.Role)
	roleB := strings.TrimSpace(b.Self.Role)
	if roleA == "" || roleB == "" {
		return false
	}
	return roleA == roleB
}

func ranksCompatible(a, b SearchCriteria, mode config.Mode) bool {
	if !mode.RankRequired {
		return true
	}
	aSelf := rankValue(mode, strings.TrimSpace(a.Self.Rank))
	bSelf := rankValue(mode, strings.TrimSpace(b.Self.Rank))
	if aSelf < 0 || bSelf < 0 {
		return false
	}
	aMin, aMax := soughtRangeValues(a, mode, aSelf)
	bMin, bMax := soughtRangeValues(b, mode, bSelf)
	return aMin <= bMax && bMin <= aMax
}

func soughtRangeValues(c SearchCriteria, mode config.Mode, selfValue int) (int, int) {
	minName := strings.TrimSpace(c.Sought.RankMin)
	maxName := strings.TrimSpace(c.Sought.RankMax)
	if minName == "" && maxName == "" {
		return selfValue, selfValue
	}
	return rankValue(mode, minName), rankValue(mode, maxName)
}

func roleInMode(mode config.Mode, role string) bool {
	for _, r := range mode.Roles {
		if r.Name == role {
			return true
		}
	}
	return false
}

func rankValue(mode config.Mode, rank string) int {
	for _, r := range mode.Ranks {
		if r.Name == rank {
			return r.Value
		}
	}
	return -1
}

func containsString(items []string, want string) bool {
	for _, s := range items {
		if s == want {
			return true
		}
	}
	return false
}
