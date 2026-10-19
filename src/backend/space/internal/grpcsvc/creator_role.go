package grpcsvc

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	rolev1 "voice.app/voice/role/v1"
)

// Native creation never treats an absent Role dependency or the legacy
// ordinary client as successful Owner bootstrap.
func (s *SpaceGRPC) bootstrapCreatorRoles(ctx context.Context, spaceID, ownerID uuid.UUID) error {
	if err := s.creatorRoleRuntimeReady(); err != nil {
		return err
	}
	request := &rolev1.BootstrapSpaceRolesRequest{SpaceId: spaceID.String(), OwnerProfileId: ownerID.String()}
	signed, err := s.ownershipRoleContext(ctx, request, rolev1.RoleService_BootstrapSpaceRoles_FullMethodName, uuid.New())
	if err != nil {
		return err
	}
	// This is a single dispatch. Ambiguous response loss remains visible; the
	// non-idempotent public CreateSpace POST must not be automatically repeated.
	_, err = s.OwnershipRoles.BootstrapSpaceRoles(signed, request)
	return err
}

func (s *SpaceGRPC) creatorRoleRuntimeReady() error {
	if s == nil || s.OwnershipRoles == nil || s.PrincipalIssuer == nil {
		return status.Error(codes.Unavailable, "protected creator role bootstrap unavailable")
	}
	return nil
}
