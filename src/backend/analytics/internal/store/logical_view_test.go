package store

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAnalyticsDDLDefinesNonDestructiveLogicalEventsView(t *testing.T) {
	ddl, err := os.ReadFile("../../../../../docker/clickhouse/init/001_events.sql")
	require.NoError(t, err)
	text := string(ddl)
	require.Contains(t, text, "CREATE VIEW IF NOT EXISTS voice.events_logical")
	require.Contains(t, text, "GROUP BY event_id")
	require.NotContains(t, text, "ReplacingMergeTree")
}

func TestOfficialAnalyticsQueriesReadLogicalEvents(t *testing.T) {
	query, err := os.ReadFile("query.go")
	require.NoError(t, err)
	text := string(query)
	require.Contains(t, text, "voice.events_logical")
	require.NotContains(t, strings.ReplaceAll(text, "voice.events_logical", ""), "voice.events")
}

func TestStagingClickHouseApplyAlwaysAppliesAdditiveLogicalView(t *testing.T) {
	script, err := os.ReadFile("../../../../../scripts/staging/apply-clickhouse-init.sh")
	require.NoError(t, err)
	text := string(script)
	require.Contains(t, text, "clickhouse_logical_view_ready")
	require.Contains(t, text, "applying additive idempotent DDL")
	require.Contains(t, text, "apply_clickhouse_init_sql")
}
