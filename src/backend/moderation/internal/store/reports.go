package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ReportStore persists reports and auto-mod audit rows in moderation_db.
type ReportStore struct {
	Pool *pgxpool.Pool
}

var errStoreNotConfigured = errors.New("report store is not configured")

type ReportRow struct {
	ID                 uuid.UUID
	ReporterProfileID  uuid.UUID
	TargetType         string
	TargetID           uuid.UUID
	Category           string
	Description        *string
	EvidenceJSON       string
	Status             string
	AssignedToProfile  *uuid.UUID
	ResolvedAt         *time.Time
	ResolutionJSON     *string
	CreatedAt          time.Time
}

func (s *ReportStore) InsertReport(
	ctx context.Context,
	reporterProfileID uuid.UUID,
	targetType string,
	targetID uuid.UUID,
	category string,
	description *string,
	evidenceJSON string,
) (*ReportRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errStoreNotConfigured
	}
	row := &ReportRow{}
	err := s.Pool.QueryRow(ctx, `
INSERT INTO reports (reporter_profile_id, target_type, target_id, category, description, evidence)
VALUES ($1, $2, $3, $4, $5, $6::jsonb)
RETURNING id, reporter_profile_id, target_type, target_id, category, description,
          evidence::text, status, assigned_to, resolved_at, resolution::text, created_at`,
		reporterProfileID, targetType, targetID, category, description, evidenceJSON,
	).Scan(
		&row.ID,
		&row.ReporterProfileID,
		&row.TargetType,
		&row.TargetID,
		&row.Category,
		&row.Description,
		&row.EvidenceJSON,
		&row.Status,
		&row.AssignedToProfile,
		&row.ResolvedAt,
		&row.ResolutionJSON,
		&row.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return row, nil
}

func (s *ReportStore) CountReports24h(ctx context.Context, targetProfileID uuid.UUID) (int, error) {
	if s == nil || s.Pool == nil {
		return 0, errStoreNotConfigured
	}
	var count int
	err := s.Pool.QueryRow(ctx, `
SELECT COUNT(*)
FROM reports
WHERE target_type = 'user'
  AND target_id = $1
  AND created_at > now() - interval '24 hours'`,
		targetProfileID,
	).Scan(&count)
	return count, err
}

func (s *ReportStore) InsertAutoModLog(ctx context.Context, targetProfileID uuid.UUID, trigger, action, detailsJSON string) error {
	if s == nil || s.Pool == nil {
		return errStoreNotConfigured
	}
	_, err := s.Pool.Exec(ctx, `
INSERT INTO auto_mod_log (target_profile_id, trigger, action, details)
VALUES ($1, $2, $3, $4::jsonb)`,
		targetProfileID, trigger, action, detailsJSON,
	)
	return err
}

func (s *ReportStore) ListReports(ctx context.Context, statusFilter string, limit int32) ([]ReportRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errStoreNotConfigured
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.Pool.Query(ctx, `
SELECT id, reporter_profile_id, target_type, target_id, category, description,
       evidence::text, status, assigned_to, resolved_at, resolution::text, created_at
FROM reports
WHERE ($1 = '' OR status = $1)
ORDER BY created_at DESC
LIMIT $2`, statusFilter, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ReportRow, 0, limit)
	for rows.Next() {
		var r ReportRow
		if err := rows.Scan(
			&r.ID,
			&r.ReporterProfileID,
			&r.TargetType,
			&r.TargetID,
			&r.Category,
			&r.Description,
			&r.EvidenceJSON,
			&r.Status,
			&r.AssignedToProfile,
			&r.ResolvedAt,
			&r.ResolutionJSON,
			&r.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
