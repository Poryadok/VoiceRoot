package store

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/botmembershipprincipal"
)

func signedNativeBotCaller(t *testing.T, st *SpaceStore) func(proto.Message) (*spacev1.BotInstallationRemovalReceipt, error) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	verifier := botmembershipprincipal.Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }}
	return func(r proto.Message) (*spacev1.BotInstallationRemovalReceipt, error) {
		method, operation := "", ""
		switch v := r.(type) {
		case *spacev1.AddBotMemberRequest:
			method, operation = spacev1.SpaceService_AddBotMember_FullMethodName, v.InstallationIntent.OperationId
		case *spacev1.RemoveBotMemberRequest:
			method, operation = spacev1.SpaceService_RemoveBotMember_FullMethodName, v.InstallationIntent.OperationId
		case *spacev1.ApplyBotInstallationRemovalRequest:
			method, operation = spacev1.SpaceService_ApplyBotInstallationRemoval_FullMethodName, v.OperationId
		default:
			t.Fatal("unexpected native Bot request")
		}
		hash, err := principal.RequestHash(r)
		require.NoError(t, err)
		token, err := issuer.IssueService(principal.ServiceInput{Audience: "space", RPC: method, RequestID: operation, RequestHash: hash})
		require.NoError(t, err)
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", operation))
		value, err := verifier.Unary()(ctx, r, &grpc.UnaryServerInfo{FullMethod: method}, func(ctx context.Context, _ any) (any, error) {
			switch v := r.(type) {
			case *spacev1.AddBotMemberRequest:
				proof, err := botmembershipprincipal.RequireAdd(ctx, v)
				if err != nil {
					return nil, err
				}
				return nil, st.AddBotMemberWithInstallation(ctx, proof, v)
			case *spacev1.RemoveBotMemberRequest:
				proof, err := botmembershipprincipal.RequireRemove(ctx, v)
				if err != nil {
					return nil, err
				}
				return nil, st.RemoveBotMemberWithInstallation(ctx, proof, v)
			case *spacev1.ApplyBotInstallationRemovalRequest:
				proof, err := botmembershipprincipal.RequireGlobalRemoval(ctx, v)
				if err != nil {
					return nil, err
				}
				return st.ApplyBotInstallationRemoval(ctx, proof, v)
			}
			panic("unexpected native Bot request")
		})
		if value == nil {
			return nil, err
		}
		return value.(*spacev1.BotInstallationRemovalReceipt), err
	}
}

func nativeGlobalCancel(add *spacev1.AddBotMemberRequest) *spacev1.RemoveBotMemberRequest {
	r := &spacev1.RemoveBotMemberRequest{SpaceId: add.SpaceId, ProfileId: add.ProfileId, InstallationIntent: proto.Clone(add.InstallationIntent).(*spacev1.BotInstallationIntent), InstallOperationId: add.InstallationIntent.OperationId}
	r.InstallationIntent.OperationId = uuid.NewString()
	return r
}

func nativeGlobalRequest(cancel *spacev1.RemoveBotMemberRequest, decision string, rootSHA []byte) *spacev1.ApplyBotInstallationRemovalRequest {
	frozen := sha256.Sum256([]byte(uuid.NewString()))
	return &spacev1.ApplyBotInstallationRemovalRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), DecisionId: decision, DecisionSha256: append([]byte(nil), rootSHA...), Purpose: "global_delete", BotId: cancel.InstallationIntent.BotId, ActorProfileId: cancel.ProfileId, SpaceId: cancel.SpaceId, NativeCancellation: proto.Clone(cancel).(*spacev1.RemoveBotMemberRequest), FrozenScopeSha256: frozen[:]}
}

func TestNativeBotGlobal46ExactRemovalAndEveryAddFence(t *testing.T) {
	if testing.Short() {
		t.Skip("requires actual native45→46 signed Store and nonowner serving PostgreSQL")
	}
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixtureWithImage(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"), "postgres:18-alpine")
	db.Run(true, "goto", "34")
	admin := &SpaceStore{Pool: db.Pool}
	spaces := make([]*SpaceRow, 3)
	for i := range spaces {
		var err error
		spaces[i], err = admin.CreateSpace(ctx, uuid.New(), "Global native Bot", "", "public")
		require.NoError(t, err)
	}
	db.Run(true, "goto", "45")
	_, err := admin.TransitionNativeMembershipCatalog45(ctx)
	require.NoError(t, err)
	for _, space := range spaces {
		require.NoError(t, admin.InitializeNativeMembership(ctx, space.ID))
	}
	serving := &SpaceStore{Pool: nativeReadinessLogin(t, ctx, db.Pool)}
	invoke := signedNativeBotCaller(t, serving)
	bot, actor := uuid.NewString(), uuid.NewString()
	add := func(space uuid.UUID) *spacev1.AddBotMemberRequest {
		return &spacev1.AddBotMemberRequest{SpaceId: space.String(), ProfileId: actor, InstallationIntent: &spacev1.BotInstallationIntent{OperationId: uuid.NewString(), BotId: bot, InstallationId: uuid.NewString(), SpaceId: space.String(), ActorProfileId: actor, Generation: 1}}
	}
	first, other := add(spaces[0].ID), add(spaces[1].ID)
	_, err = invoke(first)
	require.NoError(t, err)
	_, err = invoke(other)
	require.NoError(t, err)
	// Historical45 independently authenticated both native tuples. The new
	// global root must refuse their actual contradictory retained Bot mapping;
	// neither a single exact cancellation nor a new root can erase this fact.
	conflictingA, conflictingB := add(spaces[0].ID), add(spaces[2].ID)
	conflictingA.InstallationIntent.BotId = uuid.NewString()
	conflictingB.InstallationIntent.BotId = conflictingA.InstallationIntent.BotId
	conflictingA.ProfileId = uuid.NewString()
	conflictingA.InstallationIntent.ActorProfileId = conflictingA.ProfileId
	conflictingB.ProfileId = uuid.NewString()
	conflictingB.InstallationIntent.ActorProfileId = conflictingB.ProfileId
	_, err = invoke(conflictingA)
	require.NoError(t, err)
	_, err = invoke(conflictingB)
	require.NoError(t, err)
	db.Run(true, "up")
	_, err = invoke(other)
	require.ErrorIs(t, err, ErrMemberAuthorityUnavailable, "unactivated46 cannot replay a retained45 positive")
	_, err = admin.TransitionNativeMembershipCatalog46(ctx)
	require.NoError(t, err)
	_, err = invoke(other)
	require.NoError(t, err, "exact retained45 positive still replays before any global terminal decision")
	conflictRoot := sha256.Sum256([]byte(uuid.NewString()))
	_, err = invoke(nativeGlobalRequest(nativeGlobalCancel(conflictingA), uuid.NewString(), conflictRoot[:]))
	require.Error(t, err, "known conflicting cross-Space Bot actor mapping cannot receive global COMPLETED")
	var mappingUnchanged bool
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM native_membership_guard.bot_terminals WHERE bot_id=$1)
 AND (SELECT count(*) FROM public.space_bot_installation_attempts WHERE bot_id=$1 AND state='PUBLISHED')=2`, conflictingA.InstallationIntent.BotId).Scan(&mappingUnchanged))
	require.True(t, mappingUnchanged, "refusal rolls back exact cancellation as well as global marker")
	ordinaryCancel := nativeGlobalCancel(first)
	_, err = invoke(ordinaryCancel)
	require.NoError(t, err)
	second := proto.Clone(first).(*spacev1.AddBotMemberRequest)
	second.InstallationIntent.OperationId = uuid.NewString()
	second.InstallationIntent.InstallationId = uuid.NewString()
	second.InstallationIntent.Generation = 2
	_, err = invoke(second)
	require.NoError(t, err, "ordinary uninstall permits a distinct next installation")
	decision := uuid.NewString()
	rootSHA := sha256.Sum256([]byte(decision))
	global := nativeGlobalRequest(nativeGlobalCancel(second), decision, rootSHA[:])
	receipt, err := invoke(global)
	require.NoError(t, err)
	require.True(t, receipt.GlobalNativeAddsFenced)
	require.Equal(t, "COMPLETED", receipt.Outcome)
	requestRaw, err := proto.MarshalOptions{Deterministic: true}.Marshal(global)
	require.NoError(t, err)
	evidenceRaw, err := proto.MarshalOptions{Deterministic: true}.Marshal(global.NativeCancellation)
	require.NoError(t, err)
	requestSHA, evidenceSHA := sha256.Sum256(requestRaw), sha256.Sum256(evidenceRaw)
	require.Equal(t, requestSHA[:], receipt.RequestSha256)
	require.Equal(t, evidenceSHA[:], receipt.EvidenceSha256)
	replay, err := invoke(global)
	require.NoError(t, err)
	require.True(t, proto.Equal(receipt, replay), "lost-ACK replay returns the immutable original receiver result")
	_,err=signedNativeBotCaller(t,admin)(global)
	require.ErrorIs(t,err,ErrEffectiveEpisodeUnavailable,"saved receipt replay is not a serving-role/COMPLETE bypass for an installer connection")
	_, err = invoke(other)
	require.ErrorIs(t, err, ErrMemberAdmissionStale, "global fence rejects a different Space's still-PUBLISHED positive replay")
	_, err = invoke(second)
	require.ErrorIs(t, err, ErrMemberAdmissionStale)
	third := proto.Clone(second).(*spacev1.AddBotMemberRequest)
	third.InstallationIntent.OperationId = uuid.NewString()
	third.InstallationIntent.InstallationId = uuid.NewString()
	third.InstallationIntent.Generation = 3
	_, err = invoke(third)
	require.ErrorIs(t, err, ErrMemberAdmissionStale, "higher generation cannot resurrect a globally deleted Bot")
	_, err = invoke(add(spaces[2].ID))
	require.ErrorIs(t, err, ErrMemberAdmissionStale, "global terminal applies to a new Space too")
	changed := proto.Clone(global).(*spacev1.ApplyBotInstallationRemovalRequest)
	changed.DecisionId = uuid.NewString()
	_, err = invoke(changed)
	require.ErrorIs(t, err, ErrMemberAdmissionConflict)
	otherCancel := nativeGlobalCancel(other)
	_, err = invoke(otherCancel)
	require.NoError(t, err)
	var canceledAt time.Time
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT completed_at FROM public.space_bot_installation_cancellations WHERE operation_id=$1`, otherCancel.InstallationIntent.OperationId).Scan(&canceledAt))
	_, err = invoke(nativeGlobalRequest(otherCancel, decision, rootSHA[:]))
	require.NoError(t, err, "prior ordinary cancellation can supply its exact source obligation")
	var after time.Time
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT completed_at FROM public.space_bot_installation_cancellations WHERE operation_id=$1`, otherCancel.InstallationIntent.OperationId).Scan(&after))
	require.Equal(t, canceledAt, after, "adoption does not repeat the native cancellation")
	unknown := nativeGlobalCancel(add(spaces[2].ID))
	unknown.ProfileId = spaces[2].OwnerProfileID.String()
	unknown.InstallationIntent.ActorProfileId = unknown.ProfileId
	unknown.InstallationIntent.BotId = uuid.NewString()
	_, err = invoke(nativeGlobalRequest(unknown, uuid.NewString(), rootSHA[:]))
	require.ErrorIs(t, err, ErrMemberAuthorityUnavailable, "a signed root is not historical legacy-source ownership")
	// Hold the real shared Bot lock while both independent signed operations
	// enter. Whichever operation wins the original-installation key, the global
	// receipt must settle that exact source and fence every later Add replay.
	raceAdd := add(spaces[0].ID)
	raceAdd.ProfileId = uuid.NewString()
	raceAdd.InstallationIntent.ActorProfileId = raceAdd.ProfileId
	raceAdd.InstallationIntent.BotId = uuid.NewString()
	raceBot := uuid.MustParse(raceAdd.InstallationIntent.BotId)
	raceRoot := sha256.Sum256([]byte(uuid.NewString()))
	raceRemoval := nativeGlobalRequest(nativeGlobalCancel(raceAdd), uuid.NewString(), raceRoot[:])
	gate, err := db.Pool.Begin(ctx)
	require.NoError(t, err)
	defer gate.Rollback(context.Background())
	var terminal bool
	require.NoError(t, gate.QueryRow(ctx, `SELECT public.space_bot_native_terminal($1)`, raceBot).Scan(&terminal))
	require.False(t, terminal)
	type raceResult struct {
		receipt *spacev1.BotInstallationRemovalReceipt
		err     error
	}
	addDone, removeDone := make(chan error, 1), make(chan raceResult, 1)
	go func() { _, err := invoke(raceAdd); addDone <- err }()
	go func() { r, err := invoke(raceRemoval); removeDone <- raceResult{r, err} }()
	blocked := false
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
		require.NoError(t, gate.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE locktype='advisory' AND NOT granted)`).Scan(&blocked))
		if blocked {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	require.NoError(t, gate.Commit(ctx))
	addErr, result := <-addDone, <-removeDone
	require.True(t, blocked, "actual signed operations must reach the owned common lock")
	if addErr != nil {
		require.ErrorIs(t, addErr, ErrMemberAdmissionStale)
	}
	require.NoError(t, result.err)
	require.True(t, result.receipt.GlobalNativeAddsFenced)
	_, err = invoke(raceAdd)
	require.ErrorIs(t, err, ErrMemberAdmissionStale)
	var settled bool
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM public.space_members WHERE space_id=$1 AND profile_id=$2)
 AND EXISTS(SELECT 1 FROM public.space_bot_installation_heads WHERE space_id=$1 AND bot_id=$3 AND state='CANCELED' AND retired_through=1)
 AND EXISTS(SELECT 1 FROM native_membership_guard.bot_terminals WHERE bot_id=$3)`, raceAdd.SpaceId, raceAdd.ProfileId, raceBot).Scan(&settled))
	require.True(t, settled, "winner source is settled before global acknowledgement")
	var markers, receipts, frames int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM native_membership_guard.bot_terminals),(SELECT count(*) FROM native_membership_guard.bot_removal_receipts),(SELECT count(*) FROM native_membership_guard.frames)`).Scan(&markers, &receipts, &frames))
	require.Equal(t, 2, markers)
	require.Equal(t, 3, receipts)
	require.Zero(t, frames)
	for i, space := range spaces {
		var count int
		require.NoError(t, db.Pool.QueryRow(ctx, `SELECT member_count FROM public.spaces WHERE id=$1`, space.ID).Scan(&count))
		expected := 1
		if i == 0 || i == 2 {
			expected = 2
		}
		require.Equal(t, expected, count, "independent owner and other Bot source survive exact native cleanup")
	}
}
