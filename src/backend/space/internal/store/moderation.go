package store

import (
	"context"
	"encoding/json"
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
	if s.tx == nil {
		return withOwnershipScopeValue(s, ctx, []uuid.UUID{spaceID}, func(scoped *SpaceStore) (bool, error) {
			return scoped.IsAccountBanned(ctx, spaceID, accountID)
		})
	}
	var one int
	err := s.db().QueryRow(ctx, `
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

// BanMember cannot manufacture authenticated public intent from raw UUIDs.
func (s *SpaceStore) BanMember(ctx context.Context, spaceID, accountID, bannedBy uuid.UUID, reason *string, evictProfileID *uuid.UUID) error {
	return ErrAccountBanUnavailable
}

// UnbanMember removes a ban record.
func (s *SpaceStore) UnbanMember(ctx context.Context, spaceID, accountID, actor uuid.UUID) error {
	return ErrAccountBanUnavailable
}

func (s *SpaceStore) unbanAccountWithOutcome(ctx context.Context, spaceID, accountID, actor uuid.UUID, outcome func() error) error {
	if s == nil || s.tx == nil || outcome == nil {
		return ErrAccountBanUnavailable
	}
	var version int64
	if err := s.tx.QueryRow(ctx, `SELECT version FROM schema_migrations WHERE NOT dirty`).Scan(&version); err != nil || version != 50 && version != 51 {
		return ErrAccountBanUnavailable
	}
	if err := checkMemberAuthorityCatalog(ctx, s.tx); err != nil {
		return err
	}
	if err := s.requireRetiredUnbanSources(ctx, spaceID, accountID); err != nil {
		return err
	}
	rows, err := s.tx.Query(ctx, `SELECT profile_id FROM space_account_ban_suppressions WHERE space_id=$1 AND account_id=$2 ORDER BY profile_id`, spaceID, accountID)
	if err != nil {
		return err
	}
	var profiles []uuid.UUID
	for rows.Next() {
		var p uuid.UUID
		if err = rows.Scan(&p); err != nil {
			rows.Close()
			return err
		}
		profiles = append(profiles, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	capture, err := s.captureMemberMutationIfPresent(ctx, spaceID, profiles)
	if err != nil {
		return err
	}
	// Include revisions accepted while banned; removing the ban never lowers
	// these floors or republishes any old episode/lease/Role contribution.
	_, err = s.tx.Exec(ctx, `UPDATE space_account_ban_suppressions p SET source_revision=GREATEST(p.source_revision,a.roster_source_revision)
 FROM community_owner_authority a WHERE p.space_id=$1 AND p.account_id=$2 AND a.space_id=p.space_id AND a.owner_generation=p.owner_generation`, spaceID, accountID)
	if err != nil {
		return err
	}
	tag, err := s.db().Exec(ctx, `DELETE FROM space_bans WHERE space_id = $1 AND account_id = $2`, spaceID, accountID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	_, err = s.db().Exec(ctx, `
INSERT INTO audit_log (space_id, actor_profile_id, action, target_type, target_id, details)
VALUES ($1, $2, 'member_unbanned', 'account', $3, '{}')
`, spaceID, actor, accountID)
	if err != nil {
		return err
	}
	if capture == nil {
		return ErrAccountBanUnavailable
	}
	if err = outcome(); err != nil {
		return err
	}
	if capture != nil {
		return s.finishEffectiveMemberMutations(ctx, capture)
	}
	return nil
}

// ListBans returns active bans for a space.
func (s *SpaceStore) ListBans(ctx context.Context, spaceID uuid.UUID) ([]*BanRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("space store: pool not configured")
	}
	if s.tx == nil {
		return withOwnershipScopeValue(s, ctx, []uuid.UUID{spaceID}, func(scoped *SpaceStore) ([]*BanRow, error) {
			return scoped.ListBans(ctx, spaceID)
		})
	}
	rows, err := s.db().Query(ctx, `
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
	if s.tx == nil {
		return s.withOwnershipScope(ctx, []uuid.UUID{spaceID}, func(scoped *SpaceStore) error {
			return scoped.SetMemberTimeout(ctx, spaceID, profileID, actor, durationSeconds, reason)
		})
	}
	if durationSeconds < 60 || durationSeconds > 604800 {
		return ErrInvalidTimeoutDuration
	}
	details, err := json.Marshal(struct {
		DurationSeconds int32   `json:"duration_seconds"`
		Reason          *string `json:"reason,omitempty"`
	}{DurationSeconds: durationSeconds, Reason: reason})
	if err != nil {
		return err
	}
	_, err = s.db().Exec(ctx, `
INSERT INTO space_member_timeouts (space_id, profile_id, timed_out_until, timed_out_by_profile_id, reason)
VALUES ($1, $2, clock_timestamp() + $3::integer * interval '1 second', $4, $5)
ON CONFLICT (space_id, profile_id) DO UPDATE SET
  timed_out_until = EXCLUDED.timed_out_until,
  timed_out_by_profile_id = EXCLUDED.timed_out_by_profile_id,
  reason = EXCLUDED.reason,
  created_at = now()
`, spaceID, profileID, durationSeconds, actor, reason)
	if err != nil {
		return err
	}
	_, err = s.db().Exec(ctx, `
INSERT INTO audit_log (space_id, actor_profile_id, action, target_type, target_id, details)
VALUES ($1, $2, 'member_timed_out', 'profile', $3, $4::jsonb)
`, spaceID, actor, profileID, details)
	return err
}

// RemoveMemberTimeout clears an active timeout.
func (s *SpaceStore) RemoveMemberTimeout(ctx context.Context, spaceID, profileID, actor uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("space store: pool not configured")
	}
	if s.tx == nil {
		return s.withOwnershipScope(ctx, []uuid.UUID{spaceID}, func(scoped *SpaceStore) error {
			return scoped.RemoveMemberTimeout(ctx, spaceID, profileID, actor)
		})
	}
	tag, err := s.db().Exec(ctx, `
DELETE FROM space_member_timeouts WHERE space_id = $1 AND profile_id = $2
`, spaceID, profileID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	_, err = s.db().Exec(ctx, `
INSERT INTO audit_log (space_id, actor_profile_id, action, target_type, target_id, details)
VALUES ($1, $2, 'member_timeout_removed', 'profile', $3, '{}')
`, spaceID, actor, profileID)
	return err
}

// IsProfileTimedOut reports whether profile is currently timed out in space.
func (s *SpaceStore) IsProfileTimedOut(ctx context.Context, spaceID, profileID uuid.UUID) (bool, error) {
	if s == nil || s.Pool == nil {
		return false, errors.New("space store: pool not configured")
	}
	if s.tx == nil {
		return withOwnershipScopeValue(s, ctx, []uuid.UUID{spaceID}, func(scoped *SpaceStore) (bool, error) {
			return scoped.IsProfileTimedOut(ctx, spaceID, profileID)
		})
	}
	var timedOut bool
	err := s.db().QueryRow(ctx, `
SELECT clock_timestamp() < timed_out_until FROM space_member_timeouts
WHERE space_id = $1 AND profile_id = $2
`, spaceID, profileID).Scan(&timedOut)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return timedOut, nil
}
