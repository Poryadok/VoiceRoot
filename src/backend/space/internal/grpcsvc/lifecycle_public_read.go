package grpcsvc

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/store"
)

// PublicLifecycleSpace is registered only on the Gateway's delegated-user listener.
// The ordinary listener retains its storage fence and cannot return this projection.
type PublicLifecycleSpace struct{ *SpaceGRPC }

func (s *PublicLifecycleSpace) GetSpace(ctx context.Context, req *spacev1.GetSpaceRequest) (*spacev1.GetSpaceResponse, error) {
	p, ok := principal.FromContext(ctx)
	if !ok || p.Kind != "delegated_user" || p.Issuer != "gateway" || p.Audience != "space" || p.RPC != spacev1.SpaceService_GetSpace_FullMethodName {
		return nil, status.Error(codes.Unauthenticated, "verified lifecycle reader required")
	}
	if s == nil || s.SpaceGRPC == nil || s.Store == nil {
		return nil, status.Error(codes.Unavailable, "space unavailable")
	}
	id, err := parseUUIDField("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	a, err := s.Store.LoadLifecycle(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && a.Phase() == spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_LIVE {
		resp, err := s.SpaceGRPC.GetSpace(ctx, req)
		if status.Code(err) == codes.PermissionDenied {
			return nil, status.Error(codes.NotFound, "space not found")
		}
		return resp, err
	}
	if err != nil {
		return nil, status.Error(codes.Unavailable, "space unavailable")
	}
	actor, err := uuid.Parse(p.ProfileID)
	if err != nil || actor == uuid.Nil || actor.String() != p.ProfileID {
		return nil, status.Error(codes.Unauthenticated, "invalid lifecycle reader")
	}
	projection, err := s.Store.GetLifecycleRecoverySpace(ctx, id, actor)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, status.Error(codes.NotFound, "space not found")
	}
	if errors.Is(err, store.ErrLifecycleStateTransition) {
		return nil, status.Error(codes.Unavailable, "space lifecycle changed")
	}
	if err != nil {
		return nil, status.Error(codes.Unavailable, "space unavailable")
	}
	return &spacev1.GetSpaceResponse{Space: projection}, nil
}
