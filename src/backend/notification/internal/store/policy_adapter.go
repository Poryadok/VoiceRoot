package store

import (
	"context"

	"github.com/google/uuid"

	"voice/backend/notification/internal/delivery"
)

// PolicyAdapter exposes SettingsStore to delivery.DBPolicyLoader.
type PolicyAdapter struct {
	Store *SettingsStore
}

func (a PolicyAdapter) GetSettings(ctx context.Context, profileID uuid.UUID, scopeType string, scopeID *uuid.UUID) (delivery.SettingsRecord, error) {
	if a.Store == nil {
		return delivery.SettingsRecord{Enabled: true}, nil
	}
	row, err := a.Store.GetSettings(ctx, profileID, scopeType, scopeID)
	if err != nil {
		return delivery.SettingsRecord{}, err
	}
	return delivery.SettingsRecord{
		Enabled:       row.Enabled,
		MuteUntil:     row.MuteUntil,
		SuppressTypes: row.SuppressTypes,
	}, nil
}

func (a PolicyAdapter) GetQuietHours(ctx context.Context, profileID uuid.UUID) (delivery.QuietHoursRecord, error) {
	if a.Store == nil {
		return delivery.QuietHoursRecord{Timezone: "UTC", OverrideMentions: true}, nil
	}
	row, err := a.Store.GetQuietHours(ctx, profileID)
	if err != nil {
		return delivery.QuietHoursRecord{}, err
	}
	return delivery.QuietHoursRecord{
		Enabled:          row.Enabled,
		StartTime:        row.StartTime,
		EndTime:          row.EndTime,
		Timezone:         row.Timezone,
		OverrideMentions: row.OverrideMentions,
	}, nil
}
