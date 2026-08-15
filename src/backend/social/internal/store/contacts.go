package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrContactNotFound = errors.New("contact not found")

// ContactStore persists social_db.contacts.
type ContactStore struct {
	Pool *pgxpool.Pool
}

type ContactRow struct {
	ID               uuid.UUID
	OwnerProfileID   uuid.UUID
	ContactProfileID uuid.UUID
	Source           string
	IsFavorite       bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type ContactsListCursor struct {
	UpdatedAt time.Time
	ID        uuid.UUID
}

func (s *ContactStore) UpsertContact(ctx context.Context, ownerProfileID, contactProfileID uuid.UUID, source string, isFavorite bool) error {
	if s == nil || s.Pool == nil {
		return errors.New("contact store unavailable")
	}
	if ownerProfileID == contactProfileID {
		return errors.New("cannot add self as contact")
	}
	if source == "" {
		source = "manual"
	}
	_, err := s.Pool.Exec(ctx, `
INSERT INTO contacts (owner_profile_id, contact_profile_id, source, is_favorite)
VALUES ($1, $2, $3, $4)
ON CONFLICT (owner_profile_id, contact_profile_id) DO UPDATE SET
  source = EXCLUDED.source,
  is_favorite = contacts.is_favorite OR EXCLUDED.is_favorite,
  updated_at = now()`, ownerProfileID, contactProfileID, source, isFavorite)
	return err
}

func (s *ContactStore) RemoveContact(ctx context.Context, ownerProfileID, contactProfileID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("contact store unavailable")
	}
	tag, err := s.Pool.Exec(ctx, `
DELETE FROM contacts WHERE owner_profile_id = $1 AND contact_profile_id = $2`,
		ownerProfileID, contactProfileID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrContactNotFound
	}
	return nil
}

func (s *ContactStore) SetFavorite(ctx context.Context, ownerProfileID, contactProfileID uuid.UUID, favorite bool) error {
	if s == nil || s.Pool == nil {
		return errors.New("contact store unavailable")
	}
	tag, err := s.Pool.Exec(ctx, `
UPDATE contacts SET is_favorite = $3, updated_at = now()
WHERE owner_profile_id = $1 AND contact_profile_id = $2`,
		ownerProfileID, contactProfileID, favorite)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrContactNotFound
	}
	return nil
}

func (s *ContactStore) ListContacts(ctx context.Context, ownerProfileID uuid.UUID, after *ContactsListCursor, limit int) ([]ContactRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("contact store unavailable")
	}
	if limit <= 0 {
		limit = 20
	}
	var (
		q    string
		args []any
	)
	if after == nil {
		q = `
SELECT id, owner_profile_id, contact_profile_id, source, is_favorite, created_at, updated_at
FROM contacts
WHERE owner_profile_id = $1
ORDER BY updated_at DESC, id DESC
LIMIT $2`
		args = []any{ownerProfileID, limit + 1}
	} else {
		q = `
SELECT id, owner_profile_id, contact_profile_id, source, is_favorite, created_at, updated_at
FROM contacts
WHERE owner_profile_id = $1
  AND (updated_at, id) < ($2::timestamptz, $3::uuid)
ORDER BY updated_at DESC, id DESC
LIMIT $4`
		args = []any{ownerProfileID, after.UpdatedAt, after.ID, limit + 1}
	}
	rows, err := s.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ContactRow, 0, limit+1)
	for rows.Next() {
		var r ContactRow
		if err := rows.Scan(&r.ID, &r.OwnerProfileID, &r.ContactProfileID, &r.Source, &r.IsFavorite, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *ContactStore) ListFavorites(ctx context.Context, ownerProfileID uuid.UUID) ([]ContactRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("contact store unavailable")
	}
	rows, err := s.Pool.Query(ctx, `
SELECT id, owner_profile_id, contact_profile_id, source, is_favorite, created_at, updated_at
FROM contacts
WHERE owner_profile_id = $1 AND is_favorite = true
ORDER BY updated_at DESC, id DESC`, ownerProfileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ContactRow, 0, 8)
	for rows.Next() {
		var r ContactRow
		if err := rows.Scan(&r.ID, &r.OwnerProfileID, &r.ContactProfileID, &r.Source, &r.IsFavorite, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *ContactStore) GetContact(ctx context.Context, ownerProfileID, contactProfileID uuid.UUID) (*ContactRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("contact store unavailable")
	}
	row := &ContactRow{}
	err := s.Pool.QueryRow(ctx, `
SELECT id, owner_profile_id, contact_profile_id, source, is_favorite, created_at, updated_at
FROM contacts WHERE owner_profile_id = $1 AND contact_profile_id = $2`,
		ownerProfileID, contactProfileID).Scan(
		&row.ID, &row.OwnerProfileID, &row.ContactProfileID, &row.Source, &row.IsFavorite, &row.CreatedAt, &row.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return row, nil
}
