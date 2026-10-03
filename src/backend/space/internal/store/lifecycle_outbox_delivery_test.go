package store

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestLifecycleOutboxDeliveryRequiresExactSavedEventAndKeepsFirstDatabaseAckTime(t *testing.T) {
	st := lifecycleStoreFixture(t)
	ctx := context.Background()
	_, _, id, _, _ := completeStoredSchedule(t, st)
	aggregate, err := st.LoadLifecycle(ctx, id)
	require.NoError(t, err)
	record := *aggregate.Snapshot().ScheduleEvent
	changed := record
	changed.Generation++
	require.Error(t, st.MarkLifecycleEventDelivered(ctx, changed))
	var state string
	var deliveredAt *time.Time
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT state,delivered_at FROM space_lifecycle_outbox WHERE event_id=$1`, record.EventID).Scan(&state, &deliveredAt))
	require.Equal(t, "READY", state)
	require.Nil(t, deliveredAt)
	require.NoError(t, st.MarkLifecycleEventDelivered(ctx, record))
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT delivered_at FROM space_lifecycle_outbox WHERE event_id=$1`, record.EventID).Scan(&deliveredAt))
	first := *deliveredAt
	require.NoError(t, st.MarkLifecycleEventDelivered(ctx, record))
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT delivered_at FROM space_lifecycle_outbox WHERE event_id=$1`, record.EventID).Scan(&deliveredAt))
	require.Equal(t, first, *deliveredAt)
}
