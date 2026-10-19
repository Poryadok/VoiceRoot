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
)

// GetAuditLog adds the public disclosure boundary to the existing ledger query.
// The delegated listener verifies its exact request before the typed actor read.
func (s *PublicLifecycleSpace) GetAuditLog(ctx context.Context, req *spacev1.GetAuditLogRequest) (*spacev1.GetAuditLogResponse, error) {
	identity, err := principal.RequireDelegatedUser(ctx, "space", spacev1.SpaceService_GetAuditLog_FullMethodName, req)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "verified audit reader required")
	}
	actor := identity.ProfileID
	if s == nil || s.SpaceGRPC == nil || s.Store == nil {
		return nil, status.Error(codes.Unavailable, "audit unavailable")
	}
	spaceID, err := parseUUIDField("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	resp, err := s.SpaceGRPC.GetAuditLog(ctx, req)
	if status.Code(err) == codes.PermissionDenied {
		// A discoverable member's policy denial is 403; outsiders learn no state.
		member, memberErr := s.Store.IsSpaceMember(ctx, spaceID, actor)
		if memberErr != nil {
			return nil, status.Error(codes.Unavailable, "audit unavailable")
		}
		if !member {
			return nil, status.Error(codes.NotFound, "space not found")
		}
	}
	if status.Code(err) == codes.Unavailable {
		// The ordinary store fence deliberately returns unavailable. The public
		// contract distinguishes a frozen recorded owner from every hidden caller.
		aggregate, lookupErr := s.Store.LoadLifecycle(ctx, spaceID)
		if lookupErr == nil && aggregate.Phase() != spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_LIVE {
			return nil, s.frozenAuditReadError(ctx, spaceID, actor)
		}
	}
	return resp, err
}

func (s *PublicLifecycleSpace) frozenAuditReadError(ctx context.Context, spaceID, actor uuid.UUID) error {
	// Reuse the existing locked recorded-owner projection, including pending
	// schedule/restore and purge. Never return that projection as audit data.
	_, err := s.Store.GetLifecycleRecoverySpace(ctx, spaceID, actor)
	if errors.Is(err, pgx.ErrNoRows) {
		return status.Error(codes.NotFound, "space not found")
	}
	if err != nil {
		return status.Error(codes.Unavailable, "audit unavailable")
	}
	return status.Error(codes.FailedPrecondition, "space is frozen")
}
