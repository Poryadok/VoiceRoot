package s2s

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/voice/internal/grpcsvc"
)

// VoiceRoomGrants is the complete Role-owned authorization decision for a
// Space voice-room media grant, including the policy epoch used to make it.
type VoiceRoomGrants = grpcsvc.CanonicalVoiceRoomGrants

type VoiceRoomGrantResolver interface {
	ResolveVoiceRoomGrants(context.Context, string, string, string) (VoiceRoomGrants, error)
}

type GRPCVoiceRoomGrantResolver struct {
	Client rolev1.RoleServiceClient
}

func NewGRPCVoiceRoomGrantResolver(client rolev1.RoleServiceClient) *GRPCVoiceRoomGrantResolver {
	return &GRPCVoiceRoomGrantResolver{Client: client}
}

func (g *GRPCVoiceRoomGrantResolver) ResolveVoiceRoomGrants(ctx context.Context, spaceID, voiceRoomID, profileID string) (VoiceRoomGrants, error) {
	if g == nil || g.Client == nil {
		return VoiceRoomGrants{}, status.Error(codes.FailedPrecondition, "role voice grant resolver not configured")
	}
	spaceID, voiceRoomID, profileID = strings.TrimSpace(spaceID), strings.TrimSpace(voiceRoomID), strings.TrimSpace(profileID)
	if _, err := uuid.Parse(spaceID); err != nil {
		return VoiceRoomGrants{}, status.Error(codes.InvalidArgument, "invalid space id")
	}
	if _, err := uuid.Parse(voiceRoomID); err != nil {
		return VoiceRoomGrants{}, status.Error(codes.InvalidArgument, "invalid voice room id")
	}
	if _, err := uuid.Parse(profileID); err != nil {
		return VoiceRoomGrants{}, status.Error(codes.InvalidArgument, "invalid profile id")
	}
	resp, err := g.Client.ResolveVoiceRoomGrants(ForwardIncomingMetadata(ctx), &rolev1.ResolveVoiceRoomGrantsRequest{
		SpaceId: spaceID, VoiceRoomId: voiceRoomID, ProfileId: profileID,
	})
	if err != nil {
		if status.Code(err) == codes.Unavailable {
			return VoiceRoomGrants{}, status.Error(codes.Unavailable, "role voice grant resolver unavailable")
		}
		return VoiceRoomGrants{}, err
	}
	if resp == nil || resp.GetPolicyEpoch() == 0 || resp.GetGrants() == nil {
		return VoiceRoomGrants{}, status.Error(codes.Unavailable, "role voice grant decision incomplete")
	}
	grants := resp.GetGrants()
	return VoiceRoomGrants{
		PolicyEpoch: resp.GetPolicyEpoch(), CanJoin: grants.GetCanJoin(),
		CanPublishAudio: grants.GetCanPublishAudio(), CanSubscribe: grants.GetCanSubscribe(),
	}, nil
}
