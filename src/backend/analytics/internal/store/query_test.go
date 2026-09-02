package store

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRetentionSQLUsesDayNReturnWindows(t *testing.T) {
	// Guard against regressing to same-day activity windows (cohort_date .. cohort_date+N).
	require.Contains(t, retentionCohortQuery, "toDate(e.timestamp) = c.cohort_date + 1")
	require.Contains(t, retentionCohortQuery, "toDate(e.timestamp) = c.cohort_date + 7")
	require.Contains(t, retentionCohortQuery, "toDate(e.timestamp) = c.cohort_date + 30")
	require.NotContains(t, retentionCohortQuery, "timestamp < c.cohort_date + 1")
}

func TestResolveHealthEventTypeDefaultsToAPIRequest(t *testing.T) {
	require.Equal(t, "api_request", resolveHealthEventType(QueryFilters{}))
	require.Equal(t, "api_request", resolveHealthEventType(QueryFilters{EventType: "  "}))
	require.Equal(t, "custom_event", resolveHealthEventType(QueryFilters{EventType: "custom_event"}))
}
