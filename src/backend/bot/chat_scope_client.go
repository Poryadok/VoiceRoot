package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"os"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/bot/internal/grpcsvc"
	"voice/backend/pkg/grpcclient"
)

func wireChatScope(svc *grpcsvc.BotGRPC, getenv func(string) string) (*grpc.ClientConn, error) {
	keys := []string{"BOT_CHAT_SCOPE_GRPC_ADDR", "BOT_CHAT_SCOPE_TLS_CA_FILE", "BOT_CHAT_SCOPE_TLS_SERVER_NAME", "BOT_CHAT_SCOPE_TLS_CERT_FILE", "BOT_CHAT_SCOPE_TLS_KEY_FILE"}
	values := make([]string, len(keys))
	count := 0
	for i, key := range keys {
		values[i] = strings.TrimSpace(getenv(key))
		if values[i] != "" {
			count++
		}
	}
	if count == 0 {
		return nil, nil
	}
	if count != len(keys) {
		return nil, errors.New("Bot Chat scope mTLS configuration is incomplete")
	}
	if svc == nil || svc.PrincipalIssuer == nil {
		return nil, errors.New("Bot Chat scope requires service-principal issuer")
	}
	ca, err := os.ReadFile(values[1])
	if err != nil {
		return nil, err
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(ca) {
		return nil, errors.New("Bot Chat scope CA contains no certificates")
	}
	cert, err := tls.LoadX509KeyPair(values[3], values[4])
	if err != nil {
		return nil, err
	}
	conn, err := grpc.NewClient(grpcclient.DialTarget(values[0]), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: values[2], Certificates: []tls.Certificate{cert}})))
	if err != nil {
		return nil, err
	}
	svc.ChatScope = chatv1.NewChatScopeAuthorityServiceClient(conn)
	return conn, nil
}
