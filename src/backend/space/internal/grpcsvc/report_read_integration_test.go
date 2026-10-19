package grpcsvc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	chatv1 "voice.app/voice/chat/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/reportprincipal"
	"voice/backend/space/internal/store"
)

type countedNativeReportStore struct {
	actual *store.SpaceStore
	calls  atomic.Int64
}

func (s *countedNativeReportStore) ReadProtectedSpaceReportReadContext(c context.Context, space, viewer, chat uuid.UUID) (*store.SpaceReportReadContext, error) {
	s.calls.Add(1)
	return s.actual.ReadProtectedSpaceReportReadContext(c, space, viewer, chat)
}

func TestReportReadActualTLSNative44UNKNOWNAndDenials(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "34")
	admin := &store.SpaceStore{Pool: db.Pool}
	viewer, chat := uuid.New(), uuid.New()
	space, err := admin.CreateSpace(ctx, viewer, "real protected report read", "", "private")
	require.NoError(t, err)
	node, err := admin.UpsertTreeNode(ctx, store.UpsertTreeNodeInput{SpaceID: space.ID, Kind: store.TreeKindTextChat, ChatID: &chat})
	require.NoError(t, err)
	db.Run(true, "up")
	var schema int
	var dirty bool
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT version,dirty FROM public.schema_migrations`).Scan(&schema, &dirty))
	require.Equal(t, 44, schema)
	require.False(t, dirty)
	role := "reportreader_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	identifier := pgx.Identifier{role}.Sanitize()
	_, err = db.Pool.Exec(ctx, `CREATE ROLE `+identifier+` LOGIN PASSWORD 'private-report-fixture' NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION`)
	require.NoError(t, err)
	_, err = db.Pool.Exec(ctx, `GRANT USAGE ON SCHEMA public TO `+identifier+`; GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA public TO `+identifier)
	require.NoError(t, err)
	cfg := db.Pool.Config().Copy()
	cfg.ConnConfig.User = role
	cfg.ConnConfig.Password = "private-report-fixture"
	login, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		login.Close()
		_, err := db.Pool.Exec(ctx, `DROP OWNED BY `+identifier+`; DROP ROLE `+identifier)
		require.NoError(t, err)
	})
	var session string
	require.NoError(t, login.QueryRow(ctx, `SELECT session_user`).Scan(&session))
	require.Equal(t, role, session)
	native := &countedNativeReportStore{actual: &store.SpaceStore{Pool: login}}
	certFile, keyFile, caFile, clientCert, roots := reportReadTLSMaterial(t)
	signing, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	keys := []map[string]string{}
	for i, key := range []*rsa.PrivateKey{signing, next} {
		kid := "current"
		if i == 1 {
			kid = "next"
		}
		keys = append(keys, map[string]string{"kty": "RSA", "kid": kid, "alg": "RS256", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": "AQAB"})
	}
	jwks := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"keys": keys}))
	}))
	identity, err := tls.LoadX509KeyPair(certFile, keyFile)
	require.NoError(t, err)
	jwks.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{identity}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots}
	jwks.StartTLS()
	t.Cleanup(jwks.Close)
	redis := miniredis.RunT(t)
	dir := t.TempDir()
	clientFile, clientKeyFile := filepath.Join(dir, "client.pem"), filepath.Join(dir, "client-key.pem")
	clientKey, err := x509.MarshalPKCS8PrivateKey(clientCert.PrivateKey)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(clientFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientCert.Certificate[0]}), 0600))
	require.NoError(t, os.WriteFile(clientKeyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: clientKey}), 0600))
	runtime, err := reportprincipal.New(ctx, reportprincipal.Config{ListenAddr: "127.0.0.1:0", CertFile: certFile, KeyFile: keyFile, ClientCAFile: caFile, ReplayAddr: redis.Addr(), JWKSCAFile: caFile, JWKSClientCertFile: clientFile, JWKSClientKeyFile: clientKeyFile, JWKSURLs: map[string]string{"messaging": jwks.URL}, RefreshAfter: 30 * time.Second, HardExpiry: 2 * time.Minute, UnknownKIDCooldown: 5 * time.Second})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, runtime.Close()) })
	server := grpc.NewServer(runtime.ServerOptions()...)
	RegisterReportReadServer(server, &ReportReadGRPC{Store: native})
	t.Cleanup(server.Stop)
	listener := bufconn.Listen(1 << 20)
	t.Cleanup(func() { _ = listener.Close() })
	go func() { _ = server.Serve(listener) }()
	conn, err := grpc.NewClient("passthrough:///report", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "localhost", Certificates: []tls.Certificate{clientCert}})))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	client := spacev1.NewSpaceServiceClient(conn)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "messaging", KeyID: "current", PrivateKey: signing})
	require.NoError(t, err)
	req := &spacev1.ResolveSpaceReportReadContextRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), SpaceId: space.ID.String(), ViewerProfileId: viewer.String(), Chat: &chatv1.ChatRef{Id: chat.String(), Type: chatv1.ChatType_CHAT_TYPE_GROUP.Enum()}}
	sign := func(r *spacev1.ResolveSpaceReportReadContextRequest) context.Context {
		hash, err := principal.RequestHash(r)
		require.NoError(t, err)
		token, err := issuer.IssueService(principal.ServiceInput{Audience: "space", RPC: spacev1.SpaceService_ResolveSpaceReportReadContext_FullMethodName, RequestID: r.OperationId, RequestHash: hash})
		require.NoError(t, err)
		return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", r.OperationId))
	}
	signed := sign(req)
	response, err := client.ResolveSpaceReportReadContext(signed, req)
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.Nil(t, response, "valid native facts alone cannot skip actual COMPLETE")
	require.NoError(t, admin.InitializeNativeMembership(ctx, space.ID), "real reviewed admin-own-TX native initialization; no reflection or fake source")
	signed = sign(req)
	response, err = client.ResolveSpaceReportReadContext(signed, req)
	require.NoError(t, err)
	require.True(t, response.Eligible)
	require.False(t, response.HistoryKnown)
	require.Nil(t, response.HistoryVisibleSince)
	require.Equal(t, node.ID.String(), response.TreeAttachmentIncarnation)
	require.NotNil(t, response.MembershipBinding)
	require.Positive(t, response.LedgerRevision)
	require.Zero(t, response.LifecycleGeneration, "genuine initial native Space has no lifecycle aggregate/history")
	// Run the real Messaging owner client against this same native44 producer,
	// rather than validating a synthetic response that invents generation one.
	network, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = network.Close() })
	go func() { _ = server.Serve(network) }()
	issuerFile := filepath.Join(dir, "messaging-signing.pem")
	issuerDER, err := x509.MarshalPKCS8PrivateKey(signing)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(issuerFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: issuerDER}), 0600))
	consumer := exec.Command("go", "test", "-mod=readonly", "-race", "-count=1", "-v", "./internal/reportread", "-run", "^TestReportOwnerActualInitialNative44Space$")
	consumer.Dir = filepath.Join(repoRoot(t), "src/backend/messaging")
	consumer.Env = append(os.Environ(), "REPORT_READ_NATIVE_ENDPOINT="+network.Addr().String(), "REPORT_READ_NATIVE_CA="+caFile, "REPORT_READ_NATIVE_CERT="+clientFile, "REPORT_READ_NATIVE_KEY="+clientKeyFile, "REPORT_READ_NATIVE_ISSUER="+issuerFile, "REPORT_READ_NATIVE_SPACE="+space.ID.String(), "REPORT_READ_NATIVE_VIEWER="+viewer.String(), "REPORT_READ_NATIVE_CHAT="+chat.String())
	consumerOutput, consumerErr := consumer.CombinedOutput()
	t.Logf("actual Messaging owner client: %s", consumerOutput)
	require.NoError(t, consumerErr, "native producer must compose with real Messaging validator")

	before := native.calls.Load()
	_, err = client.ResolveSpaceReportReadContext(signed, req)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Equal(t, before, native.calls.Load(), "replay fails before SQL")
	req.ViewerProfileId = uuid.NewString()
	response, err = client.ResolveSpaceReportReadContext(sign(req), req)
	require.NoError(t, err)
	require.False(t, response.Eligible)
	require.Zero(t, response.LedgerRevision)
	require.Nil(t, response.MembershipBinding)
	require.Empty(t, response.EffectiveEpisodeId)
	req.ViewerProfileId = viewer.String()
	req.Chat.Id = uuid.NewString()
	response, err = client.ResolveSpaceReportReadContext(sign(req), req)
	require.NoError(t, err)
	require.False(t, response.Eligible)
	require.Empty(t, response.TreeAttachmentIncarnation)
	req.Chat.Id = chat.String()
	old := sign(req)
	req.SpaceId = uuid.NewString()
	before = native.calls.Load()
	_, err = client.ResolveSpaceReportReadContext(old, req)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Equal(t, before, native.calls.Load())
	_, err = client.GetSpace(sign(req), &spacev1.GetSpaceRequest{SpaceId: space.ID.String()})
	require.Equal(t, codes.Unimplemented, status.Code(err), "private registration exposes exactly one method")
}

func reportReadTLSMaterial(t *testing.T) (string, string, string, tls.Certificate, *x509.CertPool) {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "memberauthority-test-ca"}, IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	require.NoError(t, err)
	dir := t.TempDir()
	caFile := filepath.Join(dir, "ca.pem")
	require.NoError(t, os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), 0600))
	issue := func(name string, serial int64, usage x509.ExtKeyUsage) (string, string, tls.Certificate) {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)
		leaf := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: name}, DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
			KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage}, NotBefore: ca.NotBefore, NotAfter: ca.NotAfter}
		der, err := x509.CreateCertificate(rand.Reader, leaf, ca, &key.PublicKey, caKey)
		require.NoError(t, err)
		pk, err := x509.MarshalPKCS8PrivateKey(key)
		require.NoError(t, err)
		certFile, keyFile := filepath.Join(dir, name+".pem"), filepath.Join(dir, name+"-key.pem")
		require.NoError(t, os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600))
		require.NoError(t, os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pk}), 0600))
		pair, err := tls.LoadX509KeyPair(certFile, keyFile)
		require.NoError(t, err)
		return certFile, keyFile, pair
	}
	certFile, keyFile, _ := issue("server", 2, x509.ExtKeyUsageServerAuth)
	_, _, client := issue("client", 3, x509.ExtKeyUsageClientAuth)
	parsedCA, err := x509.ParseCertificate(caDER)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	roots.AddCert(parsedCA)
	return certFile, keyFile, caFile, client, roots
}
