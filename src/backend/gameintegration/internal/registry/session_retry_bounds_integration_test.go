package registry

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSessionOwnerRetryDelayUsesExponentialBackoffCappedAtThirtySeconds(t *testing.T) {
	store, ctx := startT31SessionStore(t)
	owners := newSessionOwnerScript()
	owners.blocked["chat_create"] = true
	p := newT31TestSessionPrincipal(t, ctx, store)
	o := NewSessionOrchestrator(store, owners.adapters())
	input := CreateSessionInput{
		OperationID: uuid.New(), Kind: "party", ExternalKey: "retry-backoff",
		DisplayName: "Retry", RosterRevision: 1, RosterComplete: true,
		Members: []uuid.UUID{},
	}
	started := time.Now()
	accepted, err := o.CreateSession(ctx, p, input)
	require.NoError(t, err)
	require.Less(t, time.Since(started), 2*time.Second, "healthy GIS database acceptance must complete within two seconds")
	require.Equal(t, "accepted", accepted.Stage)

	wantDelays := []int{1, 2, 4, 8, 16, 30, 30}
	for i, want := range wantDelays {
		_, err = o.AdvanceOne(ctx, accepted.OperationID)
		require.Error(t, err, "owner stage should fail transiently on attempt %d", i+1)

		var stage, status, errorCode string
		var attempts, stageRetryCount int
		var leaseOwner *uuid.UUID
		var delaySeconds float64
		err = store.Pool.QueryRow(ctx, `SELECT stage,status,error_code,attempts,stage_retry_count,lease_owner,
			EXTRACT(EPOCH FROM (next_attempt_at-updated_at)) FROM gis_session_operations WHERE operation_id=$1`, accepted.OperationID).
			Scan(&stage, &status, &errorCode, &attempts, &stageRetryCount, &leaseOwner, &delaySeconds)
		require.NoError(t, err)
		require.Equal(t, "accepted", stage, "retry must retain the failing durable stage")
		require.Equal(t, "pending", status)
		require.Equal(t, i+1, attempts, "attempt count identifies the claimed owner invocation")
		require.Equal(t, i+1, stageRetryCount, "retry count belongs to the current durable stage")
		require.Nil(t, leaseOwner, "failed claim must release its lease")
		require.NotEmpty(t, errorCode)
		require.InDelta(t, float64(want), delaySeconds, 0.01, "attempt %d retry delay", i+1)

		if i+1 < len(wantDelays) {
			// Advance the DB clock-independent schedule without waiting through backoff.
			_, err = store.Pool.Exec(ctx, `UPDATE gis_session_operations SET next_attempt_at=now() WHERE operation_id=$1`, accepted.OperationID)
			require.NoError(t, err)
		}
	}
	require.Len(t, owners.calls["chat_create"], len(wantDelays), "retries must call the same owner at the same stage")
	for _, call := range owners.calls["chat_create"] {
		require.Equal(t, owners.calls["chat_create"][0].OperationID, call.OperationID, "owner id must stay stable across retries")
		require.Equal(t, owners.calls["chat_create"][0].RequestHash, call.RequestHash, "owner request must stay stable across retries")
	}
	require.Empty(t, owners.calls["chat_roster"], "a failed owner stage must not advance the operation")

	owners.blocked["chat_create"] = false
	_, err = store.Pool.Exec(ctx, `UPDATE gis_session_operations SET next_attempt_at=now() WHERE operation_id=$1`, accepted.OperationID)
	require.NoError(t, err)
	advanced, err := o.AdvanceOne(ctx, accepted.OperationID)
	require.NoError(t, err)
	require.Equal(t, "chat_ready", advanced.Stage)
	owners.blocked["chat_roster"] = true
	_, err = o.AdvanceOne(ctx, accepted.OperationID)
	require.Error(t, err)
	var nextStage, nextErrorCode string
	var totalAttempts, nextStageRetries int
	var nextDelay float64
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT stage,error_code,attempts,stage_retry_count,
		EXTRACT(EPOCH FROM (next_attempt_at-updated_at)) FROM gis_session_operations WHERE operation_id=$1`, accepted.OperationID).
		Scan(&nextStage, &nextErrorCode, &totalAttempts, &nextStageRetries, &nextDelay))
	require.Equal(t, "chat_ready", nextStage)
	require.NotEmpty(t, nextErrorCode)
	require.Equal(t, 9, totalAttempts, "total attempts remain diagnostic across durable stages")
	require.Equal(t, 1, nextStageRetries, "a successful stage advance resets the retry sequence")
	require.InDelta(t, 1.0, nextDelay, 0.01, "the next stage starts its own retry delay")
}

func TestSessionWorkerRestartReclaimsRealFiveSecondLeaseWithinSixSeconds(t *testing.T) {
	store, ctx := startT31SessionStore(t)
	owners := newSessionOwnerScript()
	p := newT31TestSessionPrincipal(t, ctx, store)
	o := NewSessionOrchestrator(store, owners.adapters())
	accepted, err := o.CreateSession(ctx, p, CreateSessionInput{
		OperationID: uuid.New(), Kind: "party", ExternalKey: "restart-real-lease",
		DisplayName: "Restart", RosterRevision: 1, RosterComplete: true, Members: []uuid.UUID{},
	})
	require.NoError(t, err)

	_, _, claimedStage, firstOwner, err := store.claim(ctx, accepted.OperationID)
	require.NoError(t, err)
	require.Equal(t, "accepted", claimedStage)
	var attempts int
	var leaseRemaining float64
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT attempts,EXTRACT(EPOCH FROM (lease_until-now())) FROM gis_session_operations WHERE operation_id=$1 AND lease_owner=$2`, accepted.OperationID, firstOwner).Scan(&attempts, &leaseRemaining))
	require.Equal(t, 1, attempts)
	require.InDelta(t, 5.0, leaseRemaining, 0.5, "the abandoned claim must hold the production five-second lease")

	rosterEntered := make(chan SessionOwnerRequest, 1)
	ownerAdapters := owners.adapters()
	ownerAdapters.SyncChatRoster = func(ctx context.Context, request SessionOwnerRequest) (SessionOwnerReceipt, error) {
		rosterEntered <- request
		<-ctx.Done()
		return SessionOwnerReceipt{}, ctx.Err()
	}
	restarted := NewSessionOrchestrator(&Store{Pool: store.Pool}, ownerAdapters)
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	startedAt := time.Now()
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		RunSessionWorker(workerCtx, restarted, 50*time.Millisecond)
	}()
	deadline := time.NewTimer(6 * time.Second)
	defer deadline.Stop()
	select {
	case <-rosterEntered:
	case <-deadline.C:
		t.Fatal("fresh worker did not reclaim and advance the expired lease to Chat roster within six seconds")
	}
	elapsed := time.Since(startedAt)
	require.LessOrEqual(t, elapsed, 6*time.Second, "fresh worker must reclaim and advance within the restart bound")
	require.GreaterOrEqual(t, elapsed, 4500*time.Millisecond, "the fresh worker must wait for the real lease to expire")

	var stage, status string
	var currentOwner *uuid.UUID
	var currentAttempts int
	var currentLeaseRemaining float64
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT stage,status,lease_owner,attempts,
		EXTRACT(EPOCH FROM (lease_until-now())) FROM gis_session_operations WHERE operation_id=$1`, accepted.OperationID).
		Scan(&stage, &status, &currentOwner, &currentAttempts, &currentLeaseRemaining))
	require.Equal(t, "chat_ready", stage, "the barrier pins the operation at the next durable stage")
	require.Equal(t, "pending", status)
	require.NotNil(t, currentOwner, "the fresh worker must own a live lease while blocked in the stage")
	require.NotEqual(t, firstOwner, *currentOwner, "the expired lease must be replaced by the fresh worker")
	require.Equal(t, 3, currentAttempts, "one abandoned claim plus two worker stage claims are expected")
	require.InDelta(t, 5.0, currentLeaseRemaining, 0.5, "the worker must hold the production five-second lease")
	require.Len(t, owners.calls["chat_create"], 1)

	cancel()
	select {
	case <-workerDone:
	case <-time.After(time.Second):
		t.Fatal("fresh worker did not stop after cancellation")
	}
}
