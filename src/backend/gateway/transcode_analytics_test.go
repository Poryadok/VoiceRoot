package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAnalyticsTimeRangeParsesFromTo(t *testing.T) {
	fromTS := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	toTS := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/dashboard/product?from="+fromTS.Format(time.RFC3339)+"&to="+toTS.Format(time.RFC3339), nil)
	from, to, err := parseAnalyticsTimeRange(req, time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.NotNil(t, from)
	require.NotNil(t, to)
	require.Equal(t, fromTS, from.AsTime().UTC())
	require.Equal(t, toTS, to.AsTime().UTC())
}

func TestAnalyticsFiltersEventType(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/metrics?metric=health&event_type=api_request", nil)
	filters := analyticsFilters(req)
	require.Equal(t, "api_request", filters["event_type"])
}

func TestAnalyticsFiltersRejectsUndocumentedArbitraryFilterInputs(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/metrics?metric=health&filter_profile_id=raw-profile&filter_query=private+text", nil)
	require.Nil(t, analyticsFilters(req), "Analytics accepts only the existing event_type filter, never arbitrary identifier or query inputs")
}

func TestAnalyticsTimeRangeEmpty(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/retention", nil)
	from, to, err := parseAnalyticsTimeRange(req, time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Equal(t, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), from.AsTime().UTC())
	require.Equal(t, time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC), to.AsTime().UTC())
}

func TestAnalyticsTimeRangeInvalidRejected(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/retention?from=not-a-date", nil)
	_, _, err := parseAnalyticsTimeRange(req, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	require.Error(t, err)
}

func TestParseAnalyticsTimeRangeRejectsMalformedAndReversedQuery(t *testing.T) {
	tests := []string{
		"/api/v1/analytics/dashboard/health?from=not-a-date",
		"/api/v1/analytics/dashboard/health?to=not-a-date",
		"/api/v1/analytics/dashboard/health?from=2026-01-02T00:00:00Z&to=2026-01-01T00:00:00Z",
		"/api/v1/analytics/dashboard/health?from=2026-01-01T00:00:00Z&to=2026-01-01T00:00:00Z",
	}
	for _, rawURL := range tests {
		t.Run(rawURL, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, rawURL, nil)
			_, _, err := parseAnalyticsTimeRange(req, time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC))
			require.Error(t, err)
		})
	}
}

func TestParseAnalyticsTimeRangeDefaultsMissingBounds(t *testing.T) {
	now := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/dashboard/engagement", nil)
	from, to, err := parseAnalyticsTimeRange(req, now)
	require.NoError(t, err)
	require.Equal(t, now.Add(-30*24*time.Hour), from.AsTime().UTC())
	require.Equal(t, now, to.AsTime().UTC())
}
