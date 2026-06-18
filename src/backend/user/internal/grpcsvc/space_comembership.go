package grpcsvc

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc"

	spacev1 "voice.app/voice/space/v1"
)

// SpaceCoMembershipChecker checks shared space membership for privacy audiences.
type SpaceCoMembershipChecker interface {
	AreCoMembers(ctx context.Context, profileA, profileB uuid.UUID, spaceIDs []string) (bool, error)
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
