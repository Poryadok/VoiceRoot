package s2s

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/spacecommunication"
)

type CommunicationStates struct {
	Client spacev1.SpaceServiceClient
	Issuer *principal.Issuer
}

func (c *CommunicationStates) ResolveMemberCommunicationState(ctx context.Context, spaceID, profileID string) (spacecommunication.Snapshot, error) {
	if c == nil || c.Client == nil {
		return spacecommunication.Snapshot{}, status.Error(codes.Unavailable, "Space communication client unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req := &spacev1.ResolveMemberCommunicationStateRequest{ProtocolVersion: 1, SpaceId: spaceID, ProfileId: profileID}
	ctx, err := spacecommunication.Sign(ctx, c.Issuer, spacev1.SpaceService_ResolveMemberCommunicationState_FullMethodName, req)
	if err != nil {
		return spacecommunication.Snapshot{}, status.Error(codes.Unavailable, "Space communication signer unavailable")
	}
	response, err := c.Client.ResolveMemberCommunicationState(ctx, req)
	if err != nil {
		return spacecommunication.Snapshot{}, err
	}
	return spacecommunication.Snapshot{Version: response.GetProtocolVersion(), SpaceID: response.GetSpaceId(), ProfileID: response.GetProfileId(), Epoch: response.GetAccessEpoch(), EvaluatedAt: response.GetEvaluatedAt(), State: int32(response.GetCommunicationState()), Until: response.GetTimedOutUntil()}, nil
}
