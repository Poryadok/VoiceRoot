package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/pkg/principal"
)

// These are protocol authorities for runtime wiring tests, not imported bytes
// or evidence of real File processing/settlement.
type catalogRuntimeFileAuthority struct {
	filev1.UnimplementedCatalogAssetServiceServer
	snapshot *filev1.ValidateCatalogAssetResponse
}

func (s *catalogRuntimeFileAuthority) LookupCatalogAsset(_ context.Context, r *filev1.LookupCatalogAssetRequest) (*filev1.LookupCatalogAssetResponse, error) {
	if r.FileId != s.snapshot.FileId {
		return nil, status.Error(codes.NotFound, "unknown file")
	}
	if r.ActorAccountId != s.snapshot.ActorAccountId || r.ActorProfileId != s.snapshot.ActorProfileId {
		return nil, status.Error(codes.PermissionDenied, "wrong actor")
	}
	return &filev1.LookupCatalogAssetResponse{Classification: &filev1.LookupCatalogAssetResponse_Catalog{Catalog: proto.Clone(s.snapshot).(*filev1.ValidateCatalogAssetResponse)}}, nil
}
func (s *catalogRuntimeFileAuthority) ValidateCatalogAsset(_ context.Context, r *filev1.ValidateCatalogAssetRequest) (*filev1.ValidateCatalogAssetResponse, error) {
	if r.SourceId != s.snapshot.SourceId || r.FileId != s.snapshot.FileId || r.Provider != s.snapshot.Provider || r.ProviderId != s.snapshot.ProviderId || r.ActorAccountId != s.snapshot.ActorAccountId || r.ActorProfileId != s.snapshot.ActorProfileId {
		return nil, status.Error(codes.PermissionDenied, "wrong lineage")
	}
	return proto.Clone(s.snapshot).(*filev1.ValidateCatalogAssetResponse), nil
}

type catalogRuntimeChatAuthority struct {
	chatv1.UnimplementedCatalogAssetServiceServer
	snapshot    *filev1.ValidateCatalogAssetResponse
	destination string
}

func (s *catalogRuntimeChatAuthority) ValidateGifAsset(_ context.Context, r *chatv1.ValidateGifAssetRequest) (*chatv1.ValidateGifAssetResponse, error) {
	if r.GetDestinationChat().GetId() != s.destination {
		return nil, status.Error(codes.PermissionDenied, "wrong destination")
	}
	if r.ActorAccountId != s.snapshot.ActorAccountId || r.ActorProfileId != s.snapshot.ActorProfileId || r.FileId != s.snapshot.FileId || r.Provider != s.snapshot.Provider || r.ProviderId != s.snapshot.ProviderId {
		return nil, status.Error(codes.PermissionDenied, "wrong selection")
	}
	return &chatv1.ValidateGifAssetResponse{SourceId: s.snapshot.SourceId, ActorAccountId: r.ActorAccountId, ActorProfileId: r.ActorProfileId, FileId: r.FileId, Provider: r.Provider, ProviderId: r.ProviderId}, nil
}

type catalogRuntimeFixture struct {
	runtime *catalogAssetRuntime
	file    *catalogRuntimeFileAuthority
	chat    *catalogRuntimeChatAuthority
	env     map[string]string
	parent  *managedChatPurgeRuntime
	mu      sync.Mutex
	calls   []string
}

func newCatalogRuntimeFixture(t *testing.T) *catalogRuntimeFixture {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "messaging", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	snapshot := &filev1.ValidateCatalogAssetResponse{SourceId: uuid.NewString(), ActorAccountId: uuid.NewString(), ActorProfileId: uuid.NewString(), FileId: uuid.NewString(), Provider: "fixture", ProviderId: "runtime-test", ObjectRevision: 1, Intent: "gif", ScanResult: "clean", FileType: "video", SourceSha256: bytes.Repeat([]byte{1}, 32), DeliveredSha256: bytes.Repeat([]byte{2}, 32), DeliveredSizeBytes: 42, DeliveredMimeType: "video/mp4"}
	f := &catalogRuntimeFixture{env: map[string]string{}, parent: &managedChatPurgeRuntime{Issuer: issuer, JWKS: &http.Server{}}, file: &catalogRuntimeFileAuthority{snapshot: snapshot}, chat: &catalogRuntimeChatAuthority{snapshot: snapshot, destination: uuid.NewString()}}
	for _, owner := range []string{"FILE", "CHAT"} {
		pki := roleJWKSFixture(t, owner)
		cert, err := tls.LoadX509KeyPair(pki.cert, pki.key)
		require.NoError(t, err)
		clientDir := t.TempDir()
		clientCert := filepath.Join(clientDir, "client.crt")
		clientKey := filepath.Join(clientDir, "client.key")
		require.NoError(t, os.WriteFile(clientCert, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: pki.client.Certificate[0]}), 0600))
		encoded, err := x509.MarshalPKCS8PrivateKey(pki.client.PrivateKey)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(clientKey, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded}), 0600))
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		seen := map[string]bool{}
		interceptor := func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			p, ok := peer.FromContext(ctx)
			if !ok {
				return nil, status.Error(codes.Unauthenticated, "peer absent")
			}
			tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo)
			if !ok || len(tlsInfo.State.VerifiedChains) == 0 || !bytes.Equal(tlsInfo.State.PeerCertificates[0].Raw, pki.client.Certificate[0]) {
				return nil, status.Error(codes.Unauthenticated, "untrusted peer")
			}
			md, _ := metadata.FromIncomingContext(ctx)
			if len(md.Get("authorization")) != 1 || len(md.Get("x-request-id")) != 1 || len(md.Get("x-voice-user-id")) != 0 || len(md.Get("x-voice-profile-id")) != 0 {
				return nil, status.Error(codes.Unauthenticated, "invalid metadata")
			}
			hash, err := principal.RequestHash(request.(proto.Message))
			if err != nil {
				return nil, err
			}
			_, err = principal.VerifyService(ctx, strings.TrimPrefix(md.Get("authorization")[0], "Bearer "), principal.VerifyConfig{ExpectedIssuer: "messaging", ExpectedAudience: strings.ToLower(owner), ExpectedRPC: info.FullMethod, ExpectedRequestID: md.Get("x-request-id")[0], ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, ReplayGuard: func(_ context.Context, _ string, jti string, _ time.Time) error {
				f.mu.Lock()
				defer f.mu.Unlock()
				if seen[jti] {
					return errors.New("replayed credential")
				}
				seen[jti] = true
				return nil
			}})
			if err != nil {
				return nil, status.Error(codes.Unauthenticated, "invalid request principal")
			}
			f.mu.Lock()
			f.calls = append(f.calls, info.FullMethod)
			f.mu.Unlock()
			return handler(ctx, request)
		}
		server := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pki.roots})), grpc.UnaryInterceptor(interceptor))
		if owner == "FILE" {
			filev1.RegisterCatalogAssetServiceServer(server, f.file)
		} else {
			chatv1.RegisterCatalogAssetServiceServer(server, f.chat)
		}
		go func() { _ = server.Serve(listener) }()
		t.Cleanup(server.Stop)
		prefix := "MESSAGING_CATALOG_ASSET_" + owner + "_"
		for suffix, value := range map[string]string{"GRPC_ADDR": listener.Addr().String(), "TLS_CA_FILE": pki.ca, "TLS_SERVER_NAME": "messaging", "TLS_CLIENT_CERT_FILE": clientCert, "TLS_CLIENT_KEY_FILE": clientKey} {
			f.env[prefix+suffix] = value
		}
	}
	f.runtime, err = newCatalogAssetRuntime(catalogRuntimeEnv(f.env), f.parent)
	require.NoError(t, err)
	t.Cleanup(f.runtime.Close)
	return f
}

func catalogRuntimeSigned(t *testing.T, f *catalogRuntimeFixture, audience, method string, request proto.Message) context.Context {
	t.Helper()
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	id := uuid.NewString()
	token, err := f.parent.Issuer.IssueService(principal.ServiceInput{Audience: audience, RPC: method, RequestID: id, RequestHash: hash})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)
	return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
}

func TestCatalogRuntimeDedicatedMTLSAndBoundGeneratedClients(t *testing.T) {
	f := newCatalogRuntimeFixture(t)
	c := f.runtime.Coordinator
	snapshot := f.file.snapshot
	lookup := &filev1.LookupCatalogAssetRequest{ProtocolVersion: 1, ActorAccountId: snapshot.ActorAccountId, ActorProfileId: snapshot.ActorProfileId, FileId: snapshot.FileId}
	ctx := catalogRuntimeSigned(t, f, "file", filev1.CatalogAssetService_LookupCatalogAsset_FullMethodName, lookup)
	response, err := c.Files.LookupCatalogAsset(ctx, lookup)
	require.NoError(t, err)
	require.True(t, proto.Equal(snapshot, response.GetCatalog()))
	_, err = c.Files.LookupCatalogAsset(ctx, lookup)
	require.Equal(t, codes.Unauthenticated, status.Code(err), "replay rejected by actual authority")
	changed := proto.Clone(lookup).(*filev1.LookupCatalogAssetRequest)
	changed.FileId = uuid.NewString()
	ctx = catalogRuntimeSigned(t, f, "file", filev1.CatalogAssetService_LookupCatalogAsset_FullMethodName, lookup)
	_, err = c.Files.LookupCatalogAsset(ctx, changed)
	require.Equal(t, codes.Unauthenticated, status.Code(err), "whole request hash is verified")
	chat := &chatv1.ValidateGifAssetRequest{ProtocolVersion: 1, ActorAccountId: snapshot.ActorAccountId, ActorProfileId: snapshot.ActorProfileId, Provider: snapshot.Provider, ProviderId: snapshot.ProviderId, FileId: snapshot.FileId, DestinationChat: &chatv1.ChatRef{Id: f.chat.destination}}
	// Type is supplied explicitly as in the send pipeline.
	kind := chatv1.ChatType_CHAT_TYPE_GROUP
	chat.DestinationChat.Type = &kind
	_, err = c.Chat.ValidateGifAsset(catalogRuntimeSigned(t, f, "chat", chatv1.CatalogAssetService_ValidateGifAsset_FullMethodName, chat), chat)
	require.NoError(t, err)
	validation := &filev1.ValidateCatalogAssetRequest{ProtocolVersion: 1, SourceId: snapshot.SourceId, Provider: snapshot.Provider, ProviderId: snapshot.ProviderId, FileId: snapshot.FileId, ActorAccountId: snapshot.ActorAccountId, ActorProfileId: snapshot.ActorProfileId}
	final, err := c.Files.ValidateCatalogAsset(catalogRuntimeSigned(t, f, "file", filev1.CatalogAssetService_ValidateCatalogAsset_FullMethodName, validation), validation)
	require.NoError(t, err)
	require.True(t, proto.Equal(snapshot, final))
	for _, scenario := range []string{"wrong-name", "wrong-client-ca", "wrong-server-ca"} {
		t.Run(scenario, func(t *testing.T) {
			values := map[string]string{}
			for key, value := range f.env {
				values[key] = value
			}
			switch scenario {
			case "wrong-name":
				values["MESSAGING_CATALOG_ASSET_FILE_TLS_SERVER_NAME"] = "untrusted"
			case "wrong-client-ca":
				values["MESSAGING_CATALOG_ASSET_FILE_TLS_CLIENT_CERT_FILE"] = values["MESSAGING_CATALOG_ASSET_CHAT_TLS_CLIENT_CERT_FILE"]
				values["MESSAGING_CATALOG_ASSET_FILE_TLS_CLIENT_KEY_FILE"] = values["MESSAGING_CATALOG_ASSET_CHAT_TLS_CLIENT_KEY_FILE"]
			case "wrong-server-ca":
				values["MESSAGING_CATALOG_ASSET_FILE_TLS_CA_FILE"] = values["MESSAGING_CATALOG_ASSET_CHAT_TLS_CA_FILE"]
			}
			runtime, err := newCatalogAssetRuntime(catalogRuntimeEnv(values), f.parent)
			require.NoError(t, err)
			defer runtime.Close()
			_, err = runtime.Coordinator.Files.LookupCatalogAsset(catalogRuntimeSigned(t, f, "file", filev1.CatalogAssetService_LookupCatalogAsset_FullMethodName, lookup), lookup)
			require.Error(t, err)
		})
	}
	f.runtime.Close()
	_, err = c.Files.LookupCatalogAsset(catalogRuntimeSigned(t, f, "file", filev1.CatalogAssetService_LookupCatalogAsset_FullMethodName, lookup), lookup)
	require.Error(t, err)
}
