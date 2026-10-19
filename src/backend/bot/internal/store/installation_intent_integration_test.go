package store_test

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/bot/internal/store"
)

func TestInstallationIntentRecoveryCancellationAndReinstall(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL boundary")
	}
	ctx := context.Background()
	st := startBotStore(t)
	bot, _, err := st.CreateBot(ctx, uuid.New(), "Intent", "", `[]`, uuid.New())
	require.NoError(t, err)
	space, installer, chat := uuid.New(), uuid.New(), uuid.New()
	first, err := st.PrepareInstallation(ctx, bot.ID, space, installer, []uuid.UUID{chat, chat})
	require.NoError(t, err)
	require.Equal(t, int64(1), first.Generation)
	require.Equal(t, []uuid.UUID{chat}, first.Chats)
	// A fresh store instance recovers the committed request rather than minting
	// another installation when an acknowledgement or process was lost.
	restarted := &store.BotStore{Pool: st.Pool}
	retry, err := restarted.PrepareInstallation(ctx, bot.ID, space, installer, []uuid.UUID{chat})
	require.NoError(t, err)
	require.Equal(t, first, retry)
	_, err = restarted.PrepareInstallation(ctx, bot.ID, space, installer, nil)
	require.ErrorIs(t, err, store.ErrInstallationConflict)
	mutated := *first
	mutated.Generation++
	require.ErrorIs(t, st.CompleteInstallation(ctx, &mutated, []uuid.UUID{chat}), store.ErrInstallationConflict)
	require.NoError(t, st.CompleteInstallation(ctx, first, []uuid.UUID{chat}))
	require.NoError(t, st.CompleteInstallation(ctx, first, []uuid.UUID{chat}))
	var actual uuid.UUID
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT id FROM bot_space_installations WHERE bot_id=$1 AND space_id=$2`, bot.ID, space).Scan(&actual))
	require.Equal(t, first.InstallationID, actual)
	cancel, err := restarted.BeginInstallationCancellation(ctx, bot.ID, space, installer)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, cancel.CancelOperationID)
	require.NotEqual(t, cancel.OperationID, cancel.CancelOperationID)
	allowed, err := st.IsChatWhitelisted(ctx, bot.ID, chat)
	require.NoError(t, err)
	require.False(t, allowed, "uncertain cancellation must disable local use")
	_, err = st.PrepareInstallation(ctx, bot.ID, space, installer, []uuid.UUID{chat})
	require.ErrorIs(t, err, store.ErrInstallationConflict)
	require.ErrorIs(t, st.CompleteInstallation(ctx, first, []uuid.UUID{chat}), store.ErrInstallationConflict)
	again, err := restarted.BeginInstallationCancellation(ctx, bot.ID, space, installer)
	require.NoError(t, err)
	require.Equal(t, cancel, again)
	require.NoError(t, restarted.CompleteInstallationCancellation(ctx, cancel))
	next, err := st.PrepareInstallation(ctx, bot.ID, space, installer, []uuid.UUID{chat})
	require.NoError(t, err)
	require.Equal(t, int64(2), next.Generation)
	require.NotEqual(t, first.InstallationID, next.InstallationID)
	require.NotEqual(t, first.OperationID, next.OperationID)
	require.NoError(t, st.CompleteInstallation(ctx, next, []uuid.UUID{chat}))
	// Replaying a finished old cancellation cannot delete the new installation.
	require.NoError(t, st.CompleteInstallationCancellation(ctx, cancel))
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT id FROM bot_space_installations WHERE bot_id=$1 AND space_id=$2`, bot.ID, space).Scan(&actual))
	require.Equal(t, next.InstallationID, actual)
	require.ErrorIs(t, st.CompleteInstallation(ctx, first, []uuid.UUID{chat}), store.ErrInstallationConflict)
	_, err = st.Pool.Exec(ctx, `UPDATE bot_installation_intents SET generation=$2 WHERE installation_id=$1`, next.InstallationID, int64(math.MaxInt64))
	require.NoError(t, err)
	exhausted, err := st.BeginInstallationCancellation(ctx, bot.ID, space, installer)
	require.NoError(t, err)
	require.NoError(t, st.CompleteInstallationCancellation(ctx, exhausted))
	_, err = st.PrepareInstallation(ctx, bot.ID, space, installer, nil)
	require.ErrorIs(t, err, store.ErrInstallationConflict)
}

func TestInstallationIntentConcurrentPrepareAndCancelBeforeComplete(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL boundary")
	}
	ctx := context.Background()
	st := startBotStore(t)
	bot, _, err := st.CreateBot(ctx, uuid.New(), "Concurrent", "", `[]`, uuid.New())
	require.NoError(t, err)
	space, installer := uuid.New(), uuid.New()
	type result struct {
		intent *store.InstallationIntent
		err    error
	}
	out := make(chan result, 8)
	for n := 0; n < 8; n++ {
		go func() { i, e := st.PrepareInstallation(ctx, bot.ID, space, installer, nil); out <- result{i, e} }()
	}
	var first *store.InstallationIntent
	for n := 0; n < 8; n++ {
		r := <-out
		require.NoError(t, r.err)
		if first == nil {
			first = r.intent
		}
		require.Equal(t, first, r.intent)
	}
	cancel, err := st.BeginInstallationCancellation(ctx, bot.ID, space, installer)
	require.NoError(t, err)
	require.NoError(t, st.CompleteInstallationCancellation(ctx, cancel))
	require.ErrorIs(t, st.CompleteInstallation(ctx, first, nil), store.ErrInstallationConflict)
	var count int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_space_installations WHERE bot_id=$1 AND space_id=$2`, bot.ID, space).Scan(&count))
	require.Zero(t, count)
}

func TestInstallationIntentPreservesUnqualifiedLegacyOriginAfterNewAddAcknowledgement(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL legacy origin boundary")
	}
	st := startBotStore(t)
	ctx := context.Background()
	bot, _, err := st.CreateBot(ctx, uuid.New(), "Legacy", "", `[]`, uuid.New())
	require.NoError(t, err)
	space, installer := uuid.New(), uuid.New()
	actual, err := st.InstallInSpace(ctx, bot.ID, space, installer, nil)
	require.NoError(t, err)
	intent, err := st.PrepareInstallation(ctx, bot.ID, space, installer, nil)
	require.NoError(t, err)
	require.Equal(t, actual, intent.InstallationID)
	require.True(t, intent.LegacyEvidence)
	// Even an exact new-native add acknowledgement says nothing about whether
	// the historical legacy source was independently qualified/retired.
	require.NoError(t, st.CompleteInstallation(ctx, intent, nil))
	recovered, err := st.PrepareInstallation(ctx, bot.ID, space, installer, nil)
	require.NoError(t, err)
	require.True(t, recovered.LegacyEvidence, "empty Add acknowledgement cannot erase legacy provenance obligation")
	deletion, err := st.BeginBotDeletion(ctx, bot.ID, bot.OwnerAccountID)
	require.NoError(t, err)
	require.Equal(t, "PENDING", deletion.State)
	var legacy bool
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT legacy_evidence FROM bot_deletion_installations WHERE bot_id=$1`, bot.ID).Scan(&legacy))
	require.True(t, legacy)
}

func TestInstallationIntentRejectsDisabledActorChangeAndFrozenSpace(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL boundary")
	}
	ctx := context.Background()
	st := startBotStore(t)
	bot, _, err := st.CreateBot(ctx, uuid.New(), "Admission", "", `[]`, uuid.New())
	require.NoError(t, err)
	space, installer := uuid.New(), uuid.New()
	intent, err := st.PrepareInstallation(ctx, bot.ID, space, installer, nil)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `UPDATE bots SET actor_profile_id=$2 WHERE id=$1`, bot.ID, uuid.New())
	require.NoError(t, err)
	_, err = st.PrepareInstallation(ctx, bot.ID, space, installer, nil)
	require.ErrorIs(t, err, store.ErrInstallationConflict)
	require.ErrorIs(t, st.CompleteInstallation(ctx, intent, nil), store.ErrInstallationConflict)
	_, err = st.Pool.Exec(ctx, `UPDATE bots SET actor_profile_id=$2,status='disabled' WHERE id=$1`, bot.ID, bot.ActorProfileID)
	require.NoError(t, err)
	_, err = st.PrepareInstallation(ctx, bot.ID, space, installer, nil)
	require.ErrorIs(t, err, store.ErrInstallationConflict)
	require.ErrorIs(t, st.CompleteInstallation(ctx, intent, nil), store.ErrInstallationConflict)
	// Cleanup is still permitted for a disabled Bot, using the stored actor.
	cancel, err := st.BeginInstallationCancellation(ctx, bot.ID, space, installer)
	require.NoError(t, err)
	require.NoError(t, st.CompleteInstallationCancellation(ctx, cancel))
	_, err = st.Pool.Exec(ctx, `UPDATE bots SET status='live' WHERE id=$1`, bot.ID)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `INSERT INTO bot_space_lifecycle_heads(space_id,deletion_operation_id,generation,desired_state,manifest_id,manifest_sha256,manifest_item_count) VALUES($1,$2,1,'FROZEN',$3,$4,0)`, space, uuid.New(), uuid.New(), make([]byte, 32))
	require.NoError(t, err)
	_, err = st.PrepareInstallation(ctx, bot.ID, space, installer, nil)
	require.ErrorIs(t, err, store.ErrSpaceLifecycleConflict)
}

func TestInstallationIntentDownWaitsForFirstWriterAndPreservesAuthority(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL two-connection boundary")
	}
	st := startBotStore(t)
	ctx, stop := context.WithTimeout(context.Background(), 15*time.Second)
	defer stop()
	bot, _, err := st.CreateBot(ctx, uuid.New(), "FirstWriter", "", `[]`, uuid.New())
	require.NoError(t, err)
	writer, err := st.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = writer.Rollback(context.Background()) }()
	id := uuid.New()
	_, err = writer.Exec(ctx, `INSERT INTO bot_installation_intents(installation_id,operation_id,bot_id,space_id,actor_profile_id,installed_by_profile_id,generation,state,chat_ids) VALUES($1,$2,$3,$4,$5,$6,1,'PREPARING','[]')`, id, uuid.New(), bot.ID, uuid.New(), bot.ActorProfileID, uuid.New())
	require.NoError(t, err)
	_, file, _, _ := runtime.Caller(0)
	down, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations", "bot_db", "000008_installation_intents.down.sql"))
	require.NoError(t, err)
	conn, err := st.Pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	var pid int32
	require.NoError(t, conn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid))
	done := make(chan error, 1)
	go func() { _, e := conn.Exec(ctx, string(down)); done <- e }()
	// Observe the actual lock wait instead of sleeping and assuming interleaving.
	require.Eventually(t, func() bool {
		var waiting bool
		e := st.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE pid=$1 AND wait_event_type='Lock')`, pid).Scan(&waiting)
		return e == nil && waiting
	}, 5*time.Second, 10*time.Millisecond)
	require.NoError(t, writer.Commit(ctx))
	require.ErrorContains(t, <-done, "must not be dropped")
	_, err = conn.Exec(ctx, `ROLLBACK`)
	require.NoError(t, err)
	var retained uuid.UUID
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT installation_id FROM bot_installation_intents WHERE installation_id=$1`, id).Scan(&retained))
	require.Equal(t, id, retained)
}
