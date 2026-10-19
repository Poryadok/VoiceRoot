package grpcsvc

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"voice/backend/moderation/internal/automod"
	"voice/backend/moderation/internal/store"
)

func TestAutoModDeletionCycleBindingCannotChangeAtSameRevision(t *testing.T) {
	if testing.Short() {
		t.Skip("requires disposable PostgreSQL")
	}
	ctx := context.Background()
	pool := automodAdmissionDatabase(t)
	integrationtestApplyAutoMod(t, ctx, pool)
	s := automod.NewStore(pool)
	locks := &store.SanctionStore{Pool: pool}
	deadline := time.Now().Add(time.Hour).Truncate(time.Microsecond)
	binding := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Revision: 2, State: "DELETED", PurgeAt: &deadline}
	binding.CycleID = uuid.New()
	bind := func(subject automod.Subject) error {
		return locks.WithAccountLock(ctx, subject.AccountID, func(tx pgx.Tx) error { return s.BindRetainedSubjectTx(ctx, tx, subject) })
	}
	require.NoError(t, bind(binding))
	changed := binding
	changed.CycleID = uuid.New()
	require.ErrorIs(t, bind(changed), automod.ErrConflict, "same revision and deadline cannot authorize a different deletion cycle")
	require.ErrorIs(t, locks.WithAccountLock(ctx, binding.AccountID, func(tx pgx.Tx) error { return s.RequireRetainedSubjectTx(ctx, tx, changed) }), automod.ErrSubject)
	missing := binding
	missing.CycleID = uuid.Nil
	require.ErrorIs(t, bind(missing), automod.ErrSubject, "DELETED authority without its exact cycle is unavailable")
	imprecise := binding
	nanos := deadline.Add(time.Nanosecond)
	imprecise.PurgeAt = &nanos
	require.ErrorIs(t, bind(imprecise), automod.ErrSubject)
	extended := binding
	extended.Revision++
	later := deadline.Add(time.Hour)
	extended.PurgeAt = &later
	require.ErrorIs(t, bind(extended), automod.ErrConflict, "even a newer tuple cannot extend the same deletion cycle")
	_, err := pool.Exec(ctx, `UPDATE automod_subjects SET lifecycle_revision=lifecycle_revision+1,purge_at=$2 WHERE subject_id=$1`, binding.ID, later)
	require.ErrorContains(t, err, "automod_cycle_deadline_immutable")
	// Source provisioning and refresh must preserve the same equality rule.
	binding.Epoch = uuid.New()
	require.NoError(t, s.Provision(ctx, binding))
	changed.Epoch = binding.Epoch
	require.ErrorIs(t, s.Provision(ctx, changed), automod.ErrConflict)
	require.ErrorIs(t, s.RefreshSubject(ctx, changed), automod.ErrSubject)
	extended.Epoch = binding.Epoch
	require.ErrorIs(t, s.Provision(ctx, extended), automod.ErrConflict)
	require.ErrorIs(t, s.RefreshSubject(ctx, extended), automod.ErrSubject)
	var cycle uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT deletion_cycle_id FROM automod_subjects WHERE subject_id=$1`, binding.ID).Scan(&cycle))
	require.Equal(t, binding.CycleID, cycle)
	live := binding
	live.Revision++
	live.State, live.PurgeAt, live.CycleID = "LIVE", nil, uuid.Nil
	invalidLive := live
	invalidLive.CycleID = binding.CycleID
	require.ErrorIs(t, bind(invalidLive), automod.ErrSubject)
	require.NoError(t, s.RefreshSubject(ctx, live))
	require.ErrorIs(t, bind(binding), automod.ErrConflict, "old cycle cannot override a newer restore")
	var absent bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT deletion_cycle_id IS NULL AND source_epoch=$2 FROM automod_subjects WHERE subject_id=$1`, binding.ID, binding.Epoch).Scan(&absent))
	require.True(t, absent, "restore clears the cycle and preserves the existing source epoch")
	// A second deletion has its own exact cycle; no implicit UUID or deadline.
	second := binding
	second.Revision = 4
	second.CycleID = uuid.New()
	require.NoError(t, s.RefreshSubject(ctx, second))
	_, err = pool.Exec(ctx, mustReadCycleDown(t))
	require.ErrorContains(t, err, "cannot discard current deletion cycle authority")
}

func mustReadCycleDown(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/moderation_db/000009_subject_deletion_cycle.down.sql"))
	require.NoError(t, err)
	return string(data)
}

func TestAutoModCycleDownPreservesConcurrentFirstCycle(t *testing.T) {
	if testing.Short() {
		t.Skip("requires disposable PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool := automodAdmissionDatabase(t)
	integrationtestApplyAutoMod(t, ctx, pool)
	subject, cycle := uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO automod_subjects(subject_id,account_id,lifecycle_revision,lifecycle_state) VALUES($1,$2,1,'LIVE')`, subject, uuid.New())
	require.NoError(t, err)
	writer, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = writer.Rollback(context.Background()) }()
	_, err = writer.Exec(ctx, `UPDATE automod_subjects SET lifecycle_state='DELETED',lifecycle_revision=2,purge_at=clock_timestamp()+interval '1 hour',deletion_cycle_id=$2 WHERE subject_id=$1`, subject, cycle)
	require.NoError(t, err)
	down, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer down.Release()
	defer func() { _, _ = down.Exec(context.Background(), "ROLLBACK") }()
	var pid int32
	require.NoError(t, down.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&pid))
	result := make(chan error, 1)
	downSQL := mustReadCycleDown(t)
	go func() {
		_, downErr := down.Exec(ctx, downSQL)
		result <- downErr
	}()
	// With the old guard, DROP waits only after the stale emptiness check.
	// The corrected transaction waits before that check, then sees the commit.
	require.Eventually(t, func() bool {
		var waiting bool
		err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE pid=$1 AND wait_event_type='Lock')`, pid).Scan(&waiting)
		return err == nil && waiting
	}, 10*time.Second, 10*time.Millisecond)
	require.NoError(t, writer.Commit(ctx))
	require.ErrorContains(t, <-result, "cannot discard current deletion cycle authority")
	_, err = down.Exec(ctx, "ROLLBACK")
	require.NoError(t, err)
	var preserved uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT deletion_cycle_id FROM automod_subjects WHERE subject_id=$1`, subject).Scan(&preserved))
	require.Equal(t, cycle, preserved)
	var guarded bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid='automod_subjects'::regclass AND tgname='automod_cycle_deadline_guard')`).Scan(&guarded))
	require.True(t, guarded)
}

func TestAutoModCycleDownAllowsEmptyAuthority(t *testing.T) {
	if testing.Short() {
		t.Skip("requires disposable PostgreSQL")
	}
	ctx := context.Background()
	pool := automodAdmissionDatabase(t)
	integrationtestApplyAutoMod(t, ctx, pool)
	_, err := pool.Exec(ctx, mustReadCycleDown(t))
	require.NoError(t, err)
	var present bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name='automod_subjects' AND column_name='deletion_cycle_id')`).Scan(&present))
	require.False(t, present)
}
