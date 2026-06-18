package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"voice/backend/pkg/privacy"
)

type PrivacyRow struct {
	ProfileID              uuid.UUID
	Preset                 string
	ShowOnline             privacy.Audience
	ShowGameStatus         privacy.Audience
	ShowMmRating           privacy.Audience
	ShowPhone              privacy.Audience
	ShowStories            privacy.Audience
	AllowPhoneSearch       privacy.Audience
	AllowDM                privacy.Audience
	AllowCalls             privacy.Audience
	AllowChatSpaceInvites  privacy.Audience
	AllowFiles             privacy.Audience
	AllowVoiceMessages     privacy.Audience
	AllowFriendRequests    privacy.Audience
	AllowGuestDM           bool
	UpdatedAt              time.Time
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
SELECT profile_id, preset,
       show_online_audience, show_game_status_audience, show_mm_rating_audience, show_phone_audience, show_stories_audience,
       allow_phone_search_audience, allow_dm_audience, allow_calls_audience, allow_chat_space_invites_audience,
       allow_files_audience, allow_voice_messages_audience, allow_friend_requests_audience,
       allow_guest_dm, updated_at
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
	return s.CreateForPreset(ctx, profileID, "gaming")
}

func (s *PrivacyStore) CreateForPreset(ctx context.Context, profileID uuid.UUID, preset string) (*PrivacyRow, error) {
	if s == nil || s.pool == nil {
		return nil, errPrivacyStoreNotConfigured
	}
	settings := privacy.SettingsForPreset(preset)
	return s.Upsert(ctx, PrivacyRowFromSettings(profileID, settings))
}

func PrivacyRowFromSettings(profileID uuid.UUID, s privacy.Settings) PrivacyRow {
	return PrivacyRow{
		ProfileID:             profileID,
		Preset:                s.Preset,
		ShowOnline:            s.ShowOnline,
		ShowGameStatus:        s.ShowGameStatus,
		ShowMmRating:          s.ShowMmRating,
		ShowPhone:             s.ShowPhone,
		ShowStories:           s.ShowStories,
		AllowPhoneSearch:      s.AllowPhoneSearch,
		AllowDM:               s.AllowDM,
		AllowCalls:            s.AllowCalls,
		AllowChatSpaceInvites: s.AllowChatSpaceInvites,
		AllowFiles:            s.AllowFiles,
		AllowVoiceMessages:    s.AllowVoiceMessages,
		AllowFriendRequests:   s.AllowFriendRequests,
		AllowGuestDM:          s.AllowGuestDM,
	}
}

func (s *PrivacyStore) Upsert(ctx context.Context, row PrivacyRow) (*PrivacyRow, error) {
	if s == nil || s.pool == nil {
		return nil, errPrivacyStoreNotConfigured
	}
	showOnline, err := privacy.MarshalJSON(row.ShowOnline)
	if err != nil {
		return nil, err
	}
	showGameStatus, err := privacy.MarshalJSON(row.ShowGameStatus)
	if err != nil {
		return nil, err
	}
	showMmRating, err := privacy.MarshalJSON(row.ShowMmRating)
	if err != nil {
		return nil, err
	}
	showPhone, err := privacy.MarshalJSON(row.ShowPhone)
	if err != nil {
		return nil, err
	}
	showStories, err := privacy.MarshalJSON(row.ShowStories)
	if err != nil {
		return nil, err
	}
	allowPhoneSearch, err := privacy.MarshalJSON(row.AllowPhoneSearch)
	if err != nil {
		return nil, err
	}
	allowDM, err := privacy.MarshalJSON(row.AllowDM)
	if err != nil {
		return nil, err
	}
	allowCalls, err := privacy.MarshalJSON(row.AllowCalls)
	if err != nil {
		return nil, err
	}
	allowInvites, err := privacy.MarshalJSON(row.AllowChatSpaceInvites)
	if err != nil {
		return nil, err
	}
	allowFiles, err := privacy.MarshalJSON(row.AllowFiles)
	if err != nil {
		return nil, err
	}
	allowVoice, err := privacy.MarshalJSON(row.AllowVoiceMessages)
	if err != nil {
		return nil, err
	}
	allowFriendRequests, err := privacy.MarshalJSON(row.AllowFriendRequests)
	if err != nil {
		return nil, err
	}
	return scanPrivacy(s.pool.QueryRow(ctx, `
INSERT INTO privacy_settings (
  profile_id, preset,
  show_online_audience, show_game_status_audience, show_mm_rating_audience, show_phone_audience, show_stories_audience,
  allow_phone_search_audience, allow_dm_audience, allow_calls_audience, allow_chat_space_invites_audience,
  allow_files_audience, allow_voice_messages_audience, allow_friend_requests_audience,
  allow_guest_dm
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
ON CONFLICT (profile_id) DO UPDATE SET
  preset = EXCLUDED.preset,
  show_online_audience = EXCLUDED.show_online_audience,
  show_game_status_audience = EXCLUDED.show_game_status_audience,
  show_mm_rating_audience = EXCLUDED.show_mm_rating_audience,
  show_phone_audience = EXCLUDED.show_phone_audience,
  show_stories_audience = EXCLUDED.show_stories_audience,
  allow_phone_search_audience = EXCLUDED.allow_phone_search_audience,
  allow_dm_audience = EXCLUDED.allow_dm_audience,
  allow_calls_audience = EXCLUDED.allow_calls_audience,
  allow_chat_space_invites_audience = EXCLUDED.allow_chat_space_invites_audience,
  allow_files_audience = EXCLUDED.allow_files_audience,
  allow_voice_messages_audience = EXCLUDED.allow_voice_messages_audience,
  allow_friend_requests_audience = EXCLUDED.allow_friend_requests_audience,
  allow_guest_dm = EXCLUDED.allow_guest_dm,
  updated_at = now()
RETURNING profile_id, preset,
  show_online_audience, show_game_status_audience, show_mm_rating_audience, show_phone_audience, show_stories_audience,
  allow_phone_search_audience, allow_dm_audience, allow_calls_audience, allow_chat_space_invites_audience,
  allow_files_audience, allow_voice_messages_audience, allow_friend_requests_audience,
  allow_guest_dm, updated_at`,
		row.ProfileID, row.Preset,
		showOnline, showGameStatus, showMmRating, showPhone, showStories,
		allowPhoneSearch, allowDM, allowCalls, allowInvites, allowFiles, allowVoice, allowFriendRequests,
		row.AllowGuestDM,
	))
}

func scanPrivacy(row pgx.Row) (*PrivacyRow, error) {
	var out PrivacyRow
	var showOnline, showGameStatus, showMmRating, showPhone, showStories []byte
	var allowPhoneSearch, allowDM, allowCalls, allowInvites, allowFiles, allowVoice, allowFriendRequests []byte
	if err := row.Scan(
		&out.ProfileID,
		&out.Preset,
		&showOnline, &showGameStatus, &showMmRating, &showPhone, &showStories,
		&allowPhoneSearch, &allowDM, &allowCalls, &allowInvites, &allowFiles, &allowVoice, &allowFriendRequests,
		&out.AllowGuestDM,
		&out.UpdatedAt,
	); err != nil {
		return nil, err
	}
	var err error
	if out.ShowOnline, err = privacy.UnmarshalJSON(showOnline); err != nil {
		return nil, err
	}
	if out.ShowGameStatus, err = privacy.UnmarshalJSON(showGameStatus); err != nil {
		return nil, err
	}
	if out.ShowMmRating, err = privacy.UnmarshalJSON(showMmRating); err != nil {
		return nil, err
	}
	if out.ShowPhone, err = privacy.UnmarshalJSON(showPhone); err != nil {
		return nil, err
	}
	if out.ShowStories, err = privacy.UnmarshalJSON(showStories); err != nil {
		return nil, err
	}
	if out.AllowPhoneSearch, err = privacy.UnmarshalJSON(allowPhoneSearch); err != nil {
		return nil, err
	}
	if out.AllowDM, err = privacy.UnmarshalJSON(allowDM); err != nil {
		return nil, err
	}
	if out.AllowCalls, err = privacy.UnmarshalJSON(allowCalls); err != nil {
		return nil, err
	}
	if out.AllowChatSpaceInvites, err = privacy.UnmarshalJSON(allowInvites); err != nil {
		return nil, err
	}
	if out.AllowFiles, err = privacy.UnmarshalJSON(allowFiles); err != nil {
		return nil, err
	}
	if out.AllowVoiceMessages, err = privacy.UnmarshalJSON(allowVoice); err != nil {
		return nil, err
	}
	if out.AllowFriendRequests, err = privacy.UnmarshalJSON(allowFriendRequests); err != nil {
		return nil, err
	}
	return &out, nil
}
