package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	rolev1 "voice.app/voice/role/v1"
)

type creatorBootstrapRoleClient struct {
	rolev1.RoleServiceClient
	bootstrap func(context.Context, *rolev1.BootstrapSpaceRolesRequest) (*rolev1.BootstrapSpaceRolesResponse, error)
}

func (c *creatorBootstrapRoleClient) BootstrapSpaceRoles(ctx context.Context, req *rolev1.BootstrapSpaceRolesRequest, _ ...grpc.CallOption) (*rolev1.BootstrapSpaceRolesResponse, error) {
	return c.bootstrap(ctx, req)
}

func TestCreatorBootstrapUsesProtectedBoundServiceCaller(t *testing.T) {
	issuer, key := newOwnershipTestIssuer(t)
	space, owner := uuid.New(), uuid.New()
	calls := 0
	client := &creatorBootstrapRoleClient{bootstrap: func(ctx context.Context, req *rolev1.BootstrapSpaceRolesRequest) (*rolev1.BootstrapSpaceRolesResponse, error) {
		calls++
		require.Equal(t, space.String(), req.SpaceId)
		require.Equal(t, owner.String(), req.OwnerProfileId)
		verifyOwnershipCredential(t, ctx, req, rolev1.RoleService_BootstrapSpaceRoles_FullMethodName, key)
		return &rolev1.BootstrapSpaceRolesResponse{}, nil
	}}
	svc := &SpaceGRPC{Roles: &creatorBootstrapRoleClient{}, OwnershipRoles: client, PrincipalIssuer: issuer}
	poisoned := metadata.Pairs("authorization", "Bearer user", "x-request-id", "untrusted", "x-voice-profile-id", uuid.NewString())
	ctx := metadata.NewOutgoingContext(context.Background(), poisoned)
	require.NoError(t, svc.bootstrapCreatorRoles(ctx, space, owner))
	require.Equal(t, 1, calls, "exactly one dispatch, no ordinary fallback")
	for _, name := range []string{"missing_issuer", "missing_protected_client"} {
		t.Run(name, func(t *testing.T) {
			s := &SpaceGRPC{Roles: svc.Roles, OwnershipRoles: client, PrincipalIssuer: issuer}
			if name == "missing_issuer" {
				s.PrincipalIssuer = nil
			} else {
				s.OwnershipRoles = nil
			}
			require.Equal(t, codes.Unavailable, status.Code(s.bootstrapCreatorRoles(ctx, space, owner)))
			require.Equal(t, 1, calls)
		})
	}
}
