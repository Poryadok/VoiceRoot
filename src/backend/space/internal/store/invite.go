package store

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrInviteNotFound      = errors.New("invite not found")
	ErrInviteRevoked       = errors.New("invite revoked")
	ErrInviteExpired       = errors.New("invite expired")
	ErrInviteMaxUses       = errors.New("invite max uses reached")
	ErrAlreadySpaceMember  = errors.New("already a space member")
)

// InviteRow is a row from invites.
type InviteRow struct {
	ID               uuid.UUID
	SpaceID          uuid.UUID
	Code             string
	CreatorProfileID uuid.UUID
	MaxUses          *int32
	UseCount         int32
	ExpiresAt        *time.Time
	CreatedAt        time.Time
	RevokedAt        *time.Time
}

// CreateInviteInput holds optional invite limits.
type CreateInviteInput struct {
	SpaceID          uuid.UUID
	CreatorProfileID uuid.UUID
	MaxUses          *int32
	ExpiresAt        *time.Time
}

func generateInviteCode() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func scanInviteRow(row pgx.Row) (*InviteRow, error) {
	var r InviteRow
	err := row.Scan(
		&r.ID, &r.SpaceID, &r.Code, &r.CreatorProfileID,
		&r.MaxUses, &r.UseCount, &r.ExpiresAt, &r.CreatedAt, &r.RevokedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// CreateInvite inserts a new invite with a unique code.
func (s *SpaceStore) CreateInvite(ctx context.Context, in CreateInviteInput) (*InviteRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("space store: pool not configured")
	}
	if in.MaxUses != nil && *in.MaxUses < 1 {
		return nil, errors.New("max_uses must be at least 1")
	}
	for attempt := 0; attempt < 5; attempt++ {
		code, err := generateInviteCode()
		if err != nil {
			return nil, err
		}
		row, err := scanInviteRow(s.Pool.QueryRow(ctx, `
INSERT INTO invites (space_id, code, creator_profile_id, max_uses, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, space_id, code, creator_profile_id, max_uses, use_count, expires_at, created_at, revoked_at
`, in.SpaceID, code, in.CreatorProfileID, in.MaxUses, in.ExpiresAt))
		if err == nil {
			return row, nil
		}
		if !isUniqueViolation(err) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("failed to generate unique invite code")
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// ListInvites returns non-revoked invites for a space.
func (s *SpaceStore) ListInvites(ctx context.Context, spaceID uuid.UUID) ([]*InviteRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("space store: pool not configured")
	}
	rows, err := s.Pool.Query(ctx, `
SELECT id, space_id, code, creator_profile_id, max_uses, use_count, expires_at, created_at, revoked_at
FROM invites
WHERE space_id = $1 AND revoked_at IS NULL
ORDER BY created_at DESC
`, spaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*InviteRow
	for rows.Next() {
		var r InviteRow
		if err := rows.Scan(
			&r.ID, &r.SpaceID, &r.Code, &r.CreatorProfileID,
			&r.MaxUses, &r.UseCount, &r.ExpiresAt, &r.CreatedAt, &r.RevokedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, &r)
	}
	return out, rows.Err()
}

// GetInviteByCode loads an invite by code.
func (s *SpaceStore) GetInviteByCode(ctx context.Context, code string) (*InviteRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("space store: pool not configured")
	}
	return scanInviteRow(s.Pool.QueryRow(ctx, `
SELECT id, space_id, code, creator_profile_id, max_uses, use_count, expires_at, created_at, revoked_at
FROM invites
WHERE code = $1
`, code))
}

// GetInviteByID loads an invite by id.
func (s *SpaceStore) GetInviteByID(ctx context.Context, inviteID uuid.UUID) (*InviteRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("space store: pool not configured")
	}
	return scanInviteRow(s.Pool.QueryRow(ctx, `
SELECT id, space_id, code, creator_profile_id, max_uses, use_count, expires_at, created_at, revoked_at
FROM invites
WHERE id = $1
`, inviteID))
}

// RevokeInvite sets revoked_at on an invite.
func (s *SpaceStore) RevokeInvite(ctx context.Context, inviteID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("space store: pool not configured")
	}
	tag, err := s.Pool.Exec(ctx, `
UPDATE invites SET revoked_at = now()
WHERE id = $1 AND revoked_at IS NULL
`, inviteID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrInviteNotFound
	}
	return nil
}

// MembershipRow is a space_members row.
type MembershipRow struct {
	SpaceID   uuid.UUID
	ProfileID uuid.UUID
	JoinedAt  time.Time
	Nickname  *string
}

func scanMembershipRow(row pgx.Row) (*MembershipRow, error) {
	var r MembershipRow
	err := row.Scan(&r.SpaceID, &r.ProfileID, &r.JoinedAt, &r.Nickname)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// GetMembership loads membership for profile in space.
func (s *SpaceStore) GetMembership(ctx context.Context, spaceID, profileID uuid.UUID) (*MembershipRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("space store: pool not configured")
	}
	return scanMembershipRow(s.Pool.QueryRow(ctx, `
SELECT space_id, profile_id, joined_at, nickname
FROM space_members
WHERE space_id = $1 AND profile_id = $2
`, spaceID, profileID))
}

func (s *SpaceStore) validateInviteForJoin(inv *InviteRow, now time.Time) error {
	if inv == nil {
		return ErrInviteNotFound
	}
	if inv.RevokedAt != nil {
		return ErrInviteRevoked
	}
	if inv.ExpiresAt != nil && !inv.ExpiresAt.After(now) {
		return ErrInviteExpired
	}
	if inv.MaxUses != nil && inv.UseCount >= *inv.MaxUses {
		return ErrInviteMaxUses
	}
	return nil
}

// JoinByInvite adds profile to space and increments use_count atomically.
// If already a member, returns existing membership without incrementing use_count.
func (s *SpaceStore) JoinByInvite(ctx context.Context, code string, profileID, accountID uuid.UUID) (*MembershipRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("space store: pool not configured")
	}
	now := time.Now().UTC()

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	inv, err := scanInviteRow(tx.QueryRow(ctx, `
SELECT id, space_id, code, creator_profile_id, max_uses, use_count, expires_at, created_at, revoked_at
FROM invites
WHERE code = $1
FOR UPDATE
`, code))
	if err != nil {
		return nil, err
	}
	if err := s.validateInviteForJoin(inv, now); err != nil {
		return nil, err
	}
	banned, err := s.IsAccountBanned(ctx, inv.SpaceID, accountID)
	if err != nil {
		return nil, err
	}
	if banned {
		return nil, ErrAccountBanned
	}

	existing, err := scanMembershipRow(tx.QueryRow(ctx, `
SELECT space_id, profile_id, joined_at, nickname
FROM space_members
WHERE space_id = $1 AND profile_id = $2
`, inv.SpaceID, profileID))
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return existing, nil
	}

	cap, err := memberCapTx(ctx, tx, inv.SpaceID)
	if err != nil {
		return nil, err
	}
	var memberCount int32
	if err := tx.QueryRow(ctx, `SELECT member_count FROM spaces WHERE id = $1`, inv.SpaceID).Scan(&memberCount); err != nil {
		return nil, err
	}
	if memberCount >= cap {
		return nil, ErrMemberCapReached
	}

	if _, err := tx.Exec(ctx, `
INSERT INTO space_members (space_id, profile_id)
VALUES ($1, $2)
`, inv.SpaceID, profileID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
UPDATE spaces SET member_count = member_count + 1, updated_at = now()
WHERE id = $1
`, inv.SpaceID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
UPDATE invites SET use_count = use_count + 1
WHERE id = $1
`, inv.ID); err != nil {
		return nil, err
	}

	member, err := scanMembershipRow(tx.QueryRow(ctx, `
SELECT space_id, profile_id, joined_at, nickname
FROM space_members
WHERE space_id = $1 AND profile_id = $2
`, inv.SpaceID, profileID))
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return member, nil
}

func memberCapTx(ctx context.Context, tx pgx.Tx, spaceID uuid.UUID) (int32, error) {
	var hasPro bool
	err := tx.QueryRow(ctx, `
SELECT EXISTS (
	SELECT 1 FROM space_subscriptions
	WHERE space_id = $1 AND status IN ('active', 'grace_period')
)`, spaceID).Scan(&hasPro)
	if err != nil {
		return 0, err
	}
	if hasPro {
		return 5000, nil
	}
	return freeSpaceMemberCap, nil
}

// AllowGuestsForInvite reports whether guest accounts may join via invite code.
func (s *SpaceStore) AllowGuestsForInvite(ctx context.Context, code string) (bool, error) {
	if s == nil || s.Pool == nil {
		return false, errors.New("space store: pool not configured")
	}
	var allow bool
	err := s.Pool.QueryRow(ctx, `
SELECT COALESCE(sp.allow_guests, true)
FROM invites i
JOIN spaces sp ON sp.id = i.space_id
WHERE i.code = $1`, code).Scan(&allow)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, ErrInviteNotFound
		}
		return false, err
	}
	return allow, nil
}
