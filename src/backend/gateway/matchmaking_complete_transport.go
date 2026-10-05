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

	matchmakingv1 "voice.app/voice/matchmaking/v1"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/principal"
)

type matchmakingCompleteClientConfig struct{ address, ca, serverName, cert, key string }

func matchmakingCompleteClientConfigFromEnv() (matchmakingCompleteClientConfig, bool, error) {
	c := matchmakingCompleteClientConfig{
		os.Getenv("GATEWAY_MATCHMAKING_COMPLETE_GRPC_ADDR"),
		os.Getenv("GATEWAY_MATCHMAKING_COMPLETE_TLS_CA_FILE"),
		os.Getenv("GATEWAY_MATCHMAKING_COMPLETE_TLS_SERVER_NAME"),
		os.Getenv("GATEWAY_MATCHMAKING_COMPLETE_CLIENT_CERT_FILE"),
		os.Getenv("GATEWAY_MATCHMAKING_COMPLETE_CLIENT_KEY_FILE"),
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
			return c, true, errors.New("Gateway CompleteMatch TLS configuration is incomplete")
		}
	}
	if _, err := c.tlsConfig(); err != nil {
		return c, true, err
	}
	return c, true, nil
}

func (c matchmakingCompleteClientConfig) tlsConfig() (*tls.Config, error) {
	data, err := os.ReadFile(c.ca)
	if err != nil {
		return nil, err
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(data) {
		return nil, errors.New("CompleteMatch server CA has no certificates")
	}
	cert, err := tls.LoadX509KeyPair(c.cert, c.key)
	if err != nil {
		return nil, err
	}
	return &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: c.serverName, Certificates: []tls.Certificate{cert}}, nil
}

func (c matchmakingCompleteClientConfig) dial() (*grpc.ClientConn, error) {
	tlsConfig, err := c.tlsConfig()
	if err != nil {
		return nil, err
	}
	return grpc.NewClient(grpcclient.DialTarget(c.address), grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
}

func (t *transcoder) matchmakingCompleteContext(r *http.Request, req *matchmakingv1.CompleteMatchRequest) (context.Context, error) {
	claims, ok := r.Context().Value(verifiedUserClaimsKey{}).(tokenClaims)
	if !ok || !canonicalLifecycleUUID(claims.UserID) || !canonicalLifecycleUUID(claims.ProfileID) || claims.SessionEpoch <= 0 || claims.ExpiresAt.IsZero() {
		return nil, status.Error(codes.Unauthenticated, "verified Matchmaking actor required")
	}
	if effectiveAccountType(claims) != "regular" {
		return nil, status.Error(codes.PermissionDenied, "regular account required")
	}
	if t.matchmakingCompleteIssuer == nil || t.clients.matchmakingComplete == nil || req == nil || !canonicalLifecycleUUID(req.GetOperationId()) {
		return nil, status.Error(codes.Unavailable, "CompleteMatch protected transport unavailable")
	}
	hash, err := principal.RequestHash(proto.Message(req))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid CompleteMatch request")
	}
	requestID := req.GetOperationId()
	if strings.TrimSpace(req.GetOperationId()) != requestID {
		return nil, status.Error(codes.InvalidArgument, "invalid CompleteMatch operation ID")
	}
	token, err := t.matchmakingCompleteIssuer.IssueDelegatedUser(principal.DelegatedUserInput{
		Audience: "matchmaking", RPC: matchmakingv1.MatchmakingService_CompleteMatch_FullMethodName,
		RequestID: requestID, RequestHash: hash,
		AccountID: claims.UserID, ProfileID: claims.ProfileID, SessionEpoch: claims.SessionEpoch, ClientExpiresAt: claims.ExpiresAt,
	})
	if err != nil {
		return nil, status.Error(codes.Unavailable, "CompleteMatch credential unavailable")
	}
	return metadata.NewOutgoingContext(r.Context(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID)), nil
}
