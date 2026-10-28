package grpcsvc

import (
	"context"
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
)

// This peer checks the real signed outbound request; the public matrix owns
// composition proof against Role's fresh native membership qualifier.
type communityRolePrincipalProbe struct {
	rolev1.RoleServiceClient
	t              *testing.T
	key            *rsa.PublicKey
	space, profile string
	reply          *rolev1.GetMemberRolesResponse
	err            error
	calls          int
	jtis           []string
}

type deniedCommunityRoleClient struct{ rolev1.RoleServiceClient }

func (deniedCommunityRoleClient) GetMemberRoles(context.Context, *rolev1.GetMemberRolesRequest, ...grpc.CallOption) (*rolev1.GetMemberRolesResponse, error) {
	return nil, status.Error(codes.Unauthenticated, "protected verified Role actor listener required")
}

func (p *communityRolePrincipalProbe) GetMemberRoles(ctx context.Context, req *rolev1.GetMemberRolesRequest, _ ...grpc.CallOption) (*rolev1.GetMemberRolesResponse, error) {
	p.t.Helper()
	p.calls++
	require.Equal(p.t, p.space, req.SpaceId)
	require.Equal(p.t, p.profile, req.ProfileId)
	claims := verifyOwnershipCredential(p.t, ctx, req, rolev1.RoleService_GetMemberRoles_FullMethodName, p.key)
	p.jtis = append(p.jtis, claims.JWTID)
	md, _ := metadata.FromOutgoingContext(ctx)
	require.Len(p.t, md, 2)
	deadline, ok := ctx.Deadline()
	require.True(p.t, ok)
	require.LessOrEqual(p.t, time.Until(deadline), 5*time.Second)
	return p.reply, p.err
}

func TestCommunityMemberRoleUsesProtectedQualifiedProjection(t *testing.T) {
	issuer, key := newOwnershipTestIssuer(t)
	space, profile, roleID := uuid.New(), uuid.New(), uuid.New()
	member := &rolev1.Role{Id: roleID.String(), SpaceId: space.String(), Name: "Member", Managed: true}
	probe := &communityRolePrincipalProbe{t: t, key: key, space: space.String(), profile: profile.String(), reply: &rolev1.GetMemberRolesResponse{RoleList: &rolev1.RoleList{Roles: []*rolev1.Role{member}}}}
	// Any ordinary or generic mutation RPC would panic on this nil embedding.
	svc := &SpaceGRPC{Roles: deniedCommunityRoleClient{}, OwnershipRoles: probe, PrincipalIssuer: issuer}
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer inbound", "x-request-id", "inbound", "x-profile-id", profile.String()))
	require.NoError(t, svc.ensureCommunityMemberRole(ctx, space, profile))
	require.NoError(t, svc.ensureCommunityMemberRole(ctx, space, profile))
	require.Equal(t, 2, probe.calls)
	require.NotEqual(t, probe.jtis[0], probe.jtis[1])
	for _, failure := range []string{"missing client", "missing signer", "nil reply", "nil role list", "unknown reply", "unknown list", "unknown role", "missing Member", "custom Member", "wrong Space", "invalid role ID", "Role unavailable"} {
		t.Run(failure, func(t *testing.T) {
			candidate := &SpaceGRPC{Roles: svc.Roles, OwnershipRoles: probe, PrincipalIssuer: issuer}
			role := &rolev1.Role{Id: roleID.String(), SpaceId: space.String(), Name: "Member", Managed: true}
			probe.reply = &rolev1.GetMemberRolesResponse{RoleList: &rolev1.RoleList{Roles: []*rolev1.Role{role}}}
			probe.err = nil
			switch failure {
			case "missing client":
				candidate.OwnershipRoles = nil
			case "missing signer":
				candidate.PrincipalIssuer = nil
			case "nil reply":
				probe.reply = nil
			case "nil role list":
				probe.reply.RoleList = nil
			case "unknown reply":
				probe.reply.ProtoReflect().SetUnknown([]byte{0x18, 1})
			case "unknown list":
				probe.reply.RoleList.ProtoReflect().SetUnknown([]byte{0x18, 1})
			case "unknown role":
				role.ProtoReflect().SetUnknown([]byte{0x78, 1})
			case "missing Member":
				probe.reply.RoleList.Roles = nil
			case "custom Member":
				role.Managed = false
			case "wrong Space":
				role.SpaceId = uuid.NewString()
			case "invalid role ID":
				role.Id = "member-role"
			case "Role unavailable":
				probe.err = status.Error(codes.Unavailable, "authority unavailable")
			}
			require.Equal(t, codes.Unavailable, status.Code(candidate.ensureCommunityMemberRole(ctx, space, profile)))
		})
	}
}
