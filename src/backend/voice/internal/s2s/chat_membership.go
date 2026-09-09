package s2s

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"voice/backend/voice/internal/grpcsvc"

	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
)

// GRPCChatMembership validates membership via ChatService.ListMembers.
type GRPCChatMembership struct {
	Client chatv1.ChatServiceClient
}

func NewGRPCChatMembership(c chatv1.ChatServiceClient) *GRPCChatMembership {
	return &GRPCChatMembership{Client: c}
}

func (g *GRPCChatMembership) EnsureMember(ctx context.Context, chatID, profileID string) error {
	if g == nil || g.Client == nil {
		return status.Error(codes.FailedPrecondition, "chat service not configured")
	}
	cid, err := uuid.Parse(strings.TrimSpace(chatID))
	if err != nil {
		return status.Error(codes.InvalidArgument, "invalid chat id")
	}
	pid := strings.TrimSpace(profileID)
	ctx = ForwardIncomingMetadata(ctx)
	ctx = metadata.AppendToOutgoingContext(ctx, "x-voice-internal-caller", "voice")
	resp, err := g.Client.ListMembers(ctx, &chatv1.ListMembersRequest{
		ChatId: cid.String(),
		Page:   &commonv1.CursorPageRequest{PageSize: 500},
	})
	if err != nil {
		st, ok := status.FromError(err)
		if ok && (st.Code() == codes.PermissionDenied || st.Code() == codes.NotFound) {
			return grpcsvc.ErrNotChatMember
		}
		return err
	}
	for _, m := range resp.GetMemberList().GetMembers() {
		if strings.EqualFold(m.GetProfileId(), pid) {
			return nil
		}
	}
	return grpcsvc.ErrNotChatMember
}

// EnsureDirectChat verifies that the Chat-authoritative type is DM rather than
// trusting the caller-supplied ChatRef.type.
func (g *GRPCChatMembership) EnsureDirectChat(ctx context.Context, chatID string) error {
	if g == nil || g.Client == nil {
		return status.Error(codes.FailedPrecondition, "chat service not configured")
	}
	cid, err := uuid.Parse(strings.TrimSpace(chatID))
	if err != nil {
		return status.Error(codes.InvalidArgument, "invalid chat id")
	}
	ctx = ForwardIncomingMetadata(ctx)
	ctx = metadata.AppendToOutgoingContext(ctx, "x-voice-internal-caller", "voice")
	resp, err := g.Client.GetChat(ctx, &chatv1.GetChatRequest{ChatId: cid.String()})
	if err != nil {
		st, ok := status.FromError(err)
		if ok && (st.Code() == codes.PermissionDenied || st.Code() == codes.NotFound) {
			return grpcsvc.ErrNotChatMember
		}
		return err
	}
	if resp.GetChat().GetType() != chatv1.ChatType_CHAT_TYPE_DM {
		return grpcsvc.ErrNotDirectChat
	}
	return nil
}
