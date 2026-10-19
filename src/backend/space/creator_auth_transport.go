package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"os"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"voice/backend/pkg/grpcclient"
)

type creatorAuthConfig struct{ address, ca, serverName, cert, key string }

func creatorAuthConfigFromEnv() (creatorAuthConfig, bool, error) {
	c := creatorAuthConfig{os.Getenv("SPACE_AUTH_SESSION_FLOOR_GRPC_ADDR"), os.Getenv("SPACE_AUTH_SESSION_FLOOR_TLS_CA_FILE"), os.Getenv("SPACE_AUTH_SESSION_FLOOR_TLS_SERVER_NAME"), os.Getenv("SPACE_AUTH_SESSION_FLOOR_CLIENT_CERT_FILE"), os.Getenv("SPACE_AUTH_SESSION_FLOOR_CLIENT_KEY_FILE")}
	active := false
	values := []string{c.address, c.ca, c.serverName, c.cert, c.key}
	for _, value := range values {
		active = active || value != ""
	}
	if !active {
		return c, false, nil
	}
	for _, value := range values {
		if value == "" || value != strings.TrimSpace(value) {
			return c, true, errors.New("Space Auth session-floor mutual TLS configuration is incomplete")
		}
	}
	_, err := c.tlsConfig()
	return c, true, err
}

func (c creatorAuthConfig) tlsConfig() (*tls.Config, error) {
	encoded, err := os.ReadFile(c.ca)
	if err != nil {
		return nil, err
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(encoded) {
		return nil, errors.New("Auth session-floor CA contains no certificates")
	}
	identity, err := tls.LoadX509KeyPair(c.cert, c.key)
	if err != nil {
		return nil, err
	}
	return &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: c.serverName, Certificates: []tls.Certificate{identity}}, nil
}

func (c creatorAuthConfig) dial() (*grpc.ClientConn, error) {
	config, err := c.tlsConfig()
	if err != nil {
		return nil, err
	}
	return grpc.NewClient(grpcclient.DialTarget(c.address), grpc.WithTransportCredentials(credentials.NewTLS(config)), grpc.WithDisableRetry())
}
