package roomlifecycle

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// This file freezes RED-D from R22.2-VOICE-DB-PLAN.md lines 829-846. Every
// lock race uses distinct pools, an explicit blocker and pg_stat_activity proof.

type r22DecisionResult struct {
	operation LifecycleOperation
	err       error
}

type r22MembershipResult struct {
	membership LifecycleMembership
	err        error
}

type r22AdvisoryBlocker struct {
	tx  pgx.Tx
	pid int
	key LifecycleAdvisoryLockKey
}

type r22RoomCloseBlocker struct {
	tx  pgx.Tx
	pid int
}

func r22BlockRoomClose(t *testing.T, ctx context.Context, pool *pgxpool.Pool, roomID uuid.UUID) r22RoomCloseBlocker {
	t.Helper()
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `UPDATE voice_room_instances SET state='closed',closed_at=clock_timestamp(),updated_at=clock_timestamp() WHERE room_id=$1`, roomID)
	require.NoError(t, err)
	var pid int
	require.NoError(t, tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid))
	r22RegisterRollbackCleanup(t, tx)
	return r22RoomCloseBlocker{tx: tx, pid: pid}
}

func r22CloneDecision(decision LifecycleDecision) LifecycleDecision {
	decision.Effects = append([]LifecycleEffectPlan(nil), decision.Effects...)
	return decision
}

func r22NamedStore(t *testing.T, pool *pgxpool.Pool, name string) (*PostgresLifecycleStore, *pgxpool.Pool) {
	t.Helper()
	config := pool.Config().Copy()
	config.ConnConfig.RuntimeParams["application_name"] = name
	named, err := pgxpool.NewWithConfig(context.Background(), config)
	require.NoError(t, err)
	t.Cleanup(named.Close)
	return NewPostgresLifecycleStore(named), named
}

func r22BlockAdvisory(t *testing.T, ctx context.Context, pool *pgxpool.Pool, key LifecycleAdvisoryLockKey) r22AdvisoryBlocker {
	t.Helper()
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::integer,$2::integer)`, key.Namespace, key.Key)
	require.NoError(t, err)
	var pid int
	require.NoError(t, tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid))
	r22RegisterRollbackCleanup(t, tx)
	return r22AdvisoryBlocker{tx: tx, pid: pid, key: key}
}

func r22AdvisoryOID(value int32) int64 {
	return int64(uint32(value))
}

func r22GrantedAdvisoryLocksMatch(ctx context.Context, pool *pgxpool.Pool, pid int, expected []LifecycleAdvisoryLockKey) (bool, error) {
	rows, err := pool.Query(ctx, `
SELECT classid::bigint,objid::bigint,objsubid
FROM pg_locks
WHERE pid=$1 AND locktype='advisory' AND granted`, pid)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	type lockTriple struct {
		classID     int64
		objectID    int64
		objectSubID int32
	}
	got := make(map[lockTriple]int)
	count := 0
	for rows.Next() {
		var lock lockTriple
		if err := rows.Scan(&lock.classID, &lock.objectID, &lock.objectSubID); err != nil {
			return false, err
		}
		got[lock]++
		count++
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	if count != len(expected) {
		return false, nil
	}
	for _, key := range expected {
		lock := lockTriple{classID: r22AdvisoryOID(key.Namespace), objectID: r22AdvisoryOID(key.Key), objectSubID: 2}
		if got[lock] != 1 {
			return false, nil
		}
		delete(got, lock)
	}
	return len(got) == 0, nil
}

func r22WaitApplicationBlocked(t *testing.T, ctx context.Context, pool *pgxpool.Pool, blocker r22AdvisoryBlocker, name string, expectedEarlier []LifecycleAdvisoryLockKey) {
	t.Helper()
	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var workerPID int
		var exactPair, blockedByOwner bool
		err := pool.QueryRow(waitCtx, `
SELECT
	  a.pid,
  EXISTS(
    SELECT 1 FROM pg_locks l
    WHERE l.pid=a.pid AND l.locktype='advisory' AND NOT l.granted AND l.objsubid=2
      AND l.classid::bigint=$2::bigint
      AND l.objid::bigint=CASE WHEN $3::bigint<0 THEN $3::bigint+4294967296 ELSE $3::bigint END
  ),
	  $4::integer=ANY(pg_blocking_pids(a.pid))
FROM pg_stat_activity a WHERE a.application_name=$1 AND a.wait_event_type='Lock'`,
			name, blocker.key.Namespace, blocker.key.Key, blocker.pid).Scan(&workerPID, &exactPair, &blockedByOwner)
		if err != nil && waitCtx.Err() != nil {
			t.Fatalf("application never reached exact advisory lock wait: %s", name)
		}
		if err == nil && exactPair && blockedByOwner {
			matches, matchErr := r22GrantedAdvisoryLocksMatch(waitCtx, pool, workerPID, expectedEarlier)
			if matchErr == nil && matches {
				return
			}
		}
		select {
		case <-waitCtx.Done():
			t.Fatalf("application never proved exact pair/blocker/earlier locks: %s", name)
		case <-ticker.C:
		}
	}
}

func r22WaitAnyApplicationBlocked(t *testing.T, ctx context.Context, pool *pgxpool.Pool, blocker r22AdvisoryBlocker, expectedEarlier map[string][]LifecycleAdvisoryLockKey, names ...string) {
	t.Helper()
	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		for _, name := range names {
			var workerPID int
			var exactPair, blockedByOwner bool
			err := pool.QueryRow(waitCtx, `
SELECT a.pid,EXISTS(SELECT 1 FROM pg_locks l WHERE l.pid=a.pid AND l.locktype='advisory'
  AND NOT l.granted AND l.objsubid=2 AND l.classid::bigint=$2::bigint
  AND l.objid::bigint=CASE WHEN $3::bigint<0 THEN $3::bigint+4294967296 ELSE $3::bigint END),
	  $4::integer=ANY(pg_blocking_pids(a.pid))
FROM pg_stat_activity a WHERE a.application_name=$1 AND a.wait_event_type='Lock'`,
				name, blocker.key.Namespace, blocker.key.Key, blocker.pid).Scan(&workerPID, &exactPair, &blockedByOwner)
			if err == nil && exactPair && blockedByOwner {
				matches, matchErr := r22GrantedAdvisoryLocksMatch(waitCtx, pool, workerPID, expectedEarlier[name])
				if matchErr == nil && matches {
					return
				}
			}
		}
		select {
		case <-waitCtx.Done():
			t.Fatalf("no application proved ascending lock order at exact pair: %v", names)
		case <-ticker.C:
		}
	}
}

func r22ReceiveDecision(t *testing.T, results <-chan r22DecisionResult) r22DecisionResult {
	t.Helper()
	select {
	case result := <-results:
		return result
	case <-time.After(10 * time.Second):
		t.Fatal("decision result did not arrive after lock barrier release")
		return r22DecisionResult{}
	}
}

func r22ReceiveMembership(t *testing.T, results <-chan r22MembershipResult) r22MembershipResult {
	t.Helper()
	select {
	case result := <-results:
		return result
	case <-time.After(10 * time.Second):
		t.Fatal("membership result did not arrive after lock barrier release")
		return r22MembershipResult{}
	}
}

func r22BlockedApplicationPID(t *testing.T, ctx context.Context, pool *pgxpool.Pool, name string) int {
	t.Helper()
	var pid int
	require.NoError(t, pool.QueryRow(ctx, `
SELECT pid FROM pg_stat_activity
WHERE application_name=$1 AND wait_event_type='Lock'`, name).Scan(&pid))
	return pid
}

func r22ReceiveError(t *testing.T, results <-chan error) error {
	t.Helper()
	select {
	case err := <-results:
		return err
	case <-time.After(10 * time.Second):
		t.Fatal("worker result did not arrive after lock barrier release")
		return context.DeadlineExceeded
	}
}

func r22BoundedWorkerContext(t *testing.T, parent context.Context) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func r22RegisterRollbackCleanup(t *testing.T, tx pgx.Tx) {
	t.Helper()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(ctx)
	})
}

func r22WaitRowLocks(t *testing.T, ctx context.Context, pool *pgxpool.Pool, blockerPID int, names ...string) {
	t.Helper()
	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var blocked int
		err := pool.QueryRow(waitCtx, `
SELECT count(*) FROM pg_stat_activity a
WHERE a.application_name=ANY($1) AND a.wait_event_type='Lock'
  AND $2::integer=ANY(pg_blocking_pids(a.pid))`, names, blockerPID).Scan(&blocked)
		if err != nil && waitCtx.Err() != nil {
			t.Fatalf("workers never reached proved row lock wait: %v", names)
		}
		require.NoError(t, err)
		if blocked == len(names) {
			return
		}
		select {
		case <-waitCtx.Done():
			t.Fatalf("workers never reached proved row lock wait: %v", names)
		case <-ticker.C:
		}
	}
}

type r22EffectLockActivity struct {
	pid         int
	name        string
	state       string
	waitType    *string
	waitEvent   *string
	blockerPIDs []int32
	query       string
}

type r22EffectLock struct {
	pid           int
	lockType      string
	mode          string
	granted       bool
	relation      *int64
	page          *int64
	tuple         *int64
	transactionID *int64
}

func r22LoadEffectLockSnapshot(ctx context.Context, pool *pgxpool.Pool, blockerPID int, names []string) ([]r22EffectLockActivity, []r22EffectLock, error) {
	activityRows, err := pool.Query(ctx, `
SELECT pid,application_name,state,wait_event_type,wait_event,pg_blocking_pids(pid),query
FROM pg_stat_activity WHERE application_name=ANY($1) OR pid=$2 ORDER BY pid`, names, blockerPID)
	if err != nil {
		return nil, nil, err
	}
	defer activityRows.Close()
	var activities []r22EffectLockActivity
	for activityRows.Next() {
		var activity r22EffectLockActivity
		if err = activityRows.Scan(&activity.pid, &activity.name, &activity.state, &activity.waitType, &activity.waitEvent,
			&activity.blockerPIDs, &activity.query); err != nil {
			return nil, nil, err
		}
		activities = append(activities, activity)
	}
	if err = activityRows.Err(); err != nil {
		return nil, nil, err
	}
	lockRows, err := pool.Query(ctx, `
SELECT pid,locktype,mode,granted,relation::bigint,page::bigint,tuple::bigint,transactionid::text::bigint
FROM pg_locks WHERE pid=ANY($1) AND locktype IN ('tuple','transactionid')
ORDER BY pid,locktype,granted DESC,mode`, append([]int{blockerPID}, r22ActivityPIDs(activities)...))
	if err != nil {
		return nil, nil, err
	}
	defer lockRows.Close()
	var locks []r22EffectLock
	for lockRows.Next() {
		var lock r22EffectLock
		if err = lockRows.Scan(&lock.pid, &lock.lockType, &lock.mode, &lock.granted, &lock.relation, &lock.page,
			&lock.tuple, &lock.transactionID); err != nil {
			return nil, nil, err
		}
		locks = append(locks, lock)
	}
	return activities, locks, lockRows.Err()
}

func r22ActivityPIDs(activities []r22EffectLockActivity) []int {
	pids := make([]int, 0, len(activities))
	for _, activity := range activities {
		pids = append(pids, activity.pid)
	}
	return pids
}

func r22BlockingGraphReady(activities []r22EffectLockActivity, blockerPID int, names []string) bool {
	if len(activities) != 3 {
		return false
	}
	byPID := make(map[int]r22EffectLockActivity, len(activities))
	workers := make([]int, 0, 2)
	nameSet := map[string]bool{names[0]: true, names[1]: true}
	for _, activity := range activities {
		byPID[activity.pid] = activity
		if nameSet[activity.name] {
			workers = append(workers, activity.pid)
		}
	}
	if len(workers) != 2 || workers[0] == workers[1] {
		return false
	}
	blocker, exists := byPID[blockerPID]
	if !exists || len(blocker.blockerPIDs) != 0 {
		return false
	}
	direct := 0
	for _, workerPID := range workers {
		activity := byPID[workerPID]
		if activity.state != "active" || activity.waitType == nil || *activity.waitType != "Lock" || len(activity.blockerPIDs) == 0 {
			return false
		}
		seen := map[int]bool{workerPID: true}
		frontier := []int{workerPID}
		reachesBlocker := false
		for len(frontier) > 0 {
			current := frontier[0]
			frontier = frontier[1:]
			for _, next32 := range byPID[current].blockerPIDs {
				next := int(next32)
				if next == blockerPID {
					reachesBlocker = true
					if current == workerPID {
						direct++
					}
					continue
				}
				if _, allowed := byPID[next]; !allowed || seen[next] {
					return false
				}
				seen[next] = true
				frontier = append(frontier, next)
			}
		}
		if !reachesBlocker {
			return false
		}
	}
	return direct >= 1
}

func r22EffectLocksReady(locks []r22EffectLock, blockerPID int, relation, page, tuple int64) bool {
	var blockerXID *int64
	var frontWait, grantedTuple, waitingTuple *r22EffectLock
	for index := range locks {
		lock := &locks[index]
		switch lock.lockType {
		case "transactionid":
			if lock.pid == blockerPID && lock.granted && lock.mode == "ExclusiveLock" {
				blockerXID = lock.transactionID
			} else if !lock.granted && lock.mode == "ShareLock" {
				frontWait = lock
			} else {
				return false
			}
		case "tuple":
			if lock.mode != "AccessExclusiveLock" || lock.relation == nil || lock.page == nil || lock.tuple == nil ||
				*lock.relation != relation || *lock.page != page || *lock.tuple != tuple {
				return false
			}
			if lock.granted {
				grantedTuple = lock
			} else {
				waitingTuple = lock
			}
		default:
			return false
		}
	}
	return len(locks) == 4 && blockerXID != nil && frontWait != nil && frontWait.transactionID != nil &&
		*frontWait.transactionID == *blockerXID && grantedTuple != nil && waitingTuple != nil &&
		grantedTuple.pid == frontWait.pid && waitingTuple.pid != frontWait.pid && waitingTuple.pid != blockerPID
}

func r22WaitExclusiveEffectLockChain(t *testing.T, ctx context.Context, pool *pgxpool.Pool, blockerPID int,
	effectID uuid.UUID, names ...string) {
	t.Helper()
	require.Len(t, names, 2)
	var relation, page, tuple int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT tableoid::oid::bigint,(ctid::text::point)[0]::bigint,(ctid::text::point)[1]::bigint
FROM voice_lifecycle_effects WHERE effect_id=$1`, effectID).Scan(&relation, &page, &tuple))
	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	var activities []r22EffectLockActivity
	var locks []r22EffectLock
	for {
		var err error
		activities, locks, err = r22LoadEffectLockSnapshot(waitCtx, pool, blockerPID, names)
		if err != nil && waitCtx.Err() != nil {
			t.Fatalf("exclusive effect lock chain did not form: activities=%+v locks=%+v", activities, locks)
		}
		require.NoError(t, err)
		if r22BlockingGraphReady(activities, blockerPID, names) && r22EffectLocksReady(locks, blockerPID, relation, page, tuple) {
			break
		}
		select {
		case <-waitCtx.Done():
			t.Fatalf("exclusive effect lock chain did not form: activities=%+v locks=%+v", activities, locks)
		case <-ticker.C:
		}
	}
	expectedQuery := strings.ToLower(strings.Join(strings.Fields(`SELECT `+effectColumns+` FROM voice_lifecycle_effects WHERE effect_id=$1 FOR UPDATE`), ""))
	workerCount := 0
	for _, activity := range activities {
		if activity.pid == blockerPID {
			require.Equal(t, "idle in transaction", activity.state)
			require.Empty(t, activity.blockerPIDs)
			continue
		}
		workerCount++
		require.Contains(t, names, activity.name)
		require.Equal(t, "active", activity.state)
		require.NotNil(t, activity.waitType)
		require.Equal(t, "Lock", *activity.waitType)
		require.Equal(t, expectedQuery, strings.ToLower(strings.Join(strings.Fields(activity.query), "")))
	}
	require.Equal(t, 2, workerCount)
}

func r22RunBlockedDecisionPair(t *testing.T, fixture r22StoreFixture, key LifecycleAdvisoryLockKey, firstEarlier, secondEarlier []LifecycleAdvisoryLockKey, first, second LifecycleDecision) (r22DecisionResult, r22DecisionResult) {
	t.Helper()
	nameA, nameB := "r22-cd-worker-a-"+uuid.NewString(), "r22-cd-worker-b-"+uuid.NewString()
	storeA, _ := r22NamedStore(t, fixture.pool, nameA)
	storeB, _ := r22NamedStore(t, fixture.pool, nameB)
	blocker := r22BlockAdvisory(t, fixture.ctx, fixture.pool, key)
	workerCtx := r22BoundedWorkerContext(t, fixture.ctx)
	results := make(chan r22DecisionResult, 2)
	go func() {
		operation, err := storeA.DecideOperation(workerCtx, first)
		results <- r22DecisionResult{operation, err}
	}()
	go func() {
		operation, err := storeB.DecideOperation(workerCtx, second)
		results <- r22DecisionResult{operation, err}
	}()
	r22WaitApplicationBlocked(t, workerCtx, fixture.pool, blocker, nameA, firstEarlier)
	r22WaitApplicationBlocked(t, workerCtx, fixture.pool, blocker, nameB, secondEarlier)
	require.NoError(t, blocker.tx.Commit(workerCtx))
	return r22ReceiveDecision(t, results), r22ReceiveDecision(t, results)
}

func TestPostgresLifecycleStore_AdvisoryLockKeyGoldens(t *testing.T) {
	opActor := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	opID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	shared := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	require.Equal(t, LifecycleAdvisoryLockKey{Namespace: 1448038449, Key: 1109162976}, LifecycleOperationAdvisoryKey(opActor, opID))
	require.Equal(t, LifecycleAdvisoryLockKey{Namespace: 1448301873, Key: -1559024858}, LifecycleSubjectAdvisoryKey(shared))
	require.Equal(t, LifecycleAdvisoryLockKey{Namespace: 1447842353, Key: -1749405397}, LifecycleLogicalRoomAdvisoryKey(shared))
	require.NotEqual(t, LifecycleOperationAdvisoryKey(opActor, opID), LifecycleOperationAdvisoryKey(opID, opActor))
	require.NotEqual(t, LifecycleSubjectAdvisoryKey(shared), LifecycleLogicalRoomAdvisoryKey(shared))
	require.Equal(t, LifecycleAdvisoryLockKey{Namespace: 1448038449, Key: 711570607}, LifecycleOperationAdvisoryKey(uuid.UUID{15: 2}, uuid.UUID{15: 3}))
	require.Equal(t, LifecycleAdvisoryLockKey{Namespace: 1448038449, Key: -2058913551}, LifecycleOperationAdvisoryKey(uuid.UUID{15: 1}, uuid.UUID{15: 2}))
	require.Equal(t, LifecycleAdvisoryLockKey{Namespace: 1448301873, Key: 323667294}, LifecycleSubjectAdvisoryKey(uuid.UUID{15: 2}))
	require.Equal(t, LifecycleAdvisoryLockKey{Namespace: 1448301873, Key: -318109672}, LifecycleSubjectAdvisoryKey(uuid.UUID{15: 1}))
	require.Equal(t, LifecycleAdvisoryLockKey{Namespace: 1447842353, Key: 1335280350}, LifecycleLogicalRoomAdvisoryKey(uuid.UUID{15: 1}))
	require.Equal(t, LifecycleAdvisoryLockKey{Namespace: 1447842353, Key: -2075668042}, LifecycleLogicalRoomAdvisoryKey(uuid.UUID{15: 3}))

	collisionA := uuid.MustParse("00000000-0000-0000-0000-000000018cc0")
	collisionB := uuid.MustParse("00000000-0000-0000-0000-00000002c2e6")
	locks := SortLifecycleLogicalRoomLocks([]uuid.UUID{collisionB, shared, collisionA, shared})
	require.Len(t, locks, 2, "duplicate UUIDs and colliding signed keys are acquired once")
	require.Less(t, locks[0].Key.Key, locks[1].Key.Key)
	require.Equal(t, int32(587361161), LifecycleLogicalRoomAdvisoryKey(collisionA).Key)
	require.Equal(t, LifecycleLogicalRoomAdvisoryKey(collisionA), LifecycleLogicalRoomAdvisoryKey(collisionB))
	require.Equal(t, collisionA, locks[1].VoiceRoomID, "raw UUID bytes choose the collision representative independent of input order")
	require.Equal(t, LifecycleLogicalRoomAdvisoryKey(collisionA), locks[1].Key)
}

func TestPostgresLifecycleStore_D01_IdenticalOperationsConverge(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL concurrency requires testcontainers")
	}
	fixture := r22NewStoreFixture(t, "r22stored01")
	decision := fixture.decision(LifecycleMethodJoin)
	first, second := r22RunBlockedDecisionPair(t, fixture, LifecycleOperationAdvisoryKey(decision.ActorProfileID, decision.OperationID), nil, nil, decision, decision)
	require.NoError(t, first.err)
	require.NoError(t, second.err)
	require.Equal(t, first.operation.OperationID, second.operation.OperationID)
	require.Equal(t, first.operation.Fingerprint, second.operation.Fingerprint)
	require.Equal(t, int64(1), r22TableRowCount(t, fixture.ctx, fixture.pool, "voice_lifecycle_operations"))
	effects, err := fixture.store.ListOperationEffects(fixture.ctx, decision.ActorProfileID, decision.OperationID)
	require.NoError(t, err)
	require.Len(t, effects, 1)
	require.Equal(t, decision.Effects[0].EffectID, effects[0].EffectID)
}

func TestPostgresLifecycleStore_D02_ChangedFingerprintHasOneWinner(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL concurrency requires testcontainers")
	}
	fixture := r22NewStoreFixture(t, "r22stored02")
	first := fixture.decision(LifecycleMethodJoin)
	second := r22CloneDecision(first)
	second.DestinationVoiceRoomID = r22Pointer(uuid.New())
	a, b := r22RunBlockedDecisionPair(t, fixture, LifecycleOperationAdvisoryKey(first.ActorProfileID, first.OperationID), nil, nil, first, second)
	require.Equal(t, 1, boolCount(a.err == nil, b.err == nil))
	require.True(t, errors.Is(a.err, ErrOperationConflict) || errors.Is(b.err, ErrOperationConflict))
	require.Equal(t, int64(1), r22TableRowCount(t, fixture.ctx, fixture.pool, "voice_lifecycle_operations"))
}

func boolCount(values ...bool) int {
	count := 0
	for _, value := range values {
		if value {
			count++
		}
	}
	return count
}

func TestPostgresLifecycleStore_D03_OneSubjectGetsOneNonterminalDecision(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL concurrency requires testcontainers")
	}
	fixture := r22NewStoreFixture(t, "r22stored03")
	first := fixture.decision(LifecycleMethodJoin)
	second := r22CloneDecision(first)
	second.OperationID = uuid.New()
	second.DestinationVoiceRoomID = r22Pointer(uuid.New())
	second.Effects[0].EffectID = uuid.New()
	a, b := r22RunBlockedDecisionPair(t, fixture, LifecycleSubjectAdvisoryKey(fixture.subjectProfileID),
		[]LifecycleAdvisoryLockKey{LifecycleOperationAdvisoryKey(first.ActorProfileID, first.OperationID)},
		[]LifecycleAdvisoryLockKey{LifecycleOperationAdvisoryKey(second.ActorProfileID, second.OperationID)}, first, second)
	require.Equal(t, 1, boolCount(a.err == nil, b.err == nil))
	require.True(t, errors.Is(a.err, ErrSubjectTransitionActive) || errors.Is(b.err, ErrSubjectTransitionActive))
}

func TestPostgresLifecycleStore_D04_ModeratorsAreFencedBySubject(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL concurrency requires testcontainers")
	}
	fixture := r22NewStoreFixture(t, "r22stored04")
	fixture.seedMembership(t, nil)
	first := fixture.decision(LifecycleMethodModeratorMove)
	second := r22CloneDecision(first)
	second.ActorProfileID = uuid.New()
	second.ActorAccountID = uuid.New()
	second.OperationID = uuid.New()
	second.DestinationVoiceRoomID = r22Pointer(uuid.New())
	for index := range second.Effects {
		second.Effects[index].EffectID = uuid.New()
	}
	a, b := r22RunBlockedDecisionPair(t, fixture, LifecycleSubjectAdvisoryKey(fixture.subjectProfileID),
		[]LifecycleAdvisoryLockKey{LifecycleOperationAdvisoryKey(first.ActorProfileID, first.OperationID)},
		[]LifecycleAdvisoryLockKey{LifecycleOperationAdvisoryKey(second.ActorProfileID, second.OperationID)}, first, second)
	require.Equal(t, 1, boolCount(a.err == nil, b.err == nil))
	require.True(t, errors.Is(a.err, ErrSubjectTransitionActive) || errors.Is(b.err, ErrSubjectTransitionActive))
}

func TestPostgresLifecycleStore_D05_QuarantineDoesNotReleaseSubjectFence(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL concurrency requires testcontainers")
	}
	fixture := r22NewStoreFixture(t, "r22stored05")
	winner := fixture.decision(LifecycleMethodJoin)
	_, err := fixture.store.DecideOperation(fixture.ctx, winner)
	require.NoError(t, err)
	claim := r22ClaimOperation(t, fixture)
	require.NoError(t, fixture.store.QuarantineOperation(fixture.ctx, LifecycleOperationQuarantine{ActorProfileID: winner.ActorProfileID, OperationID: winner.OperationID, WorkerID: fixture.workerID, ExpectedFence: claim.Fence, Class: "manual", At: r22StoreTime}))
	loser := r22CloneDecision(winner)
	loser.OperationID = uuid.New()
	loser.Effects[0].EffectID = uuid.New()
	_, err = fixture.store.DecideOperation(fixture.ctx, loser)
	require.ErrorIs(t, err, ErrSubjectTransitionActive)
}

func TestPostgresLifecycleStore_D06_OppositeMovesUseCanonicalRoomOrder(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL concurrency requires testcontainers")
	}
	fixture := r22NewStoreFixture(t, "r22stored06")
	roomA, roomB := uuid.UUID{15: 1}, uuid.UUID{15: 3}
	runtimeA := r22InsertRoom(t, fixture.ctx, fixture.pool, fixture.spaceID, roomA, deterministicLiveKitRoomName(roomA), "active", 1, nil)
	runtimeB := r22InsertRoom(t, fixture.ctx, fixture.pool, fixture.spaceID, roomB, deterministicLiveKitRoomName(roomB), "active", 1, nil)
	profileA, profileB, mediaA, mediaB := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	r22InsertMembership(t, fixture.ctx, fixture.pool, profileA, runtimeA, mediaA, r22ValidGrants)
	r22InsertMembership(t, fixture.ctx, fixture.pool, profileB, runtimeB, mediaB, r22ValidGrants)
	makeMove := func(profile, source, destination uuid.UUID) LifecycleDecision {
		decision := fixture.decision(LifecycleMethodSelfMove)
		decision.ActorProfileID, decision.SubjectProfileID, decision.OperationID = profile, profile, uuid.New()
		decision.SourceVoiceRoomID, decision.DestinationVoiceRoomID = &source, &destination
		for index := range decision.Effects {
			decision.Effects[index].EffectID = uuid.New()
		}
		return decision
	}
	first, second := makeMove(profileA, roomA, roomB), makeMove(profileB, roomB, roomA)
	locks := SortLifecycleLogicalRoomLocks([]uuid.UUID{roomB, roomA})
	require.Len(t, locks, 2)
	require.Less(t, locks[0].Key.Key, locks[1].Key.Key, "descending logical-room acquisition is forbidden")
	nameA, nameB := "r22-d06-a-"+uuid.NewString(), "r22-d06-b-"+uuid.NewString()
	storeA, _ := r22NamedStore(t, fixture.pool, nameA)
	storeB, _ := r22NamedStore(t, fixture.pool, nameB)
	blocker := r22BlockAdvisory(t, fixture.ctx, fixture.pool, locks[1].Key)
	workerCtx := r22BoundedWorkerContext(t, fixture.ctx)
	results := make(chan r22DecisionResult, 2)
	go func() {
		operation, err := storeA.DecideOperation(workerCtx, first)
		results <- r22DecisionResult{operation, err}
	}()
	go func() {
		operation, err := storeB.DecideOperation(workerCtx, second)
		results <- r22DecisionResult{operation, err}
	}()
	r22WaitAnyApplicationBlocked(t, workerCtx, fixture.pool, blocker, map[string][]LifecycleAdvisoryLockKey{
		nameA: {
			LifecycleOperationAdvisoryKey(first.ActorProfileID, first.OperationID),
			LifecycleSubjectAdvisoryKey(first.SubjectProfileID),
			locks[0].Key,
		},
		nameB: {
			LifecycleOperationAdvisoryKey(second.ActorProfileID, second.OperationID),
			LifecycleSubjectAdvisoryKey(second.SubjectProfileID),
			locks[0].Key,
		},
	}, nameA, nameB)
	require.NoError(t, blocker.tx.Commit(workerCtx))
	a, b := r22ReceiveDecision(t, results), r22ReceiveDecision(t, results)
	require.NoError(t, a.err)
	require.NoError(t, b.err)
}

func TestPostgresLifecycleStore_C25_D07_GrantDecisionBoundaryHasOnlyTwoOutcomes(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL concurrency requires testcontainers")
	}
	for _, grantFirst := range []bool{true, false} {
		name := "decision wins"
		if grantFirst {
			name = "grant wins"
		}
		t.Run(name, func(t *testing.T) {
			fixture := r22NewStoreFixture(t, "r22stored07"+uuid.NewString()[:6])
			fixture.seedMembership(t, nil)
			decision := fixture.decision(LifecycleMethodLeave)
			grantName, decisionName := "r22-d07-grant-"+uuid.NewString(), "r22-d07-decision-"+uuid.NewString()
			grantStore, _ := r22NamedStore(t, fixture.pool, grantName)
			decisionStore, _ := r22NamedStore(t, fixture.pool, decisionName)
			blocker := r22BlockAdvisory(t, fixture.ctx, fixture.pool, LifecycleSubjectAdvisoryKey(fixture.subjectProfileID))
			workerCtx := r22BoundedWorkerContext(t, fixture.ctx)
			expiresAt := r22StoreTime.Add(time.Hour)
			grantResult, decisionResult := make(chan error, 1), make(chan error, 1)
			startGrant := func() {
				go func() {
					_, err := grantStore.AdvanceLatestGrantExpiry(workerCtx, fixture.subjectProfileID, fixture.sourceMediaEpoch, expiresAt)
					grantResult <- err
				}()
			}
			startDecision := func() {
				go func() { _, err := decisionStore.DecideOperation(workerCtx, decision); decisionResult <- err }()
			}
			if grantFirst {
				startGrant()
				r22WaitApplicationBlocked(t, workerCtx, fixture.pool, blocker, grantName, nil)
				startDecision()
				r22WaitApplicationBlocked(t, workerCtx, fixture.pool, blocker, decisionName, []LifecycleAdvisoryLockKey{LifecycleOperationAdvisoryKey(decision.ActorProfileID, decision.OperationID)})
			} else {
				startDecision()
				r22WaitApplicationBlocked(t, workerCtx, fixture.pool, blocker, decisionName, []LifecycleAdvisoryLockKey{LifecycleOperationAdvisoryKey(decision.ActorProfileID, decision.OperationID)})
				startGrant()
				r22WaitApplicationBlocked(t, workerCtx, fixture.pool, blocker, grantName, nil)
			}
			require.NoError(t, blocker.tx.Commit(workerCtx))
			grantErr, decisionErr := r22ReceiveError(t, grantResult), r22ReceiveError(t, decisionResult)
			require.NoError(t, decisionErr)
			membership, found, err := fixture.store.LoadMembership(fixture.ctx, fixture.subjectProfileID)
			require.NoError(t, err)
			require.True(t, found)
			denial, denied, err := fixture.store.LookupMediaEpochDenial(fixture.ctx, fixture.sourceMediaEpoch)
			require.NoError(t, err)
			effects, err := fixture.store.ListOperationEffects(fixture.ctx, decision.ActorProfileID, decision.OperationID)
			require.NoError(t, err)
			require.NotEmpty(t, effects)
			require.Equal(t, LifecycleEffectEjectParticipant, effects[0].Kind)
			require.Equal(t, LifecycleEffectStateReady, effects[0].State)
			if grantFirst {
				require.NoError(t, grantErr)
				require.Equal(t, expiresAt, *membership.LatestGrantExpiresAt)
				require.True(t, denied)
				require.Equal(t, expiresAt, denial.GrantExpiresAt)
				require.Equal(t, expiresAt.Add(decision.AcceptedClockSkew), denial.DenyUntil)
			} else {
				require.ErrorIs(t, grantErr, ErrSubjectTransitionActive)
				require.Nil(t, membership.LatestGrantExpiresAt)
				require.False(t, denied)
			}
		})
	}
}

func TestPostgresLifecycleStore_D08_GreaterLeaseFenceOwnsEffectPersistence(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL concurrency requires testcontainers")
	}
	fixture := r22NewStoreFixture(t, "r22stored08")
	decision := fixture.decision(LifecycleMethodJoin)
	_, err := fixture.store.DecideOperation(fixture.ctx, decision)
	require.NoError(t, err)
	old, ok, err := fixture.store.ClaimNextEffect(fixture.ctx, fixture.workerID, r22StoreTime, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	newWorker := uuid.New()
	current, ok, err := fixture.store.ClaimNextEffect(fixture.ctx, newWorker, r22StoreTime.Add(time.Minute), time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	nameA, nameB := "r22-d08-old-"+uuid.NewString(), "r22-d08-new-"+uuid.NewString()
	oldStore, _ := r22NamedStore(t, fixture.pool, nameA)
	newStore, _ := r22NamedStore(t, fixture.pool, nameB)
	blocker, err := fixture.pool.Begin(fixture.ctx)
	require.NoError(t, err)
	r22RegisterRollbackCleanup(t, blocker)
	workerCtx := r22BoundedWorkerContext(t, fixture.ctx)
	var blockerPID int
	require.NoError(t, blocker.QueryRow(fixture.ctx, `SELECT pg_backend_pid()`).Scan(&blockerPID))
	_, err = blocker.Exec(fixture.ctx, `SELECT 1 FROM voice_lifecycle_effects WHERE effect_id=$1 FOR UPDATE`, old.Effect.EffectID)
	require.NoError(t, err)
	oldResult, newResult := make(chan error, 1), make(chan error, 1)
	go func() {
		oldResult <- oldStore.MarkEffectApplied(workerCtx, LifecycleEffectApplied{EffectID: old.Effect.EffectID, WorkerID: fixture.workerID, ExpectedFence: old.Fence, Observed: old.Effect, AppliedAt: r22StoreTime.Add(2 * time.Minute)})
	}()
	go func() {
		newResult <- newStore.MarkEffectApplied(workerCtx, LifecycleEffectApplied{EffectID: current.Effect.EffectID, WorkerID: newWorker, ExpectedFence: current.Fence, Observed: current.Effect, AppliedAt: r22StoreTime.Add(2 * time.Minute)})
	}()
	r22WaitExclusiveEffectLockChain(t, workerCtx, fixture.pool, blockerPID, old.Effect.EffectID, nameA, nameB)
	require.NoError(t, blocker.Commit(workerCtx))
	require.ErrorIs(t, r22ReceiveError(t, oldResult), ErrStaleLease)
	require.NoError(t, r22ReceiveError(t, newResult))
	effects, err := fixture.store.ListOperationEffects(fixture.ctx, decision.ActorProfileID, decision.OperationID)
	require.NoError(t, err)
	require.Len(t, effects, 1)
	persisted := effects[0]
	require.Equal(t, LifecycleEffectStateApplied, persisted.State)
	require.Equal(t, current.Fence, persisted.LeaseFence)
	require.Nil(t, persisted.LeaseOwner)
	require.Nil(t, persisted.LeaseUntil)
	require.Equal(t, r22StoreTime.Add(2*time.Minute), *persisted.AppliedAt)
	require.Equal(t, current.Effect.EffectID, persisted.EffectID)
	require.Equal(t, current.Effect.ActorProfileID, persisted.ActorProfileID)
	require.Equal(t, current.Effect.OperationID, persisted.OperationID)
	require.Equal(t, current.Effect.Ordinal, persisted.Ordinal)
	require.Equal(t, current.Effect.Kind, persisted.Kind)
	require.Equal(t, current.Effect.SchemaVersion, persisted.SchemaVersion)
	require.Equal(t, current.Effect.VoiceRoomID, persisted.VoiceRoomID)
	require.Equal(t, current.Effect.LiveKitRoomName, persisted.LiveKitRoomName)
	require.Equal(t, current.Effect.TargetProfileID, persisted.TargetProfileID)
	require.Equal(t, current.Effect.ParticipantIdentity, persisted.ParticipantIdentity)
	require.Equal(t, current.Effect.MediaEpoch, persisted.MediaEpoch)
	require.Equal(t, current.Effect.RequestBytes, persisted.RequestBytes)
	require.Equal(t, current.Effect.RequestDigest, persisted.RequestDigest)
}

func TestPostgresLifecycleStore_D08_CompletionLocksEffectPlanBeforeTerminalWrites(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL concurrency requires testcontainers")
	}
	fixture := r22NewStoreFixture(t, "r22stored08effectlock")
	fixture.seedMembership(t, nil)
	decision := fixture.decision(LifecycleMethodSelfMove)
	operation, err := fixture.store.DecideOperation(fixture.ctx, decision)
	require.NoError(t, err)
	r22ApplyAllEffects(t, fixture, decision)
	claim := r22ClaimOperation(t, fixture)
	sourceRoster, destinationRoster := int64(4), int64(1)
	receipt := LifecycleReceipt{ActorProfileID: decision.ActorProfileID, OperationID: decision.OperationID,
		SubjectProfileID: decision.SubjectProfileID, SpaceID: decision.SpaceID, Method: decision.Method, Outcome: LifecycleOutcomeMoved,
		SourceVoiceRoomID: decision.SourceVoiceRoomID, DestinationVoiceRoomID: decision.DestinationVoiceRoomID,
		RoomID: operation.DestinationRoomID, SourceRosterVersion: &sourceRoster, DestinationRosterVersion: &destinationRoster,
		MediaEpoch: operation.DestinationMediaEpoch, SpaceAccessEpoch: &decision.Authority.SpaceAccessEpoch,
		RolePolicyEpoch: &decision.Authority.SubjectRolePolicyEpoch, AuthorizationDigest: &decision.Authority.AuthorizationDigest}
	left := r22TestOutbox(fixture, operation, 0, "left", sourceRoster)
	left.RoomID, left.VoiceRoomID = *operation.SourceRoomID, *operation.SourceVoiceRoomID
	completion := LifecycleCompletion{ActorProfileID: decision.ActorProfileID, OperationID: decision.OperationID,
		WorkerID: fixture.workerID, ExpectedFence: claim.Fence, Receipt: receipt,
		Outbox:      []LifecycleOutboxRecord{left, r22TestOutbox(fixture, operation, 1, "joined", destinationRoster)},
		CompletedAt: r22StoreTime.Add(2 * time.Minute), ReplayUntil: r22StoreTime.Add(26 * time.Hour)}
	name := "r22-d08-complete-" + uuid.NewString()
	store, _ := r22NamedStore(t, fixture.pool, name)
	blockerTx, err := fixture.pool.Begin(fixture.ctx)
	require.NoError(t, err)
	_, err = blockerTx.Exec(fixture.ctx, `SELECT 1 FROM voice_lifecycle_effects WHERE actor_profile_id=$1 AND operation_id=$2 AND ordinal=1 FOR UPDATE`,
		decision.ActorProfileID, decision.OperationID)
	require.NoError(t, err)
	var blockerPID int
	require.NoError(t, blockerTx.QueryRow(fixture.ctx, `SELECT pg_backend_pid()`).Scan(&blockerPID))
	r22RegisterRollbackCleanup(t, blockerTx)
	workerCtx := r22BoundedWorkerContext(t, fixture.ctx)
	results := make(chan r22DecisionResult, 1)
	go func() {
		completed, callErr := store.CompleteOperation(workerCtx, completion)
		results <- r22DecisionResult{operation: completed, err: callErr}
	}()
	r22WaitRowLocks(t, workerCtx, fixture.pool, blockerPID, name)
	var blockedQuery string
	require.NoError(t, fixture.pool.QueryRow(workerCtx,
		`SELECT query FROM pg_stat_activity WHERE application_name=$1 AND wait_event_type='Lock'`, name).Scan(&blockedQuery))
	var ordinal int16
	err = fixture.pool.QueryRow(workerCtx, `SELECT ordinal FROM voice_lifecycle_effects
WHERE actor_profile_id=$1 AND operation_id=$2 AND ordinal=0 FOR UPDATE NOWAIT`, decision.ActorProfileID, decision.OperationID).Scan(&ordinal)
	r22RequireSQLState(t, err, "55P03")
	require.NoError(t, blockerTx.Commit(workerCtx))
	result := r22ReceiveDecision(t, results)
	require.NoError(t, result.err)
	normalized := strings.ToLower(strings.Join(strings.Fields(blockedQuery), ""))
	require.Contains(t, normalized, "orderbyordinal")
	require.Contains(t, normalized, "forupdate")
	require.Len(t, decision.Effects, 2)
	r22RequireCompletedReceipt(t, fixture, result.operation, completion.Receipt)
}

func TestPostgresLifecycleStore_D08_MarkAppliedTakesExclusiveEffectLock(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL concurrency requires testcontainers")
	}
	fixture := r22NewStoreFixture(t, "r22stored08exclusive")
	decision := fixture.decision(LifecycleMethodJoin)
	_, err := fixture.store.DecideOperation(fixture.ctx, decision)
	require.NoError(t, err)
	claim, ok, err := fixture.store.ClaimNextEffect(fixture.ctx, fixture.workerID, r22StoreTime, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	name := "r22-d08-exclusive-" + uuid.NewString()
	store, _ := r22NamedStore(t, fixture.pool, name)
	blockerTx, err := fixture.pool.Begin(fixture.ctx)
	require.NoError(t, err)
	_, err = blockerTx.Exec(fixture.ctx, `SELECT 1 FROM voice_lifecycle_effects WHERE effect_id=$1 FOR SHARE`, claim.Effect.EffectID)
	require.NoError(t, err)
	var blockerPID int
	require.NoError(t, blockerTx.QueryRow(fixture.ctx, `SELECT pg_backend_pid()`).Scan(&blockerPID))
	r22RegisterRollbackCleanup(t, blockerTx)
	workerCtx := r22BoundedWorkerContext(t, fixture.ctx)
	results := make(chan error, 1)
	go func() {
		results <- store.MarkEffectApplied(workerCtx, LifecycleEffectApplied{EffectID: claim.Effect.EffectID,
			WorkerID: fixture.workerID, ExpectedFence: claim.Fence, Observed: claim.Effect, AppliedAt: r22StoreTime.Add(time.Minute)})
	}()
	r22WaitRowLocks(t, workerCtx, fixture.pool, blockerPID, name)
	var blockedQuery string
	require.NoError(t, fixture.pool.QueryRow(workerCtx, `SELECT query FROM pg_stat_activity WHERE application_name=$1 AND wait_event_type='Lock'`, name).Scan(&blockedQuery))
	require.NoError(t, blockerTx.Commit(workerCtx))
	require.NoError(t, r22ReceiveError(t, results))
	normalized := strings.ToLower(strings.Join(strings.Fields(blockedQuery), ""))
	require.Contains(t, normalized, "select")
	require.Contains(t, normalized, "forupdate", "MarkEffectApplied must take an exclusive row lock before comparing observation")
}

func TestPostgresLifecycleStore_D09_AmbiguousTerminalRetryIsObservationOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL concurrency requires testcontainers")
	}
	fixture := r22NewStoreFixture(t, "r22stored09")
	fixture.seedMembership(t, r22Pointer(r22StoreTime.Add(time.Hour)))
	decision := fixture.decision(LifecycleMethodSelfMove)
	operation, err := fixture.store.DecideOperation(fixture.ctx, decision)
	require.NoError(t, err)
	r22ApplyAllEffects(t, fixture, decision)
	claim := r22ClaimOperation(t, fixture)
	sourceRoster, destinationRoster := int64(4), int64(1)
	receipt := LifecycleReceipt{OperationID: decision.OperationID, ActorProfileID: decision.ActorProfileID, SubjectProfileID: decision.SubjectProfileID,
		SpaceID: decision.SpaceID, Method: decision.Method, Outcome: LifecycleOutcomeMoved, SourceVoiceRoomID: decision.SourceVoiceRoomID,
		DestinationVoiceRoomID: decision.DestinationVoiceRoomID, RoomID: operation.DestinationRoomID, SourceRosterVersion: &sourceRoster,
		DestinationRosterVersion: &destinationRoster, MediaEpoch: operation.DestinationMediaEpoch,
		SpaceAccessEpoch: &decision.Authority.SpaceAccessEpoch, RolePolicyEpoch: &decision.Authority.SubjectRolePolicyEpoch,
		AuthorizationDigest: &decision.Authority.AuthorizationDigest}
	left := r22TestOutbox(fixture, operation, 0, "left", sourceRoster)
	left.RoomID, left.VoiceRoomID = *operation.SourceRoomID, *operation.SourceVoiceRoomID
	completion := LifecycleCompletion{ActorProfileID: decision.ActorProfileID, OperationID: decision.OperationID, WorkerID: fixture.workerID, ExpectedFence: claim.Fence,
		Receipt: receipt, Outbox: []LifecycleOutboxRecord{left, r22TestOutbox(fixture, operation, 1, "joined", destinationRoster)}, CompletedAt: r22StoreTime.Add(2 * time.Minute), ReplayUntil: r22StoreTime.Add(26 * time.Hour)}
	denialBefore := r22TableRowsSnapshot(t, fixture.ctx, fixture.pool, "voice_media_epoch_denials")
	effectsBefore := r22TableRowsSnapshot(t, fixture.ctx, fixture.pool, "voice_lifecycle_effects")
	require.Len(t, denialBefore, 1)
	require.Len(t, effectsBefore, 2)
	roomLocks := SortLifecycleLogicalRoomLocks([]uuid.UUID{*operation.SourceVoiceRoomID, *operation.DestinationVoiceRoomID})
	require.Len(t, roomLocks, 2)
	nameA, nameB, nameGrant := "r22-d09-a-"+uuid.NewString(), "r22-d09-b-"+uuid.NewString(), "r22-d09-grant-"+uuid.NewString()
	storeA, _ := r22NamedStore(t, fixture.pool, nameA)
	storeB, _ := r22NamedStore(t, fixture.pool, nameB)
	grantStore, _ := r22NamedStore(t, fixture.pool, nameGrant)
	blocker := r22BlockAdvisory(t, fixture.ctx, fixture.pool, roomLocks[1].Key)
	workerCtx := r22BoundedWorkerContext(t, fixture.ctx)
	results := make(chan r22DecisionResult, 2)
	grantResults := make(chan r22MembershipResult, 1)
	go func() {
		op, callErr := storeA.CompleteOperation(workerCtx, completion)
		results <- r22DecisionResult{op, callErr}
	}()
	r22WaitApplicationBlocked(t, workerCtx, fixture.pool, blocker, nameA, []LifecycleAdvisoryLockKey{
		LifecycleOperationAdvisoryKey(decision.ActorProfileID, decision.OperationID),
		LifecycleSubjectAdvisoryKey(decision.SubjectProfileID),
		roomLocks[0].Key,
	})
	terminalPID := r22BlockedApplicationPID(t, workerCtx, fixture.pool, nameA)
	go func() {
		op, callErr := storeB.CompleteOperation(workerCtx, completion)
		results <- r22DecisionResult{op, callErr}
	}()
	r22WaitApplicationBlocked(t, workerCtx, fixture.pool, r22AdvisoryBlocker{
		pid: terminalPID, key: LifecycleOperationAdvisoryKey(decision.ActorProfileID, decision.OperationID),
	}, nameB, nil)
	go func() {
		membership, callErr := grantStore.AdvanceLatestGrantExpiry(workerCtx, fixture.subjectProfileID, fixture.sourceMediaEpoch, r22StoreTime.Add(3*time.Hour))
		grantResults <- r22MembershipResult{membership: membership, err: callErr}
	}()
	r22WaitApplicationBlocked(t, workerCtx, fixture.pool, r22AdvisoryBlocker{
		pid: terminalPID, key: LifecycleSubjectAdvisoryKey(decision.SubjectProfileID),
	}, nameGrant, nil)
	require.NoError(t, blocker.tx.Commit(workerCtx))
	a, b := r22ReceiveDecision(t, results), r22ReceiveDecision(t, results)
	staleGrant := r22ReceiveMembership(t, grantResults)
	require.NoError(t, a.err)
	require.NoError(t, b.err)
	require.ErrorIs(t, staleGrant.err, ErrMembershipConflict)
	require.Equal(t, LifecycleMembership{}, staleGrant.membership)
	require.Equal(t, a.operation.ReceiptBytes, b.operation.ReceiptBytes)
	r22RequireCompletedReceipt(t, fixture, a.operation, receipt)
	r22RequireCompletedReceipt(t, fixture, b.operation, receipt)
	r22RequireOutboxRows(t, fixture, completion)
	r22RequireTerminalMembership(t, fixture, fixture.subjectProfileID, *operation.DestinationRoomID, *operation.DestinationMediaEpoch, decision.Authority, completion.CompletedAt)
	require.Equal(t, int64(1), r22TableRowCount(t, fixture.ctx, fixture.pool, "voice_room_memberships"))
	require.Equal(t, int64(2), r22TableRowCount(t, fixture.ctx, fixture.pool, "voice_event_outbox"))
	for roomID, want := range map[uuid.UUID]int64{fixture.sourceRoomID: sourceRoster, *operation.DestinationRoomID: destinationRoster} {
		room, found, loadErr := fixture.store.LoadRoomSnapshot(fixture.ctx, roomID)
		require.NoError(t, loadErr)
		require.True(t, found)
		require.Equal(t, want, room.RosterVersion)
	}
	membership, membershipFound, err := fixture.store.LoadMembership(fixture.ctx, fixture.subjectProfileID)
	require.NoError(t, err)
	require.True(t, membershipFound)
	persisted, found, err := fixture.store.LoadOperation(fixture.ctx, decision.ActorProfileID, decision.OperationID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, *operation.DestinationRoomID, membership.RoomID)
	require.Equal(t, *operation.DestinationMediaEpoch, membership.MediaEpoch)
	require.Equal(t, decision.Authority.SpaceAccessEpoch, membership.SpaceAccessEpoch)
	require.Equal(t, decision.Authority.SubjectRolePolicyEpoch, membership.RolePolicyEpoch)
	require.Equal(t, decision.Authority.AuthorizationDigest, membership.AuthorizationDigest)
	r22RequireGrants(t, decision.Authority.SubjectGrants, membership.Grants)
	require.Nil(t, membership.LatestGrantExpiresAt)
	require.Equal(t, completion.CompletedAt, membership.JoinedAt)
	require.Equal(t, completion.CompletedAt, membership.UpdatedAt)
	require.Equal(t, receipt, *persisted.Receipt)
	require.Equal(t, a.operation.ReceiptHash, b.operation.ReceiptHash)
	require.Equal(t, denialBefore, r22TableRowsSnapshot(t, fixture.ctx, fixture.pool, "voice_media_epoch_denials"))
	require.Equal(t, effectsBefore, r22TableRowsSnapshot(t, fixture.ctx, fixture.pool, "voice_lifecycle_effects"))
	rows, err := fixture.pool.Query(fixture.ctx, `SELECT ordinal,convert_from(payload_bytes,'UTF8') FROM voice_event_outbox ORDER BY ordinal`)
	require.NoError(t, err)
	defer rows.Close()
	var payloads []string
	for rows.Next() {
		var ordinal int16
		var payload string
		require.NoError(t, rows.Scan(&ordinal, &payload))
		require.Equal(t, int16(len(payloads)), ordinal)
		payloads = append(payloads, payload)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []string{"left", "joined"}, payloads)
	beforeReplay := r22AllTableSnapshots(t, fixture)
	replayed, err := fixture.store.CompleteOperation(fixture.ctx, completion)
	require.NoError(t, err)
	require.Equal(t, a.operation.ReceiptBytes, replayed.ReceiptBytes)
	require.Equal(t, a.operation.ReceiptHash, replayed.ReceiptHash)
	require.Equal(t, beforeReplay, r22AllTableSnapshots(t, fixture), "terminal retry must preserve generation timestamps and every durable row")
}

func TestPostgresLifecycleStore_D10_ConcurrentDestinationCreateReloadsOneRuntime(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL concurrency requires testcontainers")
	}
	fixture := r22NewStoreFixture(t, "r22stored10")
	first := fixture.decision(LifecycleMethodJoin)
	second := r22CloneDecision(first)
	second.ActorProfileID, second.SubjectProfileID, second.ActorAccountID, second.OperationID = uuid.New(), uuid.New(), uuid.New(), uuid.New()
	second.Effects[0].EffectID = uuid.New()
	a, b := r22RunBlockedDecisionPair(t, fixture, LifecycleLogicalRoomAdvisoryKey(fixture.destinationVoiceRoomID),
		[]LifecycleAdvisoryLockKey{
			LifecycleOperationAdvisoryKey(first.ActorProfileID, first.OperationID),
			LifecycleSubjectAdvisoryKey(first.SubjectProfileID),
		},
		[]LifecycleAdvisoryLockKey{
			LifecycleOperationAdvisoryKey(second.ActorProfileID, second.OperationID),
			LifecycleSubjectAdvisoryKey(second.SubjectProfileID),
		}, first, second)
	require.NoError(t, a.err)
	require.NoError(t, b.err)
	require.NotNil(t, a.operation.DestinationRoomID)
	require.NotNil(t, b.operation.DestinationRoomID)
	require.Equal(t, a.operation.DestinationRoomID, b.operation.DestinationRoomID)
	require.Equal(t, fixture.destinationVoiceRoomID, *a.operation.DestinationVoiceRoomID)
	var effectNames []string
	for _, decision := range []LifecycleDecision{first, second} {
		persisted, found, err := fixture.store.LoadOperation(fixture.ctx, decision.ActorProfileID, decision.OperationID)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, a.operation.DestinationRoomID, persisted.DestinationRoomID)
		effects, err := fixture.store.ListOperationEffects(fixture.ctx, decision.ActorProfileID, decision.OperationID)
		require.NoError(t, err)
		require.Len(t, effects, 1)
		require.Equal(t, fixture.destinationVoiceRoomID, effects[0].VoiceRoomID)
		require.Equal(t, fixture.destinationVoiceRoomID, *persisted.DestinationVoiceRoomID)
		require.NotEmpty(t, effects[0].LiveKitRoomName)
		effectNames = append(effectNames, effects[0].LiveKitRoomName)
	}
	require.Equal(t, effectNames[0], effectNames[1], "the deterministic LiveKit target is identical for both writers")
	require.Equal(t, int64(1), r22TableRowCount(t, fixture.ctx, fixture.pool, "voice_room_instances"))
	var names int64
	require.NoError(t, fixture.pool.QueryRow(fixture.ctx, `SELECT count(DISTINCT livekit_room_name) FROM voice_room_instances WHERE voice_room_id=$1`, fixture.destinationVoiceRoomID).Scan(&names))
	require.Equal(t, int64(1), names)
	room, found, err := fixture.store.LoadRoomSnapshot(fixture.ctx, *a.operation.DestinationRoomID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, fixture.destinationVoiceRoomID, room.VoiceRoomID)
	require.NotEmpty(t, room.LiveKitRoomName)
	require.Equal(t, room.LiveKitRoomName, effectNames[0])
}

func TestPostgresLifecycleStore_D10_DestinationReloadTakesExactRowLock(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL concurrency requires testcontainers")
	}
	fixture := r22NewStoreFixture(t, "r22stored10destinationlock")
	destinationRoomID := r22InsertRoom(t, fixture.ctx, fixture.pool, fixture.spaceID, fixture.destinationVoiceRoomID,
		deterministicLiveKitRoomName(fixture.destinationVoiceRoomID), "active", 0, nil)
	name := "r22-d10-destination-lock-" + uuid.NewString()
	store, _ := r22NamedStore(t, fixture.pool, name)
	blockerTx, err := fixture.pool.Begin(fixture.ctx)
	require.NoError(t, err)
	_, err = blockerTx.Exec(fixture.ctx, `SELECT 1 FROM voice_room_instances WHERE room_id=$1 FOR SHARE`, destinationRoomID)
	require.NoError(t, err)
	var blockerPID int
	require.NoError(t, blockerTx.QueryRow(fixture.ctx, `SELECT pg_backend_pid()`).Scan(&blockerPID))
	r22RegisterRollbackCleanup(t, blockerTx)
	workerCtx := r22BoundedWorkerContext(t, fixture.ctx)
	decision := fixture.decision(LifecycleMethodJoin)
	results := make(chan r22DecisionResult, 1)
	go func() {
		operation, callErr := store.DecideOperation(workerCtx, decision)
		results <- r22DecisionResult{operation: operation, err: callErr}
	}()
	r22WaitRowLocks(t, workerCtx, fixture.pool, blockerPID, name)
	var blockedQuery string
	require.NoError(t, fixture.pool.QueryRow(workerCtx,
		`SELECT query FROM pg_stat_activity WHERE application_name=$1 AND wait_event_type='Lock'`, name).Scan(&blockedQuery))
	require.NoError(t, blockerTx.Commit(workerCtx))
	result := r22ReceiveDecision(t, results)
	require.NoError(t, result.err)
	require.Equal(t, r22Pointer(destinationRoomID), result.operation.DestinationRoomID)
	normalized := strings.ToLower(strings.Join(strings.Fields(blockedQuery), ""))
	require.Contains(t, normalized,
		"selectroom_id,space_id,voice_room_id,livekit_room_name,state,roster_version,created_at,updated_at")
	require.Contains(t, normalized, "forupdate")
}

type r22MembershipCreateResult struct {
	roomID     uuid.UUID
	mediaEpoch uuid.UUID
	err        error
}

func r22CreateMembershipWithSubjectLock(t *testing.T, ctx context.Context, pool *pgxpool.Pool, fixture r22StoreFixture, roomID, mediaEpoch uuid.UUID) (err error) {
	t.Helper()
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	r22RegisterRollbackCleanup(t, tx)
	if err = advisoryLock(ctx, tx, LifecycleSubjectAdvisoryKey(fixture.subjectProfileID)); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
INSERT INTO voice_room_instances(room_id,space_id,voice_room_id,livekit_room_name,state,roster_version,created_at,updated_at)
VALUES($1,$2,$3,$4,'active',1,$5,$5)`, roomID, fixture.spaceID, fixture.sourceVoiceRoomID,
		"r22-d11-source-"+fixture.sourceVoiceRoomID.String(), r22StoreTime)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
INSERT INTO voice_room_memberships(
 profile_id,room_id,media_epoch,space_access_epoch,role_policy_epoch,authorization_digest,
 can_join,can_publish_audio,can_publish_video,can_publish_screen_share,can_subscribe,
 can_mute_others,can_deafen_others,can_move_others,can_use_ptt,priority_speaker,joined_at,updated_at)
VALUES($1,$2,$3,1,1,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$15)`,
		fixture.subjectProfileID, roomID, mediaEpoch, bytesOfLength(32),
		r22ValidGrants[0], r22ValidGrants[1], r22ValidGrants[2], r22ValidGrants[3], r22ValidGrants[4],
		r22ValidGrants[5], r22ValidGrants[6], r22ValidGrants[7], r22ValidGrants[8], r22ValidGrants[9], r22StoreTime)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func TestPostgresLifecycleStore_D10_SourceCloseIsRevalidatedAfterRuntimeLock(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL concurrency requires testcontainers")
	}
	fixture := r22NewStoreFixture(t, "r22stored10closesource")
	fixture.seedMembership(t, nil)
	decision := fixture.decision(LifecycleMethodLeave)
	name := "r22-d10-close-source-" + uuid.NewString()
	store, _ := r22NamedStore(t, fixture.pool, name)
	blocker := r22BlockRoomClose(t, fixture.ctx, fixture.pool, fixture.sourceRoomID)
	workerCtx := r22BoundedWorkerContext(t, fixture.ctx)
	results := make(chan r22DecisionResult, 1)
	go func() {
		operation, err := store.DecideOperation(workerCtx, decision)
		results <- r22DecisionResult{operation: operation, err: err}
	}()
	r22WaitRowLocks(t, workerCtx, fixture.pool, blocker.pid, name)
	require.NoError(t, blocker.tx.Commit(workerCtx))
	result := r22ReceiveDecision(t, results)
	require.ErrorIs(t, result.err, ErrMembershipConflict)
	require.Equal(t, LifecycleOperation{}, result.operation)
	require.Zero(t, r22TableRowCount(t, fixture.ctx, fixture.pool, "voice_lifecycle_operations"))
	require.Zero(t, r22TableRowCount(t, fixture.ctx, fixture.pool, "voice_lifecycle_effects"))
	require.Zero(t, r22TableRowCount(t, fixture.ctx, fixture.pool, "voice_media_epoch_denials"))
}

func TestPostgresLifecycleStore_D11_AbsentLeaveAndMembershipCreationSerialize(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL concurrency requires testcontainers")
	}
	for _, leaveFirst := range []bool{true, false} {
		name := "membership wins"
		if leaveFirst {
			name = "absent leave wins"
		}
		t.Run(name, func(t *testing.T) {
			fixture := r22NewStoreFixture(t, "r22stored11"+uuid.NewString()[:6])
			decision := fixture.decision(LifecycleMethodLeave)
			noOp := LifecycleNoOpDecision{Decision: decision, CompletedAt: r22StoreTime, ReplayUntil: r22StoreTime.Add(24 * time.Hour)}
			roomID, mediaEpoch := uuid.New(), uuid.New()
			leaveName, membershipName := "r22-d11-leave-"+uuid.NewString(), "r22-d11-membership-"+uuid.NewString()
			leaveStore, _ := r22NamedStore(t, fixture.pool, leaveName)
			_, membershipPool := r22NamedStore(t, fixture.pool, membershipName)
			blocker := r22BlockAdvisory(t, fixture.ctx, fixture.pool, LifecycleSubjectAdvisoryKey(fixture.subjectProfileID))
			workerCtx := r22BoundedWorkerContext(t, fixture.ctx)
			leaveResult := make(chan r22DecisionResult, 1)
			membershipResult := make(chan r22MembershipCreateResult, 1)
			startLeave := func() {
				go func() {
					operation, callErr := leaveStore.CompleteNoOp(workerCtx, noOp)
					leaveResult <- r22DecisionResult{operation: operation, err: callErr}
				}()
			}
			startMembership := func() {
				go func() {
					callErr := r22CreateMembershipWithSubjectLock(t, workerCtx, membershipPool, fixture, roomID, mediaEpoch)
					membershipResult <- r22MembershipCreateResult{roomID: roomID, mediaEpoch: mediaEpoch, err: callErr}
				}()
			}
			if leaveFirst {
				startLeave()
				r22WaitApplicationBlocked(t, workerCtx, fixture.pool, blocker, leaveName,
					[]LifecycleAdvisoryLockKey{LifecycleOperationAdvisoryKey(decision.ActorProfileID, decision.OperationID)})
				startMembership()
				r22WaitApplicationBlocked(t, workerCtx, fixture.pool, blocker, membershipName, nil)
			} else {
				startMembership()
				r22WaitApplicationBlocked(t, workerCtx, fixture.pool, blocker, membershipName, nil)
				startLeave()
				r22WaitApplicationBlocked(t, workerCtx, fixture.pool, blocker, leaveName,
					[]LifecycleAdvisoryLockKey{LifecycleOperationAdvisoryKey(decision.ActorProfileID, decision.OperationID)})
			}
			require.NoError(t, blocker.tx.Commit(workerCtx))
			leave := r22ReceiveDecision(t, leaveResult)
			var membership r22MembershipCreateResult
			select {
			case membership = <-membershipResult:
			case <-time.After(10 * time.Second):
				t.Fatal("membership result did not arrive after subject lock barrier release")
			}
			require.NoError(t, membership.err)
			persistedMembership, found, err := fixture.store.LoadMembership(fixture.ctx, fixture.subjectProfileID)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, membership.roomID, persistedMembership.RoomID)
			require.Equal(t, membership.mediaEpoch, persistedMembership.MediaEpoch)
			if leaveFirst {
				require.NoError(t, leave.err)
				require.Equal(t, LifecycleOperationCompleted, leave.operation.State)
				require.NotNil(t, leave.operation.Receipt)
				require.Equal(t, LifecycleOutcomeNoOp, leave.operation.Receipt.Outcome)
				require.Nil(t, leave.operation.SourceRoomID)
				require.Nil(t, leave.operation.SourceMediaEpoch)
				beforeReplay := r22AllTableSnapshots(t, fixture)
				replayed, replayErr := fixture.store.CompleteNoOp(fixture.ctx, noOp)
				require.NoError(t, replayErr)
				require.Equal(t, leave.operation.ReceiptBytes, replayed.ReceiptBytes)
				require.Equal(t, leave.operation.ReceiptHash, replayed.ReceiptHash)
				require.Equal(t, beforeReplay, r22AllTableSnapshots(t, fixture), "historical no-op replay cannot remove a later membership")
				return
			}
			require.ErrorIs(t, leave.err, ErrMembershipConflict)
			require.Empty(t, r22TableRowsSnapshot(t, fixture.ctx, fixture.pool, "voice_lifecycle_operations"))
			operation, err := fixture.store.DecideOperation(fixture.ctx, decision)
			require.NoError(t, err)
			require.Equal(t, r22Pointer(roomID), operation.SourceRoomID)
			require.Equal(t, r22Pointer(mediaEpoch), operation.SourceMediaEpoch)
		})
	}
}
