package grpcsvc

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc"

	socialv1 "voice.app/voice/social/v1"
	userv1 "voice.app/voice/user/v1"
)

type SocialGRPCFriends struct {
	Client socialv1.SocialServiceClient
}

func NewSocialGRPCFriends(cc grpc.ClientConnInterface) *SocialGRPCFriends {
	return &SocialGRPCFriends{Client: socialv1.NewSocialServiceClient(cc)}
}

func (s *SocialGRPCFriends) AreFriends(ctx context.Context, profileA, profileB uuid.UUID) (bool, error) {
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

func (s *SocialGRPCFriends) AreFriendsOfFriends(ctx context.Context, profileA, profileB uuid.UUID) (bool, error) {
	if s == nil || s.Client == nil {
		return false, nil
	}
	resp, err := s.Client.AreFriendsOfFriends(ctx, &socialv1.AreFriendsOfFriendsRequest{
		ProfileIdA: profileA.String(),
		ProfileIdB: profileB.String(),
	})
	if err != nil {
		return false, err
	}
	return resp.GetFriends(), nil
}

type UserGRPCPrivacy struct {
	Client userv1.UserServiceClient
}
