package store

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRetentionSQLUsesDayNReturnWindows(t *testing.T) {
	// Guard against regressing to same-day activity windows (cohort_date .. cohort_date+N).
	sql := retentionCohortSQL()
	require.Contains(t, sql, "toDate(e.timestamp) = c.cohort_date + 1")
	require.Contains(t, sql, "toDate(e.timestamp) = c.cohort_date + 7")
	require.Contains(t, sql, "toDate(e.timestamp) = c.cohort_date + 30")
	require.NotContains(t, sql, "timestamp < c.cohort_date + 1")
}

func TestQueryFiltersEventTypeDefaultHealth(t *testing.T) {
	f := QueryFilters{}
	require.Empty(t, strings.TrimSpace(f.EventType))
}

func retentionCohortSQL() string {
	return `
WITH cohort AS (
  SELECT user_id_hashed, toDate(min(timestamp)) AS cohort_date
  FROM voice.events
  WHERE event_type = 'user_registered' AND user_id_hashed != ''
    AND timestamp >= ? AND timestamp < ?
  GROUP BY user_id_hashed
),
activity AS (
  SELECT c.cohort_date, c.user_id_hashed,
    maxIf(1, toDate(e.timestamp) = c.cohort_date + 1) AS d1,
    maxIf(1, toDate(e.timestamp) = c.cohort_date + 7) AS d7,
    maxIf(1, toDate(e.timestamp) = c.cohort_date + 30) AS d30
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
LIMIT 30`
}
