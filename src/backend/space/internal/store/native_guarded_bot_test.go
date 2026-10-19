package store

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
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

func TestNativeBot44VerifiedServingPublicationCancelAndExpiry(t *testing.T) {
	if testing.Short() {
		t.Skip("requires real PostgreSQL signed Bot native store")
	}
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "34")
	st := &SpaceStore{Pool: db.Pool}
	space, err := st.CreateSpace(ctx, uuid.New(), "Signed native Bot", "", "public")
	require.NoError(t, err)
	db.Run(true, "up")
	require.NoError(t, st.InitializeNativeMembership(ctx, space.ID))
	st = &SpaceStore{Pool: nativeReadinessLogin(t, ctx, db.Pool)}
	decisionSchema, decisionErr := st.memberDecisionSchema(ctx)
	require.NoError(t, decisionErr, "qualified44 must select the explicit versioned decision family")
	require.EqualValues(t, 44, decisionSchema)
	require.ErrorIs(t, st.AddBotMember(ctx, space.ID, uuid.New()), ErrMemberAuthorityUnavailable, "raw Bot insertion has no installation authority")
	require.ErrorIs(t, st.RemoveBotMember(ctx, space.ID, space.OwnerProfileID), ErrMemberAuthorityUnavailable, "raw removal cannot retire a legacy source")
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Now().UTC()
	signer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: "current", PrivateKey: key, Clock: func() time.Time { return now }})
	require.NoError(t, err)
	verifier := botmembershipprincipal.Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }, Clock: func() time.Time { return now }}
	invoke := func(req proto.Message, expire bool) error {
		method := spacev1.SpaceService_AddBotMember_FullMethodName
		var operation string
		switch r := req.(type) {
		case *spacev1.AddBotMemberRequest:
			operation = r.InstallationIntent.OperationId
		case *spacev1.RemoveBotMemberRequest:
			operation = r.InstallationIntent.OperationId
			method = spacev1.SpaceService_RemoveBotMember_FullMethodName
		}
		hash, err := principal.RequestHash(req)
		require.NoError(t, err)
		token, err := signer.IssueService(principal.ServiceInput{Audience: "space", RPC: method, RequestID: operation, RequestHash: hash})
		require.NoError(t, err)
		incoming := metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", operation))
		_, err = verifier.Unary()(incoming, req, &grpc.UnaryServerInfo{FullMethod: method}, func(call context.Context, _ any) (any, error) {
			switch r := req.(type) {
			case *spacev1.AddBotMemberRequest:
				proof, err := botmembershipprincipal.RequireAdd(call, r)
				if err != nil {
					return nil, err
				}
				if expire {
					now = now.Add(time.Minute)
				}
				return nil, st.AddBotMemberWithInstallation(call, proof, r)
			case *spacev1.RemoveBotMemberRequest:
				proof, err := botmembershipprincipal.RequireRemove(call, r)
				if err != nil {
					return nil, err
				}
				return nil, st.RemoveBotMemberWithInstallation(call, proof, r)
			}
			panic("unexpected request")
		})
		return err
	}
	add := &spacev1.AddBotMemberRequest{SpaceId: space.ID.String(), ProfileId: uuid.NewString()}
	add.InstallationIntent = &spacev1.BotInstallationIntent{OperationId: uuid.NewString(), BotId: uuid.NewString(), InstallationId: uuid.NewString(), SpaceId: add.SpaceId, ActorProfileId: add.ProfileId, Generation: 1}
	require.Error(t, st.AddBotMemberWithInstallation(ctx, nil, add))
	require.NoError(t, invoke(add, false))
	require.NoError(t, invoke(add, false))
	var count int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT member_count FROM spaces WHERE id=$1`, space.ID).Scan(&count))
	require.Equal(t, 2, count)
	cancel := &spacev1.RemoveBotMemberRequest{SpaceId: add.SpaceId, ProfileId: add.ProfileId, InstallationIntent: proto.Clone(add.InstallationIntent).(*spacev1.BotInstallationIntent), InstallOperationId: add.InstallationIntent.OperationId}
	cancel.InstallationIntent.OperationId = uuid.NewString()
	require.NoError(t, invoke(cancel, false))
	require.ErrorIs(t, invoke(add, false), ErrMemberAdmissionStale)
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT member_count FROM spaces WHERE id=$1`, space.ID).Scan(&count))
	require.Equal(t, 1, count)
	next := proto.Clone(add).(*spacev1.AddBotMemberRequest)
	next.InstallationIntent.Generation = 2
	next.InstallationIntent.OperationId = uuid.NewString()
	next.InstallationIntent.InstallationId = uuid.NewString()
	require.NoError(t, invoke(next, false), "reinstall publishes a distinct native incarnation")
	require.NoError(t, invoke(cancel, false), "old cancellation replay preserves the new incarnation")
	var currentOperation string
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT admission_operation_id FROM public.space_members WHERE space_id=$1 AND profile_id=$2`, space.ID, add.ProfileId).Scan(&currentOperation))
	require.Equal(t, next.InstallationIntent.OperationId, currentOperation)
	unknown := proto.Clone(cancel).(*spacev1.RemoveBotMemberRequest)
	unknown.ProfileId = space.OwnerProfileID.String()
	unknown.InstallationIntent.ActorProfileId = unknown.ProfileId
	unknown.InstallationIntent.OperationId = uuid.NewString()
	unknown.InstallationIntent.BotId = uuid.NewString()
	unknown.InstallationIntent.InstallationId = uuid.NewString()
	unknown.InstallOperationId = uuid.NewString()
	require.ErrorIs(t, invoke(unknown, false), ErrMemberAuthorityUnavailable, "legacy source cannot become a successful cleanup receipt")
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM public.space_bot_installation_cancellations WHERE operation_id=$1`, unknown.InstallationIntent.OperationId).Scan(&count))
	require.Zero(t, count, "unknown cleanup rolls back its complete DB frame without journal effects")
	early := proto.Clone(add).(*spacev1.AddBotMemberRequest)
	early.ProfileId = uuid.NewString()
	early.InstallationIntent.ActorProfileId = early.ProfileId
	early.InstallationIntent.BotId = uuid.NewString()
	early.InstallationIntent.InstallationId = uuid.NewString()
	early.InstallationIntent.OperationId = uuid.NewString()
	earlyCancel := &spacev1.RemoveBotMemberRequest{SpaceId: early.SpaceId, ProfileId: early.ProfileId, InstallationIntent: proto.Clone(early.InstallationIntent).(*spacev1.BotInstallationIntent), InstallOperationId: early.InstallationIntent.OperationId}
	earlyCancel.InstallationIntent.OperationId = uuid.NewString()
	require.NoError(t, invoke(earlyCancel, false))
	require.NoError(t, invoke(earlyCancel, false), "exact tombstone replay is inert")
	require.ErrorIs(t, invoke(early, false), ErrMemberAdmissionStale)
	expired := proto.Clone(early).(*spacev1.AddBotMemberRequest)
	expired.InstallationIntent.Generation = 2
	expired.InstallationIntent.OperationId = uuid.NewString()
	expired.InstallationIntent.InstallationId = uuid.NewString()
	require.Error(t, invoke(expired, true))
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM space_members WHERE space_id=$1 AND profile_id=$2`, space.ID, early.ProfileId).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM native_membership_guard.frames WHERE space_id=$1`, space.ID).Scan(&count))
	require.Zero(t, count, "all committed native journal paths dispose their DB frame")
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT member_count FROM public.spaces WHERE id=$1`, space.ID).Scan(&count))
	require.Equal(t, 2, count, "owner and actual reinstall survive negative and replay paths")
}
