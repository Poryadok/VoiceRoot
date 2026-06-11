package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// GameConfig is the validated catalog config stored in games.config JSONB.
type GameConfig struct {
	Genre     string   `json:"genre,omitempty"`
	Platforms []string `json:"platforms,omitempty"`
	Regions   []string `json:"regions"`
	Modes     []Mode   `json:"modes"`
}

type Mode struct {
	Name          string     `json:"name"`
	Slots         int        `json:"slots"`
	PartySizeMin  int        `json:"party_size_min"`
	PartySizeMax  int        `json:"party_size_max"`
	RolesRequired bool       `json:"roles_required"`
	RankRequired  bool       `json:"rank_required"`
	Roles         []Role     `json:"roles"`
	Ranks         []Rank     `json:"ranks"`
}

type Role struct {
	Name     string `json:"name"`
	Required bool   `json:"required"`
}

type Rank struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

var (
	ErrEmptyConfig   = errors.New("config is empty")
	ErrNoModes       = errors.New("at least one mode is required")
	ErrNoRegions     = errors.New("at least one region is required")
	ErrInvalidMode   = errors.New("invalid mode")
	ErrDuplicateName = errors.New("duplicate name in mode")
)

// Parse unmarshals and validates catalog config JSON.
func Parse(raw string) (GameConfig, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return GameConfig{}, ErrEmptyConfig
	}
	var cfg GameConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return GameConfig{}, fmt.Errorf("invalid json: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return GameConfig{}, err
	}
	return cfg, nil
}

// Validate checks business rules for catalog config.
func (c GameConfig) Validate() error {
	if len(c.Modes) == 0 {
		return ErrNoModes
	}
	if len(c.Regions) == 0 {
		return ErrNoRegions
	}
	for i, m := range c.Modes {
		if err := m.validate(); err != nil {
			return fmt.Errorf("mode[%d]: %w", i, err)
		}
	}
	return nil
}

func (m Mode) validate() error {
	if strings.TrimSpace(m.Name) == "" {
		return fmt.Errorf("%w: name required", ErrInvalidMode)
	}
	if m.Slots <= 0 {
		return fmt.Errorf("%w: slots must be positive", ErrInvalidMode)
	}
	if m.PartySizeMin <= 0 || m.PartySizeMax < m.PartySizeMin {
		return fmt.Errorf("%w: invalid party size range", ErrInvalidMode)
	}
	if m.PartySizeMax > m.Slots {
		return fmt.Errorf("%w: party_size_max cannot exceed slots", ErrInvalidMode)
	}
	if m.RolesRequired && len(m.Roles) == 0 {
		return fmt.Errorf("%w: roles required but none defined", ErrInvalidMode)
	}
	if m.RankRequired && len(m.Ranks) == 0 {
		return fmt.Errorf("%w: rank required but none defined", ErrInvalidMode)
	}
	seenRoles := map[string]struct{}{}
	for _, r := range m.Roles {
		name := strings.TrimSpace(r.Name)
		if name == "" {
			return fmt.Errorf("%w: role name required", ErrInvalidMode)
		}
		if _, ok := seenRoles[name]; ok {
			return fmt.Errorf("%w: duplicate role %q", ErrDuplicateName, name)
		}
		seenRoles[name] = struct{}{}
	}
	seenRanks := map[string]struct{}{}
	prevValue := -1
	for _, r := range m.Ranks {
		name := strings.TrimSpace(r.Name)
		if name == "" {
			return fmt.Errorf("%w: rank name required", ErrInvalidMode)
		}
		if _, ok := seenRanks[name]; ok {
			return fmt.Errorf("%w: duplicate rank %q", ErrDuplicateName, name)
		}
		seenRanks[name] = struct{}{}
		if prevValue >= 0 && r.Value < prevValue {
			return fmt.Errorf("%w: rank values must be non-decreasing", ErrInvalidMode)
		}
		prevValue = r.Value
	}
	return nil
}

// MustMarshal returns canonical JSON for persistence.
func MustMarshal(cfg GameConfig) string {
	b, err := json.Marshal(cfg)
	if err != nil {
		panic(err)
	}
	return string(b)
}
