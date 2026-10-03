package s2s

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"
	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
)

type entitlementChatClient struct {
	chatv1.ChatServiceClient
	request *chatv1.CheckMessageReadEntitlementRequest
	caller  string
}

func (c *entitlementChatClient) CheckMessageReadEntitlement(ctx context.Context, req *chatv1.CheckMessageReadEntitlementRequest, _ ...grpc.CallOption) (*chatv1.CheckMessageReadEntitlementResponse, error) {
	c.request = req
	md, _ := metadata.FromOutgoingContext(ctx)
	values := md.Get("x-voice-internal-caller")
	if len(values) == 1 {
		c.caller = values[0]
	}
	return &chatv1.CheckMessageReadEntitlementResponse{Entitled: true}, nil
}

type entitlementMessagingClient struct {
	messagingv1.MessagingServiceClient
	response *messagingv1.GetMessageResponse
	caller   string
	profile  string
}

func (c *entitlementMessagingClient) GetMessage(ctx context.Context, _ *messagingv1.GetMessageRequest, _ ...grpc.CallOption) (*messagingv1.GetMessageResponse, error) {
	md, _ := metadata.FromOutgoingContext(ctx)
	callers, profiles := md.Get("x-voice-internal-caller"), md.Get("x-voice-profile-id")
	if len(callers) == 1 {
		c.caller = callers[0]
	}
	if len(profiles) == 1 {
		c.profile = profiles[0]
	}
	return c.response, nil
}

func TestMessageReadEntitledForMessageResolvesMessagingTimestampAndChecksChat(t *testing.T) {
	chatID, messageID, profileID := uuid.New(), uuid.New(), uuid.New()
	createdAt := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	chat := &entitlementChatClient{}
	messaging := &entitlementMessagingClient{response: &messagingv1.GetMessageResponse{Message: &messagingv1.Message{
		Chat: &chatv1.ChatRef{Id: chatID.String()}, CreatedAt: timestamppb.New(createdAt),
	}}}
	guard := NewGRPCChatGuard(chat, messaging)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-profile-id", profileID.String(), "x-voice-internal-caller", "forged"))
	allowed, err := guard.MessageReadEntitledForMessage(ctx, messageID, profileID)
	require.NoError(t, err)
	require.True(t, allowed)
	require.Equal(t, "file", messaging.caller)
	require.Equal(t, profileID.String(), messaging.profile)
	require.Equal(t, "file", chat.caller)
	require.Equal(t, chatID.String(), chat.request.GetChatId())
	require.Equal(t, profileID.String(), chat.request.GetProfileId())
	require.Equal(t, createdAt, chat.request.GetMessageCreatedAt().AsTime())
}
