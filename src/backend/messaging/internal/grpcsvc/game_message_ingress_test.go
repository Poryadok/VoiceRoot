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
	"voice/backend/pkg/principal"
)

type gameMessageProcessorFunc func(context.Context, string, string) (*store.MessageRow, error)

func (f gameMessageProcessorFunc) ProcessGameMessage(ctx context.Context, compact, authority string) (*store.MessageRow, error) {
	return f(ctx, compact, authority)
}

func TestApplyGameMessageFailsClosedWithoutCurrentBindingProcessor(t *testing.T) {
	request := &messagingv1.ApplyGameMessageRequest{
		CompactJws: "signed-message", DeviceAuthorityAssertion: "signed-authority",
	}
	_, err := (&MessagingGRPC{}).ApplyGameMessage(gatewayApplyContext(t, request, nil), request)
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
	request := &messagingv1.ApplyGameMessageRequest{
		CompactJws: "header.payload.signature", DeviceAuthorityAssertion: "auth.header.payload.signature",
	}
	response, err := svc.ApplyGameMessage(gatewayApplyContext(t, request, nil), request)
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
	request := &messagingv1.ApplyGameMessageRequest{CompactJws: "expired-but-retained-jws"}
	_, err := svc.ApplyGameMessage(gatewayApplyContext(t, request, nil), request)
	require.NoError(t, err)
	require.True(t, called)
}

func TestApplyGameMessageRequiresExactGatewayPrincipalBeforeProcessing(t *testing.T) {
	request := &messagingv1.ApplyGameMessageRequest{CompactJws: "signed-message", DeviceAuthorityAssertion: "signed-authority"}
	calls := 0
	svc := &MessagingGRPC{GameMessages: gameMessageProcessorFunc(func(context.Context, string, string) (*store.MessageRow, error) {
		calls++
		return &store.MessageRow{ID: uuid.New(), ChatID: uuid.New(), SenderProfileID: uuid.New()}, nil
	})}
	for _, test := range []struct {
		name string
		edit func(*principal.Principal)
	}{
		{name: "missing"},
		{name: "wrong issuer", edit: func(p *principal.Principal) { p.Issuer = "moderation" }},
		{name: "wrong method", edit: func(p *principal.Principal) { p.RPC = "/voice.messaging.v1.MessagingService/SendMessage" }},
		{name: "wrong request digest", edit: func(p *principal.Principal) { p.RequestHash = "sha256:wrong" }},
		{name: "caller identity claims", edit: func(p *principal.Principal) { p.ProfileID = uuid.NewString() }},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			if test.name != "missing" {
				ctx = gatewayApplyContext(t, request, test.edit)
			}
			_, err := svc.ApplyGameMessage(ctx, request)
			require.Equal(t, codes.PermissionDenied, status.Code(err))
		})
	}
	require.Zero(t, calls, "unverified or misbound Gateway principals must not reach the processor")
}

func gatewayApplyContext(t *testing.T, req *messagingv1.ApplyGameMessageRequest, edit func(*principal.Principal)) context.Context {
	t.Helper()
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	verified := principal.Principal{Kind: "service", Issuer: "gateway", Subject: "service:gateway", Audience: "messaging", RPC: messagingv1.MessagingService_ApplyGameMessage_FullMethodName, RequestID: "request-1", RequestHash: hash}
	if edit != nil {
		edit(&verified)
	}
	return principal.WithVerified(context.Background(), verified)
}
