package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/http"
	"os"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/principal"
)

type lifecycleClientConfig struct{ address, ca, serverName, cert, key string }

func spaceLifecycleClientConfigFromEnv() (lifecycleClientConfig, bool, error) {
	c := lifecycleClientConfig{os.Getenv("GATEWAY_SPACE_LIFECYCLE_GRPC_ADDR"), os.Getenv("GATEWAY_SPACE_LIFECYCLE_TLS_CA_FILE"), os.Getenv("GATEWAY_SPACE_LIFECYCLE_TLS_SERVER_NAME"), os.Getenv("GATEWAY_SPACE_LIFECYCLE_CLIENT_CERT_FILE"), os.Getenv("GATEWAY_SPACE_LIFECYCLE_CLIENT_KEY_FILE")}
	active := false
	for _, s := range []string{c.address, c.ca, c.serverName, c.cert, c.key} {
		if s != "" {
			active = true
		}
	}
	if !active {
		return c, false, nil
	}
	for _, s := range []string{c.address, c.ca, c.serverName, c.cert, c.key} {
		if s == "" || s != strings.TrimSpace(s) {
			return c, true, errors.New("Gateway Space lifecycle TLS configuration is incomplete")
		}
	}
	if _, err := c.tlsConfig(); err != nil {
		return c, true, err
	}
	return c, true, nil
}
func (c lifecycleClientConfig) tlsConfig() (*tls.Config, error) {
	data, err := os.ReadFile(c.ca)
	if err != nil {
		return nil, err
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(data) {
		return nil, errors.New("Space lifecycle CA has no certificates")
	}
	cert, err := tls.LoadX509KeyPair(c.cert, c.key)
	if err != nil {
		return nil, err
	}
	return &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: c.serverName, Certificates: []tls.Certificate{cert}}, nil
}
func (c lifecycleClientConfig) dial() (*grpc.ClientConn, error) {
	tlsConfig, err := c.tlsConfig()
	if err != nil {
		return nil, err
	}
	return grpc.NewClient(grpcclient.DialTarget(c.address), grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
}

func (t *transcoder) lifecycleContext(r *http.Request, req proto.Message, method, requestID string) (context.Context, error) {
	claims, ok := r.Context().Value(verifiedUserClaimsKey{}).(tokenClaims)
	if !ok || !canonicalLifecycleUUID(claims.UserID) || !canonicalLifecycleUUID(claims.ProfileID) || claims.SessionEpoch <= 0 || claims.ExpiresAt.IsZero() {
		return nil, status.Error(codes.Unauthenticated, "verified lifecycle actor required")
	}
	if method != spacev1.SpaceService_GetSpace_FullMethodName && effectiveAccountType(claims) != "regular" {
		return nil, status.Error(codes.PermissionDenied, "regular account required")
	}
	if t.lifecycleIssuer == nil || t.clients.spaceLifecycle == nil {
		return nil, status.Error(codes.Unavailable, "Space lifecycle transport unavailable")
	}
	hash, err := principal.RequestHash(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid lifecycle request")
	}
	token, err := t.lifecycleIssuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: method, RequestID: requestID, RequestHash: hash, AccountID: claims.UserID, ProfileID: claims.ProfileID, SessionEpoch: claims.SessionEpoch, ClientExpiresAt: claims.ExpiresAt})
	if err != nil {
		return nil, status.Error(codes.Unavailable, "lifecycle credential unavailable")
	}
	// Replace all forwarded identity and authorization headers with the signed binding.
	return metadata.NewOutgoingContext(r.Context(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID)), nil
}
