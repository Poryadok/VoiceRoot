package store

import (
	"context"
	"crypto/sha256"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
)

func TestMinimalTerminal48Known46ProjectionAndBoundedSource(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixtureWithImage(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"), "postgres:18-alpine")
	db.Run(true, "goto", "34")
	admin := &SpaceStore{Pool: db.Pool}
	first, err := admin.CreateSpace(ctx, uuid.New(), "Minimal root source A", "", "public")
	require.NoError(t, err)
	second, err := admin.CreateSpace(ctx, uuid.New(), "Minimal root source B", "", "public")
	require.NoError(t, err)
	db.Run(true, "goto", "46")
	_, err = admin.TransitionNativeMembershipCatalog46(ctx)
	require.NoError(t, err)
	for _, id := range []uuid.UUID{first.ID, second.ID} {
		require.NoError(t, admin.InitializeNativeMembership(ctx, id))
	}
	serving := &SpaceStore{Pool: nativeReadinessLogin(t, ctx, db.Pool)}
	invoke := signedNativeBotCaller(t, serving)
	makeAdd := func(bot, actor string, space uuid.UUID) *spacev1.AddBotMemberRequest {
		return &spacev1.AddBotMemberRequest{SpaceId: space.String(), ProfileId: actor, InstallationIntent: &spacev1.BotInstallationIntent{OperationId: uuid.NewString(), BotId: bot, InstallationId: uuid.NewString(), SpaceId: space.String(), ActorProfileId: actor, Generation: 1}}
	}
	oldAdd := makeAdd(uuid.NewString(), uuid.NewString(), first.ID)
	_, err = invoke(oldAdd)
	require.NoError(t, err)
	oldRoot := uuid.NewString()
	oldHash := sha256.Sum256([]byte(oldRoot))
	oldRequest := nativeGlobalRequest(nativeGlobalCancel(oldAdd), oldRoot, oldHash[:])
	oldReceipt, err := invoke(oldRequest)
	require.NoError(t, err)
	var oldEvidence, oldActor []byte
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT decision_sha256,actor_binding_sha256 FROM native_membership_guard.bot_terminals WHERE bot_id=$1`, oldAdd.InstallationIntent.BotId).Scan(&oldEvidence, &oldActor))
	bot, actor := uuid.NewString(), uuid.NewString()
	addA, addB := makeAdd(bot, actor, first.ID), makeAdd(bot, actor, second.ID)
	_, err = invoke(addA)
	require.NoError(t, err)
	_, err = invoke(addB)
	require.NoError(t, err)
	db.Run(true, "up", "1")
	_, err = admin.TransitionNativeMembershipCatalog47(ctx)
	require.NoError(t, err)
	db.Run(true, "up", "1")
	minimal := signedMinimalTerminal(t, serving)
	root := &spacev1.RegisterBotGlobalTerminalRequest{ProtocolVersion: 1, BotId: bot, DecisionId: uuid.NewString()}
	_, err = minimal(root)
	require.ErrorIs(t, err, ErrMemberAuthorityUnavailable, "new table alone is never admission")
	transition, err := admin.TransitionNativeMembershipCatalog48(ctx)
	require.NoError(t, err)
	require.True(t, transition.Changed)
	repeated, err := admin.TransitionNativeMembershipCatalog48(ctx)
	require.NoError(t, err)
	require.False(t, repeated.Changed)
	require.Equal(t, transition.OldSealSHA256, repeated.OldSealSHA256)
	require.Equal(t, transition.NewSealSHA256, repeated.NewSealSHA256)
	var retainedEvidence, retainedActor []byte
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT decision_sha256,actor_binding_sha256 FROM native_membership_guard.bot_terminals WHERE bot_id=$1`, oldAdd.InstallationIntent.BotId).Scan(&retainedEvidence, &retainedActor))
	require.Equal(t, oldEvidence, retainedEvidence)
	require.Equal(t, oldActor, retainedActor, "no erasure trigger is invented by projection")
	oldMinimal, err := minimal(&spacev1.RegisterBotGlobalTerminalRequest{ProtocolVersion: 1, BotId: oldAdd.InstallationIntent.BotId, DecisionId: oldRoot})
	require.NoError(t, err)
	require.NotEqual(t, oldReceipt.ReceiptId, oldMinimal.ReceiptId, "projection issues an actual new minimal result")
	replay, err := invoke(oldRequest)
	require.NoError(t, err)
	require.True(t, proto.Equal(oldReceipt, replay), "retained old full proof is not reconstructed or replaced")
	fence, err := minimal(root)
	require.NoError(t, err)
	var published int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM public.space_bot_installation_attempts WHERE bot_id=$1 AND state='PUBLISHED'`, bot).Scan(&published))
	require.Equal(t, 2, published, "root-only success performs no source cleanup")
	for _, add := range []*spacev1.AddBotMemberRequest{addA, addB, makeAdd(bot, actor, uuid.New())} {
		_, err = invoke(add)
		require.ErrorIs(t, err, ErrMemberAdmissionStale)
	}
	next := proto.Clone(addA).(*spacev1.AddBotMemberRequest)
	next.InstallationIntent.Generation = 2
	next.InstallationIntent.OperationId = uuid.NewString()
	next.InstallationIntent.InstallationId = uuid.NewString()
	_, err = invoke(next)
	require.ErrorIs(t, err, ErrMemberAdmissionStale)
	rootHash := sha256.Sum256([]byte(root.DecisionId))
	request := nativeGlobalRequest(nativeGlobalCancel(addA), root.DecisionId, rootHash[:])
	receipt, err := invoke(request)
	require.NoError(t, err)
	require.True(t, receipt.GlobalNativeAddsFenced)
	replay, err = invoke(request)
	require.NoError(t, err)
	require.True(t, proto.Equal(receipt, replay))
	after, err := minimal(root)
	require.NoError(t, err)
	require.True(t, proto.Equal(fence, after))
	var legacy, minimalRows, bounded int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM native_membership_guard.bot_terminals),(SELECT count(*) FROM native_membership_guard.bot_terminal_registrations),(SELECT count(*) FROM native_membership_guard.bot_removal_receipts48)`).Scan(&legacy, &minimalRows, &bounded))
	require.Equal(t, 1, legacy, "new per-Space48 effects must not recreate personal permanent markers")
	require.Equal(t, 2, minimalRows)
	require.Equal(t, 1, bounded)
	columns := []string{}
	rows, err := db.Pool.Query(ctx, `SELECT attname FROM pg_attribute WHERE attrelid='native_membership_guard.bot_terminal_registrations'::regclass AND attnum>0 AND NOT attisdropped ORDER BY attnum`)
	require.NoError(t, err)
	for rows.Next() {
		var c string
		require.NoError(t, rows.Scan(&c))
		columns = append(columns, c)
	}
	require.NoError(t, rows.Err())
	rows.Close()
	require.Equal(t, []string{"bot_id", "decision_id", "receipt_id", "registered_at"}, columns)
	// Other Bots retain ordinary uninstall/reinstall semantics; this fence does
	// not grant a blanket Space ban or change the public ordinary ACK policy.
	control := makeAdd(uuid.NewString(), uuid.NewString(), first.ID)
	_, err = invoke(control)
	require.NoError(t, err)
	_, err = invoke(nativeGlobalCancel(control))
	require.NoError(t, err)
	control.InstallationIntent.Generation = 2
	control.InstallationIntent.OperationId = uuid.NewString()
	control.InstallationIntent.InstallationId = uuid.NewString()
	_, err = invoke(control)
	require.NoError(t, err)
}
