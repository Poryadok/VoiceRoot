package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NotificationSettings is a persisted notification_settings row.
type NotificationSettings struct {
	ProfileID     uuid.UUID
	ScopeType     string
	ScopeID       *uuid.UUID
	Enabled       bool
	MuteUntil     *time.Time
	SuppressTypes []string
}

// QuietHours is a persisted quiet_hours row.
type QuietHours struct {
	ProfileID        uuid.UUID
	Enabled          bool
	StartTime        string
	EndTime          string
	Timezone         string
	OverrideMentions bool
}

// SettingsStore persists notification_settings and quiet_hours.
type SettingsStore struct {
	Pool *pgxpool.Pool
}

// GetSettings loads settings for a profile/scope. Missing rows return defaults (enabled, no suppress).
func (s *SettingsStore) GetSettings(ctx context.Context, profileID uuid.UUID, scopeType string, scopeID *uuid.UUID) (NotificationSettings, error) {
	if s == nil || s.Pool == nil {
		return NotificationSettings{}, ErrNotImplemented
	}
	scopeType = strings.TrimSpace(scopeType)
	if scopeType == "" {
		scopeType = "global"
	}
	var (
		out       NotificationSettings
		scopeRaw  *uuid.UUID
		muteUntil *time.Time
		suppress  []byte
	)
	err := s.Pool.QueryRow(ctx, `
SELECT profile_id, scope_type, scope_id, enabled, mute_until, suppress_types
FROM notification_settings
WHERE profile_id = $1 AND scope_type = $2 AND (
  ($3::uuid IS NULL AND scope_id IS NULL) OR scope_id = $3
)`, profileID, scopeType, scopeID).Scan(
		&out.ProfileID, &out.ScopeType, &scopeRaw, &out.Enabled, &muteUntil, &suppress,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return NotificationSettings{
			ProfileID: profileID,
			ScopeType: scopeType,
			ScopeID:   scopeID,
			Enabled:   true,
		}, nil
	}
	if err != nil {
		return NotificationSettings{}, err
	}
	out.ScopeID = scopeRaw
	out.MuteUntil = muteUntil
	if len(suppress) > 0 {
		_ = json.Unmarshal(suppress, &out.SuppressTypes)
	}
	return out, nil
}

// UpsertSettings writes notification_settings for a profile/scope.
func (s *SettingsStore) UpsertSettings(ctx context.Context, settings NotificationSettings) error {
	if s == nil || s.Pool == nil {
		return ErrNotImplemented
	}
	suppress, err := json.Marshal(settings.SuppressTypes)
	if err != nil {
		return err
	}
	if settings.ScopeType == "" {
		settings.ScopeType = "global"
	}
	if settings.ScopeType == "global" && settings.ScopeID == nil {
		tag, err := s.Pool.Exec(ctx, `
UPDATE notification_settings
SET enabled = $2, mute_until = $3, suppress_types = $4::jsonb
WHERE profile_id = $1 AND scope_type = 'global' AND scope_id IS NULL`,
			settings.ProfileID, settings.Enabled, settings.MuteUntil, string(suppress),
		)
		if err != nil {
			return err
		}
		if tag.RowsAffected() > 0 {
			return nil
		}
		_, err = s.Pool.Exec(ctx, `
INSERT INTO notification_settings (profile_id, scope_type, scope_id, enabled, mute_until, suppress_types)
VALUES ($1, $2, NULL, $3, $4, $5::jsonb)`,
			settings.ProfileID, settings.ScopeType, settings.Enabled, settings.MuteUntil, string(suppress),
		)
		return err
	}
	_, err = s.Pool.Exec(ctx, `
INSERT INTO notification_settings (profile_id, scope_type, scope_id, enabled, mute_until, suppress_types)
VALUES ($1, $2, $3, $4, $5, $6::jsonb)
ON CONFLICT (profile_id, scope_type, scope_id) DO UPDATE SET
  enabled = EXCLUDED.enabled,
  mute_until = EXCLUDED.mute_until,
  suppress_types = EXCLUDED.suppress_types`,
		settings.ProfileID,
		settings.ScopeType,
		settings.ScopeID,
		settings.Enabled,
		settings.MuteUntil,
		string(suppress),
	)
	return err
}

// GetQuietHours loads quiet hours for a profile. Missing row returns disabled defaults.
func (s *SettingsStore) GetQuietHours(ctx context.Context, profileID uuid.UUID) (QuietHours, error) {
	if s == nil || s.Pool == nil {
		return QuietHours{}, ErrNotImplemented
	}
	var (
		out       QuietHours
		startTime time.Time
		endTime   time.Time
	)
	err := s.Pool.QueryRow(ctx, `
SELECT profile_id, enabled, start_time, end_time, timezone, override_mentions
FROM quiet_hours WHERE profile_id = $1`, profileID).Scan(
		&out.ProfileID, &out.Enabled, &startTime, &endTime, &out.Timezone, &out.OverrideMentions,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return QuietHours{
			ProfileID:        profileID,
			Timezone:         "UTC",
			OverrideMentions: true,
		}, nil
	}
	if err != nil {
		return QuietHours{}, err
	}
	out.StartTime = formatClock(startTime)
	out.EndTime = formatClock(endTime)
	return out, nil
}

// SetQuietHours upserts quiet hours for a profile.
func (s *SettingsStore) SetQuietHours(ctx context.Context, hours QuietHours) error {
	if s == nil || s.Pool == nil {
		return ErrNotImplemented
	}
	start, err := parseClock(hours.StartTime)
	if err != nil {
		return err
	}
	end, err := parseClock(hours.EndTime)
	if err != nil {
		return err
	}
	tz := strings.TrimSpace(hours.Timezone)
	if tz == "" {
		tz = "UTC"
	}
	_, err = s.Pool.Exec(ctx, `
INSERT INTO quiet_hours (profile_id, enabled, start_time, end_time, timezone, override_mentions)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (profile_id) DO UPDATE SET
  enabled = EXCLUDED.enabled,
  start_time = EXCLUDED.start_time,
  end_time = EXCLUDED.end_time,
  timezone = EXCLUDED.timezone,
  override_mentions = EXCLUDED.override_mentions`,
		hours.ProfileID, hours.Enabled, start, end, tz, hours.OverrideMentions,
	)
	return err
}

func formatClock(t time.Time) string {
	return t.Format("15:04")
}

func parseClock(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, errors.New("time required")
	}
	if len(s) == 5 {
		s += ":00"
	}
	parsed, err := time.Parse("15:04:05", s)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(2000, 1, 1, parsed.Hour(), parsed.Minute(), parsed.Second(), 0, time.UTC), nil
}
