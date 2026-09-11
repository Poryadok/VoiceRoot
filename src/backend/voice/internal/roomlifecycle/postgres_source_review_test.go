package roomlifecycle

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

type r22RollbackRecordingTx struct {
	pgx.Tx
	rollbackContext context.Context
}

func (tx *r22RollbackRecordingTx) Rollback(ctx context.Context) error {
	tx.rollbackContext = ctx
	return nil
}

func r22RequireStableError(t *testing.T, err, target error) {
	t.Helper()
	require.ErrorIs(t, err, target)
	require.NotContains(t, strings.ToUpper(err.Error()), "SQLSTATE")
	require.NotContains(t, strings.ToLower(err.Error()), "connection")
}

func r22DisableUserTriggers(t *testing.T, fixture r22StoreFixture, table string) func() {
	t.Helper()
	require.Contains(t, []string{"voice_lifecycle_operations", "voice_lifecycle_effects", "voice_event_outbox"}, table)
	tableSQL := pgx.Identifier{table}.Sanitize()
	_, err := fixture.pool.Exec(fixture.ctx, "ALTER TABLE "+tableSQL+" DISABLE TRIGGER USER")
	require.NoError(t, err)
	return func() {
		_, enableErr := fixture.pool.Exec(fixture.ctx, "ALTER TABLE "+tableSQL+" ENABLE TRIGGER USER")
		require.NoError(t, enableErr)
	}
}

func r22InstallTerminalUpdateSkip(t *testing.T, fixture r22StoreFixture, table string) func() {
	t.Helper()
	require.Contains(t, []string{"voice_lifecycle_operations", "voice_lifecycle_effects", "voice_event_outbox"}, table)
	tableSQL := pgx.Identifier{table}.Sanitize()
	functionSQL := pgx.Identifier{"r22_skip_" + table + "_update"}.Sanitize()
	triggerSQL := pgx.Identifier{"r22_skip_" + table + "_update_trigger"}.Sanitize()
	_, err := fixture.pool.Exec(fixture.ctx, "CREATE FUNCTION "+functionSQL+`() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NULL; END $$;
CREATE TRIGGER `+triggerSQL+" BEFORE UPDATE ON "+tableSQL+" FOR EACH ROW EXECUTE FUNCTION "+functionSQL+"()")
	require.NoError(t, err)
	return func() {
		_, dropErr := fixture.pool.Exec(fixture.ctx, "DROP TRIGGER "+triggerSQL+" ON "+tableSQL+"; DROP FUNCTION "+functionSQL+"()")
		require.NoError(t, dropErr)
	}
}

func r22InstallTerminalUpdateFailure(t *testing.T, fixture r22StoreFixture, table string) func() {
	t.Helper()
	require.Contains(t, []string{"voice_lifecycle_operations", "voice_lifecycle_effects", "voice_event_outbox"}, table)
	tableSQL := pgx.Identifier{table}.Sanitize()
	functionSQL := pgx.Identifier{"r22_fail_" + table + "_update"}.Sanitize()
	triggerSQL := pgx.Identifier{"r22_fail_" + table + "_update_trigger"}.Sanitize()
	_, err := fixture.pool.Exec(fixture.ctx, "CREATE FUNCTION "+functionSQL+`() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 RAISE EXCEPTION USING ERRCODE='P0001', MESSAGE='r22 forced terminal update failure';
END $$;
CREATE TRIGGER `+triggerSQL+" BEFORE UPDATE ON "+tableSQL+" FOR EACH ROW EXECUTE FUNCTION "+functionSQL+"()")
	require.NoError(t, err)
	return func() {
		_, dropErr := fixture.pool.Exec(fixture.ctx, "DROP TRIGGER "+triggerSQL+" ON "+tableSQL+"; DROP FUNCTION "+functionSQL+"()")
		require.NoError(t, dropErr)
	}
}

func r22InstallMembershipMutationFailure(t *testing.T, fixture r22StoreFixture, event string) func() {
	t.Helper()
	require.Contains(t, []string{"DELETE", "UPDATE"}, event)
	suffix := strings.ToLower(event)
	functionSQL := pgx.Identifier{"r22_fail_membership_" + suffix}.Sanitize()
	triggerSQL := pgx.Identifier{"r22_fail_membership_" + suffix + "_trigger"}.Sanitize()
	_, err := fixture.pool.Exec(fixture.ctx, "CREATE FUNCTION "+functionSQL+`() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 RAISE EXCEPTION USING ERRCODE='P0001', MESSAGE='r22 forced membership mutation failure';
END $$;
CREATE TRIGGER `+triggerSQL+" BEFORE "+event+" ON voice_room_memberships FOR EACH ROW EXECUTE FUNCTION "+functionSQL+"()")
	require.NoError(t, err)
	return func() {
		_, dropErr := fixture.pool.Exec(fixture.ctx, "DROP TRIGGER "+triggerSQL+" ON voice_room_memberships; DROP FUNCTION "+functionSQL+"()")
		require.NoError(t, dropErr)
	}
}

func r22InstallUpdateAdvisoryBarrier(t *testing.T, fixture r22StoreFixture, table string, key LifecycleAdvisoryLockKey) func() {
	t.Helper()
	require.Contains(t, []string{"voice_lifecycle_operations", "voice_lifecycle_effects", "voice_event_outbox"}, table)
	tableSQL := pgx.Identifier{table}.Sanitize()
	functionSQL := pgx.Identifier{"r22_barrier_" + table + "_update"}.Sanitize()
	triggerSQL := pgx.Identifier{"r22_barrier_" + table + "_update_trigger"}.Sanitize()
	body := fmt.Sprintf("() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_advisory_xact_lock(%d,%d); RETURN NEW; END $$;", key.Namespace, key.Key)
	_, err := fixture.pool.Exec(fixture.ctx, "CREATE FUNCTION "+functionSQL+body+
		" CREATE TRIGGER "+triggerSQL+" BEFORE UPDATE ON "+tableSQL+" FOR EACH ROW EXECUTE FUNCTION "+functionSQL+"()")
	require.NoError(t, err)
	return func() {
		_, dropErr := fixture.pool.Exec(fixture.ctx, "DROP TRIGGER "+triggerSQL+" ON "+tableSQL+"; DROP FUNCTION "+functionSQL+"()")
		require.NoError(t, dropErr)
	}
}

func r22BlockedQuery(t *testing.T, fixture r22StoreFixture, name string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(fixture.ctx, 5*time.Second)
	defer cancel()
	var query string
	require.NoError(t, fixture.pool.QueryRow(ctx, `SELECT query FROM pg_stat_activity WHERE application_name=$1 AND wait_event_type='Lock'`, name).Scan(&query))
	return strings.ToLower(strings.Join(strings.Fields(query), ""))
}

func r22RequireTerminalLeasePredicate(t *testing.T, query, state string) {
	t.Helper()
	require.Contains(t, query, "andstate='"+state+"'")
	require.Contains(t, query, "andlease_owner=")
	require.Contains(t, query, "andlease_fence=")
}

func TestPostgresLifecycleStore_SourceReviewRollbackUsesBoundedBackgroundContext(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	tx := &r22RollbackRecordingTx{}
	errResult := ErrInvariant
	finishLifecycleTx(cancelled, tx, &errResult)
	require.ErrorIs(t, errResult, ErrInvariant)
	require.NotNil(t, tx.rollbackContext)
	require.NoError(t, tx.rollbackContext.Err(), "rollback must not reuse the cancelled request context")
	deadline, bounded := tx.rollbackContext.Deadline()
	require.True(t, bounded, "rollback cleanup must have a finite background deadline")
	require.Positive(t, time.Until(deadline))
	require.LessOrEqual(t, time.Until(deadline), 5*time.Second)
}

func TestPostgresLifecycleStore_SourceReviewCancelledRequestReturnsConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL cancellation contract requires testcontainers")
	}
	fixture := r22NewStoreFixture(t, "r22sourcecancel")
	config := fixture.pool.Config().Copy()
	config.MaxConns = 1
	name := "r22-cancel-" + uuid.NewString()
	config.ConnConfig.RuntimeParams["application_name"] = name
	one, err := pgxpool.NewWithConfig(context.Background(), config)
	require.NoError(t, err)
	t.Cleanup(one.Close)
	store := NewPostgresLifecycleStore(one)
	decision := fixture.decision(LifecycleMethodLeave)
	blocker := r22BlockAdvisory(t, fixture.ctx, fixture.pool, LifecycleOperationAdvisoryKey(decision.ActorProfileID, decision.OperationID))
	workerContext := r22BoundedWorkerContext(t, fixture.ctx)
	requestContext, cancel := context.WithCancel(workerContext)
	result := make(chan r22DecisionResult, 1)
	go func() {
		operation, callErr := store.CompleteNoOp(requestContext, LifecycleNoOpDecision{
			Decision: decision, CompletedAt: r22StoreTime, ReplayUntil: r22StoreTime.Add(24 * time.Hour),
		})
		result <- r22DecisionResult{operation: operation, err: callErr}
	}()
	r22WaitApplicationBlocked(t, fixture.ctx, fixture.pool, blocker, name, nil)
	cancel()
	call := r22ReceiveDecision(t, result)
	r22RequireStableError(t, call.err, ErrUnavailable)
	reuseContext, reuseCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer reuseCancel()
	var oneValue int
	require.NoError(t, one.QueryRow(reuseContext, `SELECT 1`).Scan(&oneValue))
	require.Equal(t, 1, oneValue)
}

func TestPostgresLifecycleStore_SourceReviewLoadErrorsAreTyped(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL failure classification requires testcontainers")
	}
	t.Run("absence is not corruption", func(t *testing.T) {
		fixture := r22NewStoreFixture(t, "r22sourceabsence")
		_, found, err := fixture.store.LoadOperation(fixture.ctx, uuid.New(), uuid.New())
		require.NoError(t, err)
		require.False(t, found)
		_, found, err = fixture.store.LoadMembership(fixture.ctx, uuid.New())
		require.NoError(t, err)
		require.False(t, found)
		_, found, err = fixture.store.LoadRoomSnapshot(fixture.ctx, uuid.New())
		require.NoError(t, err)
		require.False(t, found)
	})
	t.Run("closed pool is unavailable", func(t *testing.T) {
		fixture := r22NewStoreFixture(t, "r22sourceclosed")
		fixture.pool.Close()
		_, _, err := fixture.store.LoadOperation(fixture.ctx, uuid.New(), uuid.New())
		r22RequireStableError(t, err, ErrUnavailable)
		_, _, err = fixture.store.LoadMembership(fixture.ctx, uuid.New())
		r22RequireStableError(t, err, ErrUnavailable)
		_, _, err = fixture.store.LoadRoomSnapshot(fixture.ctx, uuid.New())
		r22RequireStableError(t, err, ErrUnavailable)
	})
	t.Run("admin shutdown is unavailable", func(t *testing.T) {
		fixture := r22NewStoreFixture(t, "r22sourceadmin")
		config := fixture.pool.Config().Copy()
		named, err := pgxpool.NewWithConfig(fixture.ctx, config)
		require.NoError(t, err)
		t.Cleanup(named.Close)
		connection, err := named.Acquire(fixture.ctx)
		require.NoError(t, err)
		defer connection.Release()
		tx, err := connection.Begin(fixture.ctx)
		require.NoError(t, err)
		var pid int
		require.NoError(t, tx.QueryRow(fixture.ctx, `SELECT pg_backend_pid()`).Scan(&pid))
		var terminated bool
		require.NoError(t, fixture.pool.QueryRow(fixture.ctx, `SELECT pg_terminate_backend($1)`, pid).Scan(&terminated))
		require.True(t, terminated)
		_, _, err = loadOperation(fixture.ctx, tx, uuid.New(), uuid.New(), "")
		r22RequireStableError(t, err, ErrUnavailable)
		_, _, err = loadMembership(fixture.ctx, tx, uuid.New(), "")
		r22RequireStableError(t, err, ErrUnavailable)
		_, _, err = loadRoomTx(fixture.ctx, tx, uuid.New(), "")
		r22RequireStableError(t, err, ErrUnavailable)
	})
}

func TestPostgresLifecycleStore_SourceReviewRecomputesDurableHashes(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL corruption contract requires testcontainers")
	}
	for _, column := range []string{"fingerprint", "binding_hash"} {
		t.Run("operation "+column, func(t *testing.T) {
			fixture := r22NewStoreFixture(t, "r22source"+strings.ReplaceAll(column, "_", ""))
			decision := fixture.decision(LifecycleMethodJoin)
			_, err := fixture.store.DecideOperation(fixture.ctx, decision)
			require.NoError(t, err)
			enable := r22DisableUserTriggers(t, fixture, "voice_lifecycle_operations")
			_, err = fixture.pool.Exec(fixture.ctx, `UPDATE voice_lifecycle_operations SET `+pgx.Identifier{column}.Sanitize()+`=$1 WHERE actor_profile_id=$2 AND operation_id=$3`,
				[]byte(strings.Repeat("z", 32)), decision.ActorProfileID, decision.OperationID)
			require.NoError(t, err)
			enable()
			_, _, err = fixture.store.LoadOperation(fixture.ctx, decision.ActorProfileID, decision.OperationID)
			r22RequireStableError(t, err, ErrInvariant)
		})
	}
	t.Run("receipt hash", func(t *testing.T) {
		fixture := r22NewStoreFixture(t, "r22sourcereceipthash")
		decision := fixture.decision(LifecycleMethodLeave)
		operation, err := fixture.store.CompleteNoOp(fixture.ctx, LifecycleNoOpDecision{Decision: decision,
			CompletedAt: r22StoreTime, ReplayUntil: r22StoreTime.Add(24 * time.Hour)})
		require.NoError(t, err)
		enable := r22DisableUserTriggers(t, fixture, "voice_lifecycle_operations")
		_, err = fixture.pool.Exec(fixture.ctx, `UPDATE voice_lifecycle_operations SET receipt_hash=$1 WHERE actor_profile_id=$2 AND operation_id=$3`,
			[]byte(strings.Repeat("z", 32)), decision.ActorProfileID, decision.OperationID)
		require.NoError(t, err)
		enable()
		_, _, err = fixture.store.LoadOperation(fixture.ctx, operation.ActorProfileID, operation.OperationID)
		r22RequireStableError(t, err, ErrInvariant)
	})
	t.Run("receipt typed columns", func(t *testing.T) {
		fixture := r22NewStoreFixture(t, "r22sourcereceipttyped")
		decision := fixture.decision(LifecycleMethodLeave)
		operation, err := fixture.store.CompleteNoOp(fixture.ctx, LifecycleNoOpDecision{Decision: decision,
			CompletedAt: r22StoreTime, ReplayUntil: r22StoreTime.Add(24 * time.Hour)})
		require.NoError(t, err)
		changed := *operation.Receipt
		changed.OperationID = uuid.New()
		changedBytes, changedHash, err := EncodeLifecycleReceipt(changed)
		require.NoError(t, err)
		enable := r22DisableUserTriggers(t, fixture, "voice_lifecycle_operations")
		_, err = fixture.pool.Exec(fixture.ctx, `UPDATE voice_lifecycle_operations SET receipt_bytes=$1,receipt_hash=$2 WHERE actor_profile_id=$3 AND operation_id=$4`,
			changedBytes, changedHash[:], operation.ActorProfileID, operation.OperationID)
		require.NoError(t, err)
		enable()
		_, _, err = fixture.store.LoadOperation(fixture.ctx, operation.ActorProfileID, operation.OperationID)
		r22RequireStableError(t, err, ErrInvariant)
	})
	t.Run("receipt deterministic encoding", func(t *testing.T) {
		fixture := r22NewStoreFixture(t, "r22sourcereceiptcanonical")
		decision := fixture.decision(LifecycleMethodLeave)
		operation, err := fixture.store.CompleteNoOp(fixture.ctx, LifecycleNoOpDecision{Decision: decision,
			CompletedAt: r22StoreTime, ReplayUntil: r22StoreTime.Add(24 * time.Hour)})
		require.NoError(t, err)
		// Unknown protobuf field 127 with varint value 1 remains syntactically valid,
		// but is not the deterministic encoding of the durable typed receipt columns.
		noncanonical := append(append([]byte(nil), operation.ReceiptBytes...), 0xf8, 0x07, 0x01)
		noncanonicalHash := sha256.Sum256(noncanonical)
		enable := r22DisableUserTriggers(t, fixture, "voice_lifecycle_operations")
		_, err = fixture.pool.Exec(fixture.ctx, `UPDATE voice_lifecycle_operations SET receipt_bytes=$1,receipt_hash=$2 WHERE actor_profile_id=$3 AND operation_id=$4`,
			noncanonical, noncanonicalHash[:], operation.ActorProfileID, operation.OperationID)
		require.NoError(t, err)
		enable()
		_, _, err = fixture.store.LoadOperation(fixture.ctx, operation.ActorProfileID, operation.OperationID)
		r22RequireStableError(t, err, ErrInvariant)
	})
	t.Run("effect request", func(t *testing.T) {
		fixture := r22NewStoreFixture(t, "r22sourcerequest")
		decision := fixture.decision(LifecycleMethodJoin)
		_, err := fixture.store.DecideOperation(fixture.ctx, decision)
		require.NoError(t, err)
		enable := r22DisableUserTriggers(t, fixture, "voice_lifecycle_effects")
		_, err = fixture.pool.Exec(fixture.ctx, `UPDATE voice_lifecycle_effects SET request_digest=$1 WHERE effect_id=$2`,
			[]byte(strings.Repeat("z", 32)), decision.Effects[0].EffectID)
		require.NoError(t, err)
		enable()
		_, err = fixture.store.ListOperationEffects(fixture.ctx, decision.ActorProfileID, decision.OperationID)
		r22RequireStableError(t, err, ErrInvariant)
	})
	t.Run("outbox payload", func(t *testing.T) {
		fixture := r22NewStoreFixture(t, "r22sourcepayload")
		decision := fixture.decision(LifecycleMethodJoin)
		operation, err := fixture.store.DecideOperation(fixture.ctx, decision)
		require.NoError(t, err)
		r22ApplyAllEffects(t, fixture, decision)
		claim := r22ClaimOperation(t, fixture)
		completion := LifecycleCompletion{ActorProfileID: decision.ActorProfileID, OperationID: decision.OperationID,
			WorkerID: fixture.workerID, ExpectedFence: claim.Fence, Receipt: r22JoinReceipt(fixture, decision, 1),
			Outbox:      []LifecycleOutboxRecord{r22TestOutbox(fixture, operation, 0, "joined", 1)},
			CompletedAt: r22StoreTime.Add(2 * time.Minute), ReplayUntil: r22StoreTime.Add(26 * time.Hour)}
		_, err = fixture.store.CompleteOperation(fixture.ctx, completion)
		require.NoError(t, err)
		enable := r22DisableUserTriggers(t, fixture, "voice_event_outbox")
		_, err = fixture.pool.Exec(fixture.ctx, `UPDATE voice_event_outbox SET payload_hash=$1 WHERE event_id=$2`,
			[]byte(strings.Repeat("z", 32)), completion.Outbox[0].EventID)
		require.NoError(t, err)
		enable()
		_, _, err = fixture.store.ClaimNextOutbox(fixture.ctx, uuid.New(), r22StoreTime.Add(3*time.Minute), time.Minute)
		r22RequireStableError(t, err, ErrInvariant)
	})
}

func TestPostgresLifecycleStore_SourceReviewInjectedWriteErrorsAreTyped(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL write failure classification requires testcontainers")
	}
	t.Run("effect update", func(t *testing.T) {
		fixture := r22NewStoreFixture(t, "r22sourceeffectwriteerror")
		decision := fixture.decision(LifecycleMethodJoin)
		_, err := fixture.store.DecideOperation(fixture.ctx, decision)
		require.NoError(t, err)
		claim, ok, err := fixture.store.ClaimNextEffect(fixture.ctx, fixture.workerID, r22StoreTime, time.Minute)
		require.NoError(t, err)
		require.True(t, ok)
		before := r22AllTableSnapshots(t, fixture)
		drop := r22InstallTerminalUpdateFailure(t, fixture, "voice_lifecycle_effects")
		err = fixture.store.MarkEffectApplied(fixture.ctx, LifecycleEffectApplied{EffectID: claim.Effect.EffectID,
			WorkerID: fixture.workerID, ExpectedFence: claim.Fence, Observed: claim.Effect, AppliedAt: r22StoreTime.Add(time.Minute)})
		r22RequireStableError(t, err, ErrInvariant)
		require.Equal(t, before, r22AllTableSnapshots(t, fixture))
		drop()
	})
	t.Run("outbox update", func(t *testing.T) {
		fixture := r22NewStoreFixture(t, "r22sourceoutboxwriteerror")
		decision := fixture.decision(LifecycleMethodJoin)
		operation, err := fixture.store.DecideOperation(fixture.ctx, decision)
		require.NoError(t, err)
		r22ApplyAllEffects(t, fixture, decision)
		opClaim := r22ClaimOperation(t, fixture)
		completion := LifecycleCompletion{ActorProfileID: decision.ActorProfileID, OperationID: decision.OperationID,
			WorkerID: fixture.workerID, ExpectedFence: opClaim.Fence, Receipt: r22JoinReceipt(fixture, decision, 1),
			Outbox:      []LifecycleOutboxRecord{r22TestOutbox(fixture, operation, 0, "joined", 1)},
			CompletedAt: r22StoreTime.Add(2 * time.Minute), ReplayUntil: r22StoreTime.Add(26 * time.Hour)}
		_, err = fixture.store.CompleteOperation(fixture.ctx, completion)
		require.NoError(t, err)
		claim, ok, err := fixture.store.ClaimNextOutbox(fixture.ctx, fixture.workerID, r22StoreTime.Add(3*time.Minute), time.Minute)
		require.NoError(t, err)
		require.True(t, ok)
		before := r22AllTableSnapshots(t, fixture)
		drop := r22InstallTerminalUpdateFailure(t, fixture, "voice_event_outbox")
		err = fixture.store.MarkOutboxDelivered(fixture.ctx, LifecycleOutboxDelivered{EventID: claim.Outbox.EventID,
			WorkerID: fixture.workerID, ExpectedFence: claim.Fence, DeliveredAt: r22StoreTime.Add(4 * time.Minute)})
		r22RequireStableError(t, err, ErrInvariant)
		require.Equal(t, before, r22AllTableSnapshots(t, fixture))
		drop()
	})
	t.Run("operation update", func(t *testing.T) {
		fixture := r22NewStoreFixture(t, "r22sourceoperationwriteerror")
		decision := fixture.decision(LifecycleMethodJoin)
		operation, err := fixture.store.DecideOperation(fixture.ctx, decision)
		require.NoError(t, err)
		r22ApplyAllEffects(t, fixture, decision)
		claim := r22ClaimOperation(t, fixture)
		completion := LifecycleCompletion{ActorProfileID: decision.ActorProfileID, OperationID: decision.OperationID,
			WorkerID: fixture.workerID, ExpectedFence: claim.Fence, Receipt: r22JoinReceipt(fixture, decision, 1),
			Outbox:      []LifecycleOutboxRecord{r22TestOutbox(fixture, operation, 0, "joined", 1)},
			CompletedAt: r22StoreTime.Add(2 * time.Minute), ReplayUntil: r22StoreTime.Add(26 * time.Hour)}
		before := r22AllTableSnapshots(t, fixture)
		drop := r22InstallTerminalUpdateFailure(t, fixture, "voice_lifecycle_operations")
		_, err = fixture.store.CompleteOperation(fixture.ctx, completion)
		r22RequireStableError(t, err, ErrInvariant)
		require.Equal(t, before, r22AllTableSnapshots(t, fixture))
		drop()
	})
	t.Run("terminal transaction insert", func(t *testing.T) {
		fixture := r22NewStoreFixture(t, "r22sourceinsertwriteerror")
		decision := fixture.decision(LifecycleMethodJoin)
		operation, err := fixture.store.DecideOperation(fixture.ctx, decision)
		require.NoError(t, err)
		r22ApplyAllEffects(t, fixture, decision)
		claim := r22ClaimOperation(t, fixture)
		completion := LifecycleCompletion{ActorProfileID: decision.ActorProfileID, OperationID: decision.OperationID,
			WorkerID: fixture.workerID, ExpectedFence: claim.Fence, Receipt: r22JoinReceipt(fixture, decision, 1),
			Outbox:      []LifecycleOutboxRecord{r22TestOutbox(fixture, operation, 0, "joined", 1)},
			CompletedAt: r22StoreTime.Add(2 * time.Minute), ReplayUntil: r22StoreTime.Add(26 * time.Hour)}
		before := r22AllTableSnapshots(t, fixture)
		drop := r22InstallFailureAfterInsert(t, fixture, "voice_event_outbox")
		_, err = fixture.store.CompleteOperation(fixture.ctx, completion)
		r22RequireStableError(t, err, ErrInvariant)
		require.Equal(t, before, r22AllTableSnapshots(t, fixture))
		drop()
	})
}

func TestPostgresLifecycleStore_SourceReviewMembershipMutationErrorsRollBack(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL terminal rollback contract requires testcontainers")
	}
	t.Run("leave delete", func(t *testing.T) {
		fixture := r22NewStoreFixture(t, "r22sourceleavedelete")
		fixture.seedMembership(t, r22Pointer(r22StoreTime.Add(time.Hour)))
		decision := fixture.decision(LifecycleMethodLeave)
		operation, err := fixture.store.DecideOperation(fixture.ctx, decision)
		require.NoError(t, err)
		r22ApplyAllEffects(t, fixture, decision)
		claim := r22ClaimOperation(t, fixture)
		sourceRoster := int64(4)
		receipt := LifecycleReceipt{ActorProfileID: decision.ActorProfileID, OperationID: decision.OperationID,
			SubjectProfileID: decision.SubjectProfileID, SpaceID: decision.SpaceID, Method: decision.Method,
			Outcome: LifecycleOutcomeLeft, SourceVoiceRoomID: decision.SourceVoiceRoomID,
			RoomID: operation.SourceRoomID, SourceRosterVersion: &sourceRoster}
		completion := LifecycleCompletion{ActorProfileID: decision.ActorProfileID, OperationID: decision.OperationID,
			WorkerID: fixture.workerID, ExpectedFence: claim.Fence, Receipt: receipt,
			Outbox:      []LifecycleOutboxRecord{r22TestOutbox(fixture, operation, 0, "left", sourceRoster)},
			CompletedAt: r22StoreTime.Add(2 * time.Minute), ReplayUntil: r22StoreTime.Add(26 * time.Hour)}
		before := r22AllTableSnapshots(t, fixture)
		drop := r22InstallMembershipMutationFailure(t, fixture, "DELETE")
		_, err = fixture.store.CompleteOperation(fixture.ctx, completion)
		r22RequireStableError(t, err, ErrInvariant)
		require.Equal(t, before, r22AllTableSnapshots(t, fixture))
		drop()
	})
	t.Run("move update", func(t *testing.T) {
		fixture := r22NewStoreFixture(t, "r22sourcemoveupdate")
		fixture.seedMembership(t, r22Pointer(r22StoreTime.Add(time.Hour)))
		decision := fixture.decision(LifecycleMethodSelfMove)
		operation, err := fixture.store.DecideOperation(fixture.ctx, decision)
		require.NoError(t, err)
		r22ApplyAllEffects(t, fixture, decision)
		claim := r22ClaimOperation(t, fixture)
		sourceRoster, destinationRoster := int64(4), int64(1)
		receipt := LifecycleReceipt{ActorProfileID: decision.ActorProfileID, OperationID: decision.OperationID,
			SubjectProfileID: decision.SubjectProfileID, SpaceID: decision.SpaceID, Method: decision.Method,
			Outcome: LifecycleOutcomeMoved, SourceVoiceRoomID: decision.SourceVoiceRoomID,
			DestinationVoiceRoomID: decision.DestinationVoiceRoomID, RoomID: operation.DestinationRoomID,
			SourceRosterVersion: &sourceRoster, DestinationRosterVersion: &destinationRoster,
			MediaEpoch: operation.DestinationMediaEpoch, SpaceAccessEpoch: &decision.Authority.SpaceAccessEpoch,
			RolePolicyEpoch: &decision.Authority.SubjectRolePolicyEpoch, AuthorizationDigest: &decision.Authority.AuthorizationDigest}
		left := r22TestOutbox(fixture, operation, 0, "left", sourceRoster)
		left.RoomID, left.VoiceRoomID = *operation.SourceRoomID, *operation.SourceVoiceRoomID
		completion := LifecycleCompletion{ActorProfileID: decision.ActorProfileID, OperationID: decision.OperationID,
			WorkerID: fixture.workerID, ExpectedFence: claim.Fence, Receipt: receipt,
			Outbox:      []LifecycleOutboxRecord{left, r22TestOutbox(fixture, operation, 1, "joined", destinationRoster)},
			CompletedAt: r22StoreTime.Add(2 * time.Minute), ReplayUntil: r22StoreTime.Add(26 * time.Hour)}
		before := r22AllTableSnapshots(t, fixture)
		drop := r22InstallMembershipMutationFailure(t, fixture, "UPDATE")
		_, err = fixture.store.CompleteOperation(fixture.ctx, completion)
		r22RequireStableError(t, err, ErrInvariant)
		require.Equal(t, before, r22AllTableSnapshots(t, fixture))
		drop()
	})
}

func TestPostgresLifecycleStore_SourceReviewCompletionRequiresExactEffectPlan(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL effect-plan contract requires testcontainers")
	}
	for _, mutation := range []string{"missing", "extra", "changed kind"} {
		t.Run(mutation, func(t *testing.T) {
			fixture := r22NewStoreFixture(t, "r22sourceplan"+strings.ReplaceAll(mutation, " ", ""))
			fixture.seedMembership(t, nil)
			decision := fixture.decision(LifecycleMethodSelfMove)
			operation, err := fixture.store.DecideOperation(fixture.ctx, decision)
			require.NoError(t, err)
			r22ApplyAllEffects(t, fixture, decision)
			claim := r22ClaimOperation(t, fixture)
			switch mutation {
			case "missing":
				_, err = fixture.pool.Exec(fixture.ctx, `DELETE FROM voice_lifecycle_effects WHERE actor_profile_id=$1 AND operation_id=$2 AND ordinal=1`,
					decision.ActorProfileID, decision.OperationID)
			case "extra":
				_, err = fixture.pool.Exec(fixture.ctx, `
INSERT INTO voice_lifecycle_effects(
 effect_id,actor_profile_id,operation_id,ordinal,kind,schema_version,target_voice_room_id,target_livekit_room_name,
 target_profile_id,target_participant_identity,target_media_epoch,request_bytes,request_digest,state,attempt_count,
 last_error_class,last_error_at,next_attempt_at,lease_owner,lease_until,lease_fence,applied_at,
 quarantine_class,quarantine_detail,quarantined_at,created_at,updated_at)
SELECT $1,actor_profile_id,operation_id,2,kind,schema_version,target_voice_room_id,target_livekit_room_name,
 target_profile_id,target_participant_identity,target_media_epoch,request_bytes,request_digest,state,attempt_count,
 last_error_class,last_error_at,next_attempt_at,lease_owner,lease_until,lease_fence,applied_at,
 quarantine_class,quarantine_detail,quarantined_at,created_at,updated_at
FROM voice_lifecycle_effects WHERE actor_profile_id=$2 AND operation_id=$3 AND ordinal=1`,
					uuid.New(), decision.ActorProfileID, decision.OperationID)
			case "changed kind":
				enable := r22DisableUserTriggers(t, fixture, "voice_lifecycle_effects")
				_, err = fixture.pool.Exec(fixture.ctx, `
UPDATE voice_lifecycle_effects SET kind='livekit_ensure_room',target_voice_room_id=$1,
 target_livekit_room_name=$2,target_profile_id=NULL,target_participant_identity=NULL,target_media_epoch=NULL
WHERE actor_profile_id=$3 AND operation_id=$4 AND ordinal=0`, *operation.DestinationVoiceRoomID,
					deterministicLiveKitRoomName(*operation.DestinationVoiceRoomID), decision.ActorProfileID, decision.OperationID)
				enable()
			}
			require.NoError(t, err)
			sourceRoster, destinationRoster := int64(4), int64(1)
			receipt := LifecycleReceipt{OperationID: decision.OperationID, ActorProfileID: decision.ActorProfileID,
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
			before := r22AllTableSnapshots(t, fixture)
			_, err = fixture.store.CompleteOperation(fixture.ctx, completion)
			r22RequireStableError(t, err, ErrInvariant)
			require.Equal(t, before, r22AllTableSnapshots(t, fixture))
		})
	}
}

func TestPostgresLifecycleStore_SourceReviewTerminalUpdatesRequireOneLeasedRow(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL terminal write contract requires testcontainers")
	}
	t.Run("effect", func(t *testing.T) {
		fixture := r22NewStoreFixture(t, "r22sourceeffectrow")
		decision := fixture.decision(LifecycleMethodJoin)
		_, err := fixture.store.DecideOperation(fixture.ctx, decision)
		require.NoError(t, err)
		claim, ok, err := fixture.store.ClaimNextEffect(fixture.ctx, fixture.workerID, r22StoreTime, time.Minute)
		require.NoError(t, err)
		require.True(t, ok)
		before := r22AllTableSnapshots(t, fixture)
		drop := r22InstallTerminalUpdateSkip(t, fixture, "voice_lifecycle_effects")
		err = fixture.store.MarkEffectApplied(fixture.ctx, LifecycleEffectApplied{EffectID: claim.Effect.EffectID,
			WorkerID: fixture.workerID, ExpectedFence: claim.Fence, Observed: claim.Effect, AppliedAt: r22StoreTime.Add(time.Minute)})
		r22RequireStableError(t, err, ErrStaleLease)
		require.Equal(t, before, r22AllTableSnapshots(t, fixture))
		drop()
	})
	t.Run("outbox", func(t *testing.T) {
		fixture := r22NewStoreFixture(t, "r22sourceoutboxrow")
		decision := fixture.decision(LifecycleMethodJoin)
		operation, err := fixture.store.DecideOperation(fixture.ctx, decision)
		require.NoError(t, err)
		r22ApplyAllEffects(t, fixture, decision)
		opClaim := r22ClaimOperation(t, fixture)
		completion := LifecycleCompletion{ActorProfileID: decision.ActorProfileID, OperationID: decision.OperationID,
			WorkerID: fixture.workerID, ExpectedFence: opClaim.Fence, Receipt: r22JoinReceipt(fixture, decision, 1),
			Outbox:      []LifecycleOutboxRecord{r22TestOutbox(fixture, operation, 0, "joined", 1)},
			CompletedAt: r22StoreTime.Add(2 * time.Minute), ReplayUntil: r22StoreTime.Add(26 * time.Hour)}
		_, err = fixture.store.CompleteOperation(fixture.ctx, completion)
		require.NoError(t, err)
		claim, ok, err := fixture.store.ClaimNextOutbox(fixture.ctx, fixture.workerID, r22StoreTime.Add(3*time.Minute), time.Minute)
		require.NoError(t, err)
		require.True(t, ok)
		before := r22AllTableSnapshots(t, fixture)
		drop := r22InstallTerminalUpdateSkip(t, fixture, "voice_event_outbox")
		err = fixture.store.MarkOutboxDelivered(fixture.ctx, LifecycleOutboxDelivered{EventID: claim.Outbox.EventID,
			WorkerID: fixture.workerID, ExpectedFence: claim.Fence, DeliveredAt: r22StoreTime.Add(4 * time.Minute)})
		r22RequireStableError(t, err, ErrStaleLease)
		require.Equal(t, before, r22AllTableSnapshots(t, fixture))
		drop()
	})
	t.Run("operation", func(t *testing.T) {
		fixture := r22NewStoreFixture(t, "r22sourceoperationrow")
		decision := fixture.decision(LifecycleMethodJoin)
		operation, err := fixture.store.DecideOperation(fixture.ctx, decision)
		require.NoError(t, err)
		r22ApplyAllEffects(t, fixture, decision)
		claim := r22ClaimOperation(t, fixture)
		completion := LifecycleCompletion{ActorProfileID: decision.ActorProfileID, OperationID: decision.OperationID,
			WorkerID: fixture.workerID, ExpectedFence: claim.Fence, Receipt: r22JoinReceipt(fixture, decision, 1),
			Outbox:      []LifecycleOutboxRecord{r22TestOutbox(fixture, operation, 0, "joined", 1)},
			CompletedAt: r22StoreTime.Add(2 * time.Minute), ReplayUntil: r22StoreTime.Add(26 * time.Hour)}
		before := r22AllTableSnapshots(t, fixture)
		drop := r22InstallTerminalUpdateSkip(t, fixture, "voice_lifecycle_operations")
		_, err = fixture.store.CompleteOperation(fixture.ctx, completion)
		r22RequireStableError(t, err, ErrStaleLease)
		require.Equal(t, before, r22AllTableSnapshots(t, fixture))
		drop()
	})
}

func TestPostgresLifecycleStore_SourceReviewTerminalSQLUsesLeasePredicates(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL terminal SQL observation requires testcontainers")
	}
	t.Run("effect", func(t *testing.T) {
		fixture := r22NewStoreFixture(t, "r22sourceeffectsql")
		decision := fixture.decision(LifecycleMethodJoin)
		_, err := fixture.store.DecideOperation(fixture.ctx, decision)
		require.NoError(t, err)
		claim, ok, err := fixture.store.ClaimNextEffect(fixture.ctx, fixture.workerID, r22StoreTime, time.Minute)
		require.NoError(t, err)
		require.True(t, ok)
		key := LifecycleOperationAdvisoryKey(uuid.New(), uuid.New())
		name := "r22-effect-sql-" + uuid.NewString()
		store, _ := r22NamedStore(t, fixture.pool, name)
		drop := r22InstallUpdateAdvisoryBarrier(t, fixture, "voice_lifecycle_effects", key)
		blocker := r22BlockAdvisory(t, fixture.ctx, fixture.pool, key)
		workerCtx := r22BoundedWorkerContext(t, fixture.ctx)
		results := make(chan error, 1)
		go func() {
			results <- store.MarkEffectApplied(workerCtx, LifecycleEffectApplied{EffectID: claim.Effect.EffectID,
				WorkerID: fixture.workerID, ExpectedFence: claim.Fence, Observed: claim.Effect, AppliedAt: r22StoreTime.Add(time.Minute)})
		}()
		r22WaitApplicationBlocked(t, workerCtx, fixture.pool, blocker, name, nil)
		query := r22BlockedQuery(t, fixture, name)
		require.NoError(t, blocker.tx.Commit(workerCtx))
		require.NoError(t, r22ReceiveError(t, results))
		drop()
		r22RequireTerminalLeasePredicate(t, query, "ready")
	})
	t.Run("outbox", func(t *testing.T) {
		fixture := r22NewStoreFixture(t, "r22sourceoutboxsql")
		decision := fixture.decision(LifecycleMethodJoin)
		operation, err := fixture.store.DecideOperation(fixture.ctx, decision)
		require.NoError(t, err)
		r22ApplyAllEffects(t, fixture, decision)
		opClaim := r22ClaimOperation(t, fixture)
		completion := LifecycleCompletion{ActorProfileID: decision.ActorProfileID, OperationID: decision.OperationID,
			WorkerID: fixture.workerID, ExpectedFence: opClaim.Fence, Receipt: r22JoinReceipt(fixture, decision, 1),
			Outbox:      []LifecycleOutboxRecord{r22TestOutbox(fixture, operation, 0, "joined", 1)},
			CompletedAt: r22StoreTime.Add(2 * time.Minute), ReplayUntil: r22StoreTime.Add(26 * time.Hour)}
		_, err = fixture.store.CompleteOperation(fixture.ctx, completion)
		require.NoError(t, err)
		claim, ok, err := fixture.store.ClaimNextOutbox(fixture.ctx, fixture.workerID, r22StoreTime.Add(3*time.Minute), time.Minute)
		require.NoError(t, err)
		require.True(t, ok)
		key := LifecycleOperationAdvisoryKey(uuid.New(), uuid.New())
		name := "r22-outbox-sql-" + uuid.NewString()
		store, _ := r22NamedStore(t, fixture.pool, name)
		drop := r22InstallUpdateAdvisoryBarrier(t, fixture, "voice_event_outbox", key)
		blocker := r22BlockAdvisory(t, fixture.ctx, fixture.pool, key)
		workerCtx := r22BoundedWorkerContext(t, fixture.ctx)
		results := make(chan error, 1)
		go func() {
			results <- store.MarkOutboxDelivered(workerCtx, LifecycleOutboxDelivered{EventID: claim.Outbox.EventID,
				WorkerID: fixture.workerID, ExpectedFence: claim.Fence, DeliveredAt: r22StoreTime.Add(4 * time.Minute)})
		}()
		r22WaitApplicationBlocked(t, workerCtx, fixture.pool, blocker, name, nil)
		query := r22BlockedQuery(t, fixture, name)
		require.NoError(t, blocker.tx.Commit(workerCtx))
		require.NoError(t, r22ReceiveError(t, results))
		drop()
		r22RequireTerminalLeasePredicate(t, query, "ready")
	})
	t.Run("operation", func(t *testing.T) {
		fixture := r22NewStoreFixture(t, "r22sourceoperationsql")
		decision := fixture.decision(LifecycleMethodJoin)
		operation, err := fixture.store.DecideOperation(fixture.ctx, decision)
		require.NoError(t, err)
		r22ApplyAllEffects(t, fixture, decision)
		claim := r22ClaimOperation(t, fixture)
		completion := LifecycleCompletion{ActorProfileID: decision.ActorProfileID, OperationID: decision.OperationID,
			WorkerID: fixture.workerID, ExpectedFence: claim.Fence, Receipt: r22JoinReceipt(fixture, decision, 1),
			Outbox:      []LifecycleOutboxRecord{r22TestOutbox(fixture, operation, 0, "joined", 1)},
			CompletedAt: r22StoreTime.Add(2 * time.Minute), ReplayUntil: r22StoreTime.Add(26 * time.Hour)}
		key := LifecycleOperationAdvisoryKey(uuid.New(), uuid.New())
		name := "r22-operation-sql-" + uuid.NewString()
		store, _ := r22NamedStore(t, fixture.pool, name)
		drop := r22InstallUpdateAdvisoryBarrier(t, fixture, "voice_lifecycle_operations", key)
		blocker := r22BlockAdvisory(t, fixture.ctx, fixture.pool, key)
		workerCtx := r22BoundedWorkerContext(t, fixture.ctx)
		results := make(chan r22DecisionResult, 1)
		go func() {
			completed, callErr := store.CompleteOperation(workerCtx, completion)
			results <- r22DecisionResult{operation: completed, err: callErr}
		}()
		r22WaitApplicationBlocked(t, workerCtx, fixture.pool, blocker, name, []LifecycleAdvisoryLockKey{
			LifecycleOperationAdvisoryKey(decision.ActorProfileID, decision.OperationID),
			LifecycleSubjectAdvisoryKey(decision.SubjectProfileID),
			LifecycleLogicalRoomAdvisoryKey(*decision.DestinationVoiceRoomID),
		})
		query := r22BlockedQuery(t, fixture, name)
		require.NoError(t, blocker.tx.Commit(workerCtx))
		require.NoError(t, r22ReceiveDecision(t, results).err)
		drop()
		r22RequireTerminalLeasePredicate(t, query, "decided")
	})
}
