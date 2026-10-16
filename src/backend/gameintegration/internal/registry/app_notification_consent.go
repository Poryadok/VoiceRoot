package registry

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrAppNotificationConsentDenied   = errors.New("app notification consent denied")
	ErrAppNotificationConsentConflict = errors.New("app notification consent revision conflict")
)

var appNotificationCategories = []string{"game_activity", "game_actions", "game_social"}
var appNotificationChannels = []string{"in_app", "push"}

type AppNotificationConsent struct {
	BindingID     uuid.UUID `json:"binding_id"`
	ApplicationID uuid.UUID `json:"application_id"`
	EnvironmentID uuid.UUID `json:"environment_id"`
	Categories    []string  `json:"categories"`
	Channels      []string  `json:"channels"`
	Revision      int64     `json:"revision"`
	CreatedAt     string    `json:"created_at"`
	RevokedAt     *string   `json:"revoked_at,omitempty"`
}

// SetAppNotificationConsent applies a revision-checked owner decision to an active binding.
// Empty categories and channels revoke all app notification delivery for that binding.
func (s *Store) SetAppNotificationConsent(ctx context.Context, accountID, profileID, bindingID uuid.UUID,
	expectedRevision int64, categories, channels []string) (AppNotificationConsent, error) {
	if s == nil || s.Pool == nil || accountID == uuid.Nil || profileID == uuid.Nil || bindingID == uuid.Nil || expectedRevision < 0 {
		return AppNotificationConsent{}, ErrAppNotificationConsentDenied
	}
	if !validAppNotificationSet(categories, appNotificationCategories) || !validAppNotificationSet(channels, appNotificationChannels) ||
		(len(categories) == 0) != (len(channels) == 0) {
		return AppNotificationConsent{}, ErrAppNotificationConsentDenied
	}
	if categories == nil {
		categories = []string{}
	}
	if channels == nil {
		channels = []string{}
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return AppNotificationConsent{}, fmt.Errorf("begin app notification consent: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var appID, envID uuid.UUID
	var status string
	err = tx.QueryRow(ctx, `SELECT application_id,environment_id,status FROM player_bindings
		WHERE binding_id=$1 AND account_id=$2 AND profile_id=$3 FOR UPDATE`, bindingID, accountID, profileID).
		Scan(&appID, &envID, &status)
	if err != nil || status != "active" {
		return AppNotificationConsent{}, ErrAppNotificationConsentDenied
	}
	var current int64
	err = tx.QueryRow(ctx, `SELECT revision FROM gis_app_notification_consents WHERE binding_id=$1 FOR UPDATE`, bindingID).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) {
		current = 0
	} else if err != nil {
		return AppNotificationConsent{}, fmt.Errorf("read app notification consent revision: %w", err)
	}
	if current != expectedRevision || current == int64(^uint64(0)>>1) {
		return AppNotificationConsent{}, ErrAppNotificationConsentConflict
	}
	next := current + 1
	var out AppNotificationConsent
	var revokedAt *string
	err = tx.QueryRow(ctx, `INSERT INTO gis_app_notification_consents
		(binding_id,account_id,profile_id,application_id,environment_id,categories,channels,revision,created_at,revoked_at,updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,clock_timestamp(),CASE WHEN cardinality($6::text[])=0 THEN clock_timestamp() ELSE NULL END,clock_timestamp())
		ON CONFLICT(binding_id) DO UPDATE SET account_id=EXCLUDED.account_id,profile_id=EXCLUDED.profile_id,
		application_id=EXCLUDED.application_id,environment_id=EXCLUDED.environment_id,categories=EXCLUDED.categories,
		channels=EXCLUDED.channels,revision=EXCLUDED.revision,
		revoked_at=CASE WHEN cardinality(EXCLUDED.categories)=0 THEN clock_timestamp() ELSE NULL END,updated_at=clock_timestamp()
		RETURNING binding_id,application_id,environment_id,categories,channels,revision,created_at::text,revoked_at::text`,
		bindingID, accountID, profileID, appID, envID, categories, channels, next).
		Scan(&out.BindingID, &out.ApplicationID, &out.EnvironmentID, &out.Categories, &out.Channels, &out.Revision, &out.CreatedAt, &revokedAt)
	if err != nil {
		return AppNotificationConsent{}, fmt.Errorf("write app notification consent: %w", err)
	}
	out.RevokedAt = revokedAt
	if err := tx.Commit(ctx); err != nil {
		return AppNotificationConsent{}, fmt.Errorf("commit app notification consent: %w", err)
	}
	return out, nil
}

func validAppNotificationSet(values, allowed []string) bool {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !slices.Contains(allowed, value) {
			return false
		}
		if _, ok := seen[value]; ok {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func (s *Store) AllowsAppNotificationPush(ctx context.Context, appID, envID, profileID uuid.UUID, category string) (bool, error) {
	return s.AllowsAppNotificationChannel(ctx, appID, envID, profileID, category, "push")
}

func (s *Store) AllowsAppNotificationChannel(ctx context.Context, appID, envID, profileID uuid.UUID, category, channel string) (bool, error) {
	if s == nil || s.Pool == nil || appID == uuid.Nil || envID == uuid.Nil || profileID == uuid.Nil ||
		!slices.Contains(appNotificationCategories, category) || !slices.Contains(appNotificationChannels, channel) {
		return false, ErrAppNotificationConsentDenied
	}
	var allowed bool
	err := s.Pool.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM gis_app_notification_consents c
		JOIN player_bindings b ON b.binding_id=c.binding_id
		WHERE c.application_id=$1 AND c.environment_id=$2 AND c.profile_id=$3
		  AND b.application_id=c.application_id AND b.environment_id=c.environment_id
		  AND b.profile_id=c.profile_id AND b.account_id=c.account_id AND b.status='active'
		  AND c.revoked_at IS NULL AND $4=ANY(c.categories) AND $5=ANY(c.channels)
	)`, appID, envID, profileID, category, channel).Scan(&allowed)
	if err != nil {
		return false, fmt.Errorf("recheck app notification consent: %w", err)
	}
	return allowed, nil
}

// ResolveAppNotificationChat returns whether the chat is currently game-managed
// and whether this recipient has current consent in its mapped active binding.
func (s *Store) ResolveAppNotificationChat(ctx context.Context, profileID, chatID uuid.UUID, category, channel string) (bool, bool, error) {
	if s == nil || s.Pool == nil || profileID == uuid.Nil || chatID == uuid.Nil ||
		!slices.Contains(appNotificationCategories, category) || !slices.Contains(appNotificationChannels, channel) {
		return false, false, ErrAppNotificationConsentDenied
	}
	var scopeCount int
	var allowed bool
	err := s.Pool.QueryRow(ctx, `SELECT count(DISTINCT (r.application_id,r.environment_id)),
		COALESCE(bool_or(b.profile_id=$1 AND EXISTS (
			SELECT 1 FROM gis_app_notification_consents c
			WHERE c.binding_id=b.binding_id AND c.application_id=b.application_id
			  AND c.environment_id=b.environment_id AND c.profile_id=b.profile_id
			  AND c.account_id=b.account_id AND c.revoked_at IS NULL
			  AND $3=ANY(c.categories) AND $4=ANY(c.channels)
		)),false)
		FROM game_resource_binding_chats r
		JOIN applications a ON a.id=r.application_id AND a.status IN ('sandbox','active')
		JOIN environments e ON e.id=r.environment_id AND e.application_id=r.application_id AND e.status='active'
		JOIN player_bindings b ON b.application_id=r.application_id AND b.environment_id=r.environment_id
			AND b.binding_id=r.binding_id AND b.status='active'
		JOIN game_resource_mappings m ON m.application_id=r.application_id AND m.environment_id=r.environment_id
			AND m.resource_kind='chat' AND m.chat_id=r.chat_id AND m.status='active'
		WHERE r.chat_id=$2 AND r.status='active' AND r.lease_expires_at>clock_timestamp()`,
		profileID, chatID, category, channel).Scan(&scopeCount, &allowed)
	if err != nil {
		return false, false, fmt.Errorf("resolve app notification chat consent: %w", err)
	}
	if scopeCount == 0 {
		return false, false, nil
	}
	if scopeCount != 1 {
		return true, false, nil
	}
	return true, allowed, nil
}
