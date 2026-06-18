package s2s

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc"

	"voice/backend/pkg/privacy"

	spacev1 "voice.app/voice/space/v1"
	userv1 "voice.app/voice/user/v1"
)

type GRPCUserPrivacy struct {
	Client userv1.UserServiceClient
}

func (u *GRPCUserPrivacy) AllowFriendRequestsAudience(ctx context.Context, profileID uuid.UUID) (privacy.Audience, error) {
	if u == nil || u.Client == nil {
		return privacy.EveryoneWithGuests(), nil
	}
	resp, err := u.Client.GetPrivacySettings(ctx, &userv1.GetPrivacySettingsRequest{
		ProfileId: profileID.String(),
	})
	if err != nil {
		return privacy.Audience{}, err
	}
	return privacy.FromProto(resp.GetPrivacySettings().GetAllowFriendRequests()), nil
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

type storeSocialGraph struct {
	friends interface {
		AreFriendsAccepted(context.Context, uuid.UUID, uuid.UUID) (bool, error)
		AreFriendsOfFriendsAccepted(context.Context, uuid.UUID, uuid.UUID) (bool, error)
	}
}

func NewStoreSocialGraph(friends interface {
	AreFriendsAccepted(context.Context, uuid.UUID, uuid.UUID) (bool, error)
	AreFriendsOfFriendsAccepted(context.Context, uuid.UUID, uuid.UUID) (bool, error)
}) *storeSocialGraph {
	if friends == nil {
		return nil
	}
	return &storeSocialGraph{friends: friends}
}

func (g *storeSocialGraph) AreFriends(ctx context.Context, profileA, profileB uuid.UUID) (bool, error) {
	return g.friends.AreFriendsAccepted(ctx, profileA, profileB)
}

func (g *storeSocialGraph) AreFriendsOfFriends(ctx context.Context, profileA, profileB uuid.UUID) (bool, error) {
	return g.friends.AreFriendsOfFriendsAccepted(ctx, profileA, profileB)
}
