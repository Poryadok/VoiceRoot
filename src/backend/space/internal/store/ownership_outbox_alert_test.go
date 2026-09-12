package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestCountAlertingOwnershipOutbox_UsesPersistedConsecutiveFailureThreshold(t *testing.T) {
	st := ownershipOutboxDeliveryStoreFixture(t)
	ctx := context.Background()
	eventID := uuid.New()
	seedOwnershipOutboxDeliveryEvent(t, st, eventID, time.Now().UTC())

	for attempt := 1; attempt <= 9; attempt++ {
		claimed, err := st.ClaimReadyOwnershipOutbox(ctx, 1)
		require.NoError(t, err)
		require.Len(t, claimed, 1)
		require.Equal(t, eventID, claimed[0].EventID)
		applied, err := st.MarkOwnershipOutboxFailed(ctx, eventID, claimed[0].LeaseToken)
		require.NoError(t, err)
		require.True(t, applied)
		if attempt < 9 {
			_, err = st.Pool.Exec(ctx, `UPDATE ownership_outbox SET next_attempt_at='-infinity'::timestamptz WHERE event_id=$1`, eventID)
			require.NoError(t, err)
		}
	}

	alerting, err := st.CountAlertingOwnershipOutbox(ctx)
	require.NoError(t, err)
	require.Zero(t, alerting, "nine persisted failures are below the documented alert threshold")
	staleApplied, err := st.MarkOwnershipOutboxFailed(ctx, eventID, uuid.New())
	require.NoError(t, err)
	require.False(t, staleApplied)
	alerting, err = st.CountAlertingOwnershipOutbox(ctx)
	require.NoError(t, err)
	require.Zero(t, alerting, "a stale fencing token must not invent a tenth failure")

	_, err = st.Pool.Exec(ctx, `UPDATE ownership_outbox SET next_attempt_at='-infinity'::timestamptz WHERE event_id=$1`, eventID)
	require.NoError(t, err)
	claimed, err := st.ClaimReadyOwnershipOutbox(ctx, 1)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	applied, err := st.MarkOwnershipOutboxFailed(ctx, eventID, claimed[0].LeaseToken)
	require.NoError(t, err)
	require.True(t, applied)

	alerting, err = st.CountAlertingOwnershipOutbox(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1), alerting, "the persisted tenth consecutive failure must become alerting")
	restartedStore := &SpaceStore{Pool: st.Pool}
	alerting, err = restartedStore.CountAlertingOwnershipOutbox(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1), alerting, "a fresh process store must reconstruct alert state from PostgreSQL")

	var rowCount, attempts int
	var ready bool
	var deliveredAt *time.Time
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*),max(attempt_count),bool_and(ready),max(delivered_at)
		FROM ownership_outbox WHERE event_id=$1`, eventID).Scan(&rowCount, &attempts, &ready, &deliveredAt))
	require.Equal(t, 1, rowCount, "alerting must retain the retry row")
	require.Equal(t, 10, attempts)
	require.True(t, ready, "alerting must not discard or terminalize the retry row")
	require.Nil(t, deliveredAt)
}

func TestCountAlertingOwnershipOutbox_ExcludesTerminalAndIrrelevantRows(t *testing.T) {
	st := ownershipOutboxDeliveryStoreFixture(t)
	ctx := context.Background()
	deliveredID := uuid.New()
	nonReadyID := uuid.New()
	belowThresholdID := uuid.New()
	for _, eventID := range []uuid.UUID{deliveredID, nonReadyID, belowThresholdID} {
		seedOwnershipOutboxDeliveryEvent(t, st, eventID, time.Now().UTC())
	}
	_, err := st.Pool.Exec(ctx, `UPDATE ownership_outbox
		SET ready=FALSE,attempt_count=10,last_failure_at=clock_timestamp(),delivered_at=clock_timestamp()
		WHERE event_id=$1`, deliveredID)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `UPDATE ownership_outbox
		SET ready=FALSE,attempt_count=10,last_failure_at=clock_timestamp()
		WHERE event_id=$1`, nonReadyID)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `UPDATE ownership_outbox
		SET attempt_count=9,last_failure_at=clock_timestamp()
		WHERE event_id=$1`, belowThresholdID)
	require.NoError(t, err)

	alerting, err := st.CountAlertingOwnershipOutbox(ctx)
	require.NoError(t, err)
	require.Zero(t, alerting, "delivered, non-ready, and below-threshold rows are not active delivery alerts")
}

func TestCountAlertingOwnershipOutbox_DatabaseFailureFailsClosed(t *testing.T) {
	st := ownershipOutboxDeliveryStoreFixture(t)
	st.Pool.Close()

	alerting, err := st.CountAlertingOwnershipOutbox(context.Background())
	require.Error(t, err)
	require.Zero(t, alerting, "database failure must not invent a healthy or alerting count")
}
