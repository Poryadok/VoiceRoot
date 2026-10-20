package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
)

func TestMinimalTerminal48TriggerDriftCannotSealOrReplay(t *testing.T) {
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
	snapshot := func() string {
		var state string
		require.NoError(t, db.Pool.QueryRow(ctx, `SELECT jsonb_build_object('epoch',(SELECT to_jsonb(e) FROM native_membership_guard.epoch e),'transition',(SELECT jsonb_agg(to_jsonb(r)) FROM native_membership_guard.catalog48_transition r),'registrations',(SELECT jsonb_agg(to_jsonb(r)) FROM native_membership_guard.bot_terminal_registrations r))::text`).Scan(&state))
		return state
	}
	for _, drift := range []struct{ table, trigger, enable string }{
		{"native_membership_guard.bot_terminal_registrations", "bot_minimal48_immutable", "ENABLE"},
		{"public.spaces", "zzzzzzz_bot_minimal48_receipt_terminal", "ENABLE"},
		{"native_membership_guard.bot_terminal_registrations", "bot_terminal48_registration", "ENABLE ALWAYS"},
	} {
		t.Run(drift.trigger, func(t *testing.T) {
			before := snapshot()
			_, err := db.Pool.Exec(ctx, "ALTER TABLE "+drift.table+" DISABLE TRIGGER "+drift.trigger)
			require.NoError(t, err)
			_, err = admin.TransitionNativeMembershipCatalog48(ctx)
			require.ErrorIs(t, err, ErrMemberAuthorityUnavailable, "new closure drift must be rejected before sealing")
			require.Equal(t, before, snapshot(), "refusal cannot activate or mutate retained proof")
			_, err = db.Pool.Exec(ctx, "ALTER TABLE "+drift.table+" "+drift.enable+" TRIGGER "+drift.trigger)
			require.NoError(t, err)
		})
	}
	_, err = admin.TransitionNativeMembershipCatalog48(ctx)
	require.NoError(t, err)
	invoke := signedMinimalTerminal(t, &SpaceStore{Pool: nativeReadinessLogin(t, ctx, db.Pool)})
	request := &spacev1.RegisterBotGlobalTerminalRequest{ProtocolVersion: 1, BotId: uuid.NewString(), DecisionId: uuid.NewString()}
	_, err = invoke(request)
	require.NoError(t, err)
	before := snapshot()
	_, err = db.Pool.Exec(ctx, `ALTER TABLE public.spaces DISABLE TRIGGER zzzzzzz_bot_minimal48_receipt_terminal`)
	require.NoError(t, err)
	_, err = invoke(request)
	require.ErrorIs(t, err, ErrMemberAuthorityUnavailable, "saved result cannot bypass current trigger qualification")
	require.Equal(t, before, snapshot())
}
