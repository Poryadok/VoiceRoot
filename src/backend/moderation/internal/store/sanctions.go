package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type SanctionRow struct {
	ID              uuid.UUID
	TargetAccountID uuid.UUID
	Type            string
	Reason          string
	ReportID        *uuid.UUID
	IssuedBy        uuid.UUID
	ExpiresAt       *time.Time
	RevokedAt       *time.Time
	RevokedBy       *uuid.UUID
	CreatedAt       time.Time
}

func (s *SanctionStore) InsertSanction(
	ctx context.Context,
	targetAccountID uuid.UUID,
	sanctionType, reason string,
	reportID *uuid.UUID,
	issuedBy uuid.UUID,
	expiresAt *time.Time,
) (*SanctionRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errStoreNotConfigured
	}
	row := &SanctionRow{}
	var reportAny any
	if reportID != nil {
		reportAny = *reportID
	}
	var expiresAny any
	if expiresAt != nil {
		expiresAny = *expiresAt
	}
	err := s.Pool.QueryRow(ctx, `
INSERT INTO sanctions (target_account_id, type, reason, report_id, issued_by, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, target_account_id, type, reason, report_id, issued_by, expires_at, revoked_at, revoked_by, created_at`,
		targetAccountID, sanctionType, reason, reportAny, issuedBy, expiresAny,
	).Scan(
		&row.ID, &row.TargetAccountID, &row.Type, &row.Reason, &row.ReportID,
		&row.IssuedBy, &row.ExpiresAt, &row.RevokedAt, &row.RevokedBy, &row.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return row, nil
}

func (s *SanctionStore) RevokeSanction(ctx context.Context, sanctionID, revokedBy uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return errStoreNotConfigured
	}
	tag, err := s.Pool.Exec(ctx, `
UPDATE sanctions SET revoked_at = now(), revoked_by = $2, updated_at = now()
WHERE id = $1 AND revoked_at IS NULL`, sanctionID, revokedBy)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *SanctionStore) GetByID(ctx context.Context, sanctionID uuid.UUID) (*SanctionRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errStoreNotConfigured
	}
	row := &SanctionRow{}
	err := s.Pool.QueryRow(ctx, `
SELECT id, target_account_id, type, reason, report_id, issued_by, expires_at, revoked_at, revoked_by, created_at
FROM sanctions WHERE id = $1`, sanctionID).Scan(
		&row.ID, &row.TargetAccountID, &row.Type, &row.Reason, &row.ReportID,
		&row.IssuedBy, &row.ExpiresAt, &row.RevokedAt, &row.RevokedBy, &row.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	return row, nil
}

func (s *SanctionStore) ListByAccount(ctx context.Context, accountID uuid.UUID) ([]SanctionRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errStoreNotConfigured
	}
	rows, err := s.Pool.Query(ctx, `
SELECT id, target_account_id, type, reason, report_id, issued_by, expires_at, revoked_at, revoked_by, created_at
FROM sanctions WHERE target_account_id = $1 ORDER BY created_at DESC`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]SanctionRow, 0, 8)
	for rows.Next() {
		var r SanctionRow
		if err := rows.Scan(
			&r.ID, &r.TargetAccountID, &r.Type, &r.Reason, &r.ReportID,
			&r.IssuedBy, &r.ExpiresAt, &r.RevokedAt, &r.RevokedBy, &r.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *SanctionStore) GetActiveSanction(ctx context.Context, accountID uuid.UUID) (*SanctionRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errStoreNotConfigured
	}
	row := &SanctionRow{}
	err := s.Pool.QueryRow(ctx, `
SELECT id, target_account_id, type, reason, report_id, issued_by, expires_at, revoked_at, revoked_by, created_at
FROM sanctions
WHERE target_account_id = $1
  AND revoked_at IS NULL
  AND (expires_at IS NULL OR expires_at > now())
  AND type IN ('temp_ban', 'perm_ban', 'shadow_ban', 'mm_ban')
ORDER BY created_at DESC
LIMIT 1`, accountID).Scan(
		&row.ID, &row.TargetAccountID, &row.Type, &row.Reason, &row.ReportID,
		&row.IssuedBy, &row.ExpiresAt, &row.RevokedAt, &row.RevokedBy, &row.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	return row, nil
}

func (s *SanctionStore) IsShadowBanned(ctx context.Context, accountID uuid.UUID) (bool, error) {
	if s == nil || s.Pool == nil {
		return false, errStoreNotConfigured
	}
	var one int
	err := s.Pool.QueryRow(ctx, `
SELECT 1 FROM sanctions
WHERE target_account_id = $1
  AND type = 'shadow_ban'
  AND revoked_at IS NULL
  AND (expires_at IS NULL OR expires_at > now())
LIMIT 1`, accountID).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
