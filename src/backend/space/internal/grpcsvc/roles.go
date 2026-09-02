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
		if status.Code(err) == codes.Unavailable {
			return status.Error(codes.Unavailable, "role service unavailable")
		}
		return status.Error(codes.Internal, err.Error())
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
		return status.Error(codes.Internal, err.Error())
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

// reassignOwnerRole moves the Owner system role after TransferOwnership.
// When Roles is nil (ROLE_GRPC_ADDR unset), skips — spaces.owner_profile_id remains source of truth.
// When Roles is wired, fail-closed: List/Assign/Revoke errors and a missing Owner role are returned.
func (s *SpaceGRPC) reassignOwnerRole(ctx context.Context, spaceID, previousOwner, newOwner uuid.UUID) error {
	if s == nil || s.Roles == nil {
		return nil
	}
	list, err := s.Roles.ListRoles(ctx, &rolev1.ListRolesRequest{SpaceId: spaceID.String()})
	if err != nil {
		return err
	}
	var ownerRoleID string
	for _, r := range list.GetRoleList().GetRoles() {
		if r.GetName() == permissions.RoleOwner {
			ownerRoleID = r.GetId()
			break
		}
	}
	if ownerRoleID == "" {
		return status.Error(codes.FailedPrecondition, "owner system role not found")
	}
	ownerCtx := metadata.AppendToOutgoingContext(ctx, authctx.HeaderProfileID, previousOwner.String())
	if _, err := s.Roles.AssignRole(ownerCtx, &rolev1.AssignRoleRequest{
		SpaceId:   spaceID.String(),
		ProfileId: newOwner.String(),
		RoleId:    ownerRoleID,
	}); err != nil {
		return err
	}
	if _, err := s.Roles.RevokeRole(ownerCtx, &rolev1.RevokeRoleRequest{
		SpaceId:   spaceID.String(),
		ProfileId: previousOwner.String(),
		RoleId:    ownerRoleID,
	}); err != nil {
		return err
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
