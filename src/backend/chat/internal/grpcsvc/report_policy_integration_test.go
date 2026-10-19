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
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/chat/internal/reportprincipal"
	"voice/backend/chat/internal/store"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
)

type nativeReportPolicyCounter struct {
	actual *store.DMStore
	calls  atomic.Int64
}

func (s *nativeReportPolicyCounter) ManagedChatMessageEntitled(ctx context.Context, chat, viewer uuid.UUID, at time.Time) (bool, error) {
	s.calls.Add(1)
	return s.actual.ManagedChatMessageEntitled(ctx, chat, viewer, at)
}

func TestReportPolicyActualTLSNative26ManagedIntervalsAndDenials(t *testing.T) {
	ctx := context.Background()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../../../../.."))
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(root, "src/backend/migrations/chat_db"))
	db.Run(true, "up")
	var version int
	var dirty bool
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT version,dirty FROM schema_migrations`).Scan(&version, &dirty))
	require.Equal(t, 26, version)
	require.False(t, dirty)
	native := &store.DMStore{Pool: db.Pool}
	viewer := uuid.New()
	application, environment := uuid.New(), uuid.New()
	managed, err := native.ProvisionManagedChat(ctx, store.ManagedChatCreate{ApplicationID: application, EnvironmentID: environment, OperationID: uuid.New(), ExternalKey: "report-managed-native", RequestHash: "sha256:" + strings.Repeat("a", 64), Name: "report interval"})
	require.NoError(t, err)
	sync := func(profiles []uuid.UUID) {
		_, err := native.SyncManagedChatMembers(ctx, store.ManagedChatMemberSync{ApplicationID: application, EnvironmentID: environment, OperationID: uuid.New(), ChatID: managed.ChatID, RequestHash: "sha256:" + strings.Repeat("b", 64), ProfileIDs: profiles})
		require.NoError(t, err)
	}
	sync([]uuid.UUID{viewer})
	var joined time.Time
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT joined_at FROM managed_chat_member_intervals WHERE chat_id=$1 AND profile_id=$2 AND revoked_at IS NULL`, managed.ChatID, viewer).Scan(&joined))
	ordinary, err := native.CreateGroupChat(ctx, uuid.New(), "ordinary restriction only", nil)
	require.NoError(t, err)
	role := "reportpolicy_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	identifier := pgx.Identifier{role}.Sanitize()
	_, err = db.Pool.Exec(ctx, `CREATE ROLE `+identifier+` LOGIN PASSWORD 'private-policy-fixture' NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION;GRANT USAGE ON SCHEMA public TO `+identifier+`;GRANT SELECT ON ALL TABLES IN SCHEMA public TO `+identifier)
	require.NoError(t, err)
	cfgDB := db.Pool.Config().Copy()
	cfgDB.ConnConfig.User = role
	cfgDB.ConnConfig.Password = "private-policy-fixture"
	login, err := pgxpool.NewWithConfig(ctx, cfgDB)
	require.NoError(t, err)
	t.Cleanup(func() {
		login.Close()
		_, err := db.Pool.Exec(ctx, `DROP OWNED BY `+identifier+`;DROP ROLE `+identifier)
		require.NoError(t, err)
	})
	actual := &nativeReportPolicyCounter{actual: &store.DMStore{Pool: login}}
	cert, keyFile, ca, clientCert, roots := reportPolicyTLSMaterial(t)
	signing, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	keys := []map[string]string{}
	for i, k := range []*rsa.PrivateKey{signing, next} {
		kid := "current"
		if i == 1 {
			kid = "next"
		}
		keys = append(keys, map[string]string{"kty": "RSA", "kid": kid, "alg": "RS256", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(k.N.Bytes()), "e": "AQAB"})
	}
	jwks := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"keys": keys}))
	}))
	identity, err := tls.LoadX509KeyPair(cert, keyFile)
	require.NoError(t, err)
	jwks.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{identity}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots}
	jwks.StartTLS()
	t.Cleanup(jwks.Close)
	replay := miniredis.RunT(t)
	clientDir := t.TempDir()
	clientPEM, err := x509.MarshalPKCS8PrivateKey(clientCert.PrivateKey)
	require.NoError(t, err)
	clientFile, clientKeyFile := filepath.Join(clientDir, "client.pem"), filepath.Join(clientDir, "client-key.pem")
	require.NoError(t, os.WriteFile(clientFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientCert.Certificate[0]}), 0600))
	require.NoError(t, os.WriteFile(clientKeyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: clientPEM}), 0600))
	policyRuntime, err := reportprincipal.New(ctx, reportprincipal.Config{ListenAddr: "127.0.0.1:0", CertFile: cert, KeyFile: keyFile, ClientCAFile: ca, ReplayAddr: replay.Addr(), JWKSCAFile: ca, JWKSClientCertFile: clientFile, JWKSClientKeyFile: clientKeyFile, JWKSURLs: map[string]string{"messaging": jwks.URL}, RefreshAfter: 30 * time.Second, HardExpiry: 2 * time.Minute, UnknownKIDCooldown: 5 * time.Second})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, policyRuntime.Close()) })
	server := grpc.NewServer(policyRuntime.ServerOptions()...)
	RegisterReportPolicyServer(server, &ReportPolicyGRPC{Store: actual})
	t.Cleanup(server.Stop)
	listener := bufconn.Listen(1024 * 1024)
	t.Cleanup(func() { _ = listener.Close() })
	go func() { _ = server.Serve(listener) }()
	connect := func(certs []tls.Certificate) chatv1.ChatServiceClient {
		conn, err := grpc.NewClient("passthrough:///report-policy", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "localhost", Certificates: certs})))
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, conn.Close()) })
		return chatv1.NewChatServiceClient(conn)
	}
	client := connect([]tls.Certificate{clientCert})
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "messaging", KeyID: "current", PrivateKey: signing})
	require.NoError(t, err)
	signed := func(r *chatv1.CheckMessageReadEntitlementRequest) context.Context {
		hash, err := principal.RequestHash(r)
		require.NoError(t, err)
		id := uuid.NewString()
		token, err := issuer.IssueService(principal.ServiceInput{Audience: "chat", RPC: reportprincipal.Method, RequestID: id, RequestHash: hash})
		require.NoError(t, err)
		return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
	}
	request := func(chat, profile uuid.UUID, at time.Time) *chatv1.CheckMessageReadEntitlementRequest {
		return &chatv1.CheckMessageReadEntitlementRequest{ChatId: chat.String(), ProfileId: profile.String(), MessageCreatedAt: timestamppb.New(at)}
	}
	check := func(r *chatv1.CheckMessageReadEntitlementRequest, allowed bool) {
		response, err := client.CheckMessageReadEntitlement(signed(r), r)
		require.NoError(t, err)
		require.Equal(t, allowed, response.Entitled)
	}
	r := request(managed.ChatID, viewer, joined)
	tokenCtx := signed(r)
	response, err := client.CheckMessageReadEntitlement(tokenCtx, r)
	require.NoError(t, err)
	require.True(t, response.Entitled)
	count := actual.calls.Load()
	_, err = client.CheckMessageReadEntitlement(tokenCtx, r)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Equal(t, count, actual.calls.Load())
	changed := proto.Clone(r).(*chatv1.CheckMessageReadEntitlementRequest)
	changed.ProfileId = uuid.NewString()
	_, err = client.CheckMessageReadEntitlement(signed(r), changed)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Equal(t, count, actual.calls.Load())
	check(request(managed.ChatID, viewer, joined.Add(-time.Microsecond)), false)
	check(request(managed.ChatID, uuid.New(), joined), false)
	sync(nil)
	var revoked time.Time
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT revoked_at FROM managed_chat_member_intervals WHERE chat_id=$1 AND profile_id=$2 AND revoked_at IS NOT NULL`, managed.ChatID, viewer).Scan(&revoked))
	time.Sleep(2 * time.Millisecond)
	sync([]uuid.UUID{viewer})
	check(request(managed.ChatID, viewer, joined), true)
	check(request(managed.ChatID, viewer, revoked.Add(time.Microsecond)), false)
	check(request(ordinary.ID, viewer, joined), true) // Native unmanaged entitlement is NOT Space/viewer ACL.
	_, err = client.GetChat(signed(r), &chatv1.GetChatRequest{})
	require.Equal(t, codes.Unimplemented, status.Code(err))
	noCert := connect(nil)
	limited, cancel := context.WithTimeout(signed(r), time.Second)
	defer cancel()
	_, err = noCert.CheckMessageReadEntitlement(limited, r)
	require.Error(t, err)
}

func reportPolicyTLSMaterial(t *testing.T) (string, string, string, tls.Certificate, *x509.CertPool) {
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
