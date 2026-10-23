package main

import (
	"errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"os"
	"strings"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/bot/internal/grpcsvc"
	"voice/backend/bot/internal/spacemembership"
	"voice/backend/pkg/grpcclient"
)

func wireProtectedMessageSender(svc *grpcsvc.BotGRPC) (*grpc.ClientConn, error) {
	const prefix = "BOT_MESSAGE_SEND_"
	configured := false
	for _, suffix := range []string{"GRPC_ADDR", "TLS_CA_FILE", "TLS_SERVER_NAME", "CLIENT_CERT_FILE", "CLIENT_KEY_FILE"} {
		if _, ok := os.LookupEnv(prefix + suffix); ok {
			configured = true
		}
	}
	if !configured {
		return nil, nil
	}
	if svc == nil || svc.PrincipalIssuer == nil {
		return nil, errors.New("bot message signer required")
	}
	endpoint, err := spacemembership.LoadEndpoint(func(name string) (string, bool) {
		name = strings.Replace(name, "TLS_CERT_FILE", "CLIENT_CERT_FILE", 1)
		name = strings.Replace(name, "TLS_KEY_FILE", "CLIENT_KEY_FILE", 1)
		return os.LookupEnv(name)
	}, prefix)
	if err != nil {
		return nil, err
	}
	tlsConfig, err := endpoint.TLS()
	if err != nil {
		return nil, err
	}
	conn, err := grpc.NewClient(grpcclient.DialTarget(endpoint.Address), grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	if err != nil {
		return nil, err
	}
	svc.ProtectedMessaging = messagingv1.NewMessagingServiceClient(conn)
	return conn, nil
}
