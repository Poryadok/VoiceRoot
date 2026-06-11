package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DeviceToken is a persisted push endpoint for a profile.
type DeviceToken struct {
	ID          uuid.UUID
	ProfileID   uuid.UUID
	Platform    string
	Token       string
	PushService string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// DeviceTokenStore persists FCM/APNs tokens in notification_db.
type DeviceTokenStore struct {
	Pool *pgxpool.Pool
}

// Register upserts a device token for the profile (same profile+token updates platform metadata).
func (s *DeviceTokenStore) Register(ctx context.Context, profileID uuid.UUID, platform, token, pushService string) (uuid.UUID, error) {
	if s == nil || s.Pool == nil {
		return uuid.Nil, ErrNotImplemented
	}
	const q = `
INSERT INTO device_tokens (profile_id, platform, token, push_service)
VALUES ($1, $2, $3, $4)
ON CONFLICT (profile_id, token) DO UPDATE SET
  platform = EXCLUDED.platform,
  push_service = EXCLUDED.push_service,
  updated_at = now()
RETURNING id`
	var id uuid.UUID
	err := s.Pool.QueryRow(ctx, q, profileID, platform, token, pushService).Scan(&id)
	return id, err
}

// Unregister deletes a device token by id for the owning profile.
func (s *DeviceTokenStore) Unregister(ctx context.Context, profileID, deviceTokenID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return ErrNotImplemented
	}
	tag, err := s.Pool.Exec(ctx,
		`DELETE FROM device_tokens WHERE id = $1 AND profile_id = $2`,
		deviceTokenID, profileID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrDeviceTokenNotFound
	}
	return nil
}

// ListByProfile returns all tokens registered for a profile.
func (s *DeviceTokenStore) ListByProfile(ctx context.Context, profileID uuid.UUID) ([]DeviceToken, error) {
	if s == nil || s.Pool == nil {
		return nil, nil
	}
	rows, err := s.Pool.Query(ctx, `
SELECT id, profile_id, platform, token, push_service, created_at, updated_at
FROM device_tokens WHERE profile_id = $1 ORDER BY created_at`, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DeviceToken
	for rows.Next() {
		var dt DeviceToken
		if err := rows.Scan(&dt.ID, &dt.ProfileID, &dt.Platform, &dt.Token, &dt.PushService, &dt.CreatedAt, &dt.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, dt)
	}
	return out, rows.Err()
}

// DeleteByToken removes a stale token after FCM reports it invalid.
func (s *DeviceTokenStore) DeleteByToken(ctx context.Context, token string) error {
	if s == nil || s.Pool == nil {
		return ErrNotImplemented
	}
	_, err := s.Pool.Exec(ctx, `DELETE FROM device_tokens WHERE token = $1`, token)
	return err
}

// ErrNoPool indicates the store has no database connection.
var ErrNoPool = errors.New("notification store: no database pool")
