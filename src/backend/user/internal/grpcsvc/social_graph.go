package grpcsvc

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc"

	socialv1 "voice.app/voice/social/v1"
)

// SocialGraphChecker supplies friendship facts for privacy enforcement.
type SocialGraphChecker interface {
	AreFriends(ctx context.Context, profileA, profileB uuid.UUID) (bool, error)
	AreFriendsOfFriends(ctx context.Context, profileA, profileB uuid.UUID) (bool, error)
}

// SocialGRPCGraph calls voice.social.v1 SocialService friendship RPCs.
type SocialGRPCGraph struct {
	Client socialv1.SocialServiceClient
}

func NewSocialGRPCGraph(cc grpc.ClientConnInterface) *SocialGRPCGraph {
	return &SocialGRPCGraph{Client: socialv1.NewSocialServiceClient(cc)}
}

func (s *SocialGRPCGraph) AreFriends(ctx context.Context, profileA, profileB uuid.UUID) (bool, error) {
	if s == nil || s.Client == nil {
		return false, nil
	}
	resp, err := s.Client.AreFriends(ctx, &socialv1.AreFriendsRequest{
		ProfileIdA: profileA.String(),
		ProfileIdB: profileB.String(),
	})
	if err != nil {
		return false, err
	}
	return resp.GetFriends(), nil
}

func (s *SocialGRPCGraph) AreFriendsOfFriends(ctx context.Context, profileA, profileB uuid.UUID) (bool, error) {
	if s == nil || s.Client == nil {
		return false, nil
	}
	resp, err := s.Client.AreFriendsOfFriends(ctx, &socialv1.AreFriendsRequest{
		ProfileIdA: profileA.String(),
		ProfileIdB: profileB.String(),
	})
	if err != nil {
		return false, err
	}
	return resp.GetFriends(), nil
}
