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

	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/principal"
)

type matchSquadMemberClientConfig struct{ address, ca, serverName, cert, key string }

func matchSquadMemberClientConfigFromEnv() (matchSquadMemberClientConfig, bool, error) {
	c := matchSquadMemberClientConfig{
		os.Getenv("GATEWAY_MATCH_SQUAD_MEMBER_GRPC_ADDR"),
		os.Getenv("GATEWAY_MATCH_SQUAD_MEMBER_TLS_CA_FILE"),
		os.Getenv("GATEWAY_MATCH_SQUAD_MEMBER_TLS_SERVER_NAME"),
		os.Getenv("GATEWAY_MATCH_SQUAD_MEMBER_CLIENT_CERT_FILE"),
		os.Getenv("GATEWAY_MATCH_SQUAD_MEMBER_CLIENT_KEY_FILE"),
	}
	active := false
	for _, value := range []string{c.address, c.ca, c.serverName, c.cert, c.key} {
		if value != "" {
			active = true
		}
	}
	if !active {
		return c, false, nil
	}
	for _, value := range []string{c.address, c.ca, c.serverName, c.cert, c.key} {
		if value == "" || value != strings.TrimSpace(value) {
			return c, true, errors.New("Gateway MatchSquad member TLS configuration is incomplete")
		}
	}
	if _, err := c.tlsConfig(); err != nil {
		return c, true, err
	}
	return c, true, nil
}

func (c matchSquadMemberClientConfig) tlsConfig() (*tls.Config, error) {
	data, err := os.ReadFile(c.ca)
	if err != nil {
		return nil, err
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(data) {
		return nil, errors.New("MatchSquad member CA has no certificates")
	}
	cert, err := tls.LoadX509KeyPair(c.cert, c.key)
	if err != nil {
		return nil, err
	}
	return &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: c.serverName, Certificates: []tls.Certificate{cert}}, nil
}

func (c matchSquadMemberClientConfig) dial() (*grpc.ClientConn, error) {
	tlsConfig, err := c.tlsConfig()
	if err != nil {
		return nil, err
	}
	return grpc.NewClient(grpcclient.DialTarget(c.address), grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
}

func (t *transcoder) matchSquadMemberContext(r *http.Request, req proto.Message, method, requestID string) (context.Context, error) {
	claims, ok := r.Context().Value(verifiedUserClaimsKey{}).(tokenClaims)
	if !ok || !canonicalLifecycleUUID(claims.UserID) || !canonicalLifecycleUUID(claims.ProfileID) || claims.SessionEpoch <= 0 || claims.ExpiresAt.IsZero() {
		return nil, status.Error(codes.Unauthenticated, "verified MatchSquad actor required")
	}
	if effectiveAccountType(claims) != "regular" {
		return nil, status.Error(codes.PermissionDenied, "regular account required")
	}
	if t.matchSquadMemberIssuer == nil || t.clients.matchSquadMember == nil {
		return nil, status.Error(codes.Unavailable, "MatchSquad member transport unavailable")
	}
	hash, err := principal.RequestHash(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid MatchSquad request")
	}
	token, err := t.matchSquadMemberIssuer.IssueDelegatedUser(principal.DelegatedUserInput{
		Audience: "voice", RPC: method, RequestID: requestID, RequestHash: hash,
		AccountID: claims.UserID, ProfileID: claims.ProfileID, SessionEpoch: claims.SessionEpoch, ClientExpiresAt: claims.ExpiresAt,
	})
	if err != nil {
		return nil, status.Error(codes.Unavailable, "MatchSquad credential unavailable")
	}
	return metadata.NewOutgoingContext(r.Context(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID)), nil
}
