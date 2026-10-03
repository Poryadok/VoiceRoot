package grpcsvc

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"voice/backend/role/permissions"
	"voice/backend/space/internal/authctx"

	rolev1 "voice.app/voice/role/v1"
)

func (s *SpaceGRPC) requireSpacePermission(ctx context.Context, spaceID uuid.UUID, permission string) error {
	if s == nil || s.Store == nil {
		return status.Error(codes.FailedPrecondition, "space persistence not configured")
	}
	if err := s.Store.CheckOwnershipAvailable(ctx, spaceID); err != nil {
		return mapSpaceStoreError(err)
	}
	caller, ok := authctx.ProfileID(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing profile")
	}
	if s.Roles == nil {
		return s.requireSpaceOwner(ctx, spaceID)
	}
	resp, err := s.Roles.CheckPermission(ctx, &rolev1.CheckPermissionRequest{
		SpaceId:        spaceID.String(),
		ProfileId:      caller.String(),
		PermissionName: permission,
	})
	if err != nil {
		if status.Code(err) == codes.PermissionDenied {
			return status.Error(codes.PermissionDenied, "permission denied")
		}
		return status.Error(codes.Unavailable, "role service unavailable")
	}
	if !resp.GetAllowed() {
		return status.Error(codes.PermissionDenied, "permission denied")
	}
	return nil
}

func (s *SpaceGRPC) requireSpaceOwner(ctx context.Context, spaceID uuid.UUID) error {
	caller, ok := authctx.ProfileID(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing profile")
	}
	if s == nil || s.Store == nil {
		return status.Error(codes.FailedPrecondition, "space persistence not configured")
	}
	row, err := s.Store.GetSpace(ctx, spaceID)
	if err != nil {
		return mapSpaceStoreError(err)
	}
	if row == nil {
		return status.Error(codes.NotFound, "space not found")
	}
	if row.OwnerProfileID != caller {
		return status.Error(codes.PermissionDenied, "space owner required")
	}
	return nil
}

func (s *SpaceGRPC) bootstrapSpaceRoles(ctx context.Context, spaceID, ownerProfileID uuid.UUID) error {
	if s.Roles == nil {
		return nil
	}
	_, err := s.Roles.BootstrapSpaceRoles(ctx, &rolev1.BootstrapSpaceRolesRequest{
		SpaceId:        spaceID.String(),
		OwnerProfileId: ownerProfileID.String(),
	})
	return err
}

func (s *SpaceGRPC) assignDefaultMemberRole(ctx context.Context, spaceID, profileID uuid.UUID) error {
	if s.Roles == nil {
		return nil
	}
	spaceRow, err := s.Store.GetSpace(ctx, spaceID)
	if err != nil || spaceRow == nil {
		return status.Error(codes.Internal, "space not found for role assignment")
	}
	resp, err := s.Roles.GetDefaultJoinRole(ctx, &rolev1.GetDefaultJoinRoleRequest{SpaceId: spaceID.String()})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			list, listErr := s.Roles.ListRoles(ctx, &rolev1.ListRolesRequest{SpaceId: spaceID.String()})
			if listErr != nil {
				return listErr
			}
			for _, r := range list.GetRoleList().GetRoles() {
				if r.GetName() == permissions.RoleMember {
					resp = &rolev1.GetDefaultJoinRoleResponse{Role: r}
					break
				}
			}
		} else {
			return err
		}
	}
	if resp == nil || resp.GetRole() == nil || resp.GetRole().GetId() == "" {
		return status.Error(codes.FailedPrecondition, "default join role not found")
	}
	memberRoleID := resp.GetRole().GetId()
	ownerCtx := metadata.AppendToOutgoingContext(ctx, authctx.HeaderProfileID, spaceRow.OwnerProfileID.String())
	_, err = s.Roles.AssignRole(ownerCtx, &rolev1.AssignRoleRequest{
		SpaceId:   spaceID.String(),
		ProfileId: profileID.String(),
		RoleId:    memberRoleID,
	})
	return err
}

func (s *SpaceGRPC) revokeAllMemberRoles(ctx context.Context, spaceID, profileID uuid.UUID) {
	if s == nil || s.Roles == nil {
		return
	}
	resp, err := s.Roles.GetMemberRoles(ctx, &rolev1.GetMemberRolesRequest{
		SpaceId:   spaceID.String(),
		ProfileId: profileID.String(),
	})
	if err != nil {
		return
	}
	for _, r := range resp.GetRoleList().GetRoles() {
		_, _ = s.Roles.RevokeRole(ctx, &rolev1.RevokeRoleRequest{
			SpaceId:   spaceID.String(),
			ProfileId: profileID.String(),
			RoleId:    r.GetId(),
		})
	}
}

// ensureCommunityMemberRole projects only the Space's baseline Member role.
// External rank claims never select or elevate a Role role.
func (s *SpaceGRPC) ensureCommunityMemberRole(ctx context.Context, spaceID, profileID uuid.UUID) error {
	if s == nil || s.Roles == nil {
		return nil
	}
	roles, err := s.Roles.GetMemberRoles(ctx, &rolev1.GetMemberRolesRequest{SpaceId: spaceID.String(), ProfileId: profileID.String()})
	if err != nil {
		return err
	}
	for _, role := range roles.GetRoleList().GetRoles() {
		if role.GetName() == permissions.RoleMember {
			return nil
		}
	}
	return s.assignDefaultMemberRole(ctx, spaceID, profileID)
}

// revokeCommunityMemberRole removes only the baseline Member role and only
// when the profile has no independent Space membership. Elevated user roles
// are never selected by game rank or removed here.
func (s *SpaceGRPC) revokeCommunityMemberRole(ctx context.Context, spaceID, profileID uuid.UUID) error {
	if s == nil || s.Roles == nil {
		return nil
	}
	manual, err := s.Store.IsSpaceMember(ctx, spaceID, profileID)
	if err != nil || manual {
		return err
	}
	roleList, err := s.Roles.ListRoles(ctx, &rolev1.ListRolesRequest{SpaceId: spaceID.String()})
	if err != nil {
		return err
	}
	var memberRoleID string
	for _, role := range roleList.GetRoleList().GetRoles() {
		if role.GetName() == permissions.RoleMember {
			memberRoleID = role.GetId()
			break
		}
	}
	if memberRoleID == "" {
		return nil
	}
	memberRoles, err := s.Roles.GetMemberRoles(ctx, &rolev1.GetMemberRolesRequest{SpaceId: spaceID.String(), ProfileId: profileID.String()})
	if err != nil {
		return err
	}
	for _, role := range memberRoles.GetRoleList().GetRoles() {
		if role.GetId() == memberRoleID {
			_, err = s.Roles.RevokeRole(ctx, &rolev1.RevokeRoleRequest{SpaceId: spaceID.String(), ProfileId: profileID.String(), RoleId: memberRoleID})
			return err
		}
	}
	return nil
}

func (s *SpaceGRPC) memberRoleNames(ctx context.Context, spaceID, profileID uuid.UUID) []string {
	if s.Roles == nil {
		return nil
	}
	resp, err := s.Roles.GetMemberRoles(ctx, &rolev1.GetMemberRolesRequest{
		SpaceId:   spaceID.String(),
		ProfileId: profileID.String(),
	})
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(resp.GetRoleList().GetRoles()))
	for _, r := range resp.GetRoleList().GetRoles() {
		names = append(names, r.GetName())
	}
	return names
}
