package s2s

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	socialv1 "voice.app/voice/social/v1"
)

// SocialGRPCBlocks calls SocialService.IsBlocked in both directions (DM messaging gate).
type SocialGRPCBlocks struct {
	Client socialv1.SocialServiceClient
}

// NewSocialGRPCBlocks builds a checker from an existing gRPC connection.
func NewSocialGRPCBlocks(cc grpc.ClientConnInterface) *SocialGRPCBlocks {
	return &SocialGRPCBlocks{Client: socialv1.NewSocialServiceClient(cc)}
}

func (s *SocialGRPCBlocks) AccountPairBlocked(ctx context.Context, viewerAccountID, otherAccountID uuid.UUID) (bool, error) {
	if s == nil || s.Client == nil {
		return false, status.Error(codes.Unavailable, "social block status unavailable")
	}
	if viewerAccountID == otherAccountID {
		return false, nil
	}
	ctx = ForwardIncomingMetadata(ctx)
	r1, err := s.Client.IsBlocked(ctx, &socialv1.IsBlockedRequest{
		AccountIdA: viewerAccountID.String(),
		AccountIdB: otherAccountID.String(),
	})
	if err != nil {
		return false, err
	}
	if r1.GetBlocked() {
		return true, nil
	}
	r2, err := s.Client.IsBlocked(ctx, &socialv1.IsBlockedRequest{
		AccountIdA: otherAccountID.String(),
		AccountIdB: viewerAccountID.String(),
	})
	if err != nil {
		return false, err
	}
	return r2.GetBlocked(), nil
}
