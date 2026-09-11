package roomlifecycle

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func r223StoreFixture(t *testing.T, name string) r22StoreFixture {
	t.Helper()
	fixture := r22NewStoreFixture(t, name)
	_, err := fixture.pool.Exec(fixture.ctx, r223ReadMigration(t, "up"))
	require.NoError(t, err)
	return fixture
}

func r223Observation(fixture r22StoreFixture, at time.Time, seed byte) RedisDivergenceObservation {
	return RedisDivergenceObservation{
		ActorProfileID: fixture.actorProfileID, OperationID: fixture.operationID,
		RedisClass: RedisDivergenceMalformed, ObservationDigest: r22StoreDigest(seed), ObservedAt: at,
	}
}

func TestPostgresLifecycleStore_D1ClassifiesCurrentStateAndPersistsEvidence(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL divergence classifier requires testcontainers")
	}
	t.Run("absent is orphan and identical retry only advances timestamps", func(t *testing.T) {
		fixture := r223StoreFixture(t, "r223orphan")
		first, err := fixture.store.ClassifyAndRecordRedisDivergence(fixture.ctx, r223Observation(fixture, r22StoreTime, 1))
		require.NoError(t, err)
		require.Equal(t, RedisDivergenceOrphan, first.Classification)
		require.Nil(t, first.SubjectProfileID)
		require.Nil(t, first.ClassifiedPGState)
		require.Nil(t, first.ClassifiedLeaseFence)
		retry := r223Observation(fixture, r22StoreTime.Add(time.Minute), 1)
		second, err := fixture.store.ClassifyAndRecordRedisDivergence(fixture.ctx, retry)
		require.NoError(t, err)
		require.Equal(t, first.DivergenceID, second.DivergenceID)
		require.True(t, retry.ObservedAt.Equal(second.LastObservedAt))
		beforeChanged := r22TableRowsSnapshot(t, fixture.ctx, fixture.pool, r223Table)
		_, err = fixture.store.ClassifyAndRecordRedisDivergence(fixture.ctx, r223Observation(fixture, r22StoreTime.Add(2*time.Minute), 2))
		require.ErrorIs(t, err, ErrInvariant)
		require.Equal(t, beforeChanged, r22TableRowsSnapshot(t, fixture.ctx, fixture.pool, r223Table))
		reloaded, found, err := fixture.store.LoadRedisDivergence(fixture.ctx, fixture.actorProfileID, fixture.operationID)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, first.ObservationDigest, reloaded.ObservationDigest)
		require.True(t, first.FirstObservedAt.Equal(reloaded.FirstObservedAt))
		var count int
		require.NoError(t, fixture.pool.QueryRow(fixture.ctx, `SELECT count(*) FROM voice_lifecycle_redis_divergences`).Scan(&count))
		require.Equal(t, 1, count)
	})
	t.Run("decided is fenced and business quarantined atomically", func(t *testing.T) {
		fixture := r223StoreFixture(t, "r223decided")
		decision := fixture.decision(LifecycleMethodJoin)
		_, err := fixture.store.DecideOperation(fixture.ctx, decision)
		require.NoError(t, err)
		claim, ok, err := fixture.store.ClaimNextOperation(fixture.ctx, fixture.workerID, r22StoreTime, time.Minute)
		require.NoError(t, err)
		require.True(t, ok)
		record, err := fixture.store.ClassifyAndRecordRedisDivergence(fixture.ctx, r223Observation(fixture, r22StoreTime.Add(time.Minute), 3))
		require.NoError(t, err)
		require.Equal(t, RedisDivergenceDecided, record.Classification)
		require.NotNil(t, record.ClassifiedPGState)
		require.Equal(t, LifecycleOperationDecided, *record.ClassifiedPGState)
		require.Equal(t, decision.SubjectProfileID, *record.SubjectProfileID)
		require.Greater(t, *record.ClassifiedLeaseFence, claim.Fence)
		persisted, found, err := fixture.store.LoadRedisDivergence(fixture.ctx, decision.ActorProfileID, decision.OperationID)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, record.ClassifiedLeaseFence, persisted.ClassifiedLeaseFence)
		op, found, err := fixture.store.LoadOperation(fixture.ctx, decision.ActorProfileID, decision.OperationID)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, LifecycleOperationQuarantined, op.State)
		require.Nil(t, op.LeaseOwner)
		require.Equal(t, op.LeaseFence, *record.ClassifiedLeaseFence)
	})
	t.Run("completed receipt remains completed", func(t *testing.T) {
		fixture := r223StoreFixture(t, "r223completed")
		decision := fixture.decision(LifecycleMethodJoin)
		_, err := fixture.store.DecideOperation(fixture.ctx, decision)
		require.NoError(t, err)
		claim := r22ClaimOperation(t, fixture)
		r22ApplyAllEffects(t, fixture, decision)
		completion := LifecycleCompletion{ActorProfileID: decision.ActorProfileID, OperationID: decision.OperationID, WorkerID: fixture.workerID, ExpectedFence: claim.Fence, Receipt: r22JoinReceipt(fixture, decision, 1), CompletedAt: r22StoreTime.Add(time.Minute), ReplayUntil: r22StoreTime.Add(25 * time.Hour)}
		before, err := fixture.store.CompleteOperation(fixture.ctx, completion)
		require.NoError(t, err)
		beforeTables := r22AllTableSnapshots(t, fixture)
		record, err := fixture.store.ClassifyAndRecordRedisDivergence(fixture.ctx, r223Observation(fixture, r22StoreTime.Add(2*time.Minute), 4))
		require.NoError(t, err)
		require.Equal(t, RedisDivergenceCompleted, record.Classification)
		after, found, err := fixture.store.LoadOperation(fixture.ctx, decision.ActorProfileID, decision.OperationID)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, before.ReceiptBytes, after.ReceiptBytes)
		require.Equal(t, before.ReceiptHash, after.ReceiptHash)
		require.Equal(t, before.CompletedAt, after.CompletedAt)
		require.Equal(t, before.ReplayUntil, after.ReplayUntil)
		require.Equal(t, before.BindingBytes, after.BindingBytes)
		require.Equal(t, before.LeaseFence, after.LeaseFence)
		require.Equal(t, beforeTables, r22AllTableSnapshots(t, fixture))
		require.Equal(t, LifecycleOperationCompleted, after.State)
	})
	t.Run("current business quarantine remains quarantined", func(t *testing.T) {
		fixture := r223StoreFixture(t, "r223quarantined")
		decision := fixture.decision(LifecycleMethodJoin)
		_, err := fixture.store.DecideOperation(fixture.ctx, decision)
		require.NoError(t, err)
		claim := r22ClaimOperation(t, fixture)
		require.NoError(t, fixture.store.QuarantineOperation(fixture.ctx, LifecycleOperationQuarantine{ActorProfileID: decision.ActorProfileID, OperationID: decision.OperationID, WorkerID: fixture.workerID, ExpectedFence: claim.Fence, Class: "manual", At: r22StoreTime.Add(time.Minute)}))
		record, err := fixture.store.ClassifyAndRecordRedisDivergence(fixture.ctx, r223Observation(fixture, r22StoreTime.Add(2*time.Minute), 6))
		require.NoError(t, err)
		require.Equal(t, RedisDivergenceQuarantined, record.Classification)
		op, found, err := fixture.store.LoadOperation(fixture.ctx, decision.ActorProfileID, decision.OperationID)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, LifecycleOperationQuarantined, op.State)
		require.NotNil(t, op.QuarantineClass)
		require.Equal(t, "manual", *op.QuarantineClass)
	})
}

func TestPostgresLifecycleStore_D1ClassifierRollbackPreservesDecisionAndLease(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL divergence classifier requires testcontainers")
	}
	fixture := r223StoreFixture(t, "r223rollback")
	decision := fixture.decision(LifecycleMethodJoin)
	_, err := fixture.store.DecideOperation(fixture.ctx, decision)
	require.NoError(t, err)
	claim := r22ClaimOperation(t, fixture)
	before, found, err := fixture.store.LoadOperation(fixture.ctx, decision.ActorProfileID, decision.OperationID)
	require.NoError(t, err)
	require.True(t, found)
	_, err = fixture.pool.Exec(fixture.ctx, `CREATE FUNCTION r223_reject_incident_fn() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected incident failure' USING ERRCODE='23514'; END $$; CREATE TRIGGER r223_reject_incident BEFORE INSERT ON voice_lifecycle_redis_divergences FOR EACH ROW EXECUTE FUNCTION r223_reject_incident_fn()`)
	require.NoError(t, err)
	_, err = fixture.store.ClassifyAndRecordRedisDivergence(fixture.ctx, r223Observation(fixture, r22StoreTime.Add(time.Minute), 7))
	require.Error(t, err)
	after, found, err := fixture.store.LoadOperation(fixture.ctx, decision.ActorProfileID, decision.OperationID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, before, after)
	require.Equal(t, claim.Fence, after.LeaseFence)
	require.Equal(t, fixture.workerID, *after.LeaseOwner)
	var count int
	require.NoError(t, fixture.pool.QueryRow(fixture.ctx, `SELECT count(*) FROM voice_lifecycle_redis_divergences`).Scan(&count))
	require.Zero(t, count)
}

func TestPostgresLifecycleStore_D1OpenEvidenceBlocksDecisionAdmission(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL divergence admission requires testcontainers")
	}
	fixture := r223StoreFixture(t, "r223admission")
	_, err := fixture.store.ClassifyAndRecordRedisDivergence(fixture.ctx, r223Observation(fixture, r22StoreTime, 5))
	require.NoError(t, err)
	_, err = fixture.store.DecideOperation(fixture.ctx, fixture.decision(LifecycleMethodJoin))
	require.True(t, errors.Is(err, ErrRedisMirrorQuarantined))
	var count int
	require.NoError(t, fixture.pool.QueryRow(fixture.ctx, `SELECT count(*) FROM voice_lifecycle_operations`).Scan(&count))
	require.Zero(t, count)
}

func TestPostgresLifecycleStore_D1OpenEvidenceBlocksCompleteNoOpAdmissionAndReplay(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL divergence admission requires testcontainers")
	}
	t.Run("fresh no-op", func(t *testing.T) {
		fixture := r223StoreFixture(t, "r223noopfresh")
		_, err := fixture.store.ClassifyAndRecordRedisDivergence(fixture.ctx, r223Observation(fixture, r22StoreTime, 8))
		require.NoError(t, err)
		_, err = fixture.store.CompleteNoOp(fixture.ctx, LifecycleNoOpDecision{Decision: fixture.decision(LifecycleMethodLeave), CompletedAt: r22StoreTime.Add(time.Minute), ReplayUntil: r22StoreTime.Add(25 * time.Hour)})
		require.ErrorIs(t, err, ErrRedisMirrorQuarantined)
	})
	t.Run("terminal replay", func(t *testing.T) {
		fixture := r223StoreFixture(t, "r223noopreplay")
		noOp := LifecycleNoOpDecision{Decision: fixture.decision(LifecycleMethodLeave), CompletedAt: r22StoreTime.Add(time.Minute), ReplayUntil: r22StoreTime.Add(25 * time.Hour)}
		_, err := fixture.store.CompleteNoOp(fixture.ctx, noOp)
		require.NoError(t, err)
		_, err = fixture.store.ClassifyAndRecordRedisDivergence(fixture.ctx, r223Observation(fixture, r22StoreTime.Add(2*time.Minute), 9))
		require.NoError(t, err)
		_, err = fixture.store.CompleteNoOp(fixture.ctx, noOp)
		require.ErrorIs(t, err, ErrRedisMirrorQuarantined)
	})
}

type r223DivergenceResult struct {
	record RedisDivergenceRecord
	err    error
}

type r223ClaimResult struct {
	claim LifecycleOperationClaim
	found bool
	err   error
}

func r223ReceiveDivergence(t *testing.T, results <-chan r223DivergenceResult) r223DivergenceResult {
	t.Helper()
	select {
	case result := <-results:
		return result
	case <-time.After(10 * time.Second):
		t.Fatal("divergence classifier did not return after lock release")
		return r223DivergenceResult{}
	}
}

func r223ReceiveClaim(t *testing.T, results <-chan r223ClaimResult) r223ClaimResult {
	t.Helper()
	select {
	case result := <-results:
		return result
	case <-time.After(10 * time.Second):
		t.Fatal("operation claim did not return after lock release")
		return r223ClaimResult{}
	}
}

func r223WaitNamedRowLock(t *testing.T, ctx context.Context, pool *pgxpool.Pool, name string) {
	t.Helper()
	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var blocked bool
		err := pool.QueryRow(waitCtx, `SELECT EXISTS(
SELECT 1 FROM pg_stat_activity
WHERE application_name=$1 AND wait_event_type='Lock' AND wait_event IN ('transactionid','tuple')
)`, name).Scan(&blocked)
		if err == nil && blocked {
			return
		}
		select {
		case <-waitCtx.Done():
			t.Fatalf("application never reached row-lock wait: %s", name)
		case <-ticker.C:
		}
	}
}

func TestPostgresLifecycleStore_D1OrphanDecisionLockWinners(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL divergence concurrency requires testcontainers")
	}
	for _, classifierFirst := range []bool{true, false} {
		name := "decision wins"
		if classifierFirst {
			name = "orphan evidence wins"
		}
		t.Run(name, func(t *testing.T) {
			fixture := r223StoreFixture(t, "r223orphanrace"+uuid.NewString()[:6])
			operationKey := LifecycleOperationAdvisoryKey(fixture.actorProfileID, fixture.operationID)
			blocker := r22BlockAdvisory(t, fixture.ctx, fixture.pool, operationKey)
			workerCtx := r22BoundedWorkerContext(t, fixture.ctx)
			classifierName := "r223-orphan-classifier-" + uuid.NewString()
			decisionName := "r223-orphan-decision-" + uuid.NewString()
			classifierStore, _ := r22NamedStore(t, fixture.pool, classifierName)
			decisionStore, _ := r22NamedStore(t, fixture.pool, decisionName)
			classifierResults := make(chan r223DivergenceResult, 1)
			decisionResults := make(chan r22DecisionResult, 1)
			startClassifier := func() {
				go func() {
					record, err := classifierStore.ClassifyAndRecordRedisDivergence(workerCtx, r223Observation(fixture, r22StoreTime, 10))
					classifierResults <- r223DivergenceResult{record: record, err: err}
				}()
				r22WaitApplicationBlocked(t, workerCtx, fixture.pool, blocker, classifierName, nil)
			}
			startDecision := func() {
				go func() {
					operation, err := decisionStore.DecideOperation(workerCtx, fixture.decision(LifecycleMethodJoin))
					decisionResults <- r22DecisionResult{operation: operation, err: err}
				}()
				r22WaitApplicationBlocked(t, workerCtx, fixture.pool, blocker, decisionName, nil)
			}
			if classifierFirst {
				startClassifier()
				startDecision()
			} else {
				startDecision()
				startClassifier()
			}
			require.NoError(t, blocker.tx.Commit(workerCtx))
			classifierResult := r223ReceiveDivergence(t, classifierResults)
			decisionResult := r22ReceiveDecision(t, decisionResults)
			if classifierFirst {
				require.NoError(t, classifierResult.err)
				require.Equal(t, RedisDivergenceOrphan, classifierResult.record.Classification)
				require.ErrorIs(t, decisionResult.err, ErrRedisMirrorQuarantined)
				require.Zero(t, r22TableRowCount(t, fixture.ctx, fixture.pool, "voice_lifecycle_operations"))
			} else {
				require.NoError(t, decisionResult.err)
				require.NoError(t, classifierResult.err)
				require.Equal(t, RedisDivergenceDecided, classifierResult.record.Classification)
				op, found, err := fixture.store.LoadOperation(fixture.ctx, fixture.actorProfileID, fixture.operationID)
				require.NoError(t, err)
				require.True(t, found)
				require.Equal(t, LifecycleOperationQuarantined, op.State)
			}
		})
	}
}

func TestPostgresLifecycleStore_D1ClassifierCompletionLockWinnersAndStaleWorker(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL divergence concurrency requires testcontainers")
	}
	for _, classifierFirst := range []bool{true, false} {
		name := "completion wins"
		if classifierFirst {
			name = "classifier wins"
		}
		t.Run(name, func(t *testing.T) {
			fixture := r223StoreFixture(t, "r223completionrace"+uuid.NewString()[:6])
			decision := fixture.decision(LifecycleMethodJoin)
			_, err := fixture.store.DecideOperation(fixture.ctx, decision)
			require.NoError(t, err)
			claim := r22ClaimOperation(t, fixture)
			r22ApplyAllEffects(t, fixture, decision)
			completion := LifecycleCompletion{ActorProfileID: decision.ActorProfileID, OperationID: decision.OperationID,
				WorkerID: fixture.workerID, ExpectedFence: claim.Fence, Receipt: r22JoinReceipt(fixture, decision, 1),
				CompletedAt: r22StoreTime.Add(time.Minute), ReplayUntil: r22StoreTime.Add(25 * time.Hour)}
			operationKey := LifecycleOperationAdvisoryKey(fixture.actorProfileID, fixture.operationID)
			blocker := r22BlockAdvisory(t, fixture.ctx, fixture.pool, operationKey)
			workerCtx := r22BoundedWorkerContext(t, fixture.ctx)
			classifierName := "r223-completion-classifier-" + uuid.NewString()
			completionName := "r223-completion-worker-" + uuid.NewString()
			classifierStore, _ := r22NamedStore(t, fixture.pool, classifierName)
			completionStore, _ := r22NamedStore(t, fixture.pool, completionName)
			classifierResults := make(chan r223DivergenceResult, 1)
			completionResults := make(chan r22DecisionResult, 1)
			startClassifier := func() {
				go func() {
					record, callErr := classifierStore.ClassifyAndRecordRedisDivergence(workerCtx, r223Observation(fixture, r22StoreTime.Add(2*time.Minute), 11))
					classifierResults <- r223DivergenceResult{record: record, err: callErr}
				}()
				r22WaitApplicationBlocked(t, workerCtx, fixture.pool, blocker, classifierName, nil)
			}
			startCompletion := func() {
				go func() {
					operation, callErr := completionStore.CompleteOperation(workerCtx, completion)
					completionResults <- r22DecisionResult{operation: operation, err: callErr}
				}()
				r22WaitApplicationBlocked(t, workerCtx, fixture.pool, blocker, completionName, nil)
			}
			if classifierFirst {
				startClassifier()
				startCompletion()
			} else {
				startCompletion()
				startClassifier()
			}
			require.NoError(t, blocker.tx.Commit(workerCtx))
			classifierResult := r223ReceiveDivergence(t, classifierResults)
			completionResult := r22ReceiveDecision(t, completionResults)
			require.NoError(t, classifierResult.err)
			if classifierFirst {
				require.Equal(t, RedisDivergenceDecided, classifierResult.record.Classification)
				require.ErrorIs(t, completionResult.err, ErrStaleLease)
				beforeStale := r22AllTableSnapshots(t, fixture)
				_, err = fixture.store.CompleteOperation(fixture.ctx, completion)
				require.ErrorIs(t, err, ErrStaleLease)
				require.ErrorIs(t, fixture.store.RenewOperationLease(fixture.ctx, LifecycleOperationLease{ActorProfileID: decision.ActorProfileID, OperationID: decision.OperationID, WorkerID: fixture.workerID, ExpectedFence: claim.Fence, LeaseUntil: r22StoreTime.Add(3 * time.Minute)}), ErrStaleLease)
				require.ErrorIs(t, fixture.store.QuarantineOperation(fixture.ctx, LifecycleOperationQuarantine{ActorProfileID: decision.ActorProfileID, OperationID: decision.OperationID, WorkerID: fixture.workerID, ExpectedFence: claim.Fence, Class: "stale", At: r22StoreTime.Add(3 * time.Minute)}), ErrStaleLease)
				require.Equal(t, beforeStale, r22AllTableSnapshots(t, fixture))
			} else {
				require.NoError(t, completionResult.err)
				require.Equal(t, RedisDivergenceCompleted, classifierResult.record.Classification)
				require.Equal(t, LifecycleOperationCompleted, completionResult.operation.State)
			}
		})
	}
}

func TestPostgresLifecycleStore_D1ClassifierReclaimLockWinners(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL divergence concurrency requires testcontainers")
	}
	t.Run("classifier row lock wins", func(t *testing.T) {
		fixture := r223StoreFixture(t, "r223reclaimclassifier")
		decision := fixture.decision(LifecycleMethodJoin)
		_, err := fixture.store.DecideOperation(fixture.ctx, decision)
		require.NoError(t, err)
		oldClaim := r22ClaimOperation(t, fixture)
		blocker := r22BlockAdvisory(t, fixture.ctx, fixture.pool, LifecycleSubjectAdvisoryKey(decision.SubjectProfileID))
		workerCtx := r22BoundedWorkerContext(t, fixture.ctx)
		classifierName := "r223-reclaim-classifier-" + uuid.NewString()
		classifierStore, _ := r22NamedStore(t, fixture.pool, classifierName)
		classifierResults := make(chan r223DivergenceResult, 1)
		go func() {
			record, callErr := classifierStore.ClassifyAndRecordRedisDivergence(workerCtx, r223Observation(fixture, r22StoreTime.Add(2*time.Minute), 12))
			classifierResults <- r223DivergenceResult{record: record, err: callErr}
		}()
		r22WaitApplicationBlocked(t, workerCtx, fixture.pool, blocker, classifierName, []LifecycleAdvisoryLockKey{LifecycleOperationAdvisoryKey(decision.ActorProfileID, decision.OperationID)})
		newClaim, found, err := fixture.store.ClaimNextOperation(workerCtx, uuid.New(), oldClaim.LeaseUntil, time.Minute)
		require.NoError(t, err)
		require.False(t, found)
		require.Equal(t, LifecycleOperationClaim{}, newClaim)
		require.NoError(t, blocker.tx.Commit(workerCtx))
		result := r223ReceiveDivergence(t, classifierResults)
		require.NoError(t, result.err)
		require.Equal(t, RedisDivergenceDecided, result.record.Classification)
	})

	t.Run("reclaim row lock wins", func(t *testing.T) {
		fixture := r223StoreFixture(t, "r223reclaimworker")
		decision := fixture.decision(LifecycleMethodJoin)
		_, err := fixture.store.DecideOperation(fixture.ctx, decision)
		require.NoError(t, err)
		oldClaim := r22ClaimOperation(t, fixture)
		barrierKey := LifecycleOperationAdvisoryKey(uuid.New(), uuid.New())
		dropBarrier := r22InstallUpdateAdvisoryBarrier(t, fixture, "voice_lifecycle_operations", barrierKey)
		blocker := r22BlockAdvisory(t, fixture.ctx, fixture.pool, barrierKey)
		workerCtx := r22BoundedWorkerContext(t, fixture.ctx)
		reclaimName := "r223-reclaim-worker-" + uuid.NewString()
		classifierName := "r223-reclaim-after-" + uuid.NewString()
		reclaimStore, _ := r22NamedStore(t, fixture.pool, reclaimName)
		classifierStore, _ := r22NamedStore(t, fixture.pool, classifierName)
		reclaimResults := make(chan r223ClaimResult, 1)
		classifierResults := make(chan r223DivergenceResult, 1)
		newWorker := uuid.New()
		go func() {
			claim, found, callErr := reclaimStore.ClaimNextOperation(workerCtx, newWorker, oldClaim.LeaseUntil, time.Minute)
			reclaimResults <- r223ClaimResult{claim: claim, found: found, err: callErr}
		}()
		r22WaitApplicationBlocked(t, workerCtx, fixture.pool, blocker, reclaimName, nil)
		go func() {
			record, callErr := classifierStore.ClassifyAndRecordRedisDivergence(workerCtx, r223Observation(fixture, r22StoreTime.Add(2*time.Minute), 13))
			classifierResults <- r223DivergenceResult{record: record, err: callErr}
		}()
		r223WaitNamedRowLock(t, workerCtx, fixture.pool, classifierName)
		require.NoError(t, blocker.tx.Commit(workerCtx))
		reclaimResult := r223ReceiveClaim(t, reclaimResults)
		require.NoError(t, reclaimResult.err)
		require.True(t, reclaimResult.found)
		require.Greater(t, reclaimResult.claim.Fence, oldClaim.Fence)
		classifierResult := r223ReceiveDivergence(t, classifierResults)
		require.NoError(t, classifierResult.err)
		require.Equal(t, RedisDivergenceDecided, classifierResult.record.Classification)
		require.Greater(t, *classifierResult.record.ClassifiedLeaseFence, reclaimResult.claim.Fence)
		beforeStale := r22AllTableSnapshots(t, fixture)
		require.ErrorIs(t, fixture.store.RenewOperationLease(fixture.ctx, LifecycleOperationLease{ActorProfileID: decision.ActorProfileID, OperationID: decision.OperationID, WorkerID: newWorker, ExpectedFence: reclaimResult.claim.Fence, LeaseUntil: r22StoreTime.Add(4 * time.Minute)}), ErrStaleLease)
		nextClaim, found, err := fixture.store.ClaimNextOperation(fixture.ctx, uuid.New(), reclaimResult.claim.LeaseUntil, time.Minute)
		require.NoError(t, err)
		require.False(t, found)
		require.Equal(t, LifecycleOperationClaim{}, nextClaim)
		require.Equal(t, beforeStale, r22AllTableSnapshots(t, fixture))
		dropBarrier()
	})
}
