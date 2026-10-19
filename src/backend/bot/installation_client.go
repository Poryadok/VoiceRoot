package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"os"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	spacev1 "voice.app/voice/space/v1"
	grpcsvc "voice/backend/bot/internal/grpcsvc"
	"voice/backend/pkg/grpcclient"
)

// This is a distinct protected connection. The ordinary SPACE_GRPC_ADDR client
// cannot be substituted when mTLS, signing or receiver configuration is missing.
func wireInstallationSpace(svc *grpcsvc.BotGRPC, getenv func(string) string) (*grpc.ClientConn, error) {
	keys := []string{"BOT_SPACE_INSTALLATION_GRPC_ADDR", "BOT_SPACE_INSTALLATION_TLS_CA_FILE", "BOT_SPACE_INSTALLATION_TLS_SERVER_NAME", "BOT_SPACE_INSTALLATION_TLS_CERT_FILE", "BOT_SPACE_INSTALLATION_TLS_KEY_FILE"}
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
		return nil, errors.New("Bot Space installation mTLS configuration is incomplete")
	}
	if svc == nil || svc.PrincipalIssuer == nil {
		return nil, errors.New("Bot Space installation requires the Bot service-principal issuer")
	}
	ca, err := os.ReadFile(values[1])
	if err != nil {
		return nil, err
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(ca) {
		return nil, errors.New("Bot Space installation CA contains no certificates")
	}
	cert, err := tls.LoadX509KeyPair(values[3], values[4])
	if err != nil {
		return nil, err
	}
	config := &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: values[2], Certificates: []tls.Certificate{cert}}
	conn, err := grpc.NewClient(grpcclient.DialTarget(values[0]), grpc.WithTransportCredentials(credentials.NewTLS(config)))
	if err != nil {
		return nil, err
	}
	svc.InstallationSpace = spacev1.NewSpaceServiceClient(conn)
	svc.CleanupSpace = svc.InstallationSpace
	return conn, nil
}
