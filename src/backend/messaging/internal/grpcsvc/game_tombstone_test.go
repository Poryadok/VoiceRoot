package grpcsvc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/pkg/principal"
)

type tombstoneProcessorSpy struct {
	calls   int
	request *messagingv1.TombstoneGameMessageRequest
}

func (s *tombstoneProcessorSpy) ProcessTombstoneGameMessage(_ context.Context, req *messagingv1.TombstoneGameMessageRequest) error {
	s.calls++
	s.request = req
	return nil
}

func TestTombstoneGameMessageRequiresExactVerifiedModerationPrincipal(t *testing.T) {
	req := &messagingv1.TombstoneGameMessageRequest{ActionId: "action", ApplicationId: "app", EnvironmentId: "env", ChatId: "chat", MessageId: "message", ReasonClass: "moderation"}
	processor := &tombstoneProcessorSpy{}
	server := &MessagingGRPC{GameTombstones: processor}

	_, err := server.TombstoneGameMessage(context.Background(), req)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Zero(t, processor.calls)

	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	verified := principal.Principal{Kind: "service", Issuer: "moderation", Subject: "service:moderation", Audience: "messaging", RPC: "/voice.messaging.v1.MessagingService/TombstoneGameMessage", RequestID: "request-1", RequestHash: hash}
	ctx := principal.WithVerified(context.Background(), verified)
	_, err = server.TombstoneGameMessage(ctx, req)
	require.NoError(t, err)
	require.Equal(t, 1, processor.calls)
	require.Same(t, req, processor.request)
}

func TestTombstoneGameMessageRejectsForgedPrincipalBindings(t *testing.T) {
	req := &messagingv1.TombstoneGameMessageRequest{ActionId: "action", ApplicationId: "app", EnvironmentId: "env", ChatId: "chat", MessageId: "message", ReasonClass: "moderation"}
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	base := principal.Principal{Kind: "service", Issuer: "moderation", Subject: "service:moderation", Audience: "messaging", RPC: "/voice.messaging.v1.MessagingService/TombstoneGameMessage", RequestID: "request-1", RequestHash: hash}
	tests := []struct {
		name   string
		mutate func(*principal.Principal)
	}{
		{name: "wrong issuer", mutate: func(p *principal.Principal) { p.Issuer = "gateway" }},
		{name: "wrong subject", mutate: func(p *principal.Principal) { p.Subject = "service:gateway" }},
		{name: "wrong kind", mutate: func(p *principal.Principal) { p.Kind = "delegated_user" }},
		{name: "wrong audience", mutate: func(p *principal.Principal) { p.Audience = "auth" }},
		{name: "wrong rpc", mutate: func(p *principal.Principal) { p.RPC = "/voice.messaging.v1.MessagingService/SendMessage" }},
		{name: "wrong hash", mutate: func(p *principal.Principal) {
			p.RequestHash = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			processor := &tombstoneProcessorSpy{}
			server := &MessagingGRPC{GameTombstones: processor}
			verified := base
			test.mutate(&verified)
			_, err := server.TombstoneGameMessage(principal.WithVerified(context.Background(), verified), req)
			require.Equal(t, codes.PermissionDenied, status.Code(err))
			require.Zero(t, processor.calls)
		})
	}
}
