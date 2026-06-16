package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *AutoModStore) CountSpamOffenses(ctx context.Context, profileID uuid.UUID) (int, error) {
	if s == nil || s.Pool == nil {
		return 0, errStoreNotConfigured
	}
	var count int
	err := s.Pool.QueryRow(ctx, `
SELECT COUNT(*) FROM auto_mod_log
WHERE target_profile_id = $1 AND trigger = 'spam_pattern' AND reverted_at IS NULL`,
		profileID,
	).Scan(&count)
	return count, err
}

func (s *AutoModStore) InsertSpamOffense(ctx context.Context, profileID uuid.UUID, action, detailsJSON string) error {
	if s == nil || s.Pool == nil {
		return errStoreNotConfigured
	}
	if detailsJSON == "" {
		detailsJSON = "{}"
	}
	_, err := s.Pool.Exec(ctx, `
INSERT INTO auto_mod_log (target_profile_id, trigger, action, details)
VALUES ($1, 'spam_pattern', $2, $3::jsonb)`,
		profileID, action, detailsJSON,
	)
	return err
}

func (s *AutoModStore) Stats(ctx context.Context) (checked, blocked int64, err error) {
	if s == nil || s.Pool == nil {
		return 0, 0, errStoreNotConfigured
	}
	err = s.Pool.QueryRow(ctx, `
SELECT
  COALESCE(SUM(CASE WHEN trigger IN ('spam_pattern', 'report_threshold') THEN 1 ELSE 0 END), 0),
  COALESCE(SUM(CASE WHEN action IN ('mute', 'shadow_ban') THEN 1 ELSE 0 END), 0)
FROM auto_mod_log`).Scan(&checked, &blocked)
	return checked, blocked, err
}

func (s *ReportStore) GetReportByID(ctx context.Context, reportID uuid.UUID) (*ReportRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errStoreNotConfigured
	}
	row := &ReportRow{}
	err := s.Pool.QueryRow(ctx, `
SELECT id, reporter_profile_id, target_type, target_id, category, description,
       evidence::text, status, assigned_to, resolved_at, resolution::text, created_at
FROM reports WHERE id = $1`, reportID).Scan(
		&row.ID, &row.ReporterProfileID, &row.TargetType, &row.TargetID, &row.Category,
		&row.Description, &row.EvidenceJSON, &row.Status, &row.AssignedToProfile,
		&row.ResolvedAt, &row.ResolutionJSON, &row.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	return row, nil
}

func (s *ReportStore) UpdateReport(ctx context.Context, reportID uuid.UUID, status string, assignedTo *uuid.UUID, resolutionJSON *string, setResolved bool) (*ReportRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errStoreNotConfigured
	}
	row := &ReportRow{}
	var assignedAny any
	if assignedTo != nil {
		assignedAny = *assignedTo
	}
	var resolutionAny any
	if resolutionJSON != nil {
		resolutionAny = *resolutionJSON
	}
	err := s.Pool.QueryRow(ctx, `
UPDATE reports
SET status = $2,
    assigned_to = COALESCE($3, assigned_to),
    resolution = COALESCE($4::jsonb, resolution),
    resolved_at = CASE WHEN $5 THEN now() ELSE resolved_at END,
    updated_at = now()
WHERE id = $1
RETURNING id, reporter_profile_id, target_type, target_id, category, description,
          evidence::text, status, assigned_to, resolved_at, resolution::text, created_at`,
		reportID, status, assignedAny, resolutionAny, setResolved,
	).Scan(
		&row.ID, &row.ReporterProfileID, &row.TargetType, &row.TargetID, &row.Category,
		&row.Description, &row.EvidenceJSON, &row.Status, &row.AssignedToProfile,
		&row.ResolvedAt, &row.ResolutionJSON, &row.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	return row, nil
}

func (s *ReportStore) ListReportsFiltered(ctx context.Context, statusFilter, queueFilter string, limit int32) ([]ReportRow, error) {
	if s == nil || s.Pool == nil {
		return nil, errStoreNotConfigured
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	queueSQL := ""
	switch queueFilter {
	case "content":
		queueSQL = " AND target_type IN ('user', 'message', 'story')"
	case "spaces":
		queueSQL = " AND target_type = 'space'"
	}
	query := `
SELECT id, reporter_profile_id, target_type, target_id, category, description,
       evidence::text, status, assigned_to, resolved_at, resolution::text, created_at
FROM reports
WHERE ($1 = '' OR status = $1)` + queueSQL + `
ORDER BY CASE category
  WHEN 'harassment' THEN 1
  WHEN 'fake' THEN 2
  WHEN 'spam' THEN 3
  ELSE 4 END, created_at DESC
LIMIT $2`
	rows, err := s.Pool.Query(ctx, query, statusFilter, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ReportRow, 0, limit)
	for rows.Next() {
		var r ReportRow
		if err := rows.Scan(
			&r.ID, &r.ReporterProfileID, &r.TargetType, &r.TargetID, &r.Category,
			&r.Description, &r.EvidenceJSON, &r.Status, &r.AssignedToProfile,
			&r.ResolvedAt, &r.ResolutionJSON, &r.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
