package store

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type AppealRow struct {
	ID                  uuid.UUID
	SanctionID          uuid.UUID
	AppellantAccountID  uuid.UUID
	Reason              string
	Status              string
	ReviewedBy          *uuid.UUID
	ReviewedAt          *time.Time
	ReviewNotes         *string
	CreatedAt           time.Time
}

func (s *AppealStore) GetBySanctionID(ctx context.Context, sanctionID uuid.UUID) (*AppealRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errStoreNotConfigured
	}
	row := &AppealRow{}
	err := s.Pool.QueryRow(ctx, `
SELECT id, sanction_id, appellant_account_id, reason, status, reviewed_by, reviewed_at, review_notes, created_at
FROM appeals WHERE sanction_id = $1`, sanctionID).Scan(
		&row.ID, &row.SanctionID, &row.AppellantAccountID, &row.Reason, &row.Status,
		&row.ReviewedBy, &row.ReviewedAt, &row.ReviewNotes, &row.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return row, nil
}

func (s *AppealStore) InsertAppeal(ctx context.Context, sanctionID, appellantAccountID uuid.UUID, reason string) (*AppealRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errStoreNotConfigured
	}
	row := &AppealRow{}
	err := s.Pool.QueryRow(ctx, `
INSERT INTO appeals (sanction_id, appellant_account_id, reason)
VALUES ($1, $2, $3)
RETURNING id, sanction_id, appellant_account_id, reason, status, reviewed_by, reviewed_at, review_notes, created_at`,
		sanctionID, appellantAccountID, reason,
	).Scan(
		&row.ID, &row.SanctionID, &row.AppellantAccountID, &row.Reason, &row.Status,
		&row.ReviewedBy, &row.ReviewedAt, &row.ReviewNotes, &row.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return row, nil
}

func (s *AppealStore) GetByID(ctx context.Context, appealID uuid.UUID) (*AppealRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errStoreNotConfigured
	}
	row := &AppealRow{}
	err := s.Pool.QueryRow(ctx, `
SELECT id, sanction_id, appellant_account_id, reason, status, reviewed_by, reviewed_at, review_notes, created_at
FROM appeals WHERE id = $1`, appealID).Scan(
		&row.ID, &row.SanctionID, &row.AppellantAccountID, &row.Reason, &row.Status,
		&row.ReviewedBy, &row.ReviewedAt, &row.ReviewNotes, &row.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	return row, nil
}

func (s *AppealStore) ReviewAppeal(ctx context.Context, appealID uuid.UUID, status string, reviewedBy uuid.UUID, notes *string) (*AppealRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errStoreNotConfigured
	}
	row := &AppealRow{}
	err := s.Pool.QueryRow(ctx, `
UPDATE appeals
SET status = $2, reviewed_by = $3, reviewed_at = now(), review_notes = $4
WHERE id = $1 AND status = 'pending'
RETURNING id, sanction_id, appellant_account_id, reason, status, reviewed_by, reviewed_at, review_notes, created_at`,
		appealID, status, reviewedBy, notes,
	).Scan(
		&row.ID, &row.SanctionID, &row.AppellantAccountID, &row.Reason, &row.Status,
		&row.ReviewedBy, &row.ReviewedAt, &row.ReviewNotes, &row.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	return row, nil
}

type AppealListPage struct {
	Rows       []AppealRow
	NextCursor string
}

type appealListCursorPayload struct {
	S string `json:"s"` // created_at RFC3339Nano UTC
	I string `json:"i"` // appeal id UUID
}

// ErrInvalidAppealListCursor is returned when ListAppealsPage receives a bad cursor.
var ErrInvalidAppealListCursor = errors.New("invalid appeal list cursor")

func encodeAppealListCursor(createdAt time.Time, appealID uuid.UUID) string {
	p := appealListCursorPayload{
		S: createdAt.UTC().Format(time.RFC3339Nano),
		I: appealID.String(),
	}
	b, _ := json.Marshal(p)
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeAppealListCursor(raw string) (createdAt time.Time, appealID uuid.UUID, err error) {
	if raw == "" {
		return time.Time{}, uuid.Nil, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return time.Time{}, uuid.Nil, ErrInvalidAppealListCursor
	}
	var p appealListCursorPayload
	if err := json.Unmarshal(b, &p); err != nil {
		return time.Time{}, uuid.Nil, ErrInvalidAppealListCursor
	}
	ts, err := time.Parse(time.RFC3339Nano, p.S)
	if err != nil {
		return time.Time{}, uuid.Nil, ErrInvalidAppealListCursor
	}
	id, err := uuid.Parse(p.I)
	if err != nil {
		return time.Time{}, uuid.Nil, ErrInvalidAppealListCursor
	}
	return ts.UTC(), id, nil
}

func (s *AppealStore) ListAppealsPage(ctx context.Context, statusFilter, cursor string, limit int32) (*AppealListPage, error) {
	if s == nil || s.Pool == nil {
		return nil, errStoreNotConfigured
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	fetch := limit + 1

	cursorCreatedAt, cursorID, err := decodeAppealListCursor(cursor)
	if err != nil {
		return nil, err
	}

	cursorSQL := ""
	args := []any{statusFilter}
	argN := 2
	if cursor != "" {
		cursorSQL = ` AND (
  created_at < $` + strconv.Itoa(argN) + `
  OR (created_at = $` + strconv.Itoa(argN) + ` AND id < $` + strconv.Itoa(argN+1) + `::uuid)
)`
		args = append(args, cursorCreatedAt, cursorID.String())
		argN += 2
	}
	args = append(args, fetch)

	query := `
SELECT id, sanction_id, appellant_account_id, reason, status, reviewed_by, reviewed_at, review_notes, created_at
FROM appeals
WHERE ($1 = '' OR status = $1)` + cursorSQL + `
ORDER BY created_at DESC, id DESC
LIMIT $` + strconv.Itoa(argN)

	rows, err := s.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]AppealRow, 0, fetch)
	for rows.Next() {
		var row AppealRow
		if err := rows.Scan(
			&row.ID, &row.SanctionID, &row.AppellantAccountID, &row.Reason, &row.Status,
			&row.ReviewedBy, &row.ReviewedAt, &row.ReviewNotes, &row.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	page := &AppealListPage{Rows: out}
	if int32(len(out)) > limit {
		page.Rows = out[:limit]
		last := page.Rows[len(page.Rows)-1]
		page.NextCursor = encodeAppealListCursor(last.CreatedAt, last.ID)
	}
	return page, nil
}
