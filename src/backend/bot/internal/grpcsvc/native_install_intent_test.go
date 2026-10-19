package grpcsvc_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	botv1 "voice.app/voice/bot/v1"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	grpcsvc "voice/backend/bot/internal/grpcsvc"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
)

type intentObservingSpace struct {
	spacev1.SpaceServiceClient
	add    func(context.Context, *spacev1.AddBotMemberRequest) error
	remove func(context.Context, *spacev1.RemoveBotMemberRequest) error
}

type installationPermissionClient struct {
	rolev1.RoleServiceClient
	allowed bool
	err     error
}

func (s installationPermissionClient) CheckPermission(_ context.Context, r *rolev1.CheckPermissionRequest, _ ...grpc.CallOption) (*rolev1.CheckPermissionResponse, error) {
	return &rolev1.CheckPermissionResponse{Allowed: s.allowed && r.GetPermissionName() == "SPACE_MANAGE_BOTS"}, s.err
}
func (s installationPermissionClient) DeleteRolesCreatedByProfile(context.Context, *rolev1.DeleteRolesCreatedByProfileRequest, ...grpc.CallOption) (*rolev1.DeleteRolesCreatedByProfileResponse, error) {
	return &rolev1.DeleteRolesCreatedByProfileResponse{}, nil
}
func (s installationPermissionClient) GetMemberRoles(context.Context, *rolev1.GetMemberRolesRequest, ...grpc.CallOption) (*rolev1.GetMemberRolesResponse, error) {
	return &rolev1.GetMemberRolesResponse{}, nil
}

func TestNativeInstallRequiresCurrentManageBotsAdmission(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL boundary")
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "bot_native_admission", "")
	_, err := pool.Exec(ctx, migrationSQL(t))
	require.NoError(t, err)
	st := &store.BotStore{Pool: pool}
	bot, _, err := st.CreateBot(ctx, uuid.New(), "Admission", "", `[]`, uuid.New())
	require.NoError(t, err)
	svc := grpcsvc.NewBotGRPC(st, nil)
	svc.PrincipalIssuer = installationTestIssuer(t)
	calls := 0
	svc.InstallationSpace = intentObservingSpace{add: func(context.Context, *spacev1.AddBotMemberRequest) error { calls++; return nil }}
	ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("x-voice-profile-id", uuid.NewString()))
	req := &botv1.InstallBotInSpaceRequest{BotId: bot.ID.String(), SpaceId: uuid.NewString()}
	_, err = svc.InstallBotInSpace(ctx, req)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	svc.InstallationRole = installationPermissionClient{allowed: false}
	_, err = svc.InstallBotInSpace(ctx, req)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	svc.InstallationRole = installationPermissionClient{allowed: true, err: errors.New("authority unavailable")}
	_, err = svc.InstallBotInSpace(ctx, req)
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.Zero(t, calls)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM bot_installation_intents`).Scan(&count))
	require.Zero(t, count)
}

func (s intentObservingSpace) AddBotMember(ctx context.Context, req *spacev1.AddBotMemberRequest, _ ...grpc.CallOption) (*spacev1.AddBotMemberResponse, error) {
	if s.add != nil {
		return &spacev1.AddBotMemberResponse{}, s.add(ctx, req)
	}
	return &spacev1.AddBotMemberResponse{}, nil
}

func (s intentObservingSpace) RemoveBotMember(ctx context.Context, req *spacev1.RemoveBotMemberRequest, _ ...grpc.CallOption) (*spacev1.RemoveBotMemberResponse, error) {
	if s.remove != nil {
		return &spacev1.RemoveBotMemberResponse{}, s.remove(ctx, req)
	}
	return &spacev1.RemoveBotMemberResponse{}, nil
}

func installationTestIssuer(t *testing.T) *principal.Issuer {
	issuer, _ := installationTestSigningKey(t)
	return issuer
}

func installationTestSigningKey(t *testing.T) (*principal.Issuer, *rsa.PublicKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	return issuer, &key.PublicKey
}

func TestNativeInstallLostAcknowledgementsRecoverSignedExactIntent(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL boundary")
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "bot_native_retry", "")
	_, err := pool.Exec(ctx, migrationSQL(t))
	require.NoError(t, err)
	st := &store.BotStore{Pool: pool}
	actor, installer, owner, space := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	bot, _, err := st.CreateBot(ctx, owner, "Recovery", "", `[]`, actor)
	require.NoError(t, err)
	issuer, key := installationTestSigningKey(t)
	seen := map[string]bool{}
	verify := func(callCtx context.Context, req proto.Message, method, operation string) {
		md, ok := metadata.FromOutgoingContext(callCtx)
		require.True(t, ok)
		require.Empty(t, md.Get("x-voice-profile-id"))
		require.Empty(t, md.Get("x-voice-user-id"))
		require.Equal(t, []string{operation}, md.Get("x-request-id"))
		require.Len(t, md.Get("authorization"), 1)
		token := strings.TrimPrefix(md.Get("authorization")[0], "Bearer ")
		hash, e := principal.RequestHash(req)
		require.NoError(t, e)
		config := principal.VerifyConfig{ExpectedIssuer: "bot", ExpectedAudience: "space", ExpectedRPC: method, ExpectedRequestID: operation, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return key, nil }, ReplayGuard: func(_ context.Context, _ string, jti string, _ time.Time) error {
			if seen[jti] {
				return errors.New("replayed credential")
			}
			seen[jti] = true
			return nil
		}}
		p, e := principal.VerifyService(callCtx, token, config)
		require.NoError(t, e)
		require.Equal(t, "service:bot", p.Subject)
		_, e = principal.VerifyService(callCtx, token, config)
		require.Error(t, e, "same credential must be rejected")
		config.ReplayGuard = func(context.Context, string, string, time.Time) error { return nil }
		config.ExpectedRPC = spacev1.SpaceService_GetSpace_FullMethodName
		_, e = principal.VerifyService(callCtx, token, config)
		require.Error(t, e, "credential must bind exact method")
		config.ExpectedRPC = method
		config.ExpectedRequestHash = strings.Repeat("0", 64)
		_, e = principal.VerifyService(callCtx, token, config)
		require.Error(t, e, "credential must bind entire request")
	}
	var add *spacev1.AddBotMemberRequest
	var remove *spacev1.RemoveBotMemberRequest
	addCalls, removeCalls := 0, 0
	client := intentObservingSpace{add: func(c context.Context, r *spacev1.AddBotMemberRequest) error {
		verify(c, r, spacev1.SpaceService_AddBotMember_FullMethodName, r.GetInstallationIntent().GetOperationId())
		addCalls++
		if add == nil {
			add = proto.Clone(r).(*spacev1.AddBotMemberRequest)
			return status.Error(codes.Unavailable, "committed receiver response lost")
		}
		require.True(t, proto.Equal(add, r), "retry request must be byte-stable")
		return nil
	}, remove: func(c context.Context, r *spacev1.RemoveBotMemberRequest) error {
		verify(c, r, spacev1.SpaceService_RemoveBotMember_FullMethodName, r.GetInstallationIntent().GetOperationId())
		removeCalls++
		require.Equal(t, add.GetInstallationIntent().GetOperationId(), r.GetInstallOperationId())
		require.Equal(t, add.GetInstallationIntent().GetInstallationId(), r.GetInstallationIntent().GetInstallationId())
		require.NotEqual(t, r.GetInstallOperationId(), r.GetInstallationIntent().GetOperationId())
		if remove == nil {
			remove = proto.Clone(r).(*spacev1.RemoveBotMemberRequest)
			return status.Error(codes.Unavailable, "cancel committed response lost")
		}
		require.True(t, proto.Equal(remove, r))
		return nil
	}}
	newService := func() *grpcsvc.BotGRPC {
		s := grpcsvc.NewBotGRPC(&store.BotStore{Pool: pool}, nil)
		s.InstallationRole = installationPermissionClient{allowed: true}
		s.PrincipalIssuer = issuer
		s.InstallationSpace = client
		return s
	}
	ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("x-voice-user-id", owner.String(), "x-voice-profile-id", installer.String()))
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "old-user-token", "x-voice-profile-id", installer.String()))
	req := &botv1.InstallBotInSpaceRequest{BotId: bot.ID.String(), SpaceId: space.String()}
	_, err = newService().InstallBotInSpace(ctx, req)
	require.Equal(t, codes.Unavailable, status.Code(err))
	var state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM bot_installation_intents WHERE bot_id=$1`, bot.ID).Scan(&state))
	require.Equal(t, "PREPARING", state)
	response, err := newService().InstallBotInSpace(ctx, req)
	require.NoError(t, err)
	require.Equal(t, add.GetInstallationIntent().GetInstallationId(), response.GetInstallationId())
	duplicate, err := newService().InstallBotInSpace(ctx, req)
	require.NoError(t, err)
	require.Equal(t, response.GetInstallationId(), duplicate.GetInstallationId())
	uninstall := &botv1.UninstallBotFromSpaceRequest{BotId: bot.ID.String(), SpaceId: space.String()}
	_, err = newService().UninstallBotFromSpace(ctx, uninstall)
	require.Equal(t, codes.Unavailable, status.Code(err))
	_, err = newService().InstallBotInSpace(ctx, req)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	_, err = newService().UninstallBotFromSpace(ctx, uninstall)
	require.NoError(t, err)
	_, err = newService().UninstallBotFromSpace(ctx, uninstall)
	require.NoError(t, err)
	require.Equal(t, 3, addCalls)
	require.Equal(t, 2, removeCalls)
	require.Len(t, seen, 5, "each stable retry requires a new JTI")
}

func TestNativeInstallPersistsIdentityBeforeSpaceRPC(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL boundary")
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "bot_native_intent", "")
	_, err := pool.Exec(ctx, migrationSQL(t))
	require.NoError(t, err)
	st := &store.BotStore{Pool: pool}
	actor, installer, owner, space := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	bot, _, err := st.CreateBot(ctx, owner, "IntentBot", "", `["TEXT_CHAT_SEND_MESSAGES"]`, actor)
	require.NoError(t, err)
	svc := grpcsvc.NewBotGRPC(st, nil)
	svc.InstallationRole = installationPermissionClient{allowed: true}
	svc.PrincipalIssuer = installationTestIssuer(t)
	called := false
	svc.InstallationSpace = intentObservingSpace{add: func(_ context.Context, req *spacev1.AddBotMemberRequest) error {
		called = true
		var present bool
		require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass('bot_installation_intents') IS NOT NULL`).Scan(&present))
		require.True(t, present, "Space was called before any durable installation intent existed")
		var state string
		require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM bot_installation_intents WHERE bot_id=$1 AND space_id=$2`, bot.ID, space).Scan(&state))
		require.Equal(t, "PREPARING", state)
		require.Equal(t, actor.String(), req.GetProfileId())
		require.Equal(t, bot.ID.String(), req.GetInstallationIntent().GetBotId())
		require.Equal(t, space.String(), req.GetInstallationIntent().GetSpaceId())
		require.Equal(t, actor.String(), req.GetInstallationIntent().GetActorProfileId())
		require.Equal(t, uint64(1), req.GetInstallationIntent().GetGeneration())
		return nil
	}}
	ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("x-voice-user-id", owner.String(), "x-voice-profile-id", installer.String()))
	_, err = svc.InstallBotInSpace(ctx, &botv1.InstallBotInSpaceRequest{BotId: bot.ID.String(), SpaceId: space.String()})
	require.NoError(t, err)
	require.True(t, called)
}

func TestNativeInstallStaleReceiverOutcomeDoesNotActivate(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL receiver boundary")
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "bot_stale_install", "")
	_, err := pool.Exec(ctx, migrationSQL(t))
	require.NoError(t, err)
	st := &store.BotStore{Pool: pool}
	bot, _, err := st.CreateBot(ctx, uuid.New(), "Stale", "", `[]`, uuid.New())
	require.NoError(t, err)
	svc := grpcsvc.NewBotGRPC(st, nil)
	svc.InstallationRole = installationPermissionClient{allowed: true}
	svc.PrincipalIssuer = installationTestIssuer(t)
	svc.InstallationSpace = intentObservingSpace{add: func(context.Context, *spacev1.AddBotMemberRequest) error {
		return status.Error(codes.FailedPrecondition, "historical add is retired")
	}}
	ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("x-voice-profile-id", uuid.NewString()))
	_, err = svc.InstallBotInSpace(ctx, &botv1.InstallBotInSpaceRequest{BotId: bot.ID.String(), SpaceId: uuid.NewString()})
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "preserve definitive stale outcome; never reinterpret an old receipt as current activation")
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM bot_space_installations WHERE bot_id=$1`, bot.ID).Scan(&count))
	require.Zero(t, count)
	var state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM bot_installation_intents WHERE bot_id=$1`, bot.ID).Scan(&state))
	require.Equal(t, "PREPARING", state)
}
