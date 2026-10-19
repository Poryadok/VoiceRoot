package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"os"
	"strings"
	chatv1 "voice.app/voice/chat/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/principal"
	"voice/backend/role/internal/targetvalidation"
)

func loadOverrideTargetClients(issuer *principal.Issuer) (*targetvalidation.Client, func(), error) {
	chatAddr := strings.TrimSpace(os.Getenv("ROLE_CHAT_TARGET_GRPC_ADDR"))
	spaceAddr := strings.TrimSpace(os.Getenv("ROLE_SPACE_TARGET_GRPC_ADDR"))
	enabled := chatAddr != "" || spaceAddr != ""
	for _, name := range []string{"ROLE_OVERRIDE_TARGET_CLIENT_CERT_FILE", "ROLE_OVERRIDE_TARGET_CLIENT_KEY_FILE", "ROLE_CHAT_TARGET_TLS_CA_FILE", "ROLE_SPACE_TARGET_TLS_CA_FILE", "ROLE_CHAT_TARGET_TLS_SERVER_NAME", "ROLE_SPACE_TARGET_TLS_SERVER_NAME"} {
		if _, ok := os.LookupEnv(name); ok {
			enabled = true
		}
	}
	noop := func() {}
	if !enabled {
		return nil, noop, nil
	}
	if issuer == nil || chatAddr == "" || spaceAddr == "" {
		return nil, noop, errors.New("both owner target addresses and Role principal signing keys required")
	}
	cert, err := tls.LoadX509KeyPair(strings.TrimSpace(os.Getenv("ROLE_OVERRIDE_TARGET_CLIENT_CERT_FILE")), strings.TrimSpace(os.Getenv("ROLE_OVERRIDE_TARGET_CLIENT_KEY_FILE")))
	if err != nil {
		return nil, noop, errors.New("role target client identity unavailable")
	}
	dial := func(target, addr string) (*grpc.ClientConn, error) {
		prefix := "ROLE_" + strings.ToUpper(target) + "_TARGET_TLS_"
		ca, err := os.ReadFile(strings.TrimSpace(os.Getenv(prefix + "CA_FILE")))
		if err != nil {
			return nil, errors.New("owner target CA unavailable")
		}
		roots := x509.NewCertPool()
		if !roots.AppendCertsFromPEM(ca) {
			return nil, errors.New("invalid owner target CA")
		}
		name := strings.TrimSpace(os.Getenv(prefix + "SERVER_NAME"))
		if name == "" {
			return nil, errors.New("owner target TLS server name required")
		}
		return grpc.NewClient(grpcclient.DialTarget(addr), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: name, Certificates: []tls.Certificate{cert}})))
	}
	chatConn, err := dial("chat", chatAddr)
	if err != nil {
		return nil, noop, err
	}
	spaceConn, err := dial("space", spaceAddr)
	if err != nil {
		_ = chatConn.Close()
		return nil, noop, err
	}
	return &targetvalidation.Client{Chat: chatv1.NewChatServiceClient(chatConn), Space: spacev1.NewSpaceServiceClient(spaceConn), Issuer: issuer}, func() { _ = chatConn.Close(); _ = spaceConn.Close() }, nil
}
