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
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/principal"
	"voice/backend/role/internal/memberauthority"
)

// Membership authority has a separate Role-only Space listener. Reusing the
// override/ordinary connection would silently change the protected serving set.
func loadMemberAuthorityClient(issuer *principal.Issuer) (*memberauthority.Client, func(), error) {
	const prefix = "ROLE_SPACE_MEMBER_AUTHORITY_"
	values := map[string]string{}
	enabled := false
	for _, field := range []string{"GRPC_ADDR", "TLS_CA_FILE", "TLS_SERVER_NAME", "CLIENT_CERT_FILE", "CLIENT_KEY_FILE"} {
		value, present := os.LookupEnv(prefix + field)
		enabled = enabled || present
		values[field] = strings.TrimSpace(value)
	}
	noop := func() {}
	if !enabled {
		return nil, noop, nil
	}
	if issuer == nil {
		return nil, noop, errors.New("role member authority signing identity required")
	}
	for _, value := range values {
		if value == "" {
			return nil, noop, errors.New("complete protected Space member authority configuration required")
		}
	}
	certificate, err := tls.LoadX509KeyPair(values["CLIENT_CERT_FILE"], values["CLIENT_KEY_FILE"])
	if err != nil {
		return nil, noop, errors.New("role member authority client identity unavailable")
	}
	ca, err := os.ReadFile(values["TLS_CA_FILE"])
	if err != nil {
		return nil, noop, errors.New("space member authority CA unavailable")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(ca) {
		return nil, noop, errors.New("invalid Space member authority CA")
	}
	connection, err := grpc.NewClient(grpcclient.DialTarget(values["GRPC_ADDR"]), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: values["TLS_SERVER_NAME"], Certificates: []tls.Certificate{certificate}})))
	if err != nil {
		return nil, noop, errors.New("protected Space member authority connection unavailable")
	}
	return &memberauthority.Client{Space: spacev1.NewSpaceServiceClient(connection), Issuer: issuer}, func() { _ = connection.Close() }, nil
}
