package s2s

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc"

	"voice/backend/pkg/privacy"

	spacev1 "voice.app/voice/space/v1"
	socialv1 "voice.app/voice/social/v1"
	userv1 "voice.app/voice/user/v1"
)

type GRPCUserPrivacy struct {
	Client userv1.UserServiceClient
}

func (u *GRPCUserPrivacy) ShowMmRatingAudience(ctx context.Context, profileID uuid.UUID) (privacy.Audience, error) {
	if u == nil || u.Client == nil {
		return privacy.EveryoneWithGuests(), nil
	}
	ctx = ForwardIncomingMetadata(ctx)
	resp, err := u.Client.GetPrivacySettings(ctx, &userv1.GetPrivacySettingsRequest{
		ProfileId: profileID.String(),
	})
	if err != nil {
		return privacy.Audience{}, err
	}
	return privacy.FromProto(resp.GetPrivacySettings().GetShowMmRating()), nil
}

type GRPCSocialFriends struct {
	Client socialv1.SocialServiceClient
}

func NewGRPCSocialFriends(cc grpc.ClientConnInterface) *GRPCSocialFriends {
	if cc == nil {
		return nil
	}
	return &GRPCSocialFriends{Client: socialv1.NewSocialServiceClient(cc)}
}

func (s *GRPCSocialFriends) AreFriends(ctx context.Context, profileA, profileB uuid.UUID) (bool, error) {
	if s == nil || s.Client == nil {
		return false, nil
	}
	ctx = ForwardIncomingMetadata(ctx)
	resp, err := s.Client.AreFriends(ctx, &socialv1.AreFriendsRequest{
		ProfileIdA: profileA.String(),
		ProfileIdB: profileB.String(),
	})
	if err != nil {
		return false, err
	}
	return resp.GetFriends(), nil
}

func (s *GRPCSocialFriends) AreFriendsOfFriends(ctx context.Context, profileA, profileB uuid.UUID) (bool, error) {
	if s == nil || s.Client == nil {
		return false, nil
	}
	ctx = ForwardIncomingMetadata(ctx)
	resp, err := s.Client.AreFriendsOfFriends(ctx, &socialv1.AreFriendsRequest{
		ProfileIdA: profileA.String(),
		ProfileIdB: profileB.String(),
	})
	if err != nil {
		return false, err
	}
	return resp.GetFriends(), nil
}

type GRPCSpaceCoMembership struct {
	Client spacev1.SpaceServiceClient
}

func NewGRPCSpaceCoMembership(cc grpc.ClientConnInterface) *GRPCSpaceCoMembership {
	if cc == nil {
		return nil
	}
	return &GRPCSpaceCoMembership{Client: spacev1.NewSpaceServiceClient(cc)}
}

func (s *GRPCSpaceCoMembership) AreCoMembers(ctx context.Context, profileA, profileB uuid.UUID, spaceIDs []string) (bool, error) {
	if s == nil || s.Client == nil {
		return false, nil
	}
	ctx = ForwardIncomingMetadata(ctx)
	resp, err := s.Client.AreCoMembers(ctx, &spacev1.AreCoMembersRequest{
		ProfileIdA: profileA.String(),
		ProfileIdB: profileB.String(),
		SpaceIds:   spaceIDs,
	})
	if err != nil {
		return false, err
	}
	return resp.GetCoMembers(), nil
}
