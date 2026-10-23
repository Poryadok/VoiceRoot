package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
	"net"
	"path/filepath"
	"testing"
	"time"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
	roletest "voice/backend/role/testutil"
	"voice/backend/space/internal/lifecycleprincipal"
	"voice/backend/space/internal/store"
)

func TestMemberNickname_ProtectedRPCAndActualRoleStore(t *testing.T) {
	if testing.Short() {
		t.Skip("requires test-owned PostgreSQL")
	}
	ctx := context.Background()
	dir := filepath.Join(repoRoot(t), "src", "backend", "migrations", "space_db")
	db := integrationtest.NewSourceMigrationFixture(t, ctx, dir)
	// This contract exercises nickname migration26, not native catalog cutover.
	db.Run(true, "goto", "26")
	pool := db.Pool
	st := &store.SpaceStore{Pool: pool}
	owner, actor, target, account := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Signed nickname", "", "private")
	require.NoError(t, err)
	for _, id := range []uuid.UUID{actor, target} {
		_, err = pool.Exec(ctx, `INSERT INTO space_members(space_id,profile_id) VALUES($1,$2)`, space.ID, id)
		require.NoError(t, err)
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	resolve := func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }
	rolePool := roletest.StartRoleDB(t, ctx)
	roletest.ApplyRoleMigrations(t, ctx, rolePool)
	legacy, closeLegacy := roletest.StartRoleGRPC(t, rolePool)
	t.Cleanup(closeLegacy)
	_, err = legacy.BootstrapSpaceRoles(ctx, &rolev1.BootstrapSpaceRolesRequest{SpaceId: space.ID.String(), OwnerProfileId: owner.String()})
	require.NoError(t, err)
	assignSpaceRoleByName(t, legacy, withAccountProfileCtx(ctx, account, owner), owner, space.ID.String(), actor, "Member")
	signedRole, closeRole := roletest.StartRoleGRPCWithServerOptions(t, rolePool, grpc.ChainUnaryInterceptor(func(ctx context.Context, req any, info *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		transport, err := principal.IncomingMetadata(ctx)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid role credential")
		}
		message, ok := req.(proto.Message)
		if !ok {
			return nil, status.Error(codes.InvalidArgument, "invalid role message")
		}
		hash, err := principal.RequestHash(message)
		if err != nil {
			return nil, err
		}
		p, err := principal.VerifyService(ctx, transport.BearerToken, principal.VerifyConfig{ExpectedIssuer: "space", ExpectedAudience: "role", ExpectedRPC: info.FullMethod, ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash, KeyResolver: resolve, ReplayGuard: func(context.Context, string, string, time.Time) error { return nil }})
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid role principal")
		}
		return next(principal.WithVerified(ctx, p), req)
	}))
	t.Cleanup(closeRole)
	spaceIssuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "test", PrivateKey: key})
	require.NoError(t, err)
	gatewayIssuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "test", PrivateKey: key})
	require.NoError(t, err)
	verifier := lifecycleprincipal.Verifier{Keys: resolve, Replay: func(context.Context, string, string, time.Time) error { return nil }, Epoch: func(context.Context, string, int64) error { return nil }}
	service := &SpaceGRPC{Store: st, OwnershipRoles: signedRole, PrincipalIssuer: spaceIssuer}
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer(grpc.ChainUnaryInterceptor(verifier.Unary()))
	spacev1.RegisterSpaceServiceServer(server, &PublicLifecycleSpace{SpaceGRPC: service})
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	conn, err := grpc.NewClient("passthrough:///nickname", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	client := spacev1.NewSpaceServiceClient(conn)
	call := func(who, to uuid.UUID, value *string) (*spacev1.UpdateMemberNicknameResponse, error) {
		req := &spacev1.UpdateMemberNicknameRequest{SpaceId: space.ID.String(), ProfileId: to.String(), Nickname: value}
		hash, err := principal.RequestHash(req)
		require.NoError(t, err)
		id := uuid.NewString()
		token, err := gatewayIssuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: spacev1.SpaceService_UpdateMemberNickname_FullMethodName, RequestID: id, RequestHash: hash, AccountID: account.String(), ProfileID: who.String(), SessionEpoch: 7, ClientExpiresAt: time.Now().Add(time.Minute)})
		require.NoError(t, err)
		callCtx, cancel := context.WithTimeout(metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id)), 5*time.Second)
		defer cancel()
		return client.UpdateMemberNickname(callCtx, req)
	}
	value := "  Owner 😀 "
	resp, err := call(owner, owner, &value)
	require.NoError(t, err)
	require.Equal(t, "Owner 😀", resp.GetSpaceMembership().GetNickname())
	_, err = call(actor, actor, &value)
	require.Equal(t, codes.PermissionDenied, status.Code(err), "self is not permission bypass")
	created, err := legacy.CreateRole(withAccountProfileCtx(ctx, account, owner), &rolev1.CreateRoleRequest{SpaceId: space.ID.String(), Name: "Nickname manager", PermissionsMask: 1 << 11, Position: 0})
	require.NoError(t, err)
	_, err = legacy.AssignRole(withAccountProfileCtx(ctx, account, owner), &rolev1.AssignRoleRequest{SpaceId: space.ID.String(), ProfileId: actor.String(), RoleId: created.GetRole().GetId()})
	require.NoError(t, err)
	assignSpaceRoleByName(t, legacy, withAccountProfileCtx(ctx, account, owner), owner, space.ID.String(), target, "Admin")
	_, err = call(actor, actor, &value)
	require.NoError(t, err, "granted member can edit self")
	resp, err = call(actor, target, &value)
	require.NoError(t, err)

	require.Equal(t, target.String(), resp.GetSpaceMembership().ProfileId)
	_, err = call(actor, owner, &value)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	_, err = call(uuid.New(), target, &value)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	_, err = call(owner, target, nil)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	empty := ""
	resp, err = call(owner, target, &empty)
	require.NoError(t, err)
	require.Nil(t, resp.GetSpaceMembership().Nickname)
	service.OwnershipRoles = nil
	_, err = call(owner, owner, &value)
	require.Equal(t, codes.Unavailable, status.Code(err), "owner no-op cannot bypass dependency")
}
