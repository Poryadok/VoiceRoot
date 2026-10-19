package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/lifecycleprincipal"
	"voice/backend/space/internal/store"
)

// This exercises the actual public receiver, RS256 verifier and durable replay
// store. bufconn is transport-only: mTLS/JWKS and the Java Auth implementation
// are independently tested boundaries, not an assembled-runtime claim here.
func TestNativeCreatorPublicReceiverPublishesOnceAndRejectsRawMetadata(t *testing.T) {
	ctx := context.Background()
	database := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	database.Run(true, "up")
	pool := database.Pool
	st := &store.SpaceStore{Pool: pool}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	floorCalls := 0
	revoked := false
	v := lifecycleprincipal.Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, CreatorEpoch: func(context.Context, string, int64) error {
		floorCalls++
		if revoked {
			return errors.New("revoked fixture session")
		}
		return nil
	}, CreatorReplay: func(ctx context.Context, issuer, jti string, expiry time.Time) error {
		err := st.ClaimCreatorPrincipalReplay(ctx, issuer, jti, expiry)
		if errors.Is(err, store.ErrMemberAuthorityUnavailable) {
			return lifecycleprincipal.ErrDependencyUnavailable
		}
		return err
	}}
	events := &spySpaceEvents{}
	svc := &SpaceGRPC{Store: st, SpaceEvents: events}
	roleIssuer, roleKey := newOwnershipTestIssuer(t)
	roleCalls := 0
	svc.PrincipalIssuer = roleIssuer
	svc.OwnershipRoles = &creatorBootstrapRoleClient{bootstrap: func(ctx context.Context, request *rolev1.BootstrapSpaceRolesRequest) (*rolev1.BootstrapSpaceRolesResponse, error) {
		roleCalls++
		verifyOwnershipCredential(t, ctx, request, rolev1.RoleService_BootstrapSpaceRoles_FullMethodName, roleKey)
		return &rolev1.BootstrapSpaceRolesResponse{}, nil
	}}

	lis := bufconn.Listen(1 << 20)
	server := grpc.NewServer(grpc.UnaryInterceptor(v.Unary()))
	spacev1.RegisterSpaceServiceServer(server, &PublicLifecycleSpace{SpaceGRPC: svc})
	go func() { _ = server.Serve(lis) }()
	t.Cleanup(server.Stop)
	conn, err := grpc.NewClient("passthrough:///native-creator", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close(); _ = lis.Close() })
	client := spacev1.NewSpaceServiceClient(conn)
	request := &spacev1.CreateSpaceRequest{Name: "Native receiver"}
	account, profile := uuid.New(), uuid.New()
	_, err = client.CreateSpace(withAccountProfileCtx(ctx, account, profile), request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	requestID := uuid.NewString()
	token, err := issuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: spacev1.SpaceService_CreateSpace_FullMethodName, RequestID: requestID, RequestHash: hash, AccountID: account.String(), ProfileID: profile.String(), SessionEpoch: 7, AccountType: "regular", ClientExpiresAt: time.Now().Add(time.Minute)})
	require.NoError(t, err)
	signed := metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID))
	created, err := client.CreateSpace(signed, request)
	require.NoError(t, err)
	require.Equal(t, profile.String(), created.GetSpace().GetOwnerProfileId())
	require.EqualValues(t, 1, created.GetSpace().GetMemberCount())
	require.Equal(t, "private", created.GetSpace().GetVisibility())
	require.Len(t, events.snapshot(), 1)
	require.Equal(t, 1, roleCalls, "controlled protected bootstrap dispatch only")
	require.Equal(t, 2, floorCalls, "ingress and final pre-write observations")
	_, err = client.CreateSpace(signed, request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Len(t, events.snapshot(), 1, "credential replay never emits a second event")
	freshSigned := func() context.Context {
		requestID := uuid.NewString()
		token, err := issuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: spacev1.SpaceService_CreateSpace_FullMethodName, RequestID: requestID, RequestHash: hash, AccountID: account.String(), ProfileID: profile.String(), SessionEpoch: 7, AccountType: "regular", ClientExpiresAt: time.Now().Add(time.Minute)})
		require.NoError(t, err)
		return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID))
	}
	protectedRoles := svc.OwnershipRoles
	svc.OwnershipRoles = nil
	_, err = client.CreateSpace(freshSigned(), request)
	require.Equal(t, codes.Unavailable, status.Code(err), "missing protected Role dependency must fail before Space write")
	svc.OwnershipRoles = protectedRoles
	revoked = true
	_, err = client.CreateSpace(freshSigned(), request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	revoked = false
	_, err = pool.Exec(ctx, `ALTER TABLE space_members DISABLE TRIGGER space_ordinary_publication_guard`)
	require.NoError(t, err)
	_, err = client.CreateSpace(freshSigned(), request)
	require.Equal(t, codes.Unavailable, status.Code(err), "invalid native catalog must fail before Space write")
	_, err = pool.Exec(ctx, `ALTER TABLE space_members ENABLE TRIGGER space_ordinary_publication_guard`)
	require.NoError(t, err)
	require.Equal(t, 1, roleCalls, "no negative request reaches Role bootstrap")
	require.Len(t, events.snapshot(), 1)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM spaces`).Scan(&count))
	require.Equal(t, 1, count)
	observation, err := st.ReadMemberAdmissionAuthority(ctx, &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: created.GetSpace().GetId(), ProfileId: profile.String()})
	require.NoError(t, err)
	require.Equal(t, spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_OWNER_CREATE, observation.GetOrdinary().GetPublicationKind())
	require.Empty(t, observation.GetOrdinary().GetRoleReceiptSha256())
}

func TestNativeCreatorDependencyErrorCodes(t *testing.T) {
	require.Equal(t, codes.Unavailable, status.Code(mapSpaceStoreError(store.ErrMemberAuthorityUnavailable)))
	require.Equal(t, codes.FailedPrecondition, status.Code(mapSpaceStoreError(store.ErrMemberAdmissionConflict)))
}
