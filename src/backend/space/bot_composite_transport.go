package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"os"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	botv1 "voice.app/voice/bot/v1"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/s2s"
)

func newBotCompositeTransport(issuer *principal.Issuer) (*s2s.BotInstallationDecision, *grpc.ClientConn, error) {
	names := []string{"SPACE_BOT_COMPOSITE_GRPC_ADDR", "SPACE_BOT_COMPOSITE_TLS_CA_FILE", "SPACE_BOT_COMPOSITE_TLS_SERVER_NAME", "SPACE_BOT_COMPOSITE_CLIENT_CERT_FILE", "SPACE_BOT_COMPOSITE_CLIENT_KEY_FILE"}
	values := make([]string, len(names))
	enabled := false
	for i, name := range names {
		values[i] = os.Getenv(name)
		enabled = enabled || values[i] != ""
	}
	if !enabled {
		return nil, nil, nil
	}
	for _, value := range values {
		if value == "" || value != strings.TrimSpace(value) {
			return nil, nil, errors.New("space Bot composite TLS configuration incomplete")
		}
	}
	if issuer == nil {
		return nil, nil, errors.New("space Bot composite requires Space principal issuer")
	}
	pem, err := os.ReadFile(values[1])
	if err != nil {
		return nil, nil, err
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(pem) {
		return nil, nil, errors.New("bot composite CA contains no certificates")
	}
	cert, err := tls.LoadX509KeyPair(values[3], values[4])
	if err != nil {
		return nil, nil, err
	}
	conn, err := grpc.NewClient(grpcclient.DialTarget(values[0]), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: values[2], Certificates: []tls.Certificate{cert}})))
	if err != nil {
		return nil, nil, err
	}
	return &s2s.BotInstallationDecision{Issuer: issuer, Client: botv1.NewBotServiceClient(conn)}, conn, nil
}
