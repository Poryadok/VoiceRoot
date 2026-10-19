package grpcsvc

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	rolev1 "voice.app/voice/role/v1"
	"voice/backend/role/internal/roleevents"
	"voice/backend/role/permissions"
)

type adminAuthorityEvents struct {
	roleevents.NoopPublisher
	updated atomic.Int64
}

func (e *adminAuthorityEvents) PublishRoleUpdated(context.Context, string, string, []string) error {
	e.updated.Add(1)
	return nil
}

func TestAdminAndDelegatedRoleManagersStayBelowTheirHierarchy(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL Role authorization")
	}
	s, cleanup := startRoleStoreTest(t)
	defer cleanup()
	events := &adminAuthorityEvents{}
	client, stop := startRoleGRPCTestServer(t, s.Pool, func(svc *RoleGRPC) { svc.Events = events })
	defer stop()
	ctx := context.Background()
	space, owner := uuid.New(), uuid.New()
	require.NoError(t, s.BootstrapSpaceRoles(ctx, space, owner))
	ids := roleIDsByName(t, s, space)
	manage, err := permissions.MaskFor(permissions.SpaceManageRoles)
	require.NoError(t, err)
	assign, err := permissions.MaskFor(permissions.MemberAssignRoles)
	require.NoError(t, err)
	for _, tc := range []struct {
		name           string
		mask           uint64
		admin, allowed bool
		top            int32
	}{
		{"Admin", 0, true, true, 3},
		{"delegated manager", manage, false, true, 2},
		{"assignment only", assign, false, false, 2},
		{"no management permission", 0, false, false, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actor := uuid.New()
			actorRole := ids[permissions.RoleAdmin]
			if !tc.admin {
				role, err := s.CreateCustomRole(ctx, space, tc.name, tc.mask, tc.top, &owner)
				require.NoError(t, err)
				actorRole = role.ID
			}
			require.NoError(t, s.AssignMemberRole(ctx, space, actor, actorRole, owner))
			if tc.name == "no management permission" {
				require.NoError(t, s.SetChatOverride(ctx, uuid.New(), actorRole, manage, 0))
			}
			assignmentRole, err := s.CreateCustomRole(ctx, space, "assignable "+tc.name, 0, 0, &owner)
			require.NoError(t, err)
			subject := uuid.New()
			_, err = client.AssignRole(ctxWithProfile(actor), &rolev1.AssignRoleRequest{SpaceId: space.String(), ProfileId: subject.String(), RoleId: assignmentRole.ID.String()})
			if tc.admin || tc.mask&assign != 0 {
				require.NoError(t, err)
				_, err = client.RevokeRole(ctxWithProfile(actor), &rolev1.RevokeRoleRequest{SpaceId: space.String(), ProfileId: subject.String(), RoleId: assignmentRole.ID.String()})
				require.NoError(t, err)
			} else {
				require.Equal(t, codes.PermissionDenied, status.Code(err), "CRUD permission must not substitute for assignment permission")
			}
			created, err := client.CreateRole(ctxWithProfile(actor), &rolev1.CreateRoleRequest{SpaceId: space.String(), Name: "lower " + tc.name, Position: 0})
			if !tc.allowed {
				require.Equal(t, codes.PermissionDenied, status.Code(err))
				return
			}
			require.NoError(t, err)
			id := created.GetRole().GetId()
			for _, position := range []int32{tc.top, tc.top + 1} {
				before := events.updated.Load()
				_, err := client.UpdateRole(ctxWithProfile(actor), &rolev1.UpdateRoleRequest{RoleId: id, Position: &position})
				require.Equal(t, codes.PermissionDenied, status.Code(err), "proposed position must stay below actor")
				require.Equal(t, before, events.updated.Load(), "denied update must not emit a success event")
				stored, err := s.GetRoleByID(ctx, uuid.MustParse(id))
				require.NoError(t, err)
				require.Zero(t, stored.Position, "denied update must not change state")
			}
			position := int32(1)
			_, err = client.UpdateRole(ctxWithProfile(actor), &rolev1.UpdateRoleRequest{RoleId: id, Position: &position})
			require.NoError(t, err)
			_, err = client.ReorderRoles(ctxWithProfile(actor), &rolev1.ReorderRolesRequest{SpaceId: space.String(), OrderedRoleIds: []string{id}})
			require.NoError(t, err)
			_, err = client.DeleteRole(ctxWithProfile(actor), &rolev1.DeleteRoleRequest{RoleId: id})
			require.NoError(t, err)
			_, err = client.UpdateRole(ctxWithProfile(actor), &rolev1.UpdateRoleRequest{RoleId: actorRole.String()})
			require.Equal(t, codes.PermissionDenied, status.Code(err), "equal/system role cannot be edited")
		})
	}
	ownerRole, err := s.CreateCustomRole(ctx, space, "Owner hierarchy bypass", 0, 1, &owner)
	require.NoError(t, err)
	position := int32(9)
	_, err = client.UpdateRole(ctxWithProfile(owner), &rolev1.UpdateRoleRequest{RoleId: ownerRole.ID.String(), Position: &position})
	require.NoError(t, err, "genuine Owner retains its existing hierarchy bypass")
}

func TestAllOrdinaryBitsNeverAuthorizeGenericOwnerMutation(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL Owner identity")
	}
	s, cleanup := startRoleStoreTest(t)
	defer cleanup()
	client, stop := startRoleGRPCTestServer(t, s.Pool)
	defer stop()
	ctx := context.Background()
	space, owner, admin, customActor, target := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, s.BootstrapSpaceRoles(ctx, space, owner))
	ids := roleIDsByName(t, s, space)
	require.NoError(t, s.AssignMemberRole(ctx, space, admin, ids[permissions.RoleAdmin], owner))
	all, err := permissions.AllMask()
	require.NoError(t, err)
	custom, err := s.CreateCustomRole(ctx, space, "all ordinary bits", all, 2, &owner)
	require.NoError(t, err)
	require.NoError(t, s.AssignMemberRole(ctx, space, customActor, custom.ID, owner))
	for _, actor := range []uuid.UUID{owner, admin, customActor} {
		_, err := client.AssignRole(ctxWithProfile(actor), &rolev1.AssignRoleRequest{SpaceId: space.String(), ProfileId: target.String(), RoleId: ids[permissions.RoleOwner].String()})
		require.Equal(t, codes.PermissionDenied, status.Code(err))
		_, err = client.RevokeRole(ctxWithProfile(actor), &rolev1.RevokeRoleRequest{SpaceId: space.String(), ProfileId: owner.String(), RoleId: ids[permissions.RoleOwner].String()})
		require.Equal(t, codes.PermissionDenied, status.Code(err))
	}
	var owners int
	require.NoError(t, s.Pool.QueryRow(ctx, `SELECT count(*) FROM member_roles WHERE space_id=$1 AND role_id=$2 AND profile_id=$3`, space, ids[permissions.RoleOwner], owner).Scan(&owners))
	require.Equal(t, 1, owners)
}
