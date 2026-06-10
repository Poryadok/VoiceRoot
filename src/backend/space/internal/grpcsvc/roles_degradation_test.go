package grpcsvc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
)

type unavailableRoleClient struct {
	rolev1.RoleServiceClient
}

func (unavailableRoleClient) BootstrapSpaceRoles(context.Context, *rolev1.BootstrapSpaceRolesRequest, ...grpc.CallOption) (*rolev1.BootstrapSpaceRolesResponse, error) {
	return &rolev1.BootstrapSpaceRolesResponse{}, nil
}

func (unavailableRoleClient) CheckPermission(context.Context, *rolev1.CheckPermissionRequest, ...grpc.CallOption) (*rolev1.CheckPermissionResponse, error) {
	return nil, status.Error(codes.Unavailable, "role service down")
}

func (unavailableRoleClient) ListRoles(context.Context, *rolev1.ListRolesRequest, ...grpc.CallOption) (*rolev1.ListRolesResponse, error) {
	return &rolev1.ListRolesResponse{RoleList: &rolev1.RoleList{Roles: []*rolev1.Role{
		{Id: "r1", Name: "Owner", Position: 4},
		{Id: "r2", Name: "Admin", Position: 3},
		{Id: "r3", Name: "Member", Position: 1},
	}}}, nil
}

func (unavailableRoleClient) AssignRole(context.Context, *rolev1.AssignRoleRequest, ...grpc.CallOption) (*rolev1.AssignRoleResponse, error) {
	return &rolev1.AssignRoleResponse{}, nil
}

func (unavailableRoleClient) GetMemberRoles(context.Context, *rolev1.GetMemberRolesRequest, ...grpc.CallOption) (*rolev1.GetMemberRolesResponse, error) {
	return &rolev1.GetMemberRolesResponse{}, nil
}

// TestCreateInvite_RoleUnavailable documents OPERATIONS.md Tier 1 fail-closed when Role CheckPermission is down.
func TestCreateInvite_RoleUnavailable(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ownerCtx := profileFixture(t)
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool, withRoleClient(unavailableRoleClient{}))
	t.Cleanup(cleanup)

	created, err := client.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Degraded"})
	require.NoError(t, err)

	_, err = client.CreateInvite(ownerCtx, &spacev1.CreateInviteRequest{
		SpaceId: created.GetSpace().GetId(),
	})
	require.Equal(t, codes.Unavailable, status.Code(err))
}
