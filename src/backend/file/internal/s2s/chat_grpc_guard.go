package s2s

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	grpcsvc "voice/backend/file/internal/grpcsvc"

	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
)

type GRPCChatGuard struct {
	Client chatv1.ChatServiceClient
}

func NewGRPCChatGuard(c chatv1.ChatServiceClient) *GRPCChatGuard {
	return &GRPCChatGuard{Client: c}
}

func (g *GRPCChatGuard) EnsureMember(ctx context.Context, chatID, profileID uuid.UUID) error {
	if g == nil || g.Client == nil {
		return status.Error(codes.FailedPrecondition, "chat service not configured")
	}
	ctx = ForwardIncomingMetadata(ctx)
	resp, err := g.Client.ListMembers(ctx, &chatv1.ListMembersRequest{
		ChatId: chatID.String(),
		Page:   &commonv1.CursorPageRequest{PageSize: 100},
	})
	if err != nil {
		st, ok := status.FromError(err)
		if ok && (st.Code() == codes.PermissionDenied || st.Code() == codes.NotFound) {
			return grpcsvc.ErrNotChatMember
		}
		return err
	}
	for _, member := range resp.GetMemberList().GetMembers() {
		if strings.EqualFold(member.GetProfileId(), profileID.String()) {
			return nil
		}
	}
	return grpcsvc.ErrNotChatMember
}
