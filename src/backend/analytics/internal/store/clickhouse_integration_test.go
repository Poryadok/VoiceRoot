//go:build integration

package store_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"voice/backend/analytics/internal/store"
)

func TestClickHouseInsertBatch(t *testing.T) {
	dsn := os.Getenv("CLICKHOUSE_DSN")
	if dsn == "" {
		t.Skip("CLICKHOUSE_DSN not set")
	}
	ctx := context.Background()
	ch, err := store.Open(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ch.Close() })

	eventID := "11111111-1111-1111-1111-111111111111"
	err = ch.InsertBatch(ctx, []store.EventRow{{
		EventID:        eventID,
		EventType:      "integration_test",
		SourceService:  "analytics",
		Timestamp:      time.Now().UTC(),
		PropertiesJSON: `{"ok":true}`,
	}})
	require.NoError(t, err)
}
