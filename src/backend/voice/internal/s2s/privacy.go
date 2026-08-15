package s2s

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"voice/backend/pkg/guestprofile"
	"voice/backend/pkg/privacy"

	spacev1 "voice.app/voice/space/v1"
	socialv1 "voice.app/voice/social/v1"
	userv1 "voice.app/voice/user/v1"
)

type GRPCUserPrivacy struct {
	Client userv1.UserServiceClient
}

func privacyS2SContext(ctx context.Context) context.Context {
	return metadata.NewOutgoingContext(ctx, metadata.Pairs("x-voice-internal-caller", "voice"))
}

func (u *GRPCUserPrivacy) AllowCallsAudience(ctx context.Context, profileID uuid.UUID) (privacy.Audience, error) {
	if u == nil || u.Client == nil {
		return privacy.EveryoneWithGuests(), nil
	}
	s2sCtx := privacyS2SContext(ctx)
	resp, err := u.Client.GetPrivacySettings(s2sCtx, &userv1.GetPrivacySettingsRequest{
		ProfileId: profileID.String(),
	})
	if err != nil {
		return privacy.Audience{}, err
	}
	settings := resp.GetPrivacySettings()
	calls := privacy.FromProto(settings.GetAllowCalls())
	if guestCallee, err := u.isGuestCallee(s2sCtx, profileID); err == nil && guestCallee {
		// Guests receive calls under the same openness as DM (auth-and-contacts.md).
		return privacy.FromProto(settings.GetAllowDm()), nil
	}
	return calls, nil
}

func (u *GRPCUserPrivacy) isGuestCallee(ctx context.Context, profileID uuid.UUID) (bool, error) {
	resp, err := u.Client.GetProfile(ctx, &userv1.GetProfileRequest{
		By: &userv1.GetProfileRequest_ProfileId{ProfileId: profileID.String()},
	})
	if err != nil {
		return false, err
	}
	profile := resp.GetProfile()
	if profile == nil {
		return false, nil
	}
	if profile.GetIsGuestAccount() {
		return true, nil
	}
	return guestprofile.IsPlaceholderDisplayName(profile.GetAccountId(), profile.GetDisplayName()), nil
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
	resp, err := s.Client.AreFriendsOfFriends(ctx, &socialv1.AreFriendsOfFriendsRequest{
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
