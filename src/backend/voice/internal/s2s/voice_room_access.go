package s2s

import (
	"context"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/voice/internal/grpcsvc"
)

type GRPCVoiceRoomAccessResolver struct {
	Client     spacev1.SpaceServiceClient
	voiceToken string
}

func NewGRPCVoiceRoomAccessResolver(client spacev1.SpaceServiceClient, voiceToken string) *GRPCVoiceRoomAccessResolver {
	return &GRPCVoiceRoomAccessResolver{Client: client, voiceToken: strings.TrimSpace(voiceToken)}
}

func (g *GRPCVoiceRoomAccessResolver) ResolveVoiceRoomAccess(ctx context.Context, voiceRoomID, profileID string) (grpcsvc.CanonicalVoiceRoomAccess, error) {
	if g == nil || g.Client == nil {
		return grpcsvc.CanonicalVoiceRoomAccess{}, status.Error(codes.FailedPrecondition, "space voice room resolver not configured")
	}
	if g.voiceToken == "" {
		return grpcsvc.CanonicalVoiceRoomAccess{}, status.Error(codes.FailedPrecondition, "space voice S2S token not configured")
	}
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+g.voiceToken))
	resp, err := g.Client.ResolveVoiceRoomAccess(ctx, &spacev1.ResolveVoiceRoomAccessRequest{VoiceRoomId: voiceRoomID, ProfileId: profileID})
	if err != nil {
		return grpcsvc.CanonicalVoiceRoomAccess{}, err
	}
	return grpcsvc.CanonicalVoiceRoomAccess{SpaceID: resp.GetSpaceId(), Member: resp.GetMember(), Active: resp.GetActive()}, nil
}
