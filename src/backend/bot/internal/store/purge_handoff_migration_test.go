package store_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
	commonv1 "voice.app/voice/common/v1"
	"voice/backend/bot/internal/store"
)

func purgeHandoffMigration(t *testing.T, direction string) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	b, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations", "bot_db", "000015_purge_handoff."+direction+".sql"))
	require.NoError(t, err)
	return string(b)
}

func preparePurgeHandoffDecision(t *testing.T, st *store.BotStore, space uuid.UUID) *commonv1.SpacePurgeRequest {
	t.Helper()
	manifest := &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: make([]byte, 32)}
	fence := &commonv1.SpaceLifecycleFenceRequest{ProtocolVersion: 1, SpaceId: space.String(), DeletionOperationId: uuid.NewString(), Generation: 1, DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: manifest}
	_, err := st.ApplySpaceLifecycleFence(t.Context(), fence)
	require.NoError(t, err)
	fence.Generation = 2
	fence.DesiredState = commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED
	_, err = st.ApplySpaceLifecycleFence(t.Context(), fence)
	require.NoError(t, err)
	return &commonv1.SpacePurgeRequest{ProtocolVersion: 1, SpaceId: space.String(), DeletionOperationId: fence.DeletionOperationId, Generation: 2, ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_BOT, PurgeDecidedAt: timestamppb.Now(), Manifest: manifest}
}

func TestBotPurgeHandoffDownIsolationAndRetainedWork(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL handoff migration isolation")
	}
	t.Run("empty default RC and restored relation", func(t *testing.T) {
		st := startBotStore(t)
		_, err := st.Pool.Exec(t.Context(), purgeHandoffMigration(t, "down"))
		require.NoError(t, err)
		var restored bool
		require.NoError(t, st.Pool.QueryRow(t.Context(), `SELECT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='bot_uninstall_operations_installation_id_fkey')`).Scan(&restored))
		require.True(t, restored)
		_, err = st.Pool.Exec(t.Context(), purgeHandoffMigration(t, "up"))
		require.NoError(t, err)
	})
	t.Run("old RR and fresh RC preserve later handoff", func(t *testing.T) {
		st := startBotStore(t)
		ctx := t.Context()
		tx, err := st.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(context.Background()) }()
		var count int
		require.NoError(t, tx.QueryRow(ctx, `SELECT count(*) FROM bot_space_purge_handoffs`).Scan(&count))
		require.Zero(t, count)
		// This row is only a retained-data migration oracle, not cleanup proof.
		_, err = st.Pool.Exec(ctx, `INSERT INTO bot_space_purge_handoffs(space_id,deletion_operation_id,generation,request_bytes,request_sha256,state,source_count) VALUES($1,$2,2,'input',decode(repeat('00',32),'hex'),'PENDING',1)`, uuid.New(), uuid.New())
		require.NoError(t, err)
		_, err = tx.Exec(ctx, purgeHandoffMigration(t, "down"))
		require.ErrorContains(t, err, "READ COMMITTED")
		require.NoError(t, tx.Rollback(ctx))
		_, err = st.Pool.Exec(ctx, purgeHandoffMigration(t, "down"))
		require.ErrorContains(t, err, "retained Bot purge handoff")
		require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_space_purge_handoffs`).Scan(&count))
		require.Equal(t, 1, count)
	})
}

func TestBotPurgeHandoffDownWaitsForActualFirstPurgeWriter(t *testing.T) {
	if testing.Short() {
		t.Skip("real first PurgeSpace writer versus DOWN")
	}
	st := startBotLifecycleStore(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	space, actor := uuid.New(), uuid.New()
	bot, _, err := st.CreateBot(ctx, uuid.New(), "First P3 writer", "", `[]`, actor)
	require.NoError(t, err)
	_, err = st.PrepareInstallation(ctx, bot.ID, space, actor, nil)
	require.NoError(t, err)
	purge := preparePurgeHandoffDecision(t, st, space)
	_, err = st.Pool.Exec(ctx, `CREATE FUNCTION hold_purge_handoff_first_writer() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_advisory_xact_lock(247015); RETURN NEW; END $$; CREATE TRIGGER purge_handoff_first_writer BEFORE INSERT ON bot_space_purge_handoffs FOR EACH ROW EXECUTE FUNCTION hold_purge_handoff_first_writer()`)
	require.NoError(t, err)
	blocker, err := st.Pool.Acquire(ctx)
	require.NoError(t, err)
	defer blocker.Release()
	_, err = blocker.Exec(ctx, `SELECT pg_advisory_lock(247015)`)
	require.NoError(t, err)
	defer func() { _, _ = blocker.Exec(context.Background(), `SELECT pg_advisory_unlock(247015)`) }()
	written := make(chan error, 1)
	go func() { _, e := st.PurgeSpace(ctx, purge); written <- e }()
	require.Eventually(t, func() bool {
		var waiting bool
		return st.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE locktype='advisory' AND NOT granted)`).Scan(&waiting) == nil && waiting
	}, 5*time.Second, 10*time.Millisecond)
	downSQL := purgeHandoffMigration(t, "down")
	down := make(chan error, 1)
	go func() { _, e := st.Pool.Exec(ctx, downSQL); down <- e }()
	require.Eventually(t, func() bool {
		var waiting bool
		return st.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE relation='bots'::regclass AND mode='AccessExclusiveLock' AND NOT granted)`).Scan(&waiting) == nil && waiting
	}, 5*time.Second, 10*time.Millisecond)
	_, err = blocker.Exec(ctx, `SELECT pg_advisory_unlock(247015)`)
	require.NoError(t, err)
	require.ErrorIs(t, <-written, store.ErrDeletionPending)
	require.ErrorContains(t, <-down, "retained Bot purge handoff")
	var count int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_space_purge_sources WHERE space_id=$1 AND bot_id=$2`, space, bot.ID).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_space_lifecycle_purge_receipts WHERE space_id=$1`, space).Scan(&count))
	require.Zero(t, count)
}

func TestBotPurgeHandoffInvokerGuardActualNonownerLogin(t *testing.T) {
	if testing.Short() {
		t.Skip("own nonowner PostgreSQL INVOKER trigger qualification")
	}
	admin := startBotLifecycleStore(t)
	ctx := t.Context()
	const login = "bot_handoff_application"
	const password = "isolated-test-only-password"
	_, err := admin.Pool.Exec(ctx, `CREATE ROLE bot_handoff_application LOGIN PASSWORD 'isolated-test-only-password'; GRANT CONNECT ON DATABASE botdb TO bot_handoff_application; GRANT USAGE ON SCHEMA public TO bot_handoff_application; GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA public TO bot_handoff_application`)
	require.NoError(t, err)
	cfg := admin.Pool.Config().Copy()
	cfg.ConnConfig.User, cfg.ConnConfig.Password = login, password
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	defer pool.Close()
	var actual string
	var superuser, executable, definer bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT session_user,rolsuper FROM pg_roles WHERE rolname=session_user`).Scan(&actual, &superuser))
	require.Equal(t, login, actual)
	require.False(t, superuser)
	require.NoError(t, pool.QueryRow(ctx, `SELECT has_function_privilege(current_user,'public.bot_require_prepared_purge_handoff()','EXECUTE'),prosecdef FROM pg_proc WHERE oid='public.bot_require_prepared_purge_handoff()'::regprocedure`).Scan(&executable, &definer))
	require.False(t, executable, "no application EXECUTE grant is needed for automatic trigger invocation")
	require.False(t, definer)
	st := &store.BotStore{Pool: pool}
	purge := preparePurgeHandoffDecision(t, st, uuid.New())
	receipt, err := st.PurgeSpace(ctx, purge)
	require.NoError(t, err)
	require.Equal(t, commonv1.PurgeReceiptState_PURGE_RECEIPT_STATE_COMPLETED, receipt.State)
	_, err = pool.Exec(ctx, `ALTER FUNCTION public.bot_require_prepared_purge_handoff() SECURITY DEFINER`)
	var pgerr *pgconn.PgError
	require.ErrorAs(t, err, &pgerr)
	require.Equal(t, "42501", pgerr.Code)
	_, err = pool.Exec(ctx, `INSERT INTO bot_space_lifecycle_purge_receipts(space_id,deletion_operation_id,generation,request_bytes,request_sha256,receipt_bytes) SELECT $1,$2,generation,request_bytes,request_sha256,receipt_bytes FROM bot_space_lifecycle_purge_receipts LIMIT 1`, uuid.New(), uuid.New())
	require.ErrorAs(t, err, &pgerr)
	require.Equal(t, "P0001", pgerr.Code)
	require.Contains(t, pgerr.Message, "prepared and covered handoff")
}
