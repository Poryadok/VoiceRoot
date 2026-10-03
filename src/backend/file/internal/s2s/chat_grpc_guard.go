package s2s

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	grpcsvc "voice/backend/file/internal/grpcsvc"

	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
)

type GRPCChatGuard struct {
	Client    chatv1.ChatServiceClient
	Messaging messagingv1.MessagingServiceClient
}

func NewGRPCChatGuard(c chatv1.ChatServiceClient, messaging ...messagingv1.MessagingServiceClient) *GRPCChatGuard {
	g := &GRPCChatGuard{Client: c}
	if len(messaging) > 0 {
		g.Messaging = messaging[0]
	}
	return g
}

// MessageReadEntitledForMessage resolves the message's immutable chat/time via
// Messaging, then asks Chat to evaluate its member interval at that timestamp.
func (g *GRPCChatGuard) MessageReadEntitledForMessage(ctx context.Context, messageID, profileID uuid.UUID) (bool, error) {
	if g == nil || g.Client == nil || g.Messaging == nil {
		return false, status.Error(codes.FailedPrecondition, "message entitlement services not configured")
	}
	ctx = fileServiceContext(ctx)
	message, err := g.Messaging.GetMessage(ctx, &messagingv1.GetMessageRequest{MessageId: messageID.String()})
	if err != nil {
		return false, err
	}
	createdAt := message.GetMessage().GetCreatedAt()
	chatID, err := uuid.Parse(message.GetMessage().GetChat().GetId())
	if err != nil || createdAt == nil || createdAt.CheckValid() != nil {
		return false, status.Error(codes.Internal, "invalid message entitlement source")
	}
	resp, err := g.Client.CheckMessageReadEntitlement(ctx, &chatv1.CheckMessageReadEntitlementRequest{
		ChatId: chatID.String(), ProfileId: profileID.String(), MessageCreatedAt: createdAt,
	})
	if err != nil {
		return false, err
	}
	return resp.GetEntitled(), nil
}

func fileServiceContext(ctx context.Context) context.Context {
	ctx = ForwardIncomingMetadata(ctx)
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.MD{}
	} else {
		md = md.Copy()
	}
	md.Set("x-voice-internal-caller", "file")
	return metadata.NewOutgoingContext(ctx, md)
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

func (g *GRPCChatGuard) ChatE2EState(ctx context.Context, chatID uuid.UUID) (string, bool, error) {
	if g == nil || g.Client == nil {
		return "", false, status.Error(codes.FailedPrecondition, "chat service not configured")
	}
	ctx = ForwardIncomingMetadata(ctx)
	resp, err := g.Client.GetChat(ctx, &chatv1.GetChatRequest{ChatId: chatID.String()})
	if err != nil {
		st, ok := status.FromError(err)
		if ok && (st.Code() == codes.PermissionDenied || st.Code() == codes.NotFound) {
			return "", false, grpcsvc.ErrNotChatMember
		}
		return "", false, err
	}
	chat := resp.GetChat()
	var typ string
	switch chat.GetType() {
	case chatv1.ChatType_CHAT_TYPE_DM:
		typ = "dm"
	case chatv1.ChatType_CHAT_TYPE_GROUP:
		typ = "group"
	case chatv1.ChatType_CHAT_TYPE_CHANNEL:
		typ = "channel"
	default:
		typ = ""
	}
	return typ, chat.GetE2EEnabled(), nil
}
