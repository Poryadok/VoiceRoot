package grpcsvc

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"path/filepath"
	"testing"
	"time"
	"voice/backend/moderation/internal/automod"
	"voice/backend/moderation/internal/store"
)

func TestAutoModRetainedSubjectDeadlineRejectsBeforeBinding(t *testing.T) {
	if testing.Short() {
		t.Skip("requires disposable PostgreSQL")
	}
	ctx := context.Background()
	pool := automodAdmissionDatabase(t)
	integrationtestApplySQL(t, ctx, pool, filepath.Join(repoRoot(t), "src/backend/migrations/moderation_db/000006_async_automod.up.sql"))
	s := automod.NewStore(pool)
	locks := &store.SanctionStore{Pool: pool}
	deadline := time.Now().Add(-time.Second).Truncate(time.Microsecond)
	binding := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Revision: 2, State: "DELETED", PurgeAt: &deadline}
	err := locks.WithAccountLock(ctx, binding.AccountID, func(tx pgx.Tx) error { return s.BindRetainedSubjectTx(ctx, tx, binding) })
	require.ErrorIs(t, err, automod.ErrSubject, "expired privacy binding must not recreate the raw account mapping or assert a permanent terminal")
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_subjects WHERE subject_id=$1`, binding.ID).Scan(&count))
	require.Zero(t, count)
}

func TestAutoModRetainedSubjectIsPrivacyOnlyAndRechecksDatabaseDeadline(t *testing.T) {
	if testing.Short() {
		t.Skip("requires disposable PostgreSQL")
	}
	ctx := context.Background()
	pool := automodAdmissionDatabase(t)
	integrationtestApplySQL(t, ctx, pool, filepath.Join(repoRoot(t), "src/backend/migrations/moderation_db/000006_async_automod.up.sql"))
	s := automod.NewStore(pool)
	locks := &store.SanctionStore{Pool: pool}
	var deadline time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT clock_timestamp()+interval '1 hour'`).Scan(&deadline))
	binding := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Revision: 2, State: "DELETED", PurgeAt: &deadline}
	require.NoError(t, locks.WithAccountLock(ctx, binding.AccountID, func(tx pgx.Tx) error {
		if err := s.BindRetainedSubjectTx(ctx, tx, binding); err != nil {
			return err
		}
		require.ErrorIs(t, s.RequireLiveSubjectTx(ctx, tx, binding.AccountID, binding.ID, binding.Revision), automod.ErrSubject, "privacy recovery must not grant live-send/report activation authority")
		return s.RequireRetainedSubjectTx(ctx, tx, binding)
	}))
	stale := binding
	stale.Revision--
	require.ErrorIs(t, locks.WithAccountLock(ctx, binding.AccountID, func(tx pgx.Tx) error { return s.BindRetainedSubjectTx(ctx, tx, stale) }), automod.ErrConflict)
	changed := binding
	later := deadline.Add(time.Hour)
	changed.PurgeAt = &later
	require.ErrorIs(t, locks.WithAccountLock(ctx, binding.AccountID, func(tx pgx.Tx) error { return s.BindRetainedSubjectTx(ctx, tx, changed) }), automod.ErrConflict)
	live := binding
	live.State = "LIVE"
	live.PurgeAt = nil
	live.Revision++
	require.NoError(t, s.BindSubject(ctx, live))
	require.ErrorIs(t, locks.WithAccountLock(ctx, binding.AccountID, func(tx pgx.Tx) error { return s.BindRetainedSubjectTx(ctx, tx, binding) }), automod.ErrConflict, "older DELETED cannot replace a newer LIVE restore")
	expired := binding
	past := time.Now().Add(-time.Second)
	expired.PurgeAt = &past
	require.ErrorIs(t, locks.WithAccountLock(ctx, binding.AccountID, func(tx pgx.Tx) error { return s.BindRetainedSubjectTx(ctx, tx, expired) }), automod.ErrSubject, "expired captured authority is not a terminal disposition")
	var state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT lifecycle_state FROM automod_subjects WHERE subject_id=$1`, binding.ID).Scan(&state))
	require.Equal(t, "LIVE", state)
	require.NoError(t, s.Purge(ctx, binding.ID))
	require.ErrorIs(t, locks.WithAccountLock(ctx, binding.AccountID, func(tx pgx.Tx) error { return s.BindRetainedSubjectTx(ctx, tx, binding) }), automod.ErrPurged)
	// Time is checked again after intervening work, using the DB clock rather
	// than the transaction-start timestamp or a captured host-clock decision.
	require.NoError(t, pool.QueryRow(ctx, `SELECT clock_timestamp()+interval '0.25 second'`).Scan(&deadline))
	binding.ID = uuid.New()
	binding.AccountID = uuid.New()
	binding.PurgeAt = &deadline
	err := locks.WithAccountLock(ctx, binding.AccountID, func(tx pgx.Tx) error {
		if err := s.BindRetainedSubjectTx(ctx, tx, binding); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `SELECT pg_sleep(GREATEST(EXTRACT(epoch FROM ($1::timestamptz-clock_timestamp())),0)+0.02)`, deadline); err != nil {
			return err
		}
		return s.RequireRetainedSubjectTx(ctx, tx, binding)
	})
	require.ErrorIs(t, err, automod.ErrSubject)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_subjects WHERE subject_id=$1`, binding.ID).Scan(&count))
	require.Zero(t, count, "deadline-crossing transaction cannot commit a new identity mapping")
}
