package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PrivacyRow struct {
	ProfileID               uuid.UUID
	Preset                  string
	ShowOnline              string
	ShowGameStatus          string
	ShowMmRating            string
	ShowPhone               string
	ShowStories             string
	AllowDM                 string
	AllowFriendRequests     string
	AllowGuestDM            bool
	ShowOnlineIncludeGuests bool
	UpdatedAt               time.Time
}

type PrivacyStore struct {
	pool *pgxpool.Pool
}

var errPrivacyStoreNotConfigured = errors.New("privacy store is not configured")

func NewPrivacyStore(pool *pgxpool.Pool) *PrivacyStore {
	return &PrivacyStore{pool: pool}
}

func (s *PrivacyStore) GetByProfileID(ctx context.Context, profileID uuid.UUID) (*PrivacyRow, error) {
	if s == nil || s.pool == nil {
		return nil, errPrivacyStoreNotConfigured
	}
	row, err := scanPrivacy(s.pool.QueryRow(ctx, `
SELECT profile_id, preset, show_online, show_game_status, show_mm_rating, show_phone, show_stories,
       allow_dm, allow_friend_requests, allow_guest_dm, COALESCE(show_online_include_guests, false), updated_at
FROM privacy_settings
WHERE profile_id = $1`, profileID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return row, nil
}

func (s *PrivacyStore) CreateDefaultGaming(ctx context.Context, profileID uuid.UUID) (*PrivacyRow, error) {
	if s == nil || s.pool == nil {
		return nil, errPrivacyStoreNotConfigured
	}
	return scanPrivacy(s.pool.QueryRow(ctx, `
INSERT INTO privacy_settings (
  profile_id, preset, show_online, show_game_status, show_mm_rating, show_phone, show_stories,
  allow_dm, allow_friend_requests, allow_guest_dm
) VALUES (
  $1, 'gaming', 'everyone', 'everyone', 'everyone', 'nobody', 'everyone',
  'everyone', 'everyone', true
)
ON CONFLICT (profile_id) DO UPDATE SET
  updated_at = now()
RETURNING profile_id, preset, show_online, show_game_status, show_mm_rating, show_phone, show_stories,
          allow_dm, allow_friend_requests, allow_guest_dm, COALESCE(show_online_include_guests, false), updated_at`, profileID))
}

func (s *PrivacyStore) Upsert(ctx context.Context, row PrivacyRow) (*PrivacyRow, error) {
	if s == nil || s.pool == nil {
		return nil, errPrivacyStoreNotConfigured
	}
	return scanPrivacy(s.pool.QueryRow(ctx, `
INSERT INTO privacy_settings (
  profile_id, preset, show_online, show_game_status, show_mm_rating, show_phone, show_stories,
  allow_dm, allow_friend_requests, allow_guest_dm, show_online_include_guests
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
ON CONFLICT (profile_id) DO UPDATE SET
  preset = EXCLUDED.preset,
  show_online = EXCLUDED.show_online,
  show_game_status = EXCLUDED.show_game_status,
  show_mm_rating = EXCLUDED.show_mm_rating,
  show_phone = EXCLUDED.show_phone,
  show_stories = EXCLUDED.show_stories,
  allow_dm = EXCLUDED.allow_dm,
  allow_friend_requests = EXCLUDED.allow_friend_requests,
  allow_guest_dm = EXCLUDED.allow_guest_dm,
  show_online_include_guests = EXCLUDED.show_online_include_guests,
  updated_at = now()
RETURNING profile_id, preset, show_online, show_game_status, show_mm_rating, show_phone, show_stories,
          allow_dm, allow_friend_requests, allow_guest_dm, COALESCE(show_online_include_guests, false), updated_at`,
		row.ProfileID, row.Preset, row.ShowOnline, row.ShowGameStatus, row.ShowMmRating, row.ShowPhone, row.ShowStories,
		row.AllowDM, row.AllowFriendRequests, row.AllowGuestDM, row.ShowOnlineIncludeGuests,
	))
}

func scanPrivacy(row pgx.Row) (*PrivacyRow, error) {
	var out PrivacyRow
	if err := row.Scan(
		&out.ProfileID,
		&out.Preset,
		&out.ShowOnline,
		&out.ShowGameStatus,
		&out.ShowMmRating,
		&out.ShowPhone,
		&out.ShowStories,
		&out.AllowDM,
		&out.AllowFriendRequests,
		&out.AllowGuestDM,
		&out.ShowOnlineIncludeGuests,
		&out.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &out, nil
}
