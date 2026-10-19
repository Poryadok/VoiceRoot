package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/authctx"
	"voice/backend/space/internal/lifecycleprincipal"
	"voice/backend/space/internal/store"
)

type signedCreatorFixtureClient struct {
	spacev1.SpaceServiceClient
	issuer *principal.Issuer
	epoch  int64
}

// Actual Creator receiver and local PostgreSQL publication/replay path. The
// owner epoch response is controlled fixture evidence; this is not a Java Auth,
// Gateway HTTP, Role protected-transport or assembled runtime qualification.
func startSignedCreatorGRPCTestServer(t *testing.T, pool *pgxpool.Pool, options ...spaceServerOption) (spacev1.SpaceServiceClient, func()) {
	t.Helper()
	st := &store.SpaceStore{Pool: pool}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	verifier := lifecycleprincipal.Verifier{
		Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil },
		CreatorEpoch: func(_ context.Context, account string, epoch int64) error {
			if id, err := uuid.Parse(account); err != nil || id == uuid.Nil || epoch != 7 {
				return errors.New("fixture session does not match owner observation")
			}
			return nil
		},
		CreatorReplay: func(ctx context.Context, issuer, jti string, expiry time.Time) error {
			err := st.ClaimCreatorPrincipalReplay(ctx, issuer, jti, expiry)
			if errors.Is(err, store.ErrMemberAuthorityUnavailable) {
				return lifecycleprincipal.ErrDependencyUnavailable
			}
			return err
		},
	}
	svc := &SpaceGRPC{Store: st}
	roleIssuer, roleKey := newOwnershipTestIssuer(t)
	svc.PrincipalIssuer = roleIssuer
	svc.OwnershipRoles = &creatorBootstrapRoleClient{bootstrap: func(ctx context.Context, request *rolev1.BootstrapSpaceRolesRequest) (*rolev1.BootstrapSpaceRolesResponse, error) {
		verifyOwnershipCredential(t, ctx, request, rolev1.RoleService_BootstrapSpaceRoles_FullMethodName, roleKey)
		// Component-only controlled dependency. Real protected Role composition
		// and Owner ACL qualification remain separate from this Creator fixture.
		if svc.Roles != nil {
			return svc.Roles.BootstrapSpaceRoles(ctx, request)
		}
		return &rolev1.BootstrapSpaceRolesResponse{}, nil
	}}
	for _, option := range options {
		option(svc)
	}
	lis := bufconn.Listen(1 << 20)
	server := grpc.NewServer(grpc.UnaryInterceptor(verifier.Unary()))
	spacev1.RegisterSpaceServiceServer(server, &PublicLifecycleSpace{SpaceGRPC: svc})
	go func() { _ = server.Serve(lis) }()
	conn, err := grpc.NewClient("passthrough:///signed-creator-fixture", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	return &signedCreatorFixtureClient{SpaceServiceClient: spacev1.NewSpaceServiceClient(conn), issuer: issuer, epoch: 7}, func() {
		_ = conn.Close()
		server.Stop()
		_ = lis.Close()
	}
}

// Signing is explicit at every creation assertion. The embedded client keeps
// its real CreateSpace method, so raw credential-negative calls remain raw.
func signedCreateSpaceForTest(t *testing.T, client spacev1.SpaceServiceClient, ctx context.Context, request *spacev1.CreateSpaceRequest) (*spacev1.CreateSpaceResponse, error) {
	t.Helper()
	return signedCreateSpaceForAccountTypeForTest(t, client, ctx, request, "regular")
}

func signedCreateSpaceForAccountTypeForTest(t *testing.T, client spacev1.SpaceServiceClient, ctx context.Context, request *spacev1.CreateSpaceRequest, accountType string) (*spacev1.CreateSpaceResponse, error) {
	t.Helper()
	fixture, ok := client.(*signedCreatorFixtureClient)
	require.True(t, ok)
	md, ok := metadata.FromOutgoingContext(ctx)
	require.True(t, ok)
	actor := metadata.NewIncomingContext(ctx, md.Copy())
	account, ok := authctx.AccountID(actor)
	require.True(t, ok)
	profile, ok := authctx.ProfileID(actor)
	require.True(t, ok)
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	requestID := uuid.NewString()
	token, err := fixture.issuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: spacev1.SpaceService_CreateSpace_FullMethodName, RequestID: requestID, RequestHash: hash, AccountID: account.String(), ProfileID: profile.String(), SessionEpoch: fixture.epoch, AccountType: accountType, ClientExpiresAt: time.Now().Add(time.Minute)})
	require.NoError(t, err)
	signed := metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID))
	return client.CreateSpace(signed, request)
}
