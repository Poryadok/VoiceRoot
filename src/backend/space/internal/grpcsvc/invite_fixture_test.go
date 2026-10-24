package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"strings"
	"sync"
	"testing"
	"time"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/role/permissions"
	"voice/backend/space/internal/lifecycleprincipal"
)

// Explicit test identity is supplied by the fixture creator, never recovered
// from untrusted transport metadata. All CreateInvite calls use the real verifier.
type inviteFixtureActorKey struct{}
type inviteFixtureActor struct{ account, profile uuid.UUID }
type inviteIngressFixture struct {
	t              *testing.T
	key            *rsa.PrivateKey
	gateway, space *principal.Issuer
	verifier       lifecycleprincipal.Verifier
}

func newInviteIngressFixture(t *testing.T) *inviteIngressFixture {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	gateway, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "fixture", PrivateKey: key})
	require.NoError(t, err)
	space, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "fixture", PrivateKey: key})
	require.NoError(t, err)
	var mu sync.Mutex
	seen := map[string]bool{}
	replay := func(_ context.Context, issuer, id string, _ time.Time) error {
		mu.Lock()
		defer mu.Unlock()
		if seen[issuer+id] {
			return status.Error(codes.Unauthenticated, "replay")
		}
		seen[issuer+id] = true
		return nil
	}
	return &inviteIngressFixture{t: t, key: key, gateway: gateway, space: space, verifier: lifecycleprincipal.Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: replay, Epoch: func(context.Context, string, int64) error { return nil }}}
}

type inviteFixtureClient struct {
	spacev1.SpaceServiceClient
	fixture *inviteIngressFixture
}

func (c inviteFixtureClient) CreateInvite(ctx context.Context, r *spacev1.CreateInviteRequest, opts ...grpc.CallOption) (*spacev1.CreateInviteResponse, error) {
	if actor, ok := ctx.Value(inviteFixtureActorKey{}).(inviteFixtureActor); ok {
		hash, err := principal.RequestHash(r)
		if err != nil {
			return nil, err
		}
		id := uuid.NewString()
		token, err := c.fixture.gateway.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: spacev1.SpaceService_CreateInvite_FullMethodName, RequestID: id, RequestHash: hash, AccountID: actor.account.String(), ProfileID: actor.profile.String(), AccountType: "regular", SessionEpoch: 1, ClientExpiresAt: time.Now().Add(time.Minute)})
		if err != nil {
			return nil, err
		}
		ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
	}
	return c.SpaceServiceClient.CreateInvite(ctx, r, opts...)
}
func (f *inviteIngressFixture) serve(ctx context.Context, r any, info *grpc.UnaryServerInfo, svc *SpaceGRPC) (any, error) {
	return f.verifier.Unary()(ctx, r, info, func(ctx context.Context, r any) (any, error) {
		// Isolate the controlled permission dependency to this method. Other legacy
		// semantic fixtures retain their existing ownership and lifecycle dependencies.
		inviteService := &SpaceGRPC{Store: svc.Store, SpaceEvents: svc.SpaceEvents, PrincipalIssuer: f.space,
			OwnershipRoles: invitePermissionFixture{RoleServiceClient: svc.Roles, svc: svc, fixture: f}}
		return inviteService.CreateInvite(ctx, r.(*spacev1.CreateInviteRequest))
	})
}

type invitePermissionFixture struct {
	rolev1.RoleServiceClient
	svc     *SpaceGRPC
	fixture *inviteIngressFixture
}

func (c invitePermissionFixture) CheckPermission(ctx context.Context, r *rolev1.CheckPermissionRequest, _ ...grpc.CallOption) (*rolev1.CheckPermissionResponse, error) {
	md, _ := metadata.FromOutgoingContext(ctx)
	hash, err := principal.RequestHash(r)
	if err != nil {
		return nil, err
	}
	if len(md.Get("authorization")) != 1 || len(md.Get("x-request-id")) != 1 {
		return nil, status.Error(codes.Unauthenticated, "missing signed decision")
	}
	_, err = principal.VerifyService(ctx, strings.TrimPrefix(md.Get("authorization")[0], "Bearer "), principal.VerifyConfig{ExpectedIssuer: "space", ExpectedAudience: "role", ExpectedRPC: rolev1.RoleService_CheckPermission_FullMethodName, ExpectedRequestID: md.Get("x-request-id")[0], ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &c.fixture.key.PublicKey, nil }, ReplayGuard: c.fixture.verifier.Replay})
	if err != nil {
		return nil, err
	}
	if r.PermissionName != permissions.SpaceManageInvites || r.Chat != nil || r.VoiceRoomId != nil {
		return nil, status.Error(codes.PermissionDenied, "wrong fixture purpose")
	}
	if c.RoleServiceClient != nil {
		return c.RoleServiceClient.CheckPermission(ctx, r)
	}
	// Owner-only controlled dependency for fixtures that do not start Role.
	row, err := c.svc.Store.GetSpace(ctx, uuid.MustParse(r.SpaceId))
	if err != nil {
		return nil, err
	}
	return &rolev1.CheckPermissionResponse{Allowed: row != nil && row.OwnerProfileID.String() == r.ProfileId}, nil
}

// These tests deliberately exercise the prior legacy Role semantic fixture. Actual
// native25 qualified decision coverage lives in Role's current-Space acceptance.
type legacyInviteRoleFixture struct{ rolev1.RoleServiceClient }

func (c legacyInviteRoleFixture) CheckPermission(ctx context.Context, r *rolev1.CheckPermissionRequest, opts ...grpc.CallOption) (*rolev1.CheckPermissionResponse, error) {
	if r.PermissionName != permissions.SpaceManageInvites {
		return c.RoleServiceClient.CheckPermission(ctx, r, opts...)
	}
	result, err := c.GetEffectivePermissions(metadata.NewOutgoingContext(ctx, metadata.MD{}), &rolev1.GetEffectivePermissionsRequest{SpaceId: r.SpaceId, ProfileId: r.ProfileId})
	if err != nil {
		return nil, err
	}
	allowed, err := permissions.HasPermission(result.GetPermissionSet().GetEffectiveMask(), permissions.SpaceManageInvites)
	return &rolev1.CheckPermissionResponse{Allowed: allowed}, err
}
