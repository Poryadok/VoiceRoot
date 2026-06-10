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
	list, err := s.Roles.ListRoles(ctx, &rolev1.ListRolesRequest{SpaceId: spaceID.String()})
	if err != nil {
		return err
	}
	var memberRoleID string
	for _, r := range list.GetRoleList().GetRoles() {
		if r.GetName() == permissions.RoleMember {
			memberRoleID = r.GetId()
			break
		}
	}
	if memberRoleID == "" {
		return status.Error(codes.FailedPrecondition, "member role not found")
	}
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
