package store

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func ownershipOutboxDeliveryMigrationSQL(t *testing.T, direction string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "migrations", "space_db", "000014_ownership_outbox_delivery."+direction+".sql"))
	require.NoError(t, err)
	return string(raw)
}

func ownershipOutboxDeliveryStoreFixture(t *testing.T) *SpaceStore {
	t.Helper()
	st := lifecycleStoreFixture(t)
	_, err := st.Pool.Exec(context.Background(), ownershipOutboxDeliveryMigrationSQL(t, "up"))
	require.NoError(t, err)
	return st
}

func seedOwnershipOutboxDeliveryEvent(t *testing.T, st *SpaceStore, eventID uuid.UUID, createdAt time.Time) uuid.UUID {
	t.Helper()
	spaceID := uuid.New()
	_, err := st.Pool.Exec(context.Background(), `INSERT INTO ownership_outbox(
		event_id,operation_id,space_id,previous_owner_profile_id,new_owner_profile_id,event_type,ready,created_at
	) VALUES($1,$2,$3,$4,$5,'space.updated',TRUE,$6)`,
		eventID, uuid.New(), spaceID, uuid.New(), uuid.New(), createdAt)
	require.NoError(t, err)
	return spaceID
}

func TestOwnershipOutboxDeliveryMigration_AddsLeaseRetryAndRetentionFieldsWithoutLosingRows(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL ownership outbox migration")
	}
	st := lifecycleStoreFixture(t)
	eventID := uuid.New()
	createdAt := time.Date(2026, time.September, 12, 8, 0, 0, 123456000, time.UTC)
	seedOwnershipOutboxDeliveryEvent(t, st, eventID, createdAt)

	_, err := st.Pool.Exec(context.Background(), ownershipOutboxDeliveryMigrationSQL(t, "up"))
	require.NoError(t, err)

	var attempts int
	var nextAttempt, databaseNow time.Time
	var leaseToken *uuid.UUID
	var leaseExpiresAt, lastFailureAt, deliveredAt *time.Time
	require.NoError(t, st.Pool.QueryRow(context.Background(), `SELECT attempt_count,next_attempt_at,
		lease_token,lease_expires_at,last_failure_at,delivered_at,observed.database_now
		FROM ownership_outbox, LATERAL (SELECT clock_timestamp() AS database_now) AS observed
		WHERE event_id=$1`, eventID).Scan(
		&attempts, &nextAttempt, &leaseToken, &leaseExpiresAt, &lastFailureAt, &deliveredAt, &databaseNow,
	))
	require.Zero(t, attempts)
	require.False(t, nextAttempt.After(databaseNow), "existing ready rows must remain immediately claimable")
	require.Nil(t, leaseToken)
	require.Nil(t, leaseExpiresAt)
	require.Nil(t, lastFailureAt)
	require.Nil(t, deliveredAt)

	var indexDefinition string
	require.NoError(t, st.Pool.QueryRow(context.Background(), `SELECT indexdef FROM pg_indexes
		WHERE schemaname=current_schema() AND indexname='ownership_outbox_delivery_ready_idx'`).Scan(&indexDefinition))
	require.Contains(t, indexDefinition, "next_attempt_at")
	require.Contains(t, indexDefinition, "created_at")
	require.Contains(t, indexDefinition, "event_id")
	require.Contains(t, indexDefinition, "delivered_at IS NULL")
}

func TestOwnershipOutboxDelivery_ClaimLimitsOrdersLeasesAndCommits(t *testing.T) {
	st := ownershipOutboxDeliveryStoreFixture(t)
	createdAt := time.Date(2026, time.September, 12, 8, 1, 0, 0, time.UTC)
	wantIDs := make([]uuid.UUID, 0, 102)
	for index := 102; index >= 1; index-- {
		eventID := uuid.MustParse(fmt.Sprintf("00000000-0000-4000-8000-%012d", index))
		seedOwnershipOutboxDeliveryEvent(t, st, eventID, createdAt)
		wantIDs = append(wantIDs, eventID)
	}
	slices.SortFunc(wantIDs, func(left, right uuid.UUID) int { return bytes.Compare(left[:], right[:]) })

	claimed, err := st.ClaimReadyOwnershipOutbox(context.Background(), 100)
	require.NoError(t, err)
	require.Len(t, claimed, 100)
	seenTokens := make(map[uuid.UUID]struct{}, len(claimed))
	for index, event := range claimed {
		require.Equal(t, wantIDs[index], event.EventID)
		require.Equal(t, "space.updated", event.EventType)
		require.Equal(t, createdAt, event.CreatedAt)
		require.NotEqual(t, uuid.Nil, event.LeaseToken)
		_, duplicate := seenTokens[event.LeaseToken]
		require.False(t, duplicate, "every claimed row needs a random fencing token")
		seenTokens[event.LeaseToken] = struct{}{}
	}

	var storedToken uuid.UUID
	var leaseExpiresAt, databaseNow time.Time
	require.NoError(t, st.Pool.QueryRow(context.Background(), `SELECT lease_token,lease_expires_at,clock_timestamp()
		FROM ownership_outbox WHERE event_id=$1`, claimed[0].EventID).Scan(&storedToken, &leaseExpiresAt, &databaseNow))
	require.Equal(t, claimed[0].LeaseToken, storedToken, "claim must commit before returning to the network publisher")
	require.GreaterOrEqual(t, leaseExpiresAt.Sub(databaseNow), 28*time.Second)
	require.LessOrEqual(t, leaseExpiresAt.Sub(databaseNow), 31*time.Second)

	remainder, err := st.ClaimReadyOwnershipOutbox(context.Background(), 100)
	require.NoError(t, err)
	require.Len(t, remainder, 2)
	require.Equal(t, wantIDs[100], remainder[0].EventID)
	require.Equal(t, wantIDs[101], remainder[1].EventID)
}

func TestOwnershipOutboxDelivery_ClaimSkipsLockedRowsAndReclaimsExpiredLeaseWithFreshToken(t *testing.T) {
	st := ownershipOutboxDeliveryStoreFixture(t)
	createdAt := time.Date(2026, time.September, 12, 8, 2, 0, 0, time.UTC)
	firstID := uuid.MustParse("00000000-0000-4000-8000-000000000001")
	secondID := uuid.MustParse("00000000-0000-4000-8000-000000000002")
	seedOwnershipOutboxDeliveryEvent(t, st, firstID, createdAt)
	seedOwnershipOutboxDeliveryEvent(t, st, secondID, createdAt)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	blocker, err := st.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback(context.Background()) }()
	_, err = blocker.Exec(ctx, `SELECT event_id FROM ownership_outbox WHERE event_id=$1 FOR UPDATE`, firstID)
	require.NoError(t, err)

	claimed, err := st.ClaimReadyOwnershipOutbox(ctx, 1)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.Equal(t, secondID, claimed[0].EventID, "FOR UPDATE SKIP LOCKED must let another dispatcher make progress")
	require.NoError(t, blocker.Commit(ctx))

	firstClaim, err := st.ClaimReadyOwnershipOutbox(ctx, 1)
	require.NoError(t, err)
	require.Len(t, firstClaim, 1)
	require.Equal(t, firstID, firstClaim[0].EventID)
	immediate, err := st.ClaimReadyOwnershipOutbox(ctx, 1)
	require.NoError(t, err)
	require.Empty(t, immediate, "an active lease must prevent concurrent replay")

	_, err = st.Pool.Exec(ctx, `UPDATE ownership_outbox SET lease_expires_at=clock_timestamp()-interval '1 second' WHERE event_id=$1`, firstID)
	require.NoError(t, err)
	replayed, err := st.ClaimReadyOwnershipOutbox(ctx, 1)
	require.NoError(t, err)
	require.Len(t, replayed, 1)
	require.Equal(t, firstID, replayed[0].EventID)
	require.NotEqual(t, firstClaim[0].LeaseToken, replayed[0].LeaseToken)
}

func TestOwnershipOutboxDelivery_KnownFailureUsesDatabaseTimeExponentialJitterAndCap(t *testing.T) {
	st := ownershipOutboxDeliveryStoreFixture(t)
	eventID := uuid.New()
	seedOwnershipOutboxDeliveryEvent(t, st, eventID, time.Now().UTC())

	for attempt := 1; attempt <= 10; attempt++ {
		claimed, err := st.ClaimReadyOwnershipOutbox(context.Background(), 1)
		require.NoError(t, err)
		require.Len(t, claimed, 1)
		applied, err := st.MarkOwnershipOutboxFailed(context.Background(), eventID, claimed[0].LeaseToken)
		require.NoError(t, err)
		require.True(t, applied)

		var storedAttempts int
		var delaySeconds float64
		var token *uuid.UUID
		require.NoError(t, st.Pool.QueryRow(context.Background(), `SELECT attempt_count,
			EXTRACT(EPOCH FROM (next_attempt_at-last_failure_at)),lease_token
			FROM ownership_outbox WHERE event_id=$1`, eventID).Scan(&storedAttempts, &delaySeconds, &token))
		require.Equal(t, attempt, storedAttempts)
		require.Nil(t, token, "known failure must release the lease")
		baseSeconds := float64(uint64(1) << min(attempt-1, 9))
		lower := min(300, baseSeconds*0.8)
		upper := min(300, baseSeconds*1.2)
		require.GreaterOrEqual(t, delaySeconds, lower-0.001)
		require.LessOrEqual(t, delaySeconds, upper+0.001)

		_, err = st.Pool.Exec(context.Background(), `UPDATE ownership_outbox SET next_attempt_at='-infinity'::timestamptz WHERE event_id=$1`, eventID)
		require.NoError(t, err)
	}
}

func TestOwnershipOutboxDelivery_DeliveryAckUsesEventAndTokenCASAndRetainsRow(t *testing.T) {
	st := ownershipOutboxDeliveryStoreFixture(t)
	eventID := uuid.New()
	seedOwnershipOutboxDeliveryEvent(t, st, eventID, time.Now().UTC())
	claimed, err := st.ClaimReadyOwnershipOutbox(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, claimed, 1)

	applied, err := st.MarkOwnershipOutboxDelivered(context.Background(), eventID, uuid.New())
	require.NoError(t, err)
	require.False(t, applied)
	applied, err = st.MarkOwnershipOutboxFailed(context.Background(), eventID, uuid.New())
	require.NoError(t, err)
	require.False(t, applied)

	applied, err = st.MarkOwnershipOutboxDelivered(context.Background(), eventID, claimed[0].LeaseToken)
	require.NoError(t, err)
	require.True(t, applied)

	var rows int
	var deliveredAt *time.Time
	var token *uuid.UUID
	require.NoError(t, st.Pool.QueryRow(context.Background(), `SELECT delivered_at,lease_token
		FROM ownership_outbox WHERE event_id=$1`, eventID).Scan(&deliveredAt, &token))
	require.NoError(t, st.Pool.QueryRow(context.Background(), `SELECT count(*)
		FROM ownership_outbox WHERE event_id=$1`, eventID).Scan(&rows))
	require.Equal(t, 1, rows, "delivery acknowledgement retains the outbox evidence")
	require.NotNil(t, deliveredAt)
	require.Nil(t, token)
	replayed, err := st.ClaimReadyOwnershipOutbox(context.Background(), 1)
	require.NoError(t, err)
	require.Empty(t, replayed)
}

func TestOwnershipOutboxDelivery_InvalidInputAndDatabaseFailureFailClosed(t *testing.T) {
	st := ownershipOutboxDeliveryStoreFixture(t)
	for _, limit := range []int{-1, 0, 101} {
		claimed, err := st.ClaimReadyOwnershipOutbox(context.Background(), limit)
		require.Error(t, err)
		require.Nil(t, claimed)
	}
	_, err := st.MarkOwnershipOutboxDelivered(context.Background(), uuid.Nil, uuid.New())
	require.Error(t, err)
	_, err = st.MarkOwnershipOutboxFailed(context.Background(), uuid.New(), uuid.Nil)
	require.Error(t, err)

	st.Pool.Close()
	claimed, err := st.ClaimReadyOwnershipOutbox(context.Background(), 1)
	require.Error(t, err)
	require.Nil(t, claimed)
	applied, err := st.MarkOwnershipOutboxDelivered(context.Background(), uuid.New(), uuid.New())
	require.Error(t, err)
	require.False(t, applied)
}
