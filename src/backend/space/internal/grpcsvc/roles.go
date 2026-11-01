package grpcsvc

import (
	"context"
	"time"

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
	return s.checkSpacePermissionForProfile(ctx, spaceID, caller, permission)
}

// requireSpacePermissionForProfile accepts an actor already validated by the
// owning transport; it never falls back to raw metadata.
func (s *SpaceGRPC) requireSpacePermissionForProfile(ctx context.Context, spaceID, caller uuid.UUID, permission string) error {
	if s == nil || s.Store == nil {
		return status.Error(codes.FailedPrecondition, "space persistence not configured")
	}
	if err := s.Store.CheckOwnershipAvailable(ctx, spaceID); err != nil {
		return mapSpaceStoreError(err)
	}
	return s.checkSpacePermissionForProfile(ctx, spaceID, caller, permission)
}

func (s *SpaceGRPC) checkSpacePermissionForProfile(ctx context.Context, spaceID, caller uuid.UUID, permission string) error {
	if permission == permissions.MemberBan {
		return s.requireBanPermission(ctx, spaceID, caller)
	}
	if s.Roles == nil {
		return s.requireSpaceOwnerForProfile(ctx, spaceID, caller)
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
	return s.requireSpaceOwnerForProfile(ctx, spaceID, caller)
}

func (s *SpaceGRPC) requireSpaceOwnerForProfile(ctx context.Context, spaceID, caller uuid.UUID) error {
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
	if s == nil || s.OwnershipRoles == nil || s.PrincipalIssuer == nil {
		return status.Error(codes.Unavailable, "protected owner bootstrap runtime unavailable")
	}
	req := &rolev1.BootstrapSpaceRolesRequest{
		SpaceId:        spaceID.String(),
		OwnerProfileId: ownerProfileID.String(),
	}
	// A durable Space retry repeats the same owner binding with a fresh signed
	// dispatch. The ordinary Role listener cannot establish bootstrap authority.
	signed, err := s.ownershipRoleContext(ctx, req, rolev1.RoleService_BootstrapSpaceRoles_FullMethodName, uuid.New())
	if err != nil {
		return err
	}
	bounded, cancel := context.WithTimeout(signed, 5*time.Second)
	defer cancel()
	response, err := s.OwnershipRoles.BootstrapSpaceRoles(bounded, req)
	if err != nil {
		return err
	}
	if response == nil || len(response.ProtoReflect().GetUnknown()) != 0 {
		return status.Error(codes.Unavailable, "owner bootstrap outcome unavailable")
	}
	return nil
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

// Native Role derives the fixed managed Member from fresh Space community
// authority. Publication precedes this read; Space never assigns today's
// default role or impersonates the Owner to manufacture a community grant.
func (s *SpaceGRPC) ensureCommunityMemberRole(ctx context.Context, spaceID, profileID uuid.UUID) error {
	present, err := s.communityMemberRolePresent(ctx, spaceID, profileID)
	if err != nil {
		return err
	}
	if !present {
		return status.Error(codes.Unavailable, "community Member projection unavailable")
	}
	return nil
}

// Removal of Space's community episode removes its native Role projection.
// Confirm that outcome without deleting independent ordinary contributions.
func (s *SpaceGRPC) revokeCommunityMemberRole(ctx context.Context, spaceID, profileID uuid.UUID) error {
	if s == nil || s.Store == nil {
		return status.Error(codes.Unavailable, "community membership authority unavailable")
	}
	manual, err := s.Store.IsSpaceMember(ctx, spaceID, profileID)
	if err != nil || manual {
		return err
	}
	present, err := s.communityMemberRolePresent(ctx, spaceID, profileID)
	if err != nil {
		return err
	}
	if present {
		return status.Error(codes.Unavailable, "community Member revocation unavailable")
	}
	return nil
}

func (s *SpaceGRPC) communityMemberRolePresent(ctx context.Context, spaceID, profileID uuid.UUID) (bool, error) {
	if s == nil || s.OwnershipRoles == nil || s.PrincipalIssuer == nil {
		return false, status.Error(codes.Unavailable, "protected community Role runtime unavailable")
	}
	req := &rolev1.GetMemberRolesRequest{SpaceId: spaceID.String(), ProfileId: profileID.String()}
	signed, err := s.ownershipRoleContext(ctx, req, rolev1.RoleService_GetMemberRoles_FullMethodName, uuid.New())
	if err != nil {
		return false, err
	}
	bounded, cancel := context.WithTimeout(signed, 5*time.Second)
	defer cancel()
	response, err := s.OwnershipRoles.GetMemberRoles(bounded, req)
	if err != nil {
		return false, err
	}
	invalid := status.Error(codes.Unavailable, "community Role projection response unavailable")
	if response == nil || response.RoleList == nil || len(response.ProtoReflect().GetUnknown()) != 0 || len(response.RoleList.ProtoReflect().GetUnknown()) != 0 {
		return false, invalid
	}
	present := false
	for _, role := range response.RoleList.Roles {
		if role == nil || len(role.ProtoReflect().GetUnknown()) != 0 || role.SpaceId != spaceID.String() {
			return false, invalid
		}
		id, err := uuid.Parse(role.Id)
		if err != nil || id == uuid.Nil || id.String() != role.Id {
			return false, invalid
		}
		if role.Managed && role.Name == permissions.RoleMember {
			present = true
		}
	}
	return present, nil
}

func (s *SpaceGRPC) memberRoleNames(ctx context.Context, spaceID, profileID uuid.UUID) []string {
	if s.OwnershipRoles == nil || s.PrincipalIssuer == nil {
		return nil
	}
	req := &rolev1.GetMemberRolesRequest{
		SpaceId:   spaceID.String(),
		ProfileId: profileID.String(),
	}
	signed, err := s.ownershipRoleContext(ctx, req, rolev1.RoleService_GetMemberRoles_FullMethodName, uuid.New())
	if err != nil {
		return nil
	}
	resp, err := s.OwnershipRoles.GetMemberRoles(signed, req)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(resp.GetRoleList().GetRoles()))
	for _, r := range resp.GetRoleList().GetRoles() {
		names = append(names, r.GetName())
	}
	return names
}
