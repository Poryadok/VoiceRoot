package store

import (
	"context"
	"errors"
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
