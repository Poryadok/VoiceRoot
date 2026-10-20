package store

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"path/filepath"
	"testing"
	"time"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
)

func TestMinimalTerminal48ConcurrentPublicationRegistrationAndRollback(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "34")
	admin := &SpaceStore{Pool: db.Pool}
	space, err := admin.CreateSpace(ctx, uuid.New(), "Root publication race", "", "public")
	require.NoError(t, err)
	db.Run(true, "goto", "46")
	_, err = admin.TransitionNativeMembershipCatalog46(ctx)
	require.NoError(t, err)
	require.NoError(t, admin.InitializeNativeMembership(ctx, space.ID))
	db.Run(true, "up", "1")
	_, err = admin.TransitionNativeMembershipCatalog47(ctx)
	require.NoError(t, err)
	db.Run(true, "up", "1")
	_, err = admin.TransitionNativeMembershipCatalog48(ctx)
	require.NoError(t, err)
	serving := &SpaceStore{Pool: nativeReadinessLogin(t, ctx, db.Pool)}
	install := signedNativeBotCaller(t, serving)
	register := signedMinimalTerminal(t, serving)
	bot, actor := uuid.New(), uuid.New()
	add := &spacev1.AddBotMemberRequest{SpaceId: space.ID.String(), ProfileId: actor.String(), InstallationIntent: &spacev1.BotInstallationIntent{OperationId: uuid.NewString(), BotId: bot.String(), InstallationId: uuid.NewString(), SpaceId: space.ID.String(), ActorProfileId: actor.String(), Generation: 1}}
	request := &spacev1.RegisterBotGlobalTerminalRequest{ProtocolVersion: 1, BotId: bot.String(), DecisionId: uuid.NewString()}
	gate, err := db.Pool.Begin(ctx)
	require.NoError(t, err)
	defer gate.Rollback(ctx)
	var terminal bool
	require.NoError(t, gate.QueryRow(ctx, `SELECT public.space_bot_native_terminal($1)`, bot).Scan(&terminal))
	require.False(t, terminal)
	addDone := make(chan error, 1)
	type result struct {
		r   *spacev1.BotGlobalTerminalReceipt
		err error
	}
	rootDone := make(chan result, 1)
	go func() { _, e := install(add); addDone <- e }()
	go func() { r, e := register(request); rootDone <- result{r, e} }()
	blocked := 0
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); {
		require.NoError(t, gate.QueryRow(ctx, `SELECT count(DISTINCT pid) FROM pg_locks WHERE locktype='advisory' AND NOT granted`).Scan(&blocked))
		if blocked >= 2 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	require.NoError(t, gate.Commit(ctx))
	addErr, root := <-addDone, <-rootDone
	require.GreaterOrEqual(t, blocked, 2, "both real independent operations must reach the same held Bot lock")
	if addErr != nil {
		require.ErrorIs(t, addErr, ErrMemberAdmissionStale)
	}
	require.NoError(t, root.err)
	_, err = install(add)
	require.ErrorIs(t, err, ErrMemberAdmissionStale, "even a historical positive replay is denied after the root wins")
	replay, err := register(request)
	require.NoError(t, err)
	require.True(t, proto.Equal(root.r, replay))
	// Concurrent contradictory roots serialize on the actual common Bot lock.
	conflictBot := uuid.NewString()
	a := &spacev1.RegisterBotGlobalTerminalRequest{ProtocolVersion: 1, BotId: conflictBot, DecisionId: uuid.NewString()}
	b := proto.Clone(a).(*spacev1.RegisterBotGlobalTerminalRequest)
	b.DecisionId = uuid.NewString()
	done := make(chan result, 2)
	for _, r := range []*spacev1.RegisterBotGlobalTerminalRequest{a, b} {
		go func(r *spacev1.RegisterBotGlobalTerminalRequest) { v, e := register(r); done <- result{v, e} }(r)
	}
	x, y := <-done, <-done
	require.NotEqual(t, x.err == nil, y.err == nil, "exactly one committed root can own the Bot")
	if x.err != nil {
		require.ErrorIs(t, x.err, ErrMemberAdmissionConflict)
	}
	if y.err != nil {
		require.ErrorIs(t, y.err, ErrMemberAdmissionConflict)
	}
	// Rollback retains no phantom receipt or permanent fence. A later real
	// registration is newly issued; a discarded result is never replayed.
	rolledBot, rolledRoot := uuid.New(), uuid.New()
	tx, err := serving.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	require.NoError(t, err)
	var discarded uuid.UUID
	require.NoError(t, tx.QueryRow(ctx, `SELECT receipt_id FROM public.space_bot_global_terminal_register($1,$2)`, rolledBot, rolledRoot).Scan(&discarded))
	require.NoError(t, tx.Rollback(ctx))
	var rows int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM native_membership_guard.bot_terminal_registrations WHERE bot_id=$1`, rolledBot).Scan(&rows))
	require.Zero(t, rows)
	committed, err := register(&spacev1.RegisterBotGlobalTerminalRequest{ProtocolVersion: 1, BotId: rolledBot.String(), DecisionId: rolledRoot.String()})
	require.NoError(t, err)
	require.NotEqual(t, discarded.String(), committed.ReceiptId)
}

func TestMinimalTerminal48ReplayRejectsCatalogAndRoleDrift(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "46")
	admin := &SpaceStore{Pool: db.Pool}
	_, err := admin.TransitionNativeMembershipCatalog46(ctx)
	require.NoError(t, err)
	db.Run(true, "up", "1")
	_, err = admin.TransitionNativeMembershipCatalog47(ctx)
	require.NoError(t, err)
	db.Run(true, "up", "1")
	_, err = admin.TransitionNativeMembershipCatalog48(ctx)
	require.NoError(t, err)
	serving := &SpaceStore{Pool: nativeReadinessLogin(t, ctx, db.Pool)}
	invoke := signedMinimalTerminal(t, serving)
	r := &spacev1.RegisterBotGlobalTerminalRequest{ProtocolVersion: 1, BotId: uuid.NewString(), DecisionId: uuid.NewString()}
	receipt, err := invoke(r)
	require.NoError(t, err)
	for _, state := range []struct {
		version int
		dirty   bool
	}{{49, false}, {48, true}} {
		_, err = db.Pool.Exec(ctx, `UPDATE public.schema_migrations SET version=$1,dirty=$2`, state.version, state.dirty)
		require.NoError(t, err)
		_, err = invoke(r)
		require.ErrorIs(t, err, ErrMemberAuthorityUnavailable)
		_, err = db.Pool.Exec(ctx, `UPDATE public.schema_migrations SET version=48,dirty=false`)
		require.NoError(t, err)
	}
	var definition string
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT pg_get_functiondef('public.space_bot_global_terminal_register(uuid,uuid)'::regprocedure)`).Scan(&definition))
	_, err = db.Pool.Exec(ctx, `CREATE OR REPLACE FUNCTION public.space_bot_global_terminal_register(bot uuid,decision uuid) RETURNS TABLE(receipt_id uuid,registered_at timestamptz) LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$ BEGIN RETURN;END $$;`)
	require.NoError(t, err)
	_, err = invoke(r)
	require.ErrorIs(t, err, ErrMemberAuthorityUnavailable, "saved replay cannot bypass fixed function qualification")
	_, err = db.Pool.Exec(ctx, definition)
	require.NoError(t, err)
	login := serving.Pool.Config().ConnConfig.User
	_, err = db.Pool.Exec(ctx, "ALTER ROLE "+pgx.Identifier{login}.Sanitize()+" SUPERUSER")
	require.NoError(t, err)
	_, err = invoke(r)
	require.ErrorIs(t, err, ErrMemberAuthorityUnavailable)
	_, err = db.Pool.Exec(ctx, "ALTER ROLE "+pgx.Identifier{login}.Sanitize()+" NOSUPERUSER")
	require.NoError(t, err)
	replay, err := invoke(r)
	require.NoError(t, err)
	require.True(t, proto.Equal(receipt, replay), "restoring exact originals does not repair/reseal or reissue data")
	var rows int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM native_membership_guard.bot_terminal_registrations`).Scan(&rows))
	require.Equal(t, 1, rows)
}
