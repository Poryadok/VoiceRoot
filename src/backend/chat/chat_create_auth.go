package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"os"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	authv1 "voice.app/voice/auth/v1"
	"voice/backend/chat/internal/chatcreation"
	"voice/backend/pkg/principal"
)

// newChatCreateAuthCheckerFromEnv is explicit composition only. It is not
// installed on a listener here; absent configuration never supplies an allow.
func newChatCreateAuthCheckerFromEnv(issuer *principal.Issuer) (*chatcreation.AuthFloorChecker, func() error, error) {
	const prefix = "CHAT_CREATE_AUTH_"
	names := []string{"GRPC_ADDR", "TLS_CA_FILE", "TLS_SERVER_NAME", "CLIENT_CERT_FILE", "CLIENT_KEY_FILE"}
	values := make([]string, len(names))
	configured := false
	for i, name := range names {
		value, set := os.LookupEnv(prefix + name)
		configured = configured || set
		values[i] = strings.TrimSpace(value)
	}
	if !configured {
		return nil, nil, nil
	}
	for _, value := range values {
		if value == "" {
			return nil, nil, errors.New("complete Chat Create Auth mTLS configuration required")
		}
	}
	if issuer == nil {
		return nil, nil, errors.New("Chat principal issuer required for Auth floor")
	}
	if _, _, err := net.SplitHostPort(values[0]); err != nil {
		return nil, nil, errors.New("Chat Create Auth address must be host:port")
	}
	ca, err := os.ReadFile(values[1])
	if err != nil {
		return nil, nil, err
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(ca) {
		return nil, nil, errors.New("invalid Chat Create Auth CA")
	}
	cert, err := tls.LoadX509KeyPair(values[3], values[4])
	if err != nil {
		return nil, nil, err
	}
	conn, err := grpc.NewClient(values[0], grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: values[2], Certificates: []tls.Certificate{cert}})))
	if err != nil {
		return nil, nil, err
	}
	return &chatcreation.AuthFloorChecker{Client: authv1.NewAuthServiceClient(conn), Issuer: issuer}, conn.Close, nil
}
