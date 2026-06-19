package grpcsvc

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/pkg/guestguard"
	"voice/backend/pkg/privacy"

	spacev1 "voice.app/voice/space/v1"
	userv1 "voice.app/voice/user/v1"
)

func (u *UserGRPCPrivacy) AllowDMAudience(ctx context.Context, profileID uuid.UUID) (privacy.Audience, error) {
	if u == nil || u.Client == nil {
		return privacy.EveryoneWithGuests(), nil
	}
	resp, err := u.Client.GetPrivacySettings(ctx, &userv1.GetPrivacySettingsRequest{
		ProfileId: profileID.String(),
	})
	if err != nil {
		return privacy.Audience{}, err
	}
	return privacy.FromProto(resp.GetPrivacySettings().GetAllowDm()), nil
}

func (u *UserGRPCPrivacy) AllowChatSpaceInvitesAudience(ctx context.Context, profileID uuid.UUID) (privacy.Audience, error) {
	if u == nil || u.Client == nil {
		return privacy.EveryoneWithGuests(), nil
	}
	resp, err := u.Client.GetPrivacySettings(ctx, &userv1.GetPrivacySettingsRequest{
		ProfileId: profileID.String(),
	})
	if err != nil {
		return privacy.Audience{}, err
	}
	return privacy.FromProto(resp.GetPrivacySettings().GetAllowChatSpaceInvites()), nil
}

type spaceGRPCCoMembership struct {
	client spacev1.SpaceServiceClient
}

func NewSpaceGRPCCoMembership(cc grpc.ClientConnInterface) SpaceCoMembershipChecker {
	if cc == nil {
		return nil
	}
	return &spaceGRPCCoMembership{client: spacev1.NewSpaceServiceClient(cc)}
}

func (s *spaceGRPCCoMembership) AreCoMembers(ctx context.Context, profileA, profileB uuid.UUID, spaceIDs []string) (bool, error) {
	if s == nil || s.client == nil {
		return false, nil
	}
	resp, err := s.client.AreCoMembers(ctx, &spacev1.AreCoMembersRequest{
		ProfileIdA: profileA.String(),
		ProfileIdB: profileB.String(),
		SpaceIds:   spaceIDs,
	})
	if err != nil {
		return false, err
	}
	return resp.GetCoMembers(), nil
}

func ensureAudienceAllowed(ctx context.Context, recipient, caller uuid.UUID, audience privacy.Audience, friends ProfileFriendChecker, space SpaceCoMembershipChecker, deniedMessage string) error {
	matcher := privacy.Matcher{Social: friends, Space: space}
	if err := privacy.CheckAllowed(matcher, ctx, recipient, caller, audience, guestguard.IsGuest(ctx)); err != nil {
		if errors.Is(err, privacy.ErrDenied) {
			return status.Error(codes.PermissionDenied, deniedMessage)
		}
		return status.Error(codes.Internal, err.Error())
	}
	return nil
}

func (s *ChatGRPC) ensureInvitePrivacy(ctx context.Context, inviter, invitee uuid.UUID) error {
	if s == nil || s.Privacy == nil {
		return nil
	}
	audience, err := s.Privacy.AllowChatSpaceInvitesAudience(ctx, invitee)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	return ensureAudienceAllowed(ctx, invitee, inviter, audience, s.Friends, s.SpaceCoMembership, "invite blocked by recipient privacy settings")
}
