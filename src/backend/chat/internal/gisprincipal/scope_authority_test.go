package gisprincipal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/chat/internal/grpcsvc"
	"voice/backend/chat/internal/store"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
)

type scopeTransportStore struct {
	calls  int
	native *store.DMStore
}

func (s *scopeTransportStore) ReadSpaceReportAttachment(ctx context.Context, chat, space uuid.UUID, saved uint64) (*chatv1.ReadSpaceReportAttachmentResponse, error) {
	s.calls++
	if s.native != nil {
		return s.native.ReadSpaceReportAttachment(ctx, chat, space, saved)
	}
	return &chatv1.ReadSpaceReportAttachmentResponse{ProtocolVersion: 1, ChatId: chat.String(), ExpectedSpaceId: space.String(), SavedAttachmentGeneration: saved, Disposition: chatv1.SpaceReportAttachmentDisposition_SPACE_REPORT_ATTACHMENT_DISPOSITION_UNCHANGED}, nil
}

func (s *scopeTransportStore) GetSpaceChatScopeAuthority(_ context.Context, chat, space uuid.UUID) (*chatv1.GetSpaceChatScopeAuthorityResponse, error) {
	s.calls++
	if s.native != nil {
		return s.native.GetSpaceChatScopeAuthority(context.Background(), chat, space)
	}
	return &chatv1.GetSpaceChatScopeAuthorityResponse{ProtocolVersion: 1, ChatId: chat.String(), ExpectedSpaceId: space.String(), ActualSpaceId: space.String(), ChatScopeIncarnation: uuid.NewString(), ChatAuthorityRevision: 1, Exists: true, Live: true, ChatType: chatv1.ChatType_CHAT_TYPE_CHANNEL, ManagedKind: chatv1.ChatScopeManagedKind_CHAT_SCOPE_MANAGED_KIND_ORDINARY, LifecycleState: chatv1.ChatScopeLifecycleState_CHAT_SCOPE_LIFECYCLE_STATE_LIVE}, nil
}

func TestScopeAuthorityRealTLSExactPrincipalReplayAndAlternateListeners(t *testing.T) {
	var native *store.DMStore
	chatID, spaceID := uuid.New(), uuid.New()
	if !testing.Short() {
		ctx := context.Background()
		pool := integrationtest.StartPostgres(t, ctx, "chat-scope-authority", "")
		paths, err := filepath.Glob("../../../migrations/chat_db/*.up.sql")
		require.NoError(t, err)
		require.NotEmpty(t, paths)
		for _, path := range paths {
			body, e := os.ReadFile(path)
			require.NoError(t, e)
			_, e = pool.Exec(ctx, string(body))
			require.NoError(t, e, path)
		}
		native = &store.DMStore{Pool: pool}
		row, err := native.CreateSpaceChannelChat(ctx, uuid.New(), spaceID, "Native authority", nil)
		require.NoError(t, err)
		chatID = row.ID
	}
	ca, clients, foreign := newManifestTestCA(t, 101), newManifestTestCA(t, 102), newManifestTestCA(t, 103)
	serverCert, certPath, keyPath := ca.leaf(t, 104, x509.ExtKeyUsageServerAuth)
	clientCert, _, _ := clients.leaf(t, 105, x509.ExtKeyUsageClientAuth)
	foreignCert, _, _ := foreign.leaf(t, 106, x509.ExtKeyUsageClientAuth)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(ca.pem))
	caPath, clientCAPath := filepath.Join(t.TempDir(), "ca.pem"), filepath.Join(t.TempDir(), "client-ca.pem")
	require.NoError(t, os.WriteFile(caPath, ca.pem, 0600))
	require.NoError(t, os.WriteFile(clientCAPath, clients.pem, 0600))
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	keys := []map[string]string{}
	for kid, key := range map[string]*rsa.PrivateKey{"current": key, "next": next} {
		keys = append(keys, map[string]string{"kty": "RSA", "kid": kid, "use": "sig", "alg": "RS256", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": "AQAB"})
	}
	jwks := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": keys})
	}))
	jwks.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{serverCert}}
	jwks.StartTLS()
	defer jwks.Close()
	replay := miniredis.RunT(t)
	sign := func(issuer, audience, method, id string, req proto.Message) string {
		signer, e := principal.NewIssuer(principal.IssuerConfig{Issuer: issuer, KeyID: "current", PrivateKey: key})
		require.NoError(t, e)
		hash, e := principal.RequestHash(req)
		require.NoError(t, e)
		token, e := signer.IssueService(principal.ServiceInput{Audience: audience, RPC: method, RequestID: id, RequestHash: hash})
		require.NoError(t, e)
		return token
	}
	for _, issuer := range []string{"role", "space", "messaging"} {
		t.Run(issuer, func(t *testing.T) {
			methods := []string{ScopeAuthorityMethod}
			if issuer == "messaging" {
				methods = append(methods, ReportAttachmentMethod)
			}
			runtime, err := New(context.Background(), Config{Issuer: issuer, Audience: Audience, AllowedMethods: methods, TLSCertFile: certPath, TLSKeyFile: keyPath, ClientCAFile: clientCAPath, JWKSURL: jwks.URL, JWKSCAFile: caPath, ReplayAddr: replay.Addr()})
			require.NoError(t, err)
			defer runtime.Close()
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			source := &scopeTransportStore{native: native}
			server := grpc.NewServer(runtime.ServerOptions()...)
			chatv1.RegisterChatScopeAuthorityServiceServer(server, &grpcsvc.SpaceChatScopeAuthorityGRPC{Store: source})
			go func() { _ = server.Serve(listener) }()
			defer server.Stop()
			connect := func(certs []tls.Certificate) *grpc.ClientConn {
				conn, e := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "localhost", Certificates: certs})))
				require.NoError(t, e)
				t.Cleanup(func() { _ = conn.Close() })
				return conn
			}
			conn := connect([]tls.Certificate{clientCert})
			req := &chatv1.GetSpaceChatScopeAuthorityRequest{ProtocolVersion: 1, ChatId: chatID.String(), ExpectedSpaceId: spaceID.String(), OperationId: uuid.NewString()}
			call := func(conn *grpc.ClientConn, token, id string) error {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				if token != "" {
					ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
				}
				_, e := chatv1.NewChatScopeAuthorityServiceClient(conn).GetSpaceChatScopeAuthority(ctx, req)
				return e
			}
			token := sign(issuer, "chat", ScopeAuthorityMethod, req.OperationId, req)
			require.NoError(t, call(conn, token, req.OperationId))
			require.Equal(t, 1, source.calls)
			require.Equal(t, codes.Unauthenticated, status.Code(call(conn, token, req.OperationId)))
			require.Equal(t, codes.Unauthenticated, status.Code(call(conn, "", req.OperationId)))
			for _, wrong := range []string{"role", "space", "messaging", "search", "gameintegration", "gateway"} {
				if wrong != issuer {
					require.Equal(t, codes.Unauthenticated, status.Code(call(conn, sign(wrong, "chat", ScopeAuthorityMethod, req.OperationId, req), req.OperationId)))
				}
			}
			require.Equal(t, codes.Unauthenticated, status.Code(call(conn, sign(issuer, "role", ScopeAuthorityMethod, req.OperationId, req), req.OperationId)))
			require.Equal(t, codes.Unauthenticated, status.Code(call(conn, sign(issuer, "chat", PrepareManifestMethod, req.OperationId, req), req.OperationId)))
			require.Equal(t, codes.Unauthenticated, status.Code(call(conn, sign(issuer, "chat", ScopeAuthorityMethod, req.OperationId, req), uuid.NewString())))
			token = sign(issuer, "chat", ScopeAuthorityMethod, req.OperationId, req)
			original := req.ExpectedSpaceId
			req.ExpectedSpaceId = uuid.NewString()
			require.Equal(t, codes.Unauthenticated, status.Code(call(conn, token, req.OperationId)))
			req.ExpectedSpaceId = original
			parts := strings.Split(sign(issuer, "chat", ScopeAuthorityMethod, req.OperationId, req), ".")
			first := "A"
			if strings.HasPrefix(parts[2], first) {
				first = "B"
			}
			parts[2] = first + parts[2][1:]
			require.Equal(t, codes.Unauthenticated, status.Code(call(conn, strings.Join(parts, "."), req.OperationId)))
			for _, certs := range [][]tls.Certificate{nil, {foreignCert}} {
				require.Error(t, call(connect(certs), sign(issuer, "chat", ScopeAuthorityMethod, req.OperationId, req), req.OperationId))
			}
			require.Equal(t, 1, source.calls, "all denied requests must precede the native source")
			require.NoError(t, call(conn, sign(issuer, "chat", ScopeAuthorityMethod, req.OperationId, req), req.OperationId))
			require.Equal(t, 2, source.calls)
			observation := &chatv1.ReadSpaceReportAttachmentRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), ChatId: chatID.String(), ExpectedSpaceId: spaceID.String(), SavedAttachmentGeneration: 1}
			observeToken := sign(issuer, "chat", ReportAttachmentMethod, observation.OperationId, observation)
			observe := func(token string) (*chatv1.ReadSpaceReportAttachmentResponse, error) {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", observation.OperationId))
				return chatv1.NewChatScopeAuthorityServiceClient(conn).ReadSpaceReportAttachment(ctx, observation)
			}
			result, e := observe(observeToken)
			if issuer != "messaging" {
				require.Equal(t, codes.PermissionDenied, status.Code(e))
				require.Equal(t, 2, source.calls)
			} else {
				require.NoError(t, e)
				require.Equal(t, chatv1.SpaceReportAttachmentDisposition_SPACE_REPORT_ATTACHMENT_DISPOSITION_UNCHANGED, result.Disposition)
				_, e = observe(observeToken)
				require.Equal(t, codes.Unauthenticated, status.Code(e))
				observeToken = sign(issuer, "chat", ReportAttachmentMethod, observation.OperationId, observation)
				observation.SavedAttachmentGeneration = 2
				_, e = observe(observeToken)
				require.Equal(t, codes.Unauthenticated, status.Code(e))
				require.Equal(t, 3, source.calls)
			}
		})
	}
	// Ordinary serving surface deliberately does not register the new service.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer()
	chatv1.RegisterChatServiceServer(server, &grpcsvc.ChatGRPC{})
	go func() { _ = server.Serve(listener) }()
	defer server.Stop()
	conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()
	_, err = chatv1.NewChatScopeAuthorityServiceClient(conn).GetSpaceChatScopeAuthority(context.Background(), &chatv1.GetSpaceChatScopeAuthorityRequest{})
	require.Equal(t, codes.Unimplemented, status.Code(err))
}

func TestScopeAuthorityConfigAndAllowlistRemainNarrow(t *testing.T) {
	for _, issuer := range []string{"role", "space", "messaging"} {
		t.Run(issuer, func(t *testing.T) {
			prefix := "CHAT_SCOPE_" + strings.ToUpper(issuer) + "_"
			t.Setenv(prefix+"GRPC_LISTEN", ":12345")
			_, _, enabled, err := ScopeAuthorityConfigFromEnv(issuer)
			require.True(t, enabled)
			require.Error(t, err)
			for _, key := range []string{"TLS_CERT_FILE", "TLS_KEY_FILE", "CLIENT_CA_FILE", "JWKS_URL", "JWKS_CA_FILE", "REPLAY_REDIS_ADDR"} {
				t.Setenv(prefix+key, "fixture")
			}
			cfg, _, enabled, err := ScopeAuthorityConfigFromEnv(issuer)
			require.NoError(t, err)
			require.True(t, enabled)
			expected := []string{ScopeAuthorityMethod}
			if issuer == "messaging" {
				expected = append(expected, ReportAttachmentMethod)
			}
			require.Equal(t, expected, cfg.AllowedMethods)
			methods, err := configuredMethods(issuer, cfg.AllowedMethods)
			require.NoError(t, err)
			require.Len(t, methods, len(expected))
			for _, other := range []string{ProvisionMethod, GetManifestPageMethod, "/voice.authority.v1.AuthoritySourceService/ReadSnapshot"} {
				_, err = (&Runtime{methods: methods}).allowlist(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: other}, func(context.Context, any) (any, error) {
					t.Fatal("scope listener admitted another method")
					return nil, nil
				})
				require.Equal(t, codes.PermissionDenied, status.Code(err))
			}
		})
	}
	_, err := configuredMethods("search", []string{ScopeAuthorityMethod})
	require.Error(t, err)
	_, err = configuredMethods(Issuer, []string{ScopeAuthorityMethod})
	require.Error(t, err)
	for _, issuer := range []string{"role", "space", "search", Issuer} {
		_, err = configuredMethods(issuer, []string{ReportAttachmentMethod})
		require.Error(t, err, "attachment observation is Messaging-only")
	}
}
