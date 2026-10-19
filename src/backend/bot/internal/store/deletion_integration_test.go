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
	"github.com/stretchr/testify/require"
	"voice/backend/bot/internal/store"
)

func TestBotLifecycleDownRejectsOldRepeatableReadSnapshot(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL retained authority under old snapshot")
	}
	for _, name := range []string{"000008_installation_intents.down.sql", "000009_deletion_operations.down.sql"} {
		t.Run(name, func(t *testing.T) {
			st := startBotStore(t)
			ctx := t.Context()
			bot, _, err := st.CreateBot(ctx, uuid.New(), "Old snapshot", "", `[]`, uuid.New())
			require.NoError(t, err)
			table := "bot_installation_intents"
			if name == "000009_deletion_operations.down.sql" {
				table = "bot_deletion_operations"
			}
			tx, err := st.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
			require.NoError(t, err)
			defer func() { _ = tx.Rollback(context.Background()) }()
			var count int
			require.NoError(t, tx.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&count))
			require.Zero(t, count)
			if table == "bot_installation_intents" {
				_, err = st.PrepareInstallation(ctx, bot.ID, uuid.New(), uuid.New(), nil)
			} else {
				_, err = st.BeginBotDeletion(ctx, bot.ID, bot.OwnerAccountID)
			}
			require.NoError(t, err)
			_, file, _, _ := runtime.Caller(0)
			down, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations", "bot_db", name))
			require.NoError(t, err)
			_, downErr := tx.Exec(ctx, string(down))
			if downErr == nil {
				// Old DOWN commits its DROP. Observe real evidence loss rather than
				// accepting a mock lock or a rollback-only demonstration.
				var present bool
				require.NoError(t, st.Pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, table).Scan(&present))
				require.True(t, present, "old snapshot DOWN erased retained authority")
			}
			require.ErrorContains(t, downErr, "READ COMMITTED")
			require.NoError(t, tx.Rollback(ctx))
			require.NoError(t, st.Pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&count))
			require.Equal(t, 1, count)
		})
	}
}

func TestBotLifecycleDownAcceptsEmptyDefaultReadCommitted(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL default isolation control")
	}
	for _, name := range []string{"000008_installation_intents.down.sql", "000009_deletion_operations.down.sql"} {
		t.Run(name, func(t *testing.T) {
			st := startBotStore(t)
			_, file, _, _ := runtime.Caller(0)
			down, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations", "bot_db", name))
			require.NoError(t, err)
			_, err = st.Pool.Exec(t.Context(), string(down))
			require.NoError(t, err, "empty authority can roll back under normal default READ COMMITTED")
		})
	}
}

func TestBotDeletionAtomicRollbackConcurrentReplayAndTerminalAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL deletion journal")
	}
	st := startBotStore(t)
	ctx := context.Background()
	owner, installer, space, chat := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	bot, _, err := st.CreateBot(ctx, owner, "Delete", "", `[]`, uuid.New())
	require.NoError(t, err)
	intent, err := st.PrepareInstallation(ctx, bot.ID, space, installer, []uuid.UUID{chat})
	require.NoError(t, err)
	require.NoError(t, st.CompleteInstallation(ctx, intent, []uuid.UUID{chat}))
	// Force failure after the root INSERT, before mutable cleanup. PostgreSQL
	// must roll the whole operation back rather than disable and lose inputs.
	_, err = st.Pool.Exec(ctx, `ALTER TABLE bot_deletion_installations ADD CONSTRAINT injected_snapshot_failure CHECK(false)`)
	require.NoError(t, err)
	_, err = st.BeginBotDeletion(ctx, bot.ID, owner)
	require.Error(t, err)
	live, err := st.GetBotByID(ctx, bot.ID)
	require.NoError(t, err)
	require.Equal(t, "live", live.Status)
	var count int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_deletion_operations WHERE bot_id=$1`, bot.ID).Scan(&count))
	require.Zero(t, count)
	allowed, err := st.IsChatWhitelisted(ctx, bot.ID, chat)
	require.NoError(t, err)
	require.True(t, allowed)
	_, err = st.Pool.Exec(ctx, `ALTER TABLE bot_deletion_installations DROP CONSTRAINT injected_snapshot_failure`)
	require.NoError(t, err)
	type result struct {
		op  *store.BotDeletion
		err error
	}
	results := make(chan result, 8)
	for n := 0; n < 8; n++ {
		go func() { d, e := st.BeginBotDeletion(ctx, bot.ID, owner); results <- result{d, e} }()
	}
	var first *store.BotDeletion
	for n := 0; n < 8; n++ {
		r := <-results
		require.NoError(t, r.err)
		if first == nil {
			first = r.op
		}
		require.Equal(t, first, r.op)
	}
	require.Equal(t, "PENDING", first.State)
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_deletion_effects e JOIN bot_deletion_installations i ON i.target_id=e.target_id WHERE i.bot_id=$1`, bot.ID).Scan(&count))
	require.Equal(t, 3, count)
	_, err = st.BeginBotDeletion(ctx, bot.ID, uuid.New())
	require.ErrorIs(t, err, store.ErrDeletionOwner)
	require.ErrorIs(t, st.CompleteInstallation(ctx, intent, []uuid.UUID{chat}), store.ErrInstallationConflict)
	require.ErrorIs(t, st.SetChatEnabled(ctx, bot.ID, chat, space, installer, true), store.ErrInstallationConflict)
	_, err = st.InstallInSpace(ctx, bot.ID, uuid.New(), installer, nil)
	require.ErrorIs(t, err, store.ErrInstallationConflict)
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_chat_whitelist WHERE bot_id=$1`, bot.ID).Scan(&count))
	require.Zero(t, count)
}

func TestBotDeletionRestartsChangedSpaceSetBeforeBotLock(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL ordering")
	}
	st := startBotStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	owner, space, installer := uuid.New(), uuid.New(), uuid.New()
	bot, _, err := st.CreateBot(ctx, owner, "Discovery", "", `[]`, uuid.New())
	require.NoError(t, err)
	blocker, err := st.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback(context.Background()) }()
	var id uuid.UUID
	require.NoError(t, blocker.QueryRow(ctx, `SELECT id FROM bots WHERE id=$1 FOR UPDATE`, bot.ID).Scan(&id))
	done := make(chan error, 1)
	go func() { _, e := st.BeginBotDeletion(ctx, bot.ID, owner); done <- e }()
	require.Eventually(t, func() bool {
		var waiting bool
		e := st.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%FROM bots WHERE id = $1 FOR UPDATE%')`).Scan(&waiting)
		return e == nil && waiting
	}, 5*time.Second, 10*time.Millisecond)
	// A writer committed a previously unseen Space before Delete obtained Bot.
	// It must be rediscovered and guarded by a restarted transaction.
	actual := uuid.New()
	_, err = blocker.Exec(ctx, `INSERT INTO bot_space_installations(id,bot_id,space_id,installed_by_profile_id) VALUES($1,$2,$3,$4)`, actual, bot.ID, space, installer)
	require.NoError(t, err)
	require.NoError(t, blocker.Commit(ctx))
	require.NoError(t, <-done)
	var captured uuid.UUID
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT installation_id FROM bot_deletion_installations WHERE bot_id=$1 AND space_id=$2`, bot.ID, space).Scan(&captured))
	require.Equal(t, actual, captured)
	var legacy bool
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT legacy_evidence FROM bot_deletion_installations WHERE bot_id=$1`, bot.ID).Scan(&legacy))
	require.True(t, legacy)
}

func TestBotDeletionDownFirstWriterRetainsTerminalDecision(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL two-connection boundary")
	}
	st := startBotStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	bot, _, err := st.CreateBot(ctx, uuid.New(), "Down", "", `[]`, uuid.New())
	require.NoError(t, err)
	writer, err := st.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = writer.Rollback(context.Background()) }()
	op := uuid.New()
	_, err = writer.Exec(ctx, `INSERT INTO bot_deletion_operations(operation_id,bot_id,owner_account_id,actor_profile_id,decision_bytes,decision_sha256,state) VALUES($1,$2,$3,$4,$5,$6,'PENDING')`, op, bot.ID, bot.OwnerAccountID, bot.ActorProfileID, []byte("committed-authority"), make([]byte, 32))
	require.NoError(t, err)
	_, file, _, _ := runtime.Caller(0)
	down, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations", "bot_db", "000009_deletion_operations.down.sql"))
	require.NoError(t, err)
	conn, err := st.Pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	var pid int32
	require.NoError(t, conn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid))
	done := make(chan error, 1)
	go func() { _, e := conn.Exec(ctx, string(down)); done <- e }()
	require.Eventually(t, func() bool {
		var waiting bool
		e := st.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE pid=$1 AND wait_event_type='Lock')`, pid).Scan(&waiting)
		return e == nil && waiting
	}, 5*time.Second, 10*time.Millisecond)
	require.NoError(t, writer.Commit(ctx))
	require.ErrorContains(t, <-done, "must not be dropped")
	_, err = conn.Exec(ctx, `ROLLBACK`)
	require.NoError(t, err)
	var actual uuid.UUID
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT operation_id FROM bot_deletion_operations WHERE bot_id=$1`, bot.ID).Scan(&actual))
	require.Equal(t, op, actual)
}
