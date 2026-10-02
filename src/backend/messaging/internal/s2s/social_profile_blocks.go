package s2s

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	socialv1 "voice.app/voice/social/v1"
)

type profilePairBlockClient interface {
	IsProfilePairBlocked(context.Context, *socialv1.IsProfilePairBlockedRequest, ...grpc.CallOption) (*socialv1.IsProfilePairBlockedResponse, error)
}

// SocialGRPCProfileBlocks asks Social to resolve profile ownership and evaluate
// one directional block without exposing account IDs to Messaging.
type SocialGRPCProfileBlocks struct{ Client profilePairBlockClient }

func NewSocialGRPCProfileBlocks(client socialv1.SocialServiceClient) *SocialGRPCProfileBlocks {
	return &SocialGRPCProfileBlocks{Client: client}
}

func (s *SocialGRPCProfileBlocks) ProfilePairBlocked(ctx context.Context, viewerProfileID, otherProfileID uuid.UUID) (bool, error) {
	if s == nil || s.Client == nil {
		return false, status.Error(codes.Unavailable, "social service not configured")
	}
	if viewerProfileID == uuid.Nil || otherProfileID == uuid.Nil {
		return false, status.Error(codes.InvalidArgument, "invalid profile id")
	}
	if viewerProfileID == otherProfileID {
		return false, nil
	}
	resp, err := s.Client.IsProfilePairBlocked(ForwardIncomingMetadata(ctx), &socialv1.IsProfilePairBlockedRequest{
		ViewerProfileId: viewerProfileID.String(),
		OtherProfileId:  otherProfileID.String(),
	})
	if err != nil {
		return false, err
	}
	if resp == nil {
		return false, status.Error(codes.Unavailable, "social service returned empty block response")
	}
	return resp.GetBlocked(), nil
}
