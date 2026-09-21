package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"

	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/httpserver"
	"voice/backend/pkg/principal"
)

// signedUserProjectionClient is deliberately separate from ordinary USER_GRPC_ADDR.
type signedUserProjectionClient struct {
	userv1.UserServiceClient
	issuer *principal.Issuer
}

// loadSearchProjectionJWKS exposes the public half of Search's dedicated
// projection signing keys over TLS so User can verify the request-bound token.
func loadSearchProjectionJWKS() (*http.Server, error) {
	listen, configured := os.LookupEnv("SEARCH_PRINCIPAL_JWKS_LISTEN")
	if !configured {
		return nil, nil
	}
	listen = strings.TrimSpace(listen)
	certFile := strings.TrimSpace(os.Getenv("SEARCH_PRINCIPAL_JWKS_TLS_CERT_FILE"))
	keyFile := strings.TrimSpace(os.Getenv("SEARCH_PRINCIPAL_JWKS_TLS_KEY_FILE"))
	dir := strings.TrimSpace(os.Getenv("SEARCH_PRINCIPAL_SIGNING_KEYS_DIR"))
	if listen == "" || certFile == "" || keyFile == "" || dir == "" {
		return nil, errors.New("Search projection JWKS listen, TLS and signing keys are required")
	}
	keys, err := loadSearchSigningKeys(dir)
	if err != nil {
		return nil, err
	}
	document, err := json.Marshal(searchJWKS(keys))
	if err != nil {
		return nil, err
	}
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("Search projection JWKS TLS: %w", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/jwks.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "max-age=30")
		_, _ = w.Write(document)
	})
	server := &http.Server{Addr: listen, Handler: mux, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}}}
	httpserver.ApplyHTTPServerTimeouts(server)
	return server, nil
}

func (c *signedUserProjectionClient) BeginSearchProfileSnapshot(ctx context.Context, req *userv1.BeginSearchProfileSnapshotRequest, opts ...grpc.CallOption) (*userv1.BeginSearchProfileSnapshotResponse, error) {
	ctx, err := c.signedContext(ctx, userv1.UserService_BeginSearchProfileSnapshot_FullMethodName, req)
	if err != nil {
		return nil, err
	}
	return c.UserServiceClient.BeginSearchProfileSnapshot(ctx, req, opts...)
}
func (c *signedUserProjectionClient) ListSearchProfileSnapshot(ctx context.Context, req *userv1.ListSearchProfileSnapshotRequest, opts ...grpc.CallOption) (*userv1.ListSearchProfileSnapshotResponse, error) {
	ctx, err := c.signedContext(ctx, userv1.UserService_ListSearchProfileSnapshot_FullMethodName, req)
	if err != nil {
		return nil, err
	}
	return c.UserServiceClient.ListSearchProfileSnapshot(ctx, req, opts...)
}
func (c *signedUserProjectionClient) ListSearchProfileJournal(ctx context.Context, req *userv1.ListSearchProfileJournalRequest, opts ...grpc.CallOption) (*userv1.ListSearchProfileJournalResponse, error) {
	ctx, err := c.signedContext(ctx, userv1.UserService_ListSearchProfileJournal_FullMethodName, req)
	if err != nil {
		return nil, err
	}
	return c.UserServiceClient.ListSearchProfileJournal(ctx, req, opts...)
}
func (c *signedUserProjectionClient) GetSearchProfileCheckpoint(ctx context.Context, req *userv1.GetSearchProfileCheckpointRequest, opts ...grpc.CallOption) (*userv1.GetSearchProfileCheckpointResponse, error) {
	ctx, err := c.signedContext(ctx, userv1.UserService_GetSearchProfileCheckpoint_FullMethodName, req)
	if err != nil {
		return nil, err
	}
	return c.UserServiceClient.GetSearchProfileCheckpoint(ctx, req, opts...)
}

func (c *signedUserProjectionClient) signedContext(ctx context.Context, rpc string, request proto.Message) (context.Context, error) {
	hash, err := principal.RequestHash(request)
	if err != nil {
		return nil, err
	}
	requestID := uuid.NewString()
	token, err := c.issuer.IssueService(principal.ServiceInput{Audience: "user", RPC: rpc, RequestID: requestID, RequestHash: hash})
	if err != nil {
		return nil, err
	}
	return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID)), nil
}

func loadSignedUserProjectionClientFromEnv() (userv1.UserServiceClient, *grpc.ClientConn, error) {
	names := []string{"SEARCH_USER_PROJECTION_GRPC_ADDR", "SEARCH_USER_PROJECTION_TLS_CA_FILE", "SEARCH_USER_PROJECTION_TLS_SERVER_NAME", "SEARCH_PRINCIPAL_SIGNING_KEYS_DIR", "SEARCH_PRINCIPAL_ACTIVE_KID", "SEARCH_PRINCIPAL_JWKS_LISTEN"}
	enabled := false
	for _, n := range names {
		if _, ok := os.LookupEnv(n); ok {
			enabled = true
		}
	}
	if !enabled {
		return nil, nil, nil
	}
	addr, dir, kid := strings.TrimSpace(os.Getenv(names[0])), strings.TrimSpace(os.Getenv("SEARCH_PRINCIPAL_SIGNING_KEYS_DIR")), strings.TrimSpace(os.Getenv("SEARCH_PRINCIPAL_ACTIVE_KID"))
	serverName, jwksListen := strings.TrimSpace(os.Getenv("SEARCH_USER_PROJECTION_TLS_SERVER_NAME")), strings.TrimSpace(os.Getenv("SEARCH_PRINCIPAL_JWKS_LISTEN"))
	if addr == "" || dir == "" || kid == "" || serverName == "" || jwksListen == "" {
		return nil, nil, errors.New("complete Search User projection TLS, signing, and JWKS configuration is required")
	}
	keys, err := loadSearchSigningKeys(dir)
	if err != nil {
		return nil, nil, err
	}
	key := keys[kid]
	if key == nil {
		return nil, nil, errors.New("active Search principal kid is missing")
	}
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "search", KeyID: kid, PrivateKey: key})
	if err != nil {
		return nil, nil, err
	}
	roots, err := x509.SystemCertPool()
	if err != nil {
		return nil, nil, err
	}
	ca, err := os.ReadFile(strings.TrimSpace(os.Getenv("SEARCH_USER_PROJECTION_TLS_CA_FILE")))
	if err != nil || !roots.AppendCertsFromPEM(ca) {
		return nil, nil, errors.New("invalid User projection TLS CA")
	}
	conn, err := grpc.NewClient(grpcclient.DialTarget(addr), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: serverName})))
	if err != nil {
		return nil, nil, err
	}
	return &signedUserProjectionClient{UserServiceClient: userv1.NewUserServiceClient(conn), issuer: issuer}, conn, nil
}
