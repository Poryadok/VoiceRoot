package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"testing"
	"time"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/messaging/internal/grpcsvc"
	"voice/backend/messaging/internal/principalruntime"
	"voice/backend/messaging/internal/store"
	"voice/backend/pkg/principal"
)

type botSendRejectVerifier struct{}

func (botSendRejectVerifier) Verify(context.Context, string, string, string, string) (principal.Principal, error) {
	return principal.Principal{}, context.Canceled
}
func TestBotSendRuntimeRestrictedMTLSAndPartialConfiguration(t *testing.T) {
	empty, err := newBotSendRuntime(func(string) string { return "" }, nil, nil, nil)
	require.NoError(t, err)
	require.Nil(t, empty)
	_, err = newBotSendRuntime(func(name string) string {
		if name == "MESSAGING_BOT_SEND_GRPC_LISTEN" {
			return ":0"
		}
		return ""
	}, nil, nil, nil)
	require.Error(t, err)
	fixture := roleJWKSFixture(t, "bot-send")
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "messaging", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	env := map[string]string{"MESSAGING_BOT_SEND_GRPC_LISTEN": "127.0.0.1:0", "MESSAGING_BOT_SEND_TLS_CERT_FILE": fixture.cert, "MESSAGING_BOT_SEND_TLS_KEY_FILE": fixture.key, "MESSAGING_BOT_SEND_CLIENT_CA_FILE": fixture.ca}
	for _, prefix := range []string{"MESSAGING_BOT_SEND_AUTHORITY_", "MESSAGING_BOT_SEND_SPACE_"} {
		env[prefix+"GRPC_ADDR"] = "127.0.0.1:1"
		env[prefix+"TLS_CA_FILE"] = fixture.ca
		env[prefix+"TLS_SERVER_NAME"] = "messaging"
		env[prefix+"CLIENT_CERT_FILE"] = fixture.cert
		env[prefix+"CLIENT_KEY_FILE"] = fixture.key
	}
	runtime, err := newBotSendRuntime(func(name string) string { return env[name] }, &grpcsvc.MessagingGRPC{Messages: &store.MessagesStore{}, ChatThreadPolicy: &store.SQLChatThreadPolicy{}}, botSendRejectVerifier{}, issuer)
	require.NoError(t, err)
	defer runtime.Close()
	var missingVerifier *principalruntime.Runtime
	absent, err := newBotSendRuntime(func(name string) string { return env[name] }, &grpcsvc.MessagingGRPC{Messages: &store.MessagesStore{}, ChatThreadPolicy: &store.SQLChatThreadPolicy{}}, missingVerifier, issuer)
	require.Error(t, err)
	require.Nil(t, absent)
	for _, authenticated := range []bool{true, false} {
		config := &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: fixture.roots, ServerName: "messaging"}
		if authenticated {
			config.Certificates = []tls.Certificate{fixture.client}
		}
		connection, err := grpc.NewClient(runtime.listener.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(config)))
		require.NoError(t, err)
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		client := messagingv1.NewMessagingServiceClient(connection)
		_, err = client.SendBotMessage(ctx, &messagingv1.SendBotMessageRequest{})
		if authenticated {
			require.Equal(t, codes.Unauthenticated, status.Code(err))
		} else {
			require.Error(t, err)
			require.NotEqual(t, codes.Unauthenticated, status.Code(err))
		}
		if authenticated {
			_, err = client.SendMessage(ctx, &messagingv1.SendMessageRequest{})
			require.Equal(t, codes.Unimplemented, status.Code(err))
			_, err = client.GetMessage(ctx, &messagingv1.GetMessageRequest{})
			require.Equal(t, codes.Unimplemented, status.Code(err))
		}
		cancel()
		require.NoError(t, connection.Close())
	}
}
