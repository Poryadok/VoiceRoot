package store

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// DashboardMetrics returns named metrics for a dashboard type in [from, to].
func (s *CHStore) DashboardMetrics(ctx context.Context, dashboardType string, from, to time.Time) (map[string]float64, error) {
	out := map[string]float64{}
	if s == nil || s.conn == nil {
		return out, nil
	}
	switch strings.ToLower(strings.TrimSpace(dashboardType)) {
	case "product":
		dau, err := s.scalar(ctx, `
SELECT uniqExact(user_id_hashed) FROM voice.events
WHERE user_id_hashed != '' AND timestamp >= ? AND timestamp < ?`, from, to)
		if err != nil {
			return nil, err
		}
		regs, err := s.scalar(ctx, `
SELECT count() FROM voice.events
WHERE event_type = 'user_registered' AND timestamp >= ? AND timestamp < ?`, from, to)
		if err != nil {
			return nil, err
		}
		out["dau"] = dau
		out["registrations"] = regs
	case "engagement":
		msgs, err := s.scalar(ctx, `
SELECT count() FROM voice.events
WHERE event_type = 'message_sent' AND timestamp >= ? AND timestamp < ?`, from, to)
		if err != nil {
			return nil, err
		}
		calls, err := s.scalar(ctx, `
SELECT count() FROM voice.events
WHERE event_type IN ('call_started','call_ended') AND timestamp >= ? AND timestamp < ?`, from, to)
		if err != nil {
			return nil, err
		}
		out["messages_sent"] = msgs
		out["call_events"] = calls
	case "revenue":
		paid, err := s.scalar(ctx, `
SELECT count() FROM voice.events
WHERE event_type = 'payment_success' AND timestamp >= ? AND timestamp < ?`, from, to)
		if err != nil {
			return nil, err
		}
		failed, err := s.scalar(ctx, `
SELECT count() FROM voice.events
WHERE event_type = 'payment_failed' AND timestamp >= ? AND timestamp < ?`, from, to)
		if err != nil {
			return nil, err
		}
		out["payment_success"] = paid
		out["payment_failed"] = failed
	case "health":
		reqs, err := s.scalar(ctx, `
SELECT count() FROM voice.events
WHERE event_type = 'gateway_request' AND timestamp >= ? AND timestamp < ?`, from, to)
		if err != nil {
			return nil, err
		}
		out["gateway_requests"] = reqs
	case "moderation":
		reports, err := s.scalar(ctx, `
SELECT count() FROM voice.events
WHERE event_type = 'report_created' AND timestamp >= ? AND timestamp < ?`, from, to)
		if err != nil {
			return nil, err
		}
		sanctions, err := s.scalar(ctx, `
SELECT count() FROM voice.events
WHERE event_type = 'sanction_applied' AND timestamp >= ? AND timestamp < ?`, from, to)
		if err != nil {
			return nil, err
		}
		out["reports"] = reports
		out["sanctions"] = sanctions
	default:
		return nil, fmt.Errorf("unknown dashboard type %q", dashboardType)
	}
	return out, nil
}

func (s *CHStore) scalar(ctx context.Context, query string, args ...any) (float64, error) {
	row := s.conn.QueryRow(ctx, query, args...)
	var v uint64
	if err := row.Scan(&v); err != nil {
		return 0, err
	}
	return float64(v), nil
}

// FunnelSteps returns step counts for a named funnel.
func (s *CHStore) FunnelSteps(ctx context.Context, name string, from, to time.Time) (map[string]int64, error) {
	steps := map[string]int64{}
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "registration":
		for _, et := range []string{"user_registered", "profile_created", "message_sent"} {
			n, err := s.scalar(ctx, `
SELECT count() FROM voice.events WHERE event_type = ? AND timestamp >= ? AND timestamp < ?`,
				et, from, to)
			if err != nil {
				return nil, err
			}
			steps[et] = int64(n)
		}
	default:
		return nil, fmt.Errorf("unknown funnel %q", name)
	}
	return steps, nil
}

// RetentionCohorts returns D1/D7/D30 rates for registration cohorts (simplified).
func (s *CHStore) RetentionCohorts(ctx context.Context, from, to time.Time) ([]RetentionRow, error) {
	if s == nil || s.conn == nil {
		return nil, nil
	}
	rows, err := s.conn.Query(ctx, `
WITH cohort AS (
  SELECT user_id_hashed, toDate(min(timestamp)) AS cohort_date
  FROM voice.events
  WHERE event_type = 'user_registered' AND user_id_hashed != ''
    AND timestamp >= ? AND timestamp < ?
  GROUP BY user_id_hashed
),
activity AS (
  SELECT c.cohort_date, c.user_id_hashed,
    maxIf(1, e.timestamp >= c.cohort_date AND e.timestamp < c.cohort_date + 1) AS d1,
    maxIf(1, e.timestamp >= c.cohort_date AND e.timestamp < c.cohort_date + 7) AS d7,
    maxIf(1, e.timestamp >= c.cohort_date AND e.timestamp < c.cohort_date + 30) AS d30
  FROM cohort c
  LEFT JOIN voice.events e ON e.user_id_hashed = c.user_id_hashed
  GROUP BY c.cohort_date, c.user_id_hashed
)
SELECT cohort_date,
  count() AS cohort_size,
  avg(d1) AS d1_rate,
  avg(d7) AS d7_rate,
  avg(d30) AS d30_rate
FROM activity
GROUP BY cohort_date
ORDER BY cohort_date
LIMIT 30`, from, to)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []RetentionRow
	for rows.Next() {
		var r RetentionRow
		var d time.Time
		if err := rows.Scan(&d, &r.CohortSize, &r.D1, &r.D7, &r.D30); err != nil {
			return nil, err
		}
		r.CohortDate = d.Format("2006-01-02")
		out = append(out, r)
	}
	return out, rows.Err()
}

type RetentionRow struct {
	CohortDate  string
	CohortSize  int64
	D1, D7, D30 float64
}

// ExportEvents returns raw events for export.
func (s *CHStore) ExportEvents(ctx context.Context, from, to time.Time, eventType string, limit int) ([]EventRow, error) {
	if limit <= 0 || limit > 100000 {
		limit = 10000
	}
	q := `SELECT event_id, event_type, source_service, timestamp,
		user_id_hashed, profile_id_hashed, properties,
		ifNull(session_id,''), ifNull(platform,''), ifNull(app_version,''), ifNull(region,'')
		FROM voice.events WHERE timestamp >= ? AND timestamp < ?`
	args := []any{from, to}
	if strings.TrimSpace(eventType) != "" {
		q += ` AND event_type = ?`
		args = append(args, eventType)
	}
	q += ` ORDER BY timestamp LIMIT ?`
	args = append(args, limit)

	rows, err := s.conn.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []EventRow
	for rows.Next() {
		var r EventRow
		if err := rows.Scan(
			&r.EventID, &r.EventType, &r.SourceService, &r.Timestamp,
			&r.UserIDHashed, &r.ProfileIDHashed, &r.PropertiesJSON,
			&r.SessionID, &r.Platform, &r.AppVersion, &r.Region,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
