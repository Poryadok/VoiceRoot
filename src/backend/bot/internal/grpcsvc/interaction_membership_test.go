package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"voice/backend/bot/internal/store"

	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
)

type membershipProbe struct {
	chatv1.ChatServiceClient
	err      error
	profile  string
	metadata metadata.MD
	response *chatv1.GetChatResponse
	chatID   string
}

func (p *membershipProbe) ListMembers(context.Context, *chatv1.ListMembersRequest, ...grpc.CallOption) (*chatv1.ListMembersResponse, error) {
	return &chatv1.ListMembersResponse{}, nil
}

func (p *membershipProbe) GetChat(ctx context.Context, req *chatv1.GetChatRequest, _ ...grpc.CallOption) (*chatv1.GetChatResponse, error) {
	md, _ := metadata.FromOutgoingContext(ctx)
	p.metadata = md.Copy()
	p.profile = md.Get("x-voice-profile-id")[0]
	p.chatID = req.GetChatId()
	if len(md.Get("x-voice-internal")) != 0 {
		return nil, status.Error(codes.PermissionDenied, "internal bypass forwarded")
	}
	if p.err != nil {
		return nil, p.err
	}
	return p.response, nil
}

func TestSlashMembershipFailsClosedAndForwardsOnlyInvoker(t *testing.T) {
	chatID, profileID := uuid.New(), uuid.New()
	svc := &BotGRPC{}
	require.Equal(t, codes.Unavailable, status.Code(svc.requireInvokerMembership(context.Background(), chatID, profileID)))
	probe := &membershipProbe{err: status.Error(codes.PermissionDenied, "removed member")}
	svc.Chat = probe
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-internal-caller", "true"))
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("x-voice-internal-caller", "true", "x-voice-profile-id", uuid.NewString()))
	require.Equal(t, codes.PermissionDenied, status.Code(svc.requireInvokerMembership(ctx, chatID, profileID)))
	require.Equal(t, profileID.String(), probe.profile)
	require.Equal(t, metadata.Pairs("x-voice-profile-id", profileID.String()), probe.metadata)
	probe.err = status.Error(codes.Unavailable, "chat down")
	require.Equal(t, codes.Unavailable, status.Code(svc.requireInvokerMembership(ctx, chatID, profileID)))
	probe.err = status.Error(codes.NotFound, "missing chat")
	require.Equal(t, codes.PermissionDenied, status.Code(svc.requireInvokerMembership(ctx, chatID, profileID)))
	probe.err = nil
	require.Equal(t, codes.Unavailable, status.Code(svc.requireInvokerMembership(ctx, chatID, profileID)))
	probe.response = &chatv1.GetChatResponse{}
	require.Equal(t, codes.Unavailable, status.Code(svc.requireInvokerMembership(ctx, chatID, profileID)))
	probe.response = &chatv1.GetChatResponse{Chat: &chatv1.Chat{Id: uuid.NewString()}}
	require.Equal(t, codes.Unavailable, status.Code(svc.requireInvokerMembership(ctx, chatID, profileID)))
	probe.response.Chat.Id = chatID.String()
	require.NoError(t, svc.requireInvokerMembership(ctx, chatID, profileID))
	require.Equal(t, chatID.String(), probe.chatID)
}

type threadProbe struct {
	messagingv1.MessagingServiceClient
	parent string
}

func (p *threadProbe) SendMessage(_ context.Context, req *messagingv1.SendMessageRequest, _ ...grpc.CallOption) (*messagingv1.SendMessageResponse, error) {
	p.parent = req.GetThreadParentId()
	return &messagingv1.SendMessageResponse{Message: &messagingv1.Message{Id: uuid.NewString()}}, nil
}

func TestDMPostMessageForwardsThreadParent(t *testing.T) {
	probe := &threadProbe{}
	svc := &BotGRPC{Messaging: probe}
	bot := &store.BotRow{ActorProfileID: uuid.New(), OwnerAccountID: uuid.New()}
	parent := uuid.NewString()
	_, err := svc.postMessage(context.Background(), bot, &chatv1.ChatRef{Id: uuid.NewString(), Type: chatTypePtr(chatv1.ChatType_CHAT_TYPE_DM)}, "reply", parent)
	require.NoError(t, err)
	require.Equal(t, parent, probe.parent)
}
