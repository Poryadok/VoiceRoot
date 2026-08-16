package s2s

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

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
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("x-voice-internal-caller", "matchmaking"))
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

// GRPCSpaceQueueGate verifies space membership and mm_config.enabled for StartSpaceQueue.
type GRPCSpaceQueueGate struct {
	Client spacev1.SpaceServiceClient
}

func NewGRPCSpaceQueueGate(cc grpc.ClientConnInterface) *GRPCSpaceQueueGate {
	if cc == nil {
		return nil
	}
	return &GRPCSpaceQueueGate{Client: spacev1.NewSpaceServiceClient(cc)}
}

func (g *GRPCSpaceQueueGate) EnsureMemberAndMMEnabled(ctx context.Context, spaceID uuid.UUID) error {
	if g == nil || g.Client == nil {
		return status.Error(codes.Unavailable, "space matchmaking unavailable")
	}
	ctx = ForwardIncomingMetadata(ctx)
	resp, err := g.Client.GetSpace(ctx, &spacev1.GetSpaceRequest{SpaceId: spaceID.String()})
	if err != nil {
		return err
	}
	if !grpcsvcParseEnabled(resp.GetSpace().GetMmConfigJson()) {
		return status.Error(codes.FailedPrecondition, "space matchmaking disabled")
	}
	return nil
}

func grpcsvcParseEnabled(mmConfigJSON string) bool {
	raw := strings.TrimSpace(mmConfigJSON)
	if raw == "" || raw == "{}" || raw == "null" {
		return true
	}
	var cfg struct {
		Enabled *bool `json:"enabled"`
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return true
	}
	if cfg.Enabled == nil {
		return true
	}
	return *cfg.Enabled
}
