package roomlifecycle

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// This file freezes RED-B B01-B20 from R22.2-VOICE-DB-PLAN.md lines 530-557
// and 770-797. Each evidence case owns a fresh PostgreSQL 16 database.

type r22DownEvidenceCase struct {
	id   string
	name string
	seed func(*testing.T, context.Context, *pgxpool.Pool) string
}

func TestVoiceDBMigration_B01_B17_EvidenceRefusesStrictDown(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL strict DOWN contract requires testcontainers")
	}
	cases := []r22DownEvidenceCase{
		{"B01", "active room roster zero", r22SeedDownRoom("active", 0)},
		{"B02", "active room roster positive", r22SeedDownRoom("active", 7)},
		{"B03", "closed room roster zero", r22SeedDownRoom("closed", 0)},
		{"B04", "closed room roster positive", r22SeedDownRoom("closed", 7)},
		{"B05", "active membership", r22SeedDownMembership},
		{"B06", "decided operation", r22SeedDownOperation("decided", false)},
		{"B07", "quarantined operation", r22SeedDownOperation("quarantined", false)},
		{"B08", "completed operation inside replay window", r22SeedDownOperation("completed", false)},
		{"B09", "completed expired binding tombstone", r22SeedDownOperation("completed", true)},
		{"B10", "ready effect", r22SeedDownEffect("ready")},
		{"B11", "applied effect", r22SeedDownEffect("applied")},
		{"B12", "quarantined effect", r22SeedDownEffect("quarantined")},
		{"B13", "media epoch denial before expiry", r22SeedDownDenial(false)},
		{"B14", "expired denial without absence observation", r22SeedDownDenial(true)},
		{"B15", "ready outbox row", r22SeedDownOutbox("ready")},
		{"B16", "delivered outbox row", r22SeedDownOutbox("delivered")},
		{"B17", "quarantined outbox row", r22SeedDownOutbox("quarantined")},
	}
	for _, tc := range cases {
		t.Run(tc.id+" "+tc.name, func(t *testing.T) {
			ctx := context.Background()
			pool := r22StartVoicePostgres(t, ctx, "r22voice"+strings.ToLower(tc.id))
			table := tc.seed(t, ctx, pool)
			r22KeepOnlyEvidenceTable(t, ctx, pool, table)
			require.Equal(t, int64(1), r22TableRowCount(t, ctx, pool, table), "fixture must isolate the requested evidence table")
			beforeSchema := r22VoiceSchemaSnapshot(t, ctx, pool)
			beforeRows := r22TableRowsSnapshot(t, ctx, pool, table)

			downSQL := r22ReadVoiceMigration(t, "down")
			r22ExecuteDownRefusal(t, ctx, pool, downSQL, false)

			require.Equal(t, beforeSchema, r22VoiceSchemaSnapshot(t, ctx, pool))
			require.Equal(t, beforeRows, r22TableRowsSnapshot(t, ctx, pool, table))
		})
	}
}

func r22SeedDownRoom(state string, rosterVersion int64) func(*testing.T, context.Context, *pgxpool.Pool) string {
	return func(t *testing.T, ctx context.Context, pool *pgxpool.Pool) string {
		t.Helper()
		var closedAt any
		if state == "closed" {
			closedAt = time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)
		}
		r22InsertRoom(t, ctx, pool, uuid.New(), uuid.New(), "r22-down-room-"+uuid.NewString(), state, rosterVersion, closedAt)
		return "voice_room_instances"
	}
}

func r22SeedDownMembership(t *testing.T, ctx context.Context, pool *pgxpool.Pool) string {
	t.Helper()
	roomID := r22InsertRoom(t, ctx, pool, uuid.New(), uuid.New(), "r22-down-membership-"+uuid.NewString(), "active", 0, nil)
	r22InsertMembership(t, ctx, pool, uuid.New(), roomID, uuid.New(), r22ValidGrants)
	return "voice_room_memberships"
}

func r22SeedDownOperation(state string, expired bool) func(*testing.T, context.Context, *pgxpool.Pool) string {
	return func(t *testing.T, ctx context.Context, pool *pgxpool.Pool) string {
		t.Helper()
		seed := r22NewIntegritySeed(t, ctx, pool)
		outcome := ""
		if state == "completed" {
			outcome = "joined"
		}
		args := r22OperationArgs(seed, "join", state, outcome)
		if state == "completed" {
			completedAt := time.Now().UTC()
			if expired {
				completedAt = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
			}
			args["completed_at"] = completedAt
			args["replay_until"] = completedAt.Add(24 * time.Hour)
		}
		require.NoError(t, r22InsertOperation(ctx, pool, args))
		return "voice_lifecycle_operations"
	}
}

func r22SeedDownEffect(state string) func(*testing.T, context.Context, *pgxpool.Pool) string {
	return func(t *testing.T, ctx context.Context, pool *pgxpool.Pool) string {
		t.Helper()
		seed := r22NewIntegritySeed(t, ctx, pool)
		parent := r22OperationArgs(seed, "join", "completed", "joined")
		require.NoError(t, r22InsertOperation(ctx, pool, parent))
		args := r22EffectArgs(seed, parent["actor_id"].(uuid.UUID), parent["operation_id"].(uuid.UUID), "livekit_ensure_room", state)
		require.NoError(t, r22InsertEffect(ctx, pool, args))
		return "voice_lifecycle_effects"
	}
}

func r22SeedDownDenial(expired bool) func(*testing.T, context.Context, *pgxpool.Pool) string {
	return func(t *testing.T, ctx context.Context, pool *pgxpool.Pool) string {
		t.Helper()
		seed := r22NewIntegritySeed(t, ctx, pool)
		args := r22DenialArgs(seed, uuid.New(), uuid.New())
		args["actor_id"], args["operation_id"] = nil, nil
		if expired {
			past := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
			args["grant_expires_at"], args["deny_until"] = past.Add(-2*time.Second), past
		} else {
			future := time.Now().UTC().Add(48 * time.Hour)
			args["grant_expires_at"], args["deny_until"] = future, future.Add(2*time.Second)
		}
		require.NoError(t, r22InsertDenial(ctx, pool, args))
		return "voice_media_epoch_denials"
	}
}

func r22SeedDownOutbox(state string) func(*testing.T, context.Context, *pgxpool.Pool) string {
	return func(t *testing.T, ctx context.Context, pool *pgxpool.Pool) string {
		t.Helper()
		seed := r22NewIntegritySeed(t, ctx, pool)
		parent := r22OperationArgs(seed, "join", "completed", "joined")
		require.NoError(t, r22InsertOperation(ctx, pool, parent))
		args := r22OutboxArgs(seed, parent["actor_id"].(uuid.UUID), parent["operation_id"].(uuid.UUID), state)
		require.NoError(t, r22InsertOutbox(ctx, pool, args))
		return "voice_event_outbox"
	}
}

func r22KeepOnlyEvidenceTable(t *testing.T, ctx context.Context, pool *pgxpool.Pool, keep string) {
	t.Helper()
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	require.NoError(t, func() error {
		_, execErr := conn.Exec(ctx, "SET session_replication_role = replica")
		return execErr
	}())
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, cleanupErr := conn.Exec(cleanupCtx, "SET session_replication_role = origin")
		require.NoError(t, cleanupErr)
	}()
	for index := len(r22VoiceTables) - 1; index >= 0; index-- {
		table := r22VoiceTables[index]
		if table == keep {
			continue
		}
		command := "DELETE FROM " + pgx.Identifier{table}.Sanitize()
		_, err = conn.Exec(ctx, command)
		require.NoError(t, err)
	}
}

func r22TableRowCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table string) int64 {
	t.Helper()
	var count int64
	err := pool.QueryRow(ctx, "SELECT count(*) FROM "+pgx.Identifier{table}.Sanitize()).Scan(&count)
	require.NoError(t, err)
	return count
}

func r22TableRowsSnapshot(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table string) []string {
	t.Helper()
	rows, err := pool.Query(ctx, "SELECT to_jsonb(row_data)::text FROM "+pgx.Identifier{table}.Sanitize()+" AS row_data ORDER BY 1")
	require.NoError(t, err)
	defer rows.Close()
	var snapshot []string
	for rows.Next() {
		var row string
		require.NoError(t, rows.Scan(&row))
		snapshot = append(snapshot, row)
	}
	require.NoError(t, rows.Err())
	return snapshot
}

func r22VoiceSchemaSnapshot(t *testing.T, ctx context.Context, pool *pgxpool.Pool) []string {
	t.Helper()
	rows, err := pool.Query(ctx, `
SELECT object_kind,object_name,definition
FROM (
  SELECT 'table'::text AS object_kind,c.relname AS object_name,
         array_to_string(ARRAY(
           SELECT a.attname||':'||format_type(a.atttypid,a.atttypmod)||':'||a.attnotnull::text
           FROM pg_attribute a WHERE a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped
           ORDER BY a.attnum
         ),',') AS definition
  FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
  WHERE n.nspname=current_schema() AND c.relkind='r' AND c.relname=ANY($1)
  UNION ALL
  SELECT 'constraint',r.relname||'.'||c.conname,pg_get_constraintdef(c.oid)
  FROM pg_constraint c JOIN pg_class r ON r.oid=c.conrelid JOIN pg_namespace n ON n.oid=r.relnamespace
  WHERE n.nspname=current_schema() AND r.relname=ANY($1)
  UNION ALL
  SELECT 'index',tablename||'.'||indexname,indexdef
  FROM pg_indexes WHERE schemaname=current_schema() AND tablename=ANY($1)
  UNION ALL
  SELECT 'trigger',r.relname||'.'||trigger.tgname,pg_get_triggerdef(trigger.oid)
  FROM pg_trigger trigger JOIN pg_class r ON r.oid=trigger.tgrelid JOIN pg_namespace n ON n.oid=r.relnamespace
  WHERE n.nspname=current_schema() AND r.relname=ANY($1) AND NOT trigger.tgisinternal
  UNION ALL
  SELECT 'function',p.proname,pg_get_functiondef(p.oid)
  FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
  WHERE n.nspname=current_schema() AND p.proname LIKE 'voice_%_guard_fn'
) objects
ORDER BY object_kind,object_name,definition`, r22VoiceTables)
	require.NoError(t, err)
	defer rows.Close()
	var snapshot []string
	for rows.Next() {
		var kind, name, definition string
		require.NoError(t, rows.Scan(&kind, &name, &definition))
		snapshot = append(snapshot, kind+"|"+name+"|"+definition)
	}
	require.NoError(t, rows.Err())
	return snapshot
}

func r22ExecuteDownRefusal(t *testing.T, ctx context.Context, pool *pgxpool.Pool, downSQL string, proveAborted bool) {
	t.Helper()
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	_, err = conn.Exec(ctx, downSQL)
	r22RequireSQLState(t, err, "55000")
	if proveAborted {
		_, abortedErr := conn.Exec(ctx, "SELECT 1")
		r22RequireSQLState(t, abortedErr, "25P02")
	}
	_, err = conn.Exec(ctx, "ROLLBACK")
	require.NoError(t, err)
	var one int
	require.NoError(t, conn.QueryRow(ctx, "SELECT 1").Scan(&one))
	require.Equal(t, 1, one)
}

func TestVoiceDBMigration_B18_ConcurrentCommitIsRechecked(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL strict DOWN contract requires testcontainers")
	}
	ctx := context.Background()
	pool := r22StartVoicePostgres(t, ctx, "r22voiceb18")
	beforeSchema := r22VoiceSchemaSnapshot(t, ctx, pool)
	writer, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer writer.Release()
	tx, err := writer.Begin(ctx)
	require.NoError(t, err)
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanupCtx)
	}()
	roomID := uuid.New()
	_, err = tx.Exec(ctx, `
INSERT INTO voice_room_instances(room_id,space_id,voice_room_id,livekit_room_name,state,roster_version,created_at,updated_at)
VALUES($1,$2,$3,$4,'active',0,now(),now())`, roomID, uuid.New(), uuid.New(), "r22-down-concurrent-"+uuid.NewString())
	require.NoError(t, err)
	var insertedRow string
	require.NoError(t, tx.QueryRow(ctx, `
SELECT to_jsonb(room_row)::text
FROM voice_room_instances AS room_row
WHERE room_id=$1`, roomID).Scan(&insertedRow))

	downSQL := r22ReadVoiceMigration(t, "down")
	downConn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer downConn.Release()
	var downPID int
	require.NoError(t, downConn.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&downPID))
	result := make(chan error, 1)
	go func() {
		_, execErr := downConn.Exec(ctx, downSQL)
		result <- execErr
	}()
	r22WaitForBackendLock(t, ctx, pool, downPID)
	require.NoError(t, tx.Commit(ctx))
	downErr := <-result
	r22RequireSQLState(t, downErr, "55000")
	_, err = downConn.Exec(ctx, "ROLLBACK")
	require.NoError(t, err)
	require.Equal(t, beforeSchema, r22VoiceSchemaSnapshot(t, ctx, pool))
	var persistedRow string
	require.NoError(t, pool.QueryRow(ctx, `
SELECT to_jsonb(room_row)::text
FROM voice_room_instances AS room_row
WHERE room_id=$1`, roomID).Scan(&persistedRow))
	require.Equal(t, insertedRow, persistedRow)
	require.Equal(t, []string{insertedRow}, r22TableRowsSnapshot(t, ctx, pool, "voice_room_instances"))
}

func r22WaitForBackendLock(t *testing.T, ctx context.Context, pool *pgxpool.Pool, backendPID int) {
	t.Helper()
	waitCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var waiting bool
		err := pool.QueryRow(waitCtx, `
SELECT COALESCE(wait_event_type='Lock',false)
FROM pg_stat_activity WHERE pid=$1`, backendPID).Scan(&waiting)
		require.NoError(t, err)
		if waiting {
			return
		}
		select {
		case <-waitCtx.Done():
			t.Fatal("DOWN did not reach the room ACCESS EXCLUSIVE lock")
		case <-ticker.C:
		}
	}
}

func TestVoiceDBMigration_B19_EmptyDownRoundTripsWithoutCollateralDamage(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL strict DOWN contract requires testcontainers")
	}
	ctx := context.Background()
	pool := r22StartVoicePostgres(t, ctx, "r22voiceb19")
	_, err := pool.Exec(ctx, `CREATE TABLE r22_down_sentinel(id integer PRIMARY KEY, payload text NOT NULL)`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO r22_down_sentinel(id,payload) VALUES(1,'preserve-me')`)
	require.NoError(t, err)
	downSQL := r22ReadVoiceMigration(t, "down")
	r22RequireDownTextContract(t, downSQL)

	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, downSQL)
	require.NoError(t, err)
	conn.Release()
	require.Empty(t, r22VoiceSchemaSnapshot(t, ctx, pool))
	var payload string
	require.NoError(t, pool.QueryRow(ctx, `SELECT payload FROM r22_down_sentinel WHERE id=1`).Scan(&payload))
	require.Equal(t, "preserve-me", payload)

	upSQL := r22ReadVoiceMigration(t, "up")
	_, err = pool.Exec(ctx, upSQL)
	require.NoError(t, err)
	for _, table := range r22VoiceTables {
		var exists bool
		require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, table).Scan(&exists))
		require.True(t, exists, "%s must exist after UP reapplies", table)
	}
	require.NoError(t, pool.QueryRow(ctx, `SELECT payload FROM r22_down_sentinel WHERE id=1`).Scan(&payload))
	require.Equal(t, "preserve-me", payload)
}

func r22RequireDownTextContract(t *testing.T, downSQL string) {
	t.Helper()
	normalized := strings.ToLower(strings.Join(strings.Fields(downSQL), " "))
	require.True(t, strings.HasPrefix(normalized, "begin;"))
	require.True(t, strings.HasSuffix(normalized, "commit;"))
	require.NotContains(t, normalized, "cascade")
	lockOrder := []string{
		"lock table voice_lifecycle_operations in access exclusive mode",
		"lock table voice_room_memberships in access exclusive mode",
		"lock table voice_room_instances in access exclusive mode",
		"lock table voice_lifecycle_effects in access exclusive mode",
		"lock table voice_media_epoch_denials in access exclusive mode",
		"lock table voice_event_outbox in access exclusive mode",
	}
	previous := -1
	for _, lock := range lockOrder {
		position := strings.Index(normalized, lock)
		require.Greater(t, position, previous, "lock order must match plan lines 532-541")
		previous = position
	}
	require.Equal(t, 1, strings.Count(normalized, "do $$"), "one guard must recheck every table after all locks")
	for _, table := range r22VoiceTables {
		require.Contains(t, normalized, "exists (select 1 from "+table+")")
	}
	require.Contains(t, normalized, "errcode = '55000'")

	triggerDrops := []string{
		"drop trigger voice_room_instance_immutable_guard on voice_room_instances;",
		"drop trigger voice_room_membership_generation_guard on voice_room_memberships;",
		"drop trigger voice_lifecycle_operation_immutable_guard on voice_lifecycle_operations;",
		"drop trigger voice_lifecycle_operation_transition_guard on voice_lifecycle_operations;",
		"drop trigger voice_lifecycle_effect_immutable_guard on voice_lifecycle_effects;",
		"drop trigger voice_lifecycle_effect_transition_guard on voice_lifecycle_effects;",
		"drop trigger voice_media_epoch_denial_update_guard on voice_media_epoch_denials;",
		"drop trigger voice_media_epoch_denial_delete_guard on voice_media_epoch_denials;",
		"drop trigger voice_event_outbox_immutable_guard on voice_event_outbox;",
		"drop trigger voice_event_outbox_transition_guard on voice_event_outbox;",
	}
	functionDrops := []string{
		"drop function voice_room_instance_immutable_guard_fn();",
		"drop function voice_room_membership_generation_guard_fn();",
		"drop function voice_lifecycle_operation_immutable_guard_fn();",
		"drop function voice_lifecycle_operation_transition_guard_fn();",
		"drop function voice_lifecycle_effect_immutable_guard_fn();",
		"drop function voice_lifecycle_effect_transition_guard_fn();",
		"drop function voice_media_epoch_denial_update_guard_fn();",
		"drop function voice_media_epoch_denial_delete_guard_fn();",
		"drop function voice_event_outbox_immutable_guard_fn();",
		"drop function voice_event_outbox_transition_guard_fn();",
	}
	tableDrops := []string{
		"drop table voice_lifecycle_effects;",
		"drop table voice_media_epoch_denials;",
		"drop table voice_event_outbox;",
		"drop table voice_room_memberships;",
		"drop table voice_lifecycle_operations;",
		"drop table voice_room_instances;",
	}

	require.Equal(t, len(triggerDrops), strings.Count(normalized, "drop trigger "))
	require.Equal(t, len(functionDrops), strings.Count(normalized, "drop function "))
	require.Equal(t, len(tableDrops), strings.Count(normalized, "drop table "))
	triggerPositions := r22RequireExactDropStatements(t, normalized, triggerDrops)
	functionPositions := r22RequireExactDropStatements(t, normalized, functionDrops)
	tablePositions := r22RequireExactDropStatements(t, normalized, tableDrops)
	require.Less(t, r22MaxPosition(triggerPositions), r22MinPosition(functionPositions), "all triggers must be dropped before functions")
	require.Less(t, r22MaxPosition(functionPositions), r22MinPosition(tablePositions), "all functions must be dropped before tables")
	for index := 1; index < len(tablePositions); index++ {
		require.Less(t, tablePositions[index-1], tablePositions[index], "tables must be dropped child-before-parent in FK-safe order")
	}
}

func r22RequireExactDropStatements(t *testing.T, normalized string, statements []string) []int {
	t.Helper()
	positions := make([]int, 0, len(statements))
	for _, statement := range statements {
		require.Equal(t, 1, strings.Count(normalized, statement), "%q must appear exactly once", statement)
		positions = append(positions, strings.Index(normalized, statement))
	}
	return positions
}

func r22MinPosition(positions []int) int {
	minimum := positions[0]
	for _, position := range positions[1:] {
		if position < minimum {
			minimum = position
		}
	}
	return minimum
}

func r22MaxPosition(positions []int) int {
	maximum := positions[0]
	for _, position := range positions[1:] {
		if position > maximum {
			maximum = position
		}
	}
	return maximum
}

func TestVoiceDBMigration_B20_RefusalNeedsRollbackAndPreservesEverything(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL strict DOWN contract requires testcontainers")
	}
	ctx := context.Background()
	pool := r22StartVoicePostgres(t, ctx, "r22voiceb20")
	r22SeedDownRoom("active", 0)(t, ctx, pool)
	beforeSchema := r22VoiceSchemaSnapshot(t, ctx, pool)
	beforeRows := r22TableRowsSnapshot(t, ctx, pool, "voice_room_instances")
	downSQL := r22ReadVoiceMigration(t, "down")
	r22ExecuteDownRefusal(t, ctx, pool, downSQL, true)
	require.Equal(t, beforeSchema, r22VoiceSchemaSnapshot(t, ctx, pool))
	require.Equal(t, beforeRows, r22TableRowsSnapshot(t, ctx, pool, "voice_room_instances"))
}
