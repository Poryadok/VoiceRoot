package grpcsvc

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/lifecycleprincipal"
	"voice/backend/space/internal/store"
)

type creatorSpaceWriter interface {
	CreateSpaceWithCreator(context.Context, *lifecycleprincipal.VerifiedCreator, *spacev1.CreateSpaceRequest) (*store.SpaceRow, error)
}

// CreateSpace overrides the promoted legacy metadata/raw-store handler only on
// the protected listener. The ordinary listener's denial remains unchanged.
func (s *PublicLifecycleSpace) CreateSpace(ctx context.Context, req *spacev1.CreateSpaceRequest) (*spacev1.CreateSpaceResponse, error) {
	var writer creatorSpaceWriter
	if s != nil && s.SpaceGRPC != nil && s.Store != nil {
		writer = s.Store
	}
	return s.createSpaceWithVerifiedCreator(ctx, req, writer)
}

func (s *PublicLifecycleSpace) createSpaceWithVerifiedCreator(ctx context.Context, req *spacev1.CreateSpaceRequest, writer creatorSpaceWriter) (*spacev1.CreateSpaceResponse, error) {
	creator, err := lifecycleprincipal.RequireCreator(ctx, req)
	if err != nil {
		return nil, err
	}
	actor, err := creator.ActorFor(req)
	if err != nil {
		return nil, err
	}
	if s == nil || s.SpaceGRPC == nil || writer == nil || s.OwnershipRoles == nil || s.PrincipalIssuer == nil {
		return nil, status.Error(codes.Unavailable, "creator persistence and owner bootstrap required")
	}
	row, err := writer.CreateSpaceWithCreator(ctx, creator, req)
	if err != nil {
		if errors.Is(err, store.ErrMemberAuthorityUnavailable) || errors.Is(err, store.ErrEffectiveEpisodeUnavailable) {
			return nil, status.Error(codes.Unavailable, "creator native authority unavailable")
		}
		return nil, mapSpaceStoreError(err)
	}
	if row == nil || row.ID == uuid.Nil || row.OwnerProfileID.String() != actor.ProfileID {
		return nil, status.Error(codes.Unavailable, "creator publication binding unavailable")
	}
	bootstrap := &rolev1.BootstrapSpaceRolesRequest{SpaceId: row.ID.String(), OwnerProfileId: actor.ProfileID}
	signed, err := s.ownershipRoleContext(ctx, bootstrap, rolev1.RoleService_BootstrapSpaceRoles_FullMethodName, uuid.New())
	if err != nil {
		return nil, err
	}
	bounded, cancel := context.WithTimeout(signed, 5*time.Second)
	response, err := s.OwnershipRoles.BootstrapSpaceRoles(bounded, bootstrap)
	cancel()
	if err != nil {
		code := status.Code(err)
		if code == codes.Unknown {
			code = codes.Unavailable
		}
		return nil, status.Error(code, "creator owner bootstrap failed")
	}
	if response == nil || len(response.ProtoReflect().GetUnknown()) != 0 {
		return nil, status.Error(codes.Unavailable, "creator owner bootstrap outcome unavailable")
	}
	if s.SpaceEvents != nil {
		if err := s.SpaceEvents.PublishSpaceCreated(ctx, row.ID.String(), row.OwnerProfileID.String()); err != nil {
			s.logPublishError(ctx, "space.created", err, slog.String("space_id", row.ID.String()))
		}
	}
	return &spacev1.CreateSpaceResponse{Space: spaceRowToProto(row)}, nil
}
