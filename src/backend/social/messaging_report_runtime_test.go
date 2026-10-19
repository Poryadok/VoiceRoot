package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
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
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
	socialv1 "voice.app/voice/social/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/socialprincipal"
	"voice/backend/social/internal/grpcsvc"
	"voice/backend/social/internal/store"
)

type reportProfileOwners struct {
	profiles map[uuid.UUID]uuid.UUID
	calls    atomic.Int32
}

func (r *reportProfileOwners) AccountIDByProfileID(_ context.Context, id uuid.UUID) (uuid.UUID, error) {
	r.calls.Add(1)
	return r.profiles[id], nil
}
func (r *reportProfileOwners) AccountIDsByProfileIDs(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]uuid.UUID, error) {
	r.calls.Add(1)
	out := map[uuid.UUID]uuid.UUID{}
	for _, id := range ids {
		if account, ok := r.profiles[id]; ok {
			out[id] = account
		}
	}
	return out, nil
}

func TestMessagingReportProductionFactoryNativePolicyAndTLS(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	ctx := t.Context()
	pool := integrationtest.StartPostgres(t, ctx, "social-report-policy", "")
	paths, err := filepath.Glob("../migrations/social_db/*.up.sql")
	require.NoError(t, err)
	require.NotEmpty(t, paths)
	for _, path := range paths {
		body, e := os.ReadFile(path)
		require.NoError(t, e)
		_, e = pool.Exec(ctx, string(body))
		require.NoError(t, e, path)
	}
	viewer, sender, neutral := uuid.New(), uuid.New(), uuid.New()
	viewerAccount, senderAccount := uuid.New(), uuid.New()
	owners := &reportProfileOwners{profiles: map[uuid.UUID]uuid.UUID{viewer: viewerAccount, sender: senderAccount, neutral: uuid.New()}}
	blocks := &store.BlockStore{Pool: pool}
	require.NoError(t, blocks.BlockAccount(ctx, viewerAccount, senderAccount))
	service := &grpcsvc.SocialGRPC{Blocks: blocks, ProfileAccounts: owners}
	cert, certPEM, keyPEM := reportPolicyTLS(t)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(certPEM))
	dir := t.TempDir()
	write := func(name string, body []byte) string {
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, body, 0600))
		return path
	}
	certPath, keyPath := write("identity.pem", certPEM), write("identity.key", keyPEM)
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
	jwks.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots}
	jwks.StartTLS()
	defer jwks.Close()
	replay := miniredis.RunT(t)
	cfg := socialprincipal.Config{Target: "social", Capability: "social-messaging-report", JWKSURLs: map[string]string{"messaging": jwks.URL}, RefreshAfter: 30 * time.Second, HardExpiry: 2 * time.Minute, UnknownKIDCooldown: time.Second, ReplayAddr: replay.Addr(), JWKSCAFile: certPath, TLSCertFile: certPath, TLSKeyFile: keyPath, ListenAddr: "127.0.0.1:0", ClientCAFile: certPath, ClientCAs: roots, JWKSClientCertFile: certPath, JWKSClientKeyFile: keyPath}
	for suffix, value := range map[string]string{"GRPC_LISTEN": cfg.ListenAddr, "REPLAY_REDIS_ADDR": cfg.ReplayAddr, "TLS_CERT_FILE": certPath, "TLS_KEY_FILE": keyPath, "CLIENT_CA_FILE": certPath, "JWKS_CLIENT_CERT_FILE": certPath, "JWKS_CLIENT_KEY_FILE": keyPath} {
		t.Setenv("SOCIAL_REPORT_MESSAGING_"+suffix, value)
	}
	t.Setenv("S2S_JWKS_CA_FILE", certPath)
	urls, e := json.Marshal(map[string]string{"messaging": jwks.URL, "role": "https://untrusted.invalid/jwks"})
	require.NoError(t, e)
	t.Setenv("S2S_JWKS_URLS_JSON", string(urls))
	cfg, enabled, err := loadMessagingReportConfig()
	require.NoError(t, err)
	require.True(t, enabled)
	require.Equal(t, "social", cfg.Target)
	require.Equal(t, "social-messaging-report", cfg.Capability)
	require.Equal(t, map[string]string{"messaging": jwks.URL}, cfg.JWKSURLs)
	server, closeServer, err := newMessagingReportServer(ctx, cfg, service)
	require.NoError(t, err)
	defer closeServer()
	listener := bufconn.Listen(1 << 20)
	go func() { _ = server.Serve(listener) }()
	dial := func(certs []tls.Certificate) *grpc.ClientConn {
		conn, e := grpc.NewClient("passthrough:///social-report.test", grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "social-report.test", Certificates: certs})))
		require.NoError(t, e)
		t.Cleanup(func() { _ = conn.Close() })
		return conn
	}
	conn := dial([]tls.Certificate{cert})
	client := socialv1.NewSocialServiceClient(conn)
	req := &socialv1.IsProfilePairsBlockedRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), ViewerProfileId: viewer.String(), OtherProfileIds: []string{sender.String(), neutral.String()}}
	const method = socialv1.SocialService_IsProfilePairsBlocked_FullMethodName
	sign := func(issuer, audience, rpc string, r *socialv1.IsProfilePairsBlockedRequest) string {
		signer, e := principal.NewIssuer(principal.IssuerConfig{Issuer: issuer, KeyID: "current", PrivateKey: key})
		require.NoError(t, e)
		hash, e := principal.RequestHash(r)
		require.NoError(t, e)
		token, e := signer.IssueService(principal.ServiceInput{Audience: audience, RPC: rpc, RequestID: r.OperationId, RequestHash: hash})
		require.NoError(t, e)
		return token
	}
	call := func(c *grpc.ClientConn, r *socialv1.IsProfilePairsBlockedRequest, token string, extra ...string) (*socialv1.IsProfilePairsBlockedResponse, error) {
		ctx, stop := context.WithTimeout(t.Context(), time.Second)
		defer stop()
		pairs := append([]string{"authorization", "Bearer " + token, "x-request-id", r.OperationId}, extra...)
		return socialv1.NewSocialServiceClient(c).IsProfilePairsBlocked(metadata.NewOutgoingContext(ctx, metadata.Pairs(pairs...)), r)
	}
	token := sign("messaging", "social", method, req)
	got, err := call(conn, req, token)
	require.NoError(t, err)
	require.Equal(t, req.OperationId, got.OperationId)
	require.Equal(t, req.ViewerProfileId, got.ViewerProfileId)
	require.Equal(t, uint32(1), got.ProtocolVersion)
	require.Len(t, got.Results, 2)
	require.True(t, got.Results[0].Blocked)
	require.False(t, got.Results[1].Blocked)
	before := owners.calls.Load()
	_, err = call(conn, req, token)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	for _, test := range []struct{ issuer, audience, rpc string }{{"role", "social", method}, {"messaging", "user", method}, {"messaging", "social", socialv1.SocialService_IsBlocked_FullMethodName}} {
		_, err = call(conn, req, sign(test.issuer, test.audience, test.rpc, req))
		require.Equal(t, codes.Unauthenticated, status.Code(err))
	}
	changed := proto.Clone(req).(*socialv1.IsProfilePairsBlockedRequest)
	changed.ViewerProfileId = sender.String()
	_, err = call(conn, changed, sign("messaging", "social", method, req))
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = call(conn, req, sign("messaging", "social", method, req), "x-voice-profile-id", viewer.String())
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = call(conn, req, sign("messaging", "social", method, req), "authorization", "Bearer duplicate")
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = call(dial(nil), req, sign("messaging", "social", method, req))
	require.Error(t, err)
	for _, mutate := range []func(*socialv1.IsProfilePairsBlockedRequest){func(r *socialv1.IsProfilePairsBlockedRequest) { r.ProtocolVersion = 2 }, func(r *socialv1.IsProfilePairsBlockedRequest) { r.OperationId = "bad" }, func(r *socialv1.IsProfilePairsBlockedRequest) {
		r.OtherProfileIds = []string{sender.String(), sender.String()}
	}, func(r *socialv1.IsProfilePairsBlockedRequest) { r.ProtoReflect().SetUnknown([]byte{0x78, 1}) }, func(r *socialv1.IsProfilePairsBlockedRequest) { r.OtherProfileIds = make([]string, 101) }} {
		changed := proto.Clone(req).(*socialv1.IsProfilePairsBlockedRequest)
		mutate(changed)
		_, err = call(conn, changed, sign("messaging", "social", method, changed))
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	}
	require.Equal(t, before, owners.calls.Load(), "invalid transport/body must not resolve private owners")
	ctxCall, stop := context.WithTimeout(t.Context(), time.Second)
	defer stop()
	ctxCall = metadata.NewOutgoingContext(ctxCall, metadata.Pairs("authorization", "Bearer "+sign("messaging", "social", method, req), "x-request-id", req.OperationId))
	_, err = client.BlockAccount(ctxCall, &socialv1.BlockAccountRequest{BlockedAccountId: senderAccount.String()})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	reverse := proto.Clone(req).(*socialv1.IsProfilePairsBlockedRequest)
	reverse.OperationId = uuid.NewString()
	reverse.ViewerProfileId = sender.String()
	reverse.OtherProfileIds = []string{viewer.String()}
	got, err = call(conn, reverse, sign("messaging", "social", method, reverse))
	require.NoError(t, err)
	require.False(t, got.Results[0].Blocked)
	maximum := proto.Clone(req).(*socialv1.IsProfilePairsBlockedRequest)
	maximum.OperationId = uuid.NewString()
	maximum.OtherProfileIds = nil
	for range 100 {
		id := uuid.New()
		owners.profiles[id] = uuid.New()
		maximum.OtherProfileIds = append(maximum.OtherProfileIds, id.String())
	}
	got, err = call(conn, maximum, sign("messaging", "social", method, maximum))
	require.NoError(t, err)
	require.Len(t, got.Results, 100)
	missing := proto.Clone(req).(*socialv1.IsProfilePairsBlockedRequest)
	missing.OperationId = uuid.NewString()
	missing.OtherProfileIds = []string{uuid.NewString()}
	_, err = call(conn, missing, sign("messaging", "social", method, missing))
	require.Equal(t, codes.Unavailable, status.Code(err))
	ordinary := grpc.NewServer(grpc.ChainUnaryInterceptor(grpcsvc.OrdinaryMessagingReportUnary()))
	socialv1.RegisterSocialServiceServer(ordinary, service)
	plain := bufconn.Listen(1 << 20)
	go func() { _ = ordinary.Serve(plain) }()
	defer ordinary.Stop()
	legacy, err := grpc.NewClient("passthrough:///ordinary", grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return plain.DialContext(ctx) }), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer legacy.Close()
	_, err = call(legacy, req, sign("messaging", "social", method, req))
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestMessagingReportConfigFailsClosed(t *testing.T) {
	t.Setenv("SOCIAL_REPORT_MESSAGING_GRPC_LISTEN", "127.0.0.1:0")
	_, enabled, err := loadMessagingReportConfig()
	require.True(t, enabled)
	require.Error(t, err)
	_, _, err = newMessagingReportServer(t.Context(), socialprincipal.Config{}, nil)
	require.ErrorContains(t, err, "native storage")
}

func reportPolicyTLS(t *testing.T) (tls.Certificate, []byte, []byte) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	template := &x509.Certificate{SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: "social-report.test"}, DNSNames: []string{"social-report.test"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), BasicConstraintsValid: true, IsCA: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	private, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	certPEM, keyPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)
	return cert, certPEM, keyPEM
}
