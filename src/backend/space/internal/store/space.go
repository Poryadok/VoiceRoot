package store

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	// ErrInvalidListCursor is returned when ListMySpacesPage receives a non-empty cursor that cannot be decoded.
	ErrInvalidListCursor = errors.New("invalid list spaces cursor")
	// ErrTransferToSelf is returned when TransferOwnership targets the current owner.
	ErrTransferToSelf = errors.New("cannot transfer ownership to current owner")
	// ErrNotSpaceOwner is returned when the caller is not spaces.owner_profile_id.
	ErrNotSpaceOwner = errors.New("space owner required")
)

// SpaceRow is a row from spaces.
type SpaceRow struct {
	ID               uuid.UUID
	Name             string
	Description      string
	IconURL          *string
	BannerURL        *string
	Visibility       string
	OwnerProfileID   uuid.UUID
	MemberCount      int32
	IsVerified       bool
	VerificationType string
	EntryRequirement   string
	EntryQuestionsJSON *string
	MMConfigJSON       *string
	CreatedAt          time.Time
	UpdatedAt        time.Time
}

// ListMySpacesPage holds one page of spaces the profile is a member of.
type ListMySpacesPage struct {
	Rows       []*SpaceRow
	NextCursor string
}

const spaceSelectColumns = `id, name, description, icon_url, banner_url, visibility, owner_profile_id, member_count,
       is_verified, verification_type, entry_requirement, entry_questions::text, mm_config::text, created_at, updated_at`

const spaceListSelectColumns = `s.id, s.name, s.description, s.icon_url, s.banner_url, s.visibility, s.owner_profile_id, s.member_count,
       s.is_verified, s.verification_type, s.entry_requirement, s.entry_questions::text, s.mm_config::text, s.created_at, s.updated_at`

type listSpaceCursorPayload struct {
	S string `json:"s"` // RFC3339Nano UTC, joined_at
	I string `json:"i"` // space id UUID
}

func encodeListSpaceCursor(joinedAt time.Time, spaceID uuid.UUID) string {
	p := listSpaceCursorPayload{
		S: joinedAt.UTC().Format(time.RFC3339Nano),
		I: spaceID.String(),
	}
	b, _ := json.Marshal(p)
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeListSpaceCursor(raw string) (time.Time, uuid.UUID, error) {
	if raw == "" {
		return time.Time{}, uuid.Nil, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return time.Time{}, uuid.Nil, ErrInvalidListCursor
	}
	var p listSpaceCursorPayload
	if err := json.Unmarshal(b, &p); err != nil {
		return time.Time{}, uuid.Nil, ErrInvalidListCursor
	}
	ts, err := time.Parse(time.RFC3339Nano, p.S)
	if err != nil {
		return time.Time{}, uuid.Nil, ErrInvalidListCursor
	}
	id, err := uuid.Parse(p.I)
	if err != nil {
		return time.Time{}, uuid.Nil, ErrInvalidListCursor
	}
	return ts.UTC(), id, nil
}

// CreateSpace inserts a space and adds the creator as a member.
func (s *SpaceStore) CreateSpace(ctx context.Context, ownerProfileID uuid.UUID, name, description, visibility string) (*SpaceRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("space store: pool not configured")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("space name is required")
	}
	visibility = strings.TrimSpace(visibility)
	if visibility == "" {
		visibility = "private"
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row, err := scanSpaceRow(tx.QueryRow(ctx, `
INSERT INTO spaces (name, description, visibility, owner_profile_id, member_count)
VALUES ($1, $2, $3, $4, 1)
RETURNING `+spaceSelectColumns+`
`, name, description, visibility, ownerProfileID))
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO space_members (space_id, profile_id)
VALUES ($1, $2)
`, row.ID, ownerProfileID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return row, nil
}

// GetSpace loads a space by id.
func (s *SpaceStore) GetSpace(ctx context.Context, spaceID uuid.UUID) (*SpaceRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("space store: pool not configured")
	}
	return scanSpaceRow(s.Pool.QueryRow(ctx, `
SELECT `+spaceSelectColumns+`
FROM spaces
WHERE id = $1
`, spaceID))
}

// IsSpaceMember reports whether profile_id is a member of space_id.
func (s *SpaceStore) IsSpaceMember(ctx context.Context, spaceID, profileID uuid.UUID) (bool, error) {
	if s == nil || s.Pool == nil {
		return false, errors.New("space store: pool not configured")
	}
	var n int
	err := s.Pool.QueryRow(ctx, `
SELECT COUNT(*)::int FROM space_members WHERE space_id = $1 AND profile_id = $2
`, spaceID, profileID).Scan(&n)
	return n > 0, err
}

// DeleteSpace removes a space row; child tables cascade via FK ON DELETE CASCADE.
func (s *SpaceStore) DeleteSpace(ctx context.Context, spaceID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("space store: pool not configured")
	}
	tag, err := s.Pool.Exec(ctx, `DELETE FROM spaces WHERE id = $1`, spaceID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// TransferOwnership moves spaces.owner_profile_id to an existing member.
func (s *SpaceStore) TransferOwnership(ctx context.Context, spaceID, currentOwner, newOwner uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errors.New("space store: pool not configured")
	}
	if currentOwner == newOwner {
		return ErrTransferToSelf
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var ownerProfile uuid.UUID
	err = tx.QueryRow(ctx, `SELECT owner_profile_id FROM spaces WHERE id = $1 FOR UPDATE`, spaceID).Scan(&ownerProfile)
	if errors.Is(err, pgx.ErrNoRows) {
		return pgx.ErrNoRows
	}
	if err != nil {
		return err
	}
	if ownerProfile != currentOwner {
		return ErrNotSpaceOwner
	}

	var memberExists int
	err = tx.QueryRow(ctx, `
SELECT 1 FROM space_members WHERE space_id = $1 AND profile_id = $2
`, spaceID, newOwner).Scan(&memberExists)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrMemberNotFound
	}
	if err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
UPDATE spaces SET owner_profile_id = $2, updated_at = now() WHERE id = $1
`, spaceID, newOwner); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
INSERT INTO audit_log (space_id, actor_profile_id, action, target_type, target_id, details)
VALUES ($1, $2, 'ownership_transferred', 'profile', $3, '{}')
`, spaceID, currentOwner, newOwner); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// UpdateSpace updates mutable space fields.
func (s *SpaceStore) UpdateSpace(ctx context.Context, spaceID uuid.UUID, in UpdateSpaceInput) (*SpaceRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("space store: pool not configured")
	}
	if in.Name == nil && in.Description == nil && in.IconURL == nil && in.BannerURL == nil &&
		in.Visibility == nil && in.EntryRequirement == nil && in.EntryQuestionsJSON == nil && in.MMConfigJSON == nil {
		return s.GetSpace(ctx, spaceID)
	}
	sets := make([]string, 0, 9)
	args := make([]any, 0, 10)
	argN := 1
	if in.Name != nil {
		sets = append(sets, fmt.Sprintf("name = $%d", argN))
		args = append(args, strings.TrimSpace(*in.Name))
		argN++
	}
	if in.Description != nil {
		sets = append(sets, fmt.Sprintf("description = $%d", argN))
		args = append(args, *in.Description)
		argN++
	}
	if in.IconURL != nil {
		sets = append(sets, fmt.Sprintf("icon_url = $%d", argN))
		args = append(args, *in.IconURL)
		argN++
	}
	if in.BannerURL != nil {
		sets = append(sets, fmt.Sprintf("banner_url = $%d", argN))
		args = append(args, *in.BannerURL)
		argN++
	}
	if in.Visibility != nil {
		sets = append(sets, fmt.Sprintf("visibility = $%d", argN))
		args = append(args, strings.TrimSpace(*in.Visibility))
		argN++
	}
	if in.EntryRequirement != nil {
		sets = append(sets, fmt.Sprintf("entry_requirement = $%d", argN))
		args = append(args, strings.TrimSpace(*in.EntryRequirement))
		argN++
	}
	if in.EntryQuestionsJSON != nil {
		sets = append(sets, fmt.Sprintf("entry_questions = $%d::jsonb", argN))
		args = append(args, *in.EntryQuestionsJSON)
		argN++
	}
	if in.MMConfigJSON != nil {
		sets = append(sets, fmt.Sprintf("mm_config = $%d::jsonb", argN))
		args = append(args, *in.MMConfigJSON)
		argN++
	}
	sets = append(sets, "updated_at = now()")
	args = append(args, spaceID)
	q := fmt.Sprintf(`
UPDATE spaces
SET %s
WHERE id = $%d
RETURNING `+spaceSelectColumns+`
`, strings.Join(sets, ", "), argN)
	return scanSpaceRow(s.Pool.QueryRow(ctx, q, args...))
}

// UpdateSpaceInput holds optional mutable space fields.
type UpdateSpaceInput struct {
	Name               *string
	Description        *string
	IconURL            *string
	BannerURL          *string
	Visibility         *string
	EntryRequirement   *string
	EntryQuestionsJSON *string
	MMConfigJSON       *string
}

// ListMySpacesPage returns spaces the profile is a member of, ordered by joined_at DESC, space id DESC.
func (s *SpaceStore) ListMySpacesPage(ctx context.Context, profileID uuid.UUID, cursor string, limit int) (*ListMySpacesPage, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("space store: pool not configured")
	}
	if limit < 1 {
		limit = 1
	}
	fetch := limit + 1

	joinedAt, spaceID, err := decodeListSpaceCursor(cursor)
	if err != nil {
		return nil, err
	}

	var rows pgx.Rows
	if joinedAt.IsZero() {
		rows, err = s.Pool.Query(ctx, `
SELECT `+spaceListSelectColumns+`, m.joined_at
FROM space_members m
JOIN spaces s ON s.id = m.space_id
WHERE m.profile_id = $1
ORDER BY m.joined_at DESC, s.id DESC
LIMIT $2
`, profileID, fetch)
	} else {
		rows, err = s.Pool.Query(ctx, `
SELECT `+spaceListSelectColumns+`, m.joined_at
FROM space_members m
JOIN spaces s ON s.id = m.space_id
WHERE m.profile_id = $1
  AND (m.joined_at, s.id) < ($2, $3)
ORDER BY m.joined_at DESC, s.id DESC
LIMIT $4
`, profileID, joinedAt, spaceID, fetch)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*SpaceRow
	var lastJoined time.Time
	var lastID uuid.UUID
	for rows.Next() {
		row, joined, scanErr := scanSpaceRowWithJoinedAt(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		if len(out) >= limit {
			return &ListMySpacesPage{Rows: out, NextCursor: encodeListSpaceCursor(lastJoined, lastID)}, nil
		}
		out = append(out, row)
		lastJoined = joined
		lastID = row.ID
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &ListMySpacesPage{Rows: out}, nil
}

func scanSpaceRow(row pgx.Row) (*SpaceRow, error) {
	var id, owner uuid.UUID
	var name, description, visibility, verificationType, entryRequirement string
	var iconURL, bannerURL, entryQuestions, mmConfig sql.NullString
	var memberCount int32
	var isVerified bool
	var createdAt, updatedAt time.Time
	err := row.Scan(&id, &name, &description, &iconURL, &bannerURL, &visibility, &owner, &memberCount,
		&isVerified, &verificationType, &entryRequirement, &entryQuestions, &mmConfig, &createdAt, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return spaceRowFromScan(id, name, description, iconURL, bannerURL, visibility, owner, memberCount,
		isVerified, verificationType, entryRequirement, entryQuestions, mmConfig, createdAt, updatedAt), nil
}

func scanSpaceRowWithJoinedAt(row pgx.Row) (*SpaceRow, time.Time, error) {
	var id, owner uuid.UUID
	var name, description, visibility, verificationType, entryRequirement string
	var iconURL, bannerURL, entryQuestions, mmConfig sql.NullString
	var memberCount int32
	var isVerified bool
	var createdAt, updatedAt, joinedAt time.Time
	err := row.Scan(&id, &name, &description, &iconURL, &bannerURL, &visibility, &owner, &memberCount,
		&isVerified, &verificationType, &entryRequirement, &entryQuestions, &mmConfig, &createdAt, &updatedAt, &joinedAt)
	if err != nil {
		return nil, time.Time{}, err
	}
	return spaceRowFromScan(id, name, description, iconURL, bannerURL, visibility, owner, memberCount,
		isVerified, verificationType, entryRequirement, entryQuestions, mmConfig, createdAt, updatedAt), joinedAt.UTC(), nil
}

func spaceRowFromScan(id uuid.UUID, name, description string, iconURL, bannerURL sql.NullString,
	visibility string, owner uuid.UUID, memberCount int32, isVerified bool,
	verificationType, entryRequirement string, entryQuestions, mmConfig sql.NullString, createdAt, updatedAt time.Time) *SpaceRow {
	r := &SpaceRow{
		ID:               id,
		Name:             name,
		Description:      description,
		Visibility:       visibility,
		OwnerProfileID:   owner,
		MemberCount:      memberCount,
		IsVerified:       isVerified,
		VerificationType: verificationType,
		EntryRequirement: entryRequirement,
		CreatedAt:        createdAt.UTC(),
		UpdatedAt:        updatedAt.UTC(),
	}
	if iconURL.Valid {
		v := iconURL.String
		r.IconURL = &v
	}
	if bannerURL.Valid {
		v := bannerURL.String
		r.BannerURL = &v
	}
	if entryQuestions.Valid {
		v := entryQuestions.String
		r.EntryQuestionsJSON = &v
	}
	if mmConfig.Valid {
		v := mmConfig.String
		r.MMConfigJSON = &v
	}
	return r
}
