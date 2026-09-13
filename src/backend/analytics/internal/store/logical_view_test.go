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
