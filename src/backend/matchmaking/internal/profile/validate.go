package profile

import (
	"errors"
	"fmt"
	"strings"

	"voice/backend/matchmaking/internal/config"
)

var (
	ErrRegionRequired = errors.New("region is required")
	ErrInvalidRegion  = errors.New("invalid region")
	ErrInvalidRole    = errors.New("invalid role")
	ErrInvalidRank    = errors.New("invalid rank")
	ErrNoModes        = errors.New("game has no modes")
)

// EntryInput is the validated shape of a player game profile entry.
type EntryInput struct {
	Region string
	Role   *string
	Rank   *string
}

// ValidateEntry checks region/role/rank against catalog config (first mode for role/rank).
func ValidateEntry(cfg config.GameConfig, in EntryInput) error {
	region := strings.TrimSpace(in.Region)
	if region == "" {
		return ErrRegionRequired
	}
	if !containsString(cfg.Regions, region) {
		return fmt.Errorf("%w: %q", ErrInvalidRegion, region)
	}
	if len(cfg.Modes) == 0 {
		return ErrNoModes
	}
	mode := cfg.Modes[0]
	if in.Role != nil {
		role := strings.TrimSpace(*in.Role)
		if role != "" && !roleInMode(mode, role) {
			return fmt.Errorf("%w: %q", ErrInvalidRole, role)
		}
	}
	if in.Rank != nil {
		rank := strings.TrimSpace(*in.Rank)
		if rank != "" && !rankInMode(mode, rank) {
			return fmt.Errorf("%w: %q", ErrInvalidRank, rank)
		}
	}
	return nil
}

func containsString(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func roleInMode(m config.Mode, name string) bool {
	for _, r := range m.Roles {
		if r.Name == name {
			return true
		}
	}
	return false
}

func rankInMode(m config.Mode, name string) bool {
	for _, r := range m.Ranks {
		if r.Name == name {
			return true
		}
	}
	return false
}
