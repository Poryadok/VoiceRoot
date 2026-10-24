package s2s

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/spacecommunication"
	"voice/backend/voice/internal/grpcsvc"
)

type GRPCVoiceRoomAccessResolver struct {
	Client spacev1.SpaceServiceClient
	issuer *principal.Issuer
}

func NewGRPCVoiceRoomAccessResolver(client spacev1.SpaceServiceClient, issuer *principal.Issuer) *GRPCVoiceRoomAccessResolver {
	return &GRPCVoiceRoomAccessResolver{Client: client, issuer: issuer}
}

func (g *GRPCVoiceRoomAccessResolver) ResolveVoiceRoomAccess(ctx context.Context, expectedSpaceID, voiceRoomID, profileID string) (grpcsvc.CanonicalVoiceRoomAccess, error) {
	if g == nil || g.Client == nil {
		return grpcsvc.CanonicalVoiceRoomAccess{}, status.Error(codes.FailedPrecondition, "space voice room resolver not configured")
	}
	request := &spacev1.ResolveVoiceRoomAccessRequest{
		Space: &spacev1.SpaceRef{Id: expectedSpaceID}, VoiceRoomId: voiceRoomID, ProfileId: profileID,
	}
	callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	callCtx, err := spacecommunication.Sign(callCtx, g.issuer, spacev1.SpaceService_ResolveVoiceRoomAccess_FullMethodName, request)
	if err != nil {
		return grpcsvc.CanonicalVoiceRoomAccess{}, status.Error(codes.Unavailable, "Space communication principal unavailable")
	}
	resp, err := g.Client.ResolveVoiceRoomAccess(callCtx, request)
	if err != nil {
		return grpcsvc.CanonicalVoiceRoomAccess{}, err
	}
	snapshot := spacecommunication.Snapshot{Version: resp.GetProtocolVersion(), SpaceID: resp.GetSpaceId(), ProfileID: resp.GetProfileId(), Epoch: resp.GetAccessEpoch(), EvaluatedAt: resp.GetEvaluatedAt(), State: int32(resp.GetCommunicationState()), Until: resp.GetTimedOutUntil()}
	timedOut, err := snapshot.Validate(expectedSpaceID, profileID)
	if err != nil {
		return grpcsvc.CanonicalVoiceRoomAccess{}, status.Error(codes.Unavailable, "Space communication decision incomplete")
	}
	return grpcsvc.CanonicalVoiceRoomAccess{
		SpaceID: resp.GetSpaceId(), Member: resp.GetMember(), Active: resp.GetActive(), AccessEpoch: resp.GetAccessEpoch(),
		CommunicationStateResolved: true, TimedOut: timedOut,
	}, nil
}
