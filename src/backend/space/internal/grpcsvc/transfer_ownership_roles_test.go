package grpcsvc

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/role/permissions"

	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
)

// transferRoleStub wires Role Service for TransferOwnership fail-closed tests.
type transferRoleStub struct {
	rolev1.RoleServiceClient

	mu           sync.Mutex
	ownerRoleID  string
	memberRoleID string
	assignErr    error
	revokeErr    error
	listErr      error

	ownerAssignCalls int
	ownerRevokeCalls int
}

func newTransferRoleStub() *transferRoleStub {
	return &transferRoleStub{
		ownerRoleID:  "role-owner",
		memberRoleID: "role-member",
	}
}

func (s *transferRoleStub) BootstrapSpaceRoles(context.Context, *rolev1.BootstrapSpaceRolesRequest, ...grpc.CallOption) (*rolev1.BootstrapSpaceRolesResponse, error) {
	return &rolev1.BootstrapSpaceRolesResponse{}, nil
}

func (s *transferRoleStub) CheckPermission(context.Context, *rolev1.CheckPermissionRequest, ...grpc.CallOption) (*rolev1.CheckPermissionResponse, error) {
	return &rolev1.CheckPermissionResponse{Allowed: true}, nil
}

func (s *transferRoleStub) ListRoles(context.Context, *rolev1.ListRolesRequest, ...grpc.CallOption) (*rolev1.ListRolesResponse, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return &rolev1.ListRolesResponse{RoleList: &rolev1.RoleList{Roles: []*rolev1.Role{
		{Id: s.ownerRoleID, Name: permissions.RoleOwner, Position: 4},
		{Id: s.memberRoleID, Name: permissions.RoleMember, Position: 1},
	}}}, nil
}

func (s *transferRoleStub) GetDefaultJoinRole(context.Context, *rolev1.GetDefaultJoinRoleRequest, ...grpc.CallOption) (*rolev1.GetDefaultJoinRoleResponse, error) {
	return &rolev1.GetDefaultJoinRoleResponse{Role: &rolev1.Role{Id: s.memberRoleID, Name: permissions.RoleMember}}, nil
}

func (s *transferRoleStub) AssignRole(_ context.Context, req *rolev1.AssignRoleRequest, _ ...grpc.CallOption) (*rolev1.AssignRoleResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if req.GetRoleId() == s.ownerRoleID {
		s.ownerAssignCalls++
		if s.assignErr != nil {
			return nil, s.assignErr
		}
	}
	return &rolev1.AssignRoleResponse{}, nil
}

func (s *transferRoleStub) RevokeRole(_ context.Context, req *rolev1.RevokeRoleRequest, _ ...grpc.CallOption) (*rolev1.RevokeRoleResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if req.GetRoleId() == s.ownerRoleID {
		s.ownerRevokeCalls++
		if s.revokeErr != nil {
			return nil, s.revokeErr
		}
	}
	return &rolev1.RevokeRoleResponse{}, nil
}

func (s *transferRoleStub) GetMemberRoles(context.Context, *rolev1.GetMemberRolesRequest, ...grpc.CallOption) (*rolev1.GetMemberRolesResponse, error) {
	return &rolev1.GetMemberRolesResponse{}, nil
}

func setupTransferWithRoles(t *testing.T, stub *transferRoleStub) (
	client spacev1.SpaceServiceClient,
	owner uuid.UUID,
	memberProfile uuid.UUID,
	ownerCtx context.Context,
	spaceID string,
) {
	t.Helper()
	owner, _, ownerCtx = profileFixture(t)
	memberAccount, memberProfile := uuid.New(), uuid.New()
	memberCtx := withAccountProfileCtx(context.Background(), memberAccount, memberProfile)

	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool, withRoleClient(stub))
	t.Cleanup(cleanup)

	created, err := client.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Role transfer"})
	require.NoError(t, err)
	spaceID = created.GetSpace().GetId()

	inv, err := client.CreateInvite(ownerCtx, &spacev1.CreateInviteRequest{SpaceId: spaceID})
	require.NoError(t, err)
	_, err = client.JoinByInvite(memberCtx, &spacev1.JoinByInviteRequest{Code: inv.GetInvite().GetCode()})
	require.NoError(t, err)

	return client, owner, memberProfile, ownerCtx, spaceID
}

// TestTransferOwnership_RoleAssignFails_RollsBackOwnership documents fail-closed when Assign Owner fails.
func TestTransferOwnership_RoleAssignFails_RollsBackOwnership(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	stub := newTransferRoleStub()
	stub.assignErr = status.Error(codes.Unavailable, "role assign down")
	client, owner, memberProfile, ownerCtx, spaceID := setupTransferWithRoles(t, stub)

	_, err := client.TransferOwnership(ownerCtx, &spacev1.TransferOwnershipRequest{
		SpaceId:           spaceID,
		NewOwnerProfileId: memberProfile.String(),
	})
	require.Equal(t, codes.Unavailable, status.Code(err))

	got, err := client.GetSpace(ownerCtx, &spacev1.GetSpaceRequest{SpaceId: spaceID})
	require.NoError(t, err)
	require.Equal(t, owner.String(), got.GetSpace().GetOwnerProfileId())
	require.Equal(t, 1, stub.ownerAssignCalls)
	require.Equal(t, 0, stub.ownerRevokeCalls)
}

// TestTransferOwnership_RoleRevokeFails_RollsBackOwnership documents fail-closed when Revoke Owner fails.
func TestTransferOwnership_RoleRevokeFails_RollsBackOwnership(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	stub := newTransferRoleStub()
	stub.revokeErr = status.Error(codes.Internal, "role revoke failed")
	client, owner, memberProfile, ownerCtx, spaceID := setupTransferWithRoles(t, stub)

	_, err := client.TransferOwnership(ownerCtx, &spacev1.TransferOwnershipRequest{
		SpaceId:           spaceID,
		NewOwnerProfileId: memberProfile.String(),
	})
	require.Equal(t, codes.Internal, status.Code(err))

	got, err := client.GetSpace(ownerCtx, &spacev1.GetSpaceRequest{SpaceId: spaceID})
	require.NoError(t, err)
	require.Equal(t, owner.String(), got.GetSpace().GetOwnerProfileId())
	require.Equal(t, 1, stub.ownerAssignCalls)
	require.Equal(t, 1, stub.ownerRevokeCalls)
}

// TestTransferOwnership_WithRoles_ReassignsOwnerRole documents Assign+Revoke Owner on success.
func TestTransferOwnership_WithRoles_ReassignsOwnerRole(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	stub := newTransferRoleStub()
	client, _, memberProfile, ownerCtx, spaceID := setupTransferWithRoles(t, stub)

	_, err := client.TransferOwnership(ownerCtx, &spacev1.TransferOwnershipRequest{
		SpaceId:           spaceID,
		NewOwnerProfileId: memberProfile.String(),
	})
	require.NoError(t, err)

	got, err := client.GetSpace(ownerCtx, &spacev1.GetSpaceRequest{SpaceId: spaceID})
	require.NoError(t, err)
	require.Equal(t, memberProfile.String(), got.GetSpace().GetOwnerProfileId())
	require.Equal(t, 1, stub.ownerAssignCalls)
	require.Equal(t, 1, stub.ownerRevokeCalls)
}

// TestTransferOwnership_RoleListFails_RollsBackOwnership documents fail-closed when ListRoles fails.
func TestTransferOwnership_RoleListFails_RollsBackOwnership(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	stub := newTransferRoleStub()
	client, owner, memberProfile, ownerCtx, spaceID := setupTransferWithRoles(t, stub)
	stub.listErr = status.Error(codes.Unavailable, "role list down")

	_, err := client.TransferOwnership(ownerCtx, &spacev1.TransferOwnershipRequest{
		SpaceId:           spaceID,
		NewOwnerProfileId: memberProfile.String(),
	})
	require.Equal(t, codes.Unavailable, status.Code(err))

	got, err := client.GetSpace(ownerCtx, &spacev1.GetSpaceRequest{SpaceId: spaceID})
	require.NoError(t, err)
	require.Equal(t, owner.String(), got.GetSpace().GetOwnerProfileId())
	require.Equal(t, 0, stub.ownerAssignCalls)
}
