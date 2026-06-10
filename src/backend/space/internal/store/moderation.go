package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrAccountBanned          = errors.New("account is banned from space")
	ErrCannotBanOwner         = errors.New("cannot ban space owner")
	ErrInvalidTimeoutDuration = errors.New("timeout duration must be between 60 and 604800 seconds")
)

// BanRow is a space-level account ban.
type BanRow struct {
	SpaceID           uuid.UUID
	AccountID         uuid.UUID
	BannedByProfileID uuid.UUID
	Reason            *string
	BannedAt          time.Time
}

// IsAccountBanned reports whether account_id is banned from space_id.
func (s *SpaceStore) IsAccountBanned(ctx context.Context, spaceID, accountID uuid.UUID) (bool, error) {
	if s == nil || s.Pool == nil {
		return false, errors.New("space store: pool not configured")
	}
	var one int
	err := s.Pool.QueryRow(ctx, `
SELECT 1 FROM space_bans WHERE space_id = $1 AND account_id = $2 LIMIT 1
`, spaceID, accountID).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// BanMember records ban and optionally removes a member profile from the space.
func (s *SpaceStore) BanMember(
	ctx context.Context,
	spaceID, accountID, bannedBy uuid.UUID,
	reason *string,
	evictProfileID *uuid.UUID,
) error {
	if s == nil || s.Pool == nil {
		return errors.New("space store: pool not configured")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var ownerProfile uuid.UUID
	err = tx.QueryRow(ctx, `SELECT owner_profile_id FROM spaces WHERE id = $1`, spaceID).Scan(&ownerProfile)
	if errors.Is(err, pgx.ErrNoRows) {
		return pgx.ErrNoRows
	}
	if err != nil {
		return err
	}
	if evictProfileID != nil && *evictProfileID == ownerProfile {
		return ErrCannotBanOwner
	}

	if evictProfileID != nil {
		tag, err := tx.Exec(ctx, `
DELETE FROM space_members WHERE space_id = $1 AND profile_id = $2
`, spaceID, *evictProfileID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() > 0 {
			if _, err := tx.Exec(ctx, `
UPDATE spaces SET member_count = GREATEST(member_count - 1, 0), updated_at = now() WHERE id = $1
`, spaceID); err != nil {
				return err
			}
		}
		_, _ = tx.Exec(ctx, `DELETE FROM space_member_timeouts WHERE space_id = $1 AND profile_id = $2`, spaceID, *evictProfileID)
	}

	_, err = tx.Exec(ctx, `
INSERT INTO space_bans (space_id, account_id, banned_by_profile_id, reason)
VALUES ($1, $2, $3, $4)
ON CONFLICT (space_id, account_id) DO UPDATE SET
  banned_by_profile_id = EXCLUDED.banned_by_profile_id,
  reason = EXCLUDED.reason,
  banned_at = now()
`, spaceID, accountID, bannedBy, reason)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
INSERT INTO audit_log (space_id, actor_profile_id, action, target_type, target_id, details)
VALUES ($1, $2, 'member_banned', 'account', $3, '{}')
`, spaceID, bannedBy, accountID)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// UnbanMember removes a ban record.
func (s *SpaceStore) UnbanMember(ctx context.Context, spaceID, accountID, actor uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("space store: pool not configured")
	}
	tag, err := s.Pool.Exec(ctx, `DELETE FROM space_bans WHERE space_id = $1 AND account_id = $2`, spaceID, accountID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	_, _ = s.Pool.Exec(ctx, `
INSERT INTO audit_log (space_id, actor_profile_id, action, target_type, target_id, details)
VALUES ($1, $2, 'member_unbanned', 'account', $3, '{}')
`, spaceID, actor, accountID)
	return nil
}

// ListBans returns active bans for a space.
func (s *SpaceStore) ListBans(ctx context.Context, spaceID uuid.UUID) ([]*BanRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("space store: pool not configured")
	}
	rows, err := s.Pool.Query(ctx, `
SELECT space_id, account_id, banned_by_profile_id, reason, banned_at
FROM space_bans
WHERE space_id = $1
ORDER BY banned_at DESC
`, spaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*BanRow
	for rows.Next() {
		var r BanRow
		var reason *string
		if err := rows.Scan(&r.SpaceID, &r.AccountID, &r.BannedByProfileID, &reason, &r.BannedAt); err != nil {
			return nil, err
		}
		r.Reason = reason
		out = append(out, &r)
	}
	return out, rows.Err()
}

// SetMemberTimeout applies or replaces a communication timeout for a member.
func (s *SpaceStore) SetMemberTimeout(
	ctx context.Context,
	spaceID, profileID, actor uuid.UUID,
	durationSeconds int32,
	reason *string,
) error {
	if s == nil || s.Pool == nil {
		return errors.New("space store: pool not configured")
	}
	if durationSeconds < 60 || durationSeconds > 604800 {
		return ErrInvalidTimeoutDuration
	}
	until := time.Now().UTC().Add(time.Duration(durationSeconds) * time.Second)
	_, err := s.Pool.Exec(ctx, `
INSERT INTO space_member_timeouts (space_id, profile_id, timed_out_until, timed_out_by_profile_id, reason)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (space_id, profile_id) DO UPDATE SET
  timed_out_until = EXCLUDED.timed_out_until,
  timed_out_by_profile_id = EXCLUDED.timed_out_by_profile_id,
  reason = EXCLUDED.reason,
  created_at = now()
`, spaceID, profileID, until, actor, reason)
	if err != nil {
		return err
	}
	_, _ = s.Pool.Exec(ctx, `
INSERT INTO audit_log (space_id, actor_profile_id, action, target_type, target_id, details)
VALUES ($1, $2, 'member_timed_out', 'profile', $3, '{}')
`, spaceID, actor, profileID)
	return nil
}

// RemoveMemberTimeout clears an active timeout.
func (s *SpaceStore) RemoveMemberTimeout(ctx context.Context, spaceID, profileID, actor uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("space store: pool not configured")
	}
	tag, err := s.Pool.Exec(ctx, `
DELETE FROM space_member_timeouts WHERE space_id = $1 AND profile_id = $2
`, spaceID, profileID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	_, _ = s.Pool.Exec(ctx, `
INSERT INTO audit_log (space_id, actor_profile_id, action, target_type, target_id, details)
VALUES ($1, $2, 'member_timeout_removed', 'profile', $3, '{}')
`, spaceID, actor, profileID)
	return nil
}

// IsProfileTimedOut reports whether profile is currently timed out in space.
func (s *SpaceStore) IsProfileTimedOut(ctx context.Context, spaceID, profileID uuid.UUID) (bool, error) {
	if s == nil || s.Pool == nil {
		return false, errors.New("space store: pool not configured")
	}
	var until time.Time
	err := s.Pool.QueryRow(ctx, `
SELECT timed_out_until FROM space_member_timeouts
WHERE space_id = $1 AND profile_id = $2
`, spaceID, profileID).Scan(&until)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return time.Now().UTC().Before(until), nil
}
