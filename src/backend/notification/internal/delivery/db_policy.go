package delivery

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
)

// SettingsReader loads persisted notification settings and quiet hours.
type SettingsReader interface {
	GetSettings(ctx context.Context, profileID uuid.UUID, scopeType string, scopeID *uuid.UUID) (SettingsRecord, error)
	GetQuietHours(ctx context.Context, profileID uuid.UUID) (QuietHoursRecord, error)
}

// SettingsRecord is the store-facing settings shape for policy loading.
type SettingsRecord struct {
	Enabled       bool
	MuteUntil     *time.Time
	SuppressTypes []string
}

// QuietHoursRecord is the store-facing quiet hours shape for policy loading.
type QuietHoursRecord struct {
	Enabled          bool
	StartTime        string
	EndTime          string
	Timezone         string
	OverrideMentions bool
}

// DBPolicyLoader applies notification_settings and quiet_hours from the database.
type DBPolicyLoader struct {
	Reader SettingsReader
}

func (l DBPolicyLoader) LoadPolicy(
	ctx context.Context,
	profileID uuid.UUID,
	chatID string,
	typ NotificationType,
	at time.Time,
) (SettingsSnapshot, QuietHoursSnapshot, error) {
	if l.Reader == nil {
		return PermissivePolicyLoader{}.LoadPolicy(ctx, profileID, chatID, typ, at)
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}

	global, err := l.Reader.GetSettings(ctx, profileID, "global", nil)
	if err != nil {
		return SettingsSnapshot{}, QuietHoursSnapshot{}, err
	}
	settings := settingsFromRecord(global, at)

	if chatID != "" {
		if chatUUID, err := uuid.Parse(chatID); err == nil {
			chatSettings, err := l.Reader.GetSettings(ctx, profileID, "chat", &chatUUID)
			if err != nil {
				return SettingsSnapshot{}, QuietHoursSnapshot{}, err
			}
			settings = mergeSettings(settings, settingsFromRecord(chatSettings, at))
		}
	}

	quietRec, err := l.Reader.GetQuietHours(ctx, profileID)
	if err != nil {
		return SettingsSnapshot{}, QuietHoursSnapshot{}, err
	}
	quiet := QuietHoursSnapshot{
		Enabled:          quietRec.Enabled,
		StartTime:        quietRec.StartTime,
		EndTime:          quietRec.EndTime,
		Timezone:         quietRec.Timezone,
		OverrideMentions: quietRec.OverrideMentions,
		At:               at,
	}
	return settings, quiet, nil
}

func settingsFromRecord(rec SettingsRecord, at time.Time) SettingsSnapshot {
	out := SettingsSnapshot{MentionOverridesMute: true}
	if !rec.Enabled {
		out.ChatMuted = true
	}
	if rec.MuteUntil != nil && rec.MuteUntil.After(at) {
		out.ChatMuted = true
	}
	for _, raw := range rec.SuppressTypes {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		out.SuppressTypes = append(out.SuppressTypes, NotificationType(raw))
	}
	return out
}

func mergeSettings(base, overlay SettingsSnapshot) SettingsSnapshot {
	out := base
	if overlay.ChatMuted {
		out.ChatMuted = true
	}
	if len(overlay.SuppressTypes) > 0 {
		out.SuppressTypes = append(out.SuppressTypes, overlay.SuppressTypes...)
	}
	if overlay.MentionOverridesMute {
		out.MentionOverridesMute = true
	}
	return out
}
