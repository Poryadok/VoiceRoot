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


// WithAccountLock serializes sanction changes and Auth status reconciliation for
// one account across Moderation service instances. Callers must persist sanction
// changes and pending status-sync intent before reconciling Auth in a later call.
func (s *SanctionStore) WithAccountLock(ctx context.Context, accountID uuid.UUID, fn func(pgx.Tx) error) error {
	if s == nil || s.Pool == nil {
		return errStoreNotConfigured
	}
	if accountID == uuid.Nil || fn == nil {
		return errors.New("account lock requires account ID and callback")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('moderation-account-sanction:' || $1::text, 0))`, accountID.String()); err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// InsertSanctionTx inserts a sanction in the caller's account-serialized transaction.
func (s *SanctionStore) InsertSanctionTx(ctx context.Context, tx pgx.Tx, targetAccountID uuid.UUID, sanctionType, reason string, reportID *uuid.UUID, issuedBy uuid.UUID, expiresAt *time.Time) (*SanctionRow, error) {
	if s == nil || tx == nil {
		return nil, errStoreNotConfigured
	}
	row := &SanctionRow{}
	var reportAny, expiresAny any
	if reportID != nil {
		reportAny = *reportID
	}
	if expiresAt != nil {
		expiresAny = *expiresAt
	}
	err := tx.QueryRow(ctx, `
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

// RevokeSanctionTx revokes an active sanction in the caller's account transaction.
func (s *SanctionStore) RevokeSanctionTx(ctx context.Context, tx pgx.Tx, sanctionID, revokedBy uuid.UUID) error {
	if s == nil || tx == nil {
		return errStoreNotConfigured
	}
	tag, err := tx.Exec(ctx, `
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

// GetByIDTx locks and reads a sanction in the caller's transaction.
func (s *SanctionStore) GetByIDTx(ctx context.Context, tx pgx.Tx, sanctionID uuid.UUID) (*SanctionRow, error) {
	if s == nil || tx == nil {
		return nil, errStoreNotConfigured
	}
	row := &SanctionRow{}
	err := tx.QueryRow(ctx, `
SELECT id, target_account_id, type, reason, report_id, issued_by, expires_at, revoked_at, revoked_by, created_at
FROM sanctions WHERE id = $1 FOR UPDATE`, sanctionID).Scan(
		&row.ID, &row.TargetAccountID, &row.Type, &row.Reason, &row.ReportID,
		&row.IssuedBy, &row.ExpiresAt, &row.RevokedAt, &row.RevokedBy, &row.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return row, nil
}

// QueueAccountStatusSyncTx records durable work to reconcile Auth from current sanctions.
func (s *SanctionStore) QueueAccountStatusSyncTx(ctx context.Context, tx pgx.Tx, accountID uuid.UUID) error {
	if s == nil || tx == nil || accountID == uuid.Nil {
		return errStoreNotConfigured
	}
	_, err := tx.Exec(ctx, `
INSERT INTO moderation_account_status_sync (account_id, requested_at)
VALUES ($1, clock_timestamp())
ON CONFLICT (account_id) DO UPDATE SET requested_at = EXCLUDED.requested_at`, accountID)
	return err
}

// ListPendingAccountStatusSync returns accounts with durable Auth reconciliation work.
func (s *SanctionStore) ListPendingAccountStatusSync(ctx context.Context, limit int) ([]uuid.UUID, error) {
	if s == nil || s.Pool == nil {
		return nil, errStoreNotConfigured
	}
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.Pool.Query(ctx, `
SELECT account_id FROM moderation_account_status_sync ORDER BY requested_at, account_id LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	accounts := make([]uuid.UUID, 0, limit)
	for rows.Next() {
		var accountID uuid.UUID
		if err := rows.Scan(&accountID); err != nil {
			return nil, err
		}
		accounts = append(accounts, accountID)
	}
	return accounts, rows.Err()
}

// HasPendingAccountStatusSyncTx reports whether this account still has durable work.
func (s *SanctionStore) HasPendingAccountStatusSyncTx(ctx context.Context, tx pgx.Tx, accountID uuid.UUID) (bool, error) {
	if s == nil || tx == nil {
		return false, errStoreNotConfigured
	}
	var pending bool
	err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM moderation_account_status_sync WHERE account_id = $1)", accountID).Scan(&pending)
	return pending, err
}

// CompleteAccountStatusSyncTx removes durable work after Auth accepted the derived status.
func (s *SanctionStore) CompleteAccountStatusSyncTx(ctx context.Context, tx pgx.Tx, accountID uuid.UUID) error {
	if s == nil || tx == nil {
		return errStoreNotConfigured
	}
	_, err := tx.Exec(ctx, `DELETE FROM moderation_account_status_sync WHERE account_id = $1`, accountID)
	return err
}

// HasEffectiveAccountBanTx reports whether Auth must remain suspended for the account.
func (s *SanctionStore) HasEffectiveAccountBanTx(ctx context.Context, tx pgx.Tx, accountID uuid.UUID) (bool, error) {
	if s == nil || tx == nil {
		return false, errStoreNotConfigured
	}
	var exists bool
	err := tx.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1 FROM sanctions
  WHERE target_account_id = $1
    AND type IN ('temp_ban', 'perm_ban')
    AND revoked_at IS NULL
    AND (expires_at IS NULL OR expires_at > statement_timestamp())
)`, accountID).Scan(&exists)
	return exists, err
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
  AND (expires_at IS NULL OR expires_at > statement_timestamp())
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
  AND (expires_at IS NULL OR expires_at > statement_timestamp())
LIMIT 1`, accountID).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *SanctionStore) IsMMBanned(ctx context.Context, accountID uuid.UUID) (bool, error) {
	if s == nil || s.Pool == nil {
		return false, errStoreNotConfigured
	}
	var one int
	err := s.Pool.QueryRow(ctx, `
SELECT 1 FROM sanctions
WHERE target_account_id = $1
  AND type = 'mm_ban'
  AND revoked_at IS NULL
  AND (expires_at IS NULL OR expires_at > statement_timestamp())
LIMIT 1`, accountID).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// ListExpiredActiveTempBans returns temp_ban rows past expires_at that are not revoked.
func (s *SanctionStore) ListExpiredActiveTempBans(ctx context.Context, limit int) ([]SanctionRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errStoreNotConfigured
	}
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.Pool.Query(ctx, `
SELECT id, target_account_id, type, reason, report_id, issued_by, expires_at, revoked_at, revoked_by, created_at
FROM sanctions
WHERE type = 'temp_ban'
  AND revoked_at IS NULL
  AND expires_at IS NOT NULL
  AND expires_at <= now()
ORDER BY expires_at ASC
LIMIT $1`, limit)
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
