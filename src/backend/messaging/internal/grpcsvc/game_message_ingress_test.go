package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/messaging/internal/store"
)

type gameMessageProcessorFunc func(context.Context, string, string) (*store.MessageRow, error)

func (f gameMessageProcessorFunc) ProcessGameMessage(ctx context.Context, compact, authority string) (*store.MessageRow, error) {
	return f(ctx, compact, authority)
}

func TestApplyGameMessageFailsClosedWithoutCurrentBindingProcessor(t *testing.T) {
	_, err := (&MessagingGRPC{}).ApplyGameMessage(context.Background(), &messagingv1.ApplyGameMessageRequest{
		CompactJws: "signed-message", DeviceAuthorityAssertion: "signed-authority",
	})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestApplyGameMessageForwardsExactSignedBytesAndReturnsStoredMessage(t *testing.T) {
	chatID, messageID, profileID := uuid.New(), uuid.New(), uuid.New()
	called := false
	svc := &MessagingGRPC{GameMessages: gameMessageProcessorFunc(func(_ context.Context, compact, authority string) (*store.MessageRow, error) {
		called = true
		require.Equal(t, "header.payload.signature", compact)
		require.Equal(t, "auth.header.payload.signature", authority)
		return &store.MessageRow{ID: messageID, ChatID: chatID, SenderProfileID: profileID, Content: "verified"}, nil
	})}
	response, err := svc.ApplyGameMessage(context.Background(), &messagingv1.ApplyGameMessageRequest{
		CompactJws: "header.payload.signature", DeviceAuthorityAssertion: "auth.header.payload.signature",
	})
	require.NoError(t, err)
	require.True(t, called)
	require.Equal(t, messageID.String(), response.GetMessage().GetId())
	require.Equal(t, chatID.String(), response.GetMessage().GetChat().GetId())
	require.Equal(t, "verified", response.GetMessage().GetContent())
}

func TestApplyGameMessageRejectsMissingEnvelopeParts(t *testing.T) {
	svc := &MessagingGRPC{GameMessages: gameMessageProcessorFunc(func(context.Context, string, string) (*store.MessageRow, error) {
		t.Fatal("processor must not receive an incomplete envelope")
		return nil, nil
	})}
	_, err := svc.ApplyGameMessage(context.Background(), &messagingv1.ApplyGameMessageRequest{DeviceAuthorityAssertion: "only-authority"})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestApplyGameMessageAllowsReceiptRetryWithoutFreshAuthority(t *testing.T) {
	called := false
	svc := &MessagingGRPC{GameMessages: gameMessageProcessorFunc(func(_ context.Context, compact, authority string) (*store.MessageRow, error) {
		called = true
		require.Equal(t, "expired-but-retained-jws", compact)
		require.Empty(t, authority)
		return &store.MessageRow{ID: uuid.New(), ChatID: uuid.New(), SenderProfileID: uuid.New(), Content: "receipt"}, nil
	})}
	_, err := svc.ApplyGameMessage(context.Background(), &messagingv1.ApplyGameMessageRequest{CompactJws: "expired-but-retained-jws"})
	require.NoError(t, err)
	require.True(t, called)
}
