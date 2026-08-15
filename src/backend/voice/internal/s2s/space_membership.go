package s2s

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/voice/internal/grpcsvc"

	spacev1 "voice.app/voice/space/v1"
	commonv1 "voice.app/voice/common/v1"
)

// GRPCSpaceMembership validates space membership via SpaceService.ListMembers.
type GRPCSpaceMembership struct {
	Client spacev1.SpaceServiceClient
}

func NewGRPCSpaceMembership(c spacev1.SpaceServiceClient) *GRPCSpaceMembership {
	return &GRPCSpaceMembership{Client: c}
}

func (g *GRPCSpaceMembership) EnsureMember(ctx context.Context, spaceID, profileID string) error {
	if g == nil || g.Client == nil {
		return status.Error(codes.FailedPrecondition, "space service not configured")
	}
	sid, err := uuid.Parse(strings.TrimSpace(spaceID))
	if err != nil {
		return status.Error(codes.InvalidArgument, "invalid space id")
	}
	pid := strings.TrimSpace(profileID)
	ctx = ForwardIncomingMetadata(ctx)
	resp, err := g.Client.ListMembers(ctx, &spacev1.ListMembersRequest{
		SpaceId: sid.String(),
		Page:    &commonv1.CursorPageRequest{PageSize: 500},
	})
	if err != nil {
		st, ok := status.FromError(err)
		if ok && (st.Code() == codes.PermissionDenied || st.Code() == codes.NotFound) {
			return grpcsvc.ErrNotSpaceMember
		}
		return err
	}
	for _, m := range resp.GetSpaceMemberList().GetMembers() {
		if strings.EqualFold(m.GetProfileId(), pid) {
			return nil
		}
	}
	return grpcsvc.ErrNotSpaceMember
}
