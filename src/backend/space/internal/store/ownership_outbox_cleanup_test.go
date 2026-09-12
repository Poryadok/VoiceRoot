package store

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func ownershipOutboxRowExists(t *testing.T, st *SpaceStore, eventID uuid.UUID) bool {
	t.Helper()
	return ownershipOutboxRowExistsIn(t, context.Background(), st.db(), eventID)
}

func ownershipOutboxRowExistsIn(t *testing.T, ctx context.Context, db spaceStoreDB, eventID uuid.UUID) bool {
	t.Helper()
	var exists bool
	require.NoError(t, db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM ownership_outbox WHERE event_id=$1)`, eventID).Scan(&exists))
	return exists
}

func TestOwnershipOutboxCleanup_UsesPostgresThirtyDayBoundaryAndTerminalPredicate(t *testing.T) {
	st := ownershipOutboxDeliveryStoreFixture(t)
	createdAt := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	exactBoundaryID := uuid.New()
	recentID := uuid.New()
	undeliveredID := uuid.New()
	leasedID := uuid.New()
	for _, eventID := range []uuid.UUID{exactBoundaryID, recentID, undeliveredID, leasedID} {
		seedOwnershipOutboxDeliveryEvent(t, st, eventID, createdAt)
	}

	ctx := context.Background()
	tx, err := st.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(context.Background()) }()
	txStore := *st
	txStore.tx = tx
	_, err = tx.Exec(ctx, `UPDATE ownership_outbox
		SET ready=FALSE,
			delivered_at=CASE event_id
				WHEN $1 THEN transaction_timestamp()-interval '30 days'
				WHEN $2 THEN transaction_timestamp()-interval '30 days'+interval '1 microsecond'
			END
		WHERE event_id IN ($1,$2)`, exactBoundaryID, recentID)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `UPDATE ownership_outbox
		SET lease_token=$2,lease_expires_at=clock_timestamp()+interval '30 seconds'
		WHERE event_id=$1`, leasedID, uuid.New())
	require.NoError(t, err)

	deleted, err := txStore.CleanupDeliveredOwnershipOutbox(ctx, 100)
	require.NoError(t, err)
	require.Equal(t, int64(1), deleted)
	require.False(t, ownershipOutboxRowExistsIn(t, ctx, tx, exactBoundaryID), "the documented <= boundary expires at equality using one stable PostgreSQL transaction time")
	require.True(t, ownershipOutboxRowExistsIn(t, ctx, tx, recentID), "a row exactly one microsecond newer than the boundary must survive")
	require.True(t, ownershipOutboxRowExistsIn(t, ctx, tx, undeliveredID), "age alone must never delete an unacknowledged row")
	require.True(t, ownershipOutboxRowExistsIn(t, ctx, tx, leasedID), "an in-flight undelivered lease must never be cleanup-eligible")
}

func TestOwnershipOutboxCleanup_DeletesAtMostHundredOldestDeliveredRows(t *testing.T) {
	st := ownershipOutboxDeliveryStoreFixture(t)
	base := time.Now().UTC().Add(-40 * 24 * time.Hour)
	wantRemaining := make([]uuid.UUID, 0, 2)
	for index := 0; index < 102; index++ {
		eventID := uuid.MustParse(fmt.Sprintf("00000000-0000-4000-8000-%012d", index+1))
		seedOwnershipOutboxDeliveryEvent(t, st, eventID, base)
		deliveredAt := base.Add(time.Duration(index) * time.Minute)
		_, err := st.Pool.Exec(context.Background(), `UPDATE ownership_outbox
			SET ready=FALSE,delivered_at=$2 WHERE event_id=$1`, eventID, deliveredAt)
		require.NoError(t, err)
		if index >= 100 {
			wantRemaining = append(wantRemaining, eventID)
		}
	}

	deleted, err := st.CleanupDeliveredOwnershipOutbox(context.Background(), 100)
	require.NoError(t, err)
	require.Equal(t, int64(100), deleted)
	rows, err := st.Pool.Query(context.Background(), `SELECT event_id FROM ownership_outbox ORDER BY delivered_at,event_id`)
	require.NoError(t, err)
	defer rows.Close()
	var remaining []uuid.UUID
	for rows.Next() {
		var eventID uuid.UUID
		require.NoError(t, rows.Scan(&eventID))
		remaining = append(remaining, eventID)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, wantRemaining, remaining, "cleanup must remove the oldest delivered evidence first")
}

func TestOwnershipOutboxCleanup_SkipsLockedExpiredRows(t *testing.T) {
	st := ownershipOutboxDeliveryStoreFixture(t)
	oldestID := uuid.MustParse("00000000-0000-4000-8000-000000000001")
	secondID := uuid.MustParse("00000000-0000-4000-8000-000000000002")
	base := time.Now().UTC().Add(-40 * 24 * time.Hour)
	for index, eventID := range []uuid.UUID{oldestID, secondID} {
		seedOwnershipOutboxDeliveryEvent(t, st, eventID, base)
		_, err := st.Pool.Exec(context.Background(), `UPDATE ownership_outbox
			SET ready=FALSE,delivered_at=$2 WHERE event_id=$1`, eventID, base.Add(time.Duration(index)*time.Minute))
		require.NoError(t, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	blocker, err := st.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback(context.Background()) }()
	_, err = blocker.Exec(ctx, `SELECT event_id FROM ownership_outbox WHERE event_id=$1 FOR UPDATE`, oldestID)
	require.NoError(t, err)

	deleted, err := st.CleanupDeliveredOwnershipOutbox(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, int64(1), deleted)
	require.True(t, ownershipOutboxRowExists(t, st, oldestID))
	require.False(t, ownershipOutboxRowExists(t, st, secondID), "FOR UPDATE SKIP LOCKED must let cleanup make progress")
	require.NoError(t, blocker.Commit(ctx))

	deleted, err = st.CleanupDeliveredOwnershipOutbox(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, int64(1), deleted)
	require.False(t, ownershipOutboxRowExists(t, st, oldestID))
}

func TestOwnershipOutboxCleanup_InvalidLimitAndDatabaseFailureFailClosed(t *testing.T) {
	st := ownershipOutboxDeliveryStoreFixture(t)
	for _, limit := range []int{-1, 0, 101} {
		deleted, err := st.CleanupDeliveredOwnershipOutbox(context.Background(), limit)
		require.Error(t, err)
		require.Zero(t, deleted)
	}
	st.Pool.Close()
	deleted, err := st.CleanupDeliveredOwnershipOutbox(context.Background(), 1)
	require.Error(t, err)
	require.Zero(t, deleted)
}
