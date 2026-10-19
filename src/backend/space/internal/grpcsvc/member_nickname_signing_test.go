package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

type nicknamePermissionClient struct {
	rolev1.RoleServiceClient
	check func(context.Context, *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error)
}

func (c nicknamePermissionClient) CheckPermission(ctx context.Context, req *rolev1.CheckPermissionRequest, _ ...grpc.CallOption) (*rolev1.CheckPermissionResponse, error) {
	return c.check(ctx, req)
}

func TestNicknamePermissionSignsOnlyActorAndExactRequest(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "test", PrivateKey: key})
	require.NoError(t, err)
	space, actor := uuid.New(), uuid.New()
	ids := map[string]bool{}
	client := nicknamePermissionClient{check: func(ctx context.Context, req *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error) {
		require.Equal(t, space.String(), req.SpaceId)
		require.Equal(t, actor.String(), req.ProfileId)
		require.Equal(t, "MEMBER_MANAGE_NICKNAMES", req.PermissionName)
		require.Nil(t, req.Chat)
		require.Nil(t, req.VoiceRoomId)
		md, _ := metadata.FromOutgoingContext(ctx)
		require.Len(t, md, 2)
		id := md.Get("x-request-id")[0]
		require.False(t, ids[id])
		ids[id] = true
		hash, err := principal.RequestHash(req)
		require.NoError(t, err)
		p, err := principal.VerifyService(ctx, md.Get("authorization")[0][7:], principal.VerifyConfig{ExpectedIssuer: "space", ExpectedAudience: "role", ExpectedRPC: rolev1.RoleService_CheckPermission_FullMethodName, ExpectedRequestID: id, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, ReplayGuard: func(context.Context, string, string, time.Time) error { return nil }})
		require.NoError(t, err)
		require.Empty(t, p.ProfileID)
		require.Empty(t, p.AccountID)
		return &rolev1.CheckPermissionResponse{Allowed: true}, nil
	}}
	service := &SpaceGRPC{OwnershipRoles: client, PrincipalIssuer: issuer}
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-voice-profile-id", uuid.NewString(), "authorization", "Bearer client"))
	require.NoError(t, service.requireNicknamePermission(ctx, space, actor))
	require.NoError(t, service.requireNicknamePermission(ctx, space, actor))
	service.OwnershipRoles = nil
	require.Equal(t, codes.Unavailable, status.Code(service.requireNicknamePermission(ctx, space, actor)))
	service.OwnershipRoles = client
	service.PrincipalIssuer = nil
	require.Equal(t, codes.Unavailable, status.Code(service.requireNicknamePermission(ctx, space, actor)))
}
