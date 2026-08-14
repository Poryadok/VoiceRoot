package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// InsertMMPeerBanParams records a peer MM ban.
type InsertMMPeerBanParams struct {
	BannerProfileID uuid.UUID
	TargetProfileID uuid.UUID
	Reason          string
}

// BanStore persists peer MM bans.
type BanStore struct {
	Pool *pgxpool.Pool
}

// InsertMMPeerBan blocks target from matching with banner.
func (s *BanStore) InsertMMPeerBan(ctx context.Context, p InsertMMPeerBanParams) error {
	if s == nil || s.Pool == nil {
		return errors.New("ban store unavailable")
	}
	if p.BannerProfileID == p.TargetProfileID {
		return errors.New("cannot ban self")
	}
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO mm_bans (blocker_profile_id, blocked_profile_id, reason)
		VALUES ($1, $2, NULLIF($3, ''))
		ON CONFLICT (blocker_profile_id, blocked_profile_id) DO NOTHING
	`, p.BannerProfileID, p.TargetProfileID, p.Reason)
	return err
}

// IsPeerBanned reports whether blocker has banned target.
func (s *BanStore) IsPeerBanned(ctx context.Context, blocker, target uuid.UUID) (bool, error) {
	if s == nil || s.Pool == nil {
		return false, errors.New("ban store unavailable")
	}
	var exists bool
	err := s.Pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM mm_bans
			WHERE blocker_profile_id = $1 AND blocked_profile_id = $2
		)
	`, blocker, target).Scan(&exists)
	return exists, err
}

// IsPairBanned reports whether either profile has banned the other.
func (s *BanStore) IsPairBanned(ctx context.Context, a, b uuid.UUID) (bool, error) {
	forward, err := s.IsPeerBanned(ctx, a, b)
	if err != nil || forward {
		return forward, err
	}
	return s.IsPeerBanned(ctx, b, a)
}

// RemoveMMPeerBan deletes a peer ban.
func (s *BanStore) RemoveMMPeerBan(ctx context.Context, blocker, target uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("ban store unavailable")
	}
	_, err := s.Pool.Exec(ctx, `
		DELETE FROM mm_bans WHERE blocker_profile_id = $1 AND blocked_profile_id = $2
	`, blocker, target)
	return err
}

// InsertPlatformMMBanParams records a moderation platform MM ban.
type InsertPlatformMMBanParams struct {
	AccountID         uuid.UUID
	Reason            string
	BannedByProfileID uuid.UUID
	ExpiresAt         *time.Time
}

// InsertPlatformMMBan blocks an account from matchmaking (moderation mm_ban).
func (s *BanStore) InsertPlatformMMBan(ctx context.Context, p InsertPlatformMMBanParams) error {
	if s == nil || s.Pool == nil {
		return errors.New("ban store unavailable")
	}
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO mm_platform_bans (account_id, reason, banned_by_profile_id, expires_at, revoked_at)
		VALUES ($1, $2, $3, $4, NULL)
		ON CONFLICT (account_id) DO UPDATE SET
			reason = EXCLUDED.reason,
			banned_by_profile_id = EXCLUDED.banned_by_profile_id,
			expires_at = EXCLUDED.expires_at,
			revoked_at = NULL,
			created_at = now()
	`, p.AccountID, p.Reason, p.BannedByProfileID, p.ExpiresAt)
	return err
}

// RevokePlatformMMBan lifts a platform MM ban.
func (s *BanStore) RevokePlatformMMBan(ctx context.Context, accountID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("ban store unavailable")
	}
	now := time.Now().UTC()
	_, err := s.Pool.Exec(ctx, `
		UPDATE mm_platform_bans SET revoked_at = $2 WHERE account_id = $1 AND revoked_at IS NULL
	`, accountID, now)
	return err
}

// IsPlatformBanned reports whether account has an active platform MM ban.
func (s *BanStore) IsPlatformBanned(ctx context.Context, accountID uuid.UUID) (bool, error) {
	if s == nil || s.Pool == nil {
		return false, errors.New("ban store unavailable")
	}
	var exists bool
	err := s.Pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM mm_platform_bans
			WHERE account_id = $1
			  AND revoked_at IS NULL
			  AND (expires_at IS NULL OR expires_at > now())
		)
	`, accountID).Scan(&exists)
	return exists, err
}
