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
	from, to := analyticsTimeRange(req)
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

func TestAnalyticsTimeRangeEmpty(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/retention", nil)
	from, to := analyticsTimeRange(req)
	require.Nil(t, from)
	require.Nil(t, to)
}

func TestAnalyticsTimeRangeInvalidIgnored(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/retention?from=not-a-date", nil)
	from, to := analyticsTimeRange(req)
	require.Nil(t, from)
	require.Nil(t, to)
}
