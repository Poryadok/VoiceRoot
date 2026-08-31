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

// ReportListPage is one page of the moderation queue (priority sort + cursor).
type ReportListPage struct {
	Rows       []ReportRow
	NextCursor string
}

type reportListCursorPayload struct {
	P int    `json:"p"` // category priority rank (1=harassment … 4=other)
	S string `json:"s"` // created_at RFC3339Nano UTC
	I string `json:"i"` // report id UUID
}

func reportCategoryPriority(category string) int {
	switch category {
	case "harassment":
		return 1
	case "fake":
		return 2
	case "spam":
		return 3
	default:
		return 4
	}
}

func encodeReportListCursor(priority int, createdAt time.Time, reportID uuid.UUID) string {
	p := reportListCursorPayload{
		P: priority,
		S: createdAt.UTC().Format(time.RFC3339Nano),
		I: reportID.String(),
	}
	b, _ := json.Marshal(p)
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeReportListCursor(raw string) (priority int, createdAt time.Time, reportID uuid.UUID, err error) {
	if raw == "" {
		return 0, time.Time{}, uuid.Nil, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return 0, time.Time{}, uuid.Nil, ErrInvalidReportListCursor
	}
	var p reportListCursorPayload
	if err := json.Unmarshal(b, &p); err != nil {
		return 0, time.Time{}, uuid.Nil, ErrInvalidReportListCursor
	}
	ts, err := time.Parse(time.RFC3339Nano, p.S)
	if err != nil {
		return 0, time.Time{}, uuid.Nil, ErrInvalidReportListCursor
	}
	id, err := uuid.Parse(p.I)
	if err != nil {
		return 0, time.Time{}, uuid.Nil, ErrInvalidReportListCursor
	}
	return p.P, ts.UTC(), id, nil
}

// ErrInvalidReportListCursor is returned when ListReportsFilteredPage receives a bad cursor.
var ErrInvalidReportListCursor = errors.New("invalid report list cursor")

func (s *ReportStore) ListReportsFiltered(ctx context.Context, statusFilter, queueFilter string, limit int32) ([]ReportRow, error) {
	page, err := s.ListReportsFilteredPage(ctx, statusFilter, queueFilter, "", limit)
	if err != nil {
		return nil, err
	}
	return page.Rows, nil
}

func (s *ReportStore) ListReportsFilteredPage(ctx context.Context, statusFilter, queueFilter, cursor string, limit int32) (*ReportListPage, error) {
	if s == nil || s.Pool == nil {
		return nil, errStoreNotConfigured
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	fetch := limit + 1

	cursorPriority, cursorCreatedAt, cursorID, err := decodeReportListCursor(cursor)
	if err != nil {
		return nil, err
	}

	queueSQL := ""
	switch queueFilter {
	case "content":
		queueSQL = " AND target_type IN ('user', 'message', 'story')"
	case "spaces":
		queueSQL = " AND target_type = 'space'"
	}

	priorityExpr := `CASE category
  WHEN 'harassment' THEN 1
  WHEN 'fake' THEN 2
  WHEN 'spam' THEN 3
  ELSE 4 END`

	cursorSQL := ""
	args := []any{statusFilter}
	argN := 2
	if cursor != "" {
		cursorSQL = ` AND (
  (` + priorityExpr + `) > $` + strconv.Itoa(argN) + `
  OR ((` + priorityExpr + `) = $` + strconv.Itoa(argN) + ` AND created_at < $` + strconv.Itoa(argN+1) + `)
  OR ((` + priorityExpr + `) = $` + strconv.Itoa(argN) + ` AND created_at = $` + strconv.Itoa(argN+1) + ` AND id < $` + strconv.Itoa(argN+2) + `::uuid)
)`
		args = append(args, cursorPriority, cursorCreatedAt, cursorID.String())
		argN += 3
	}
	args = append(args, fetch)

	query := `
SELECT id, reporter_profile_id, target_type, target_id, category, description,
       evidence::text, status, assigned_to, resolved_at, resolution::text, created_at
FROM reports
WHERE ($1 = '' OR status = $1)` + queueSQL + cursorSQL + `
ORDER BY ` + priorityExpr + `, created_at DESC, id DESC
LIMIT $` + strconv.Itoa(argN)

	rows, err := s.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ReportRow, 0, fetch)
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
	if err := rows.Err(); err != nil {
		return nil, err
	}

	page := &ReportListPage{Rows: out}
	if int32(len(out)) > limit {
		page.Rows = out[:limit]
		last := page.Rows[len(page.Rows)-1]
		page.NextCursor = encodeReportListCursor(reportCategoryPriority(last.Category), last.CreatedAt, last.ID)
	}
	return page, nil
}
