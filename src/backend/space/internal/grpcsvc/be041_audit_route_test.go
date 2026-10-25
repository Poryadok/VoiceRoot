package grpcsvc

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
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/lifecycleprincipal"
	"voice/backend/space/internal/store"
)

func be041TLSFixture(t *testing.T, dir string) (tls.Certificate, string, string, string, string, string) {
	t.Helper()
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Now()
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "BE041 isolated fixture"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	require.NoError(t, err)
	caPath := filepath.Join(dir, "ca.pem")
	require.NoError(t, os.WriteFile(caPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), 0600))
	leaf := func(name string, serial int64, usage x509.ExtKeyUsage) (tls.Certificate, string, string) {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		template := &x509.Certificate{SerialNumber: big.NewInt(serial), NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), BasicConstraintsValid: true, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage}, DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}}
		der, err := x509.CreateCertificate(rand.Reader, template, ca, &key.PublicKey, caKey)
		require.NoError(t, err)
		certPath, keyPath := filepath.Join(dir, name+".crt"), filepath.Join(dir, name+".key")
		require.NoError(t, os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600))
		require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0600))
		cert, err := tls.LoadX509KeyPair(certPath, keyPath)
		require.NoError(t, err)
		return cert, certPath, keyPath
	}
	server, serverCert, serverKey := leaf("server", 2, x509.ExtKeyUsageServerAuth)
	_, clientCert, clientKey := leaf("client", 3, x509.ExtKeyUsageClientAuth)
	return server, caPath, serverCert, serverKey, clientCert, clientKey
}

// Full real route fixture: separate Gateway test process uses its actual REST
// handler, issuer and mTLS client against this production Space listener/handler.
// PostgreSQL and Redis are real disposable containers, never host services.
func TestBE041AuditRealGatewayRoute(t *testing.T) {
	if testing.Short() {
		t.Skip("requires isolated Linux Docker PostgreSQL/Redis and Gateway test binary")
	}
	ctx := context.Background()
	dir := t.TempDir()
	owner, member, outsider, account := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	pool, row := nativeAuditFixture(t, ctx, owner, member, "typed audit fixture")
	redisAddr := os.Getenv("VOICE_SPACE_AUDIT_REDIS_ADDR")
	if redisAddr == "" {
		rc, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: testcontainers.ContainerRequest{Image: "redis:7-alpine", ExposedPorts: []string{"6379/tcp"}, WaitingFor: wait.ForLog("Ready to accept connections")}, Started: true})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, rc.Terminate(context.Background())) })
		redisAddr, err = rc.Endpoint(ctx, "")
		require.NoError(t, err)
	}
	rdb := redis.NewClient(&redis.Options{Addr: redisAddr, MaxRetries: -1})
	defer rdb.Close()
	st := &store.SpaceStore{Pool: pool}
	for i := 0; i < 2; i++ {
		insertSpaceAuditEntry(t, ctx, pool, row.ID, uuid.New(), owner, "member_banned", "account", `{}`, time.Now().UTC().Add(time.Duration(-i)*time.Second))
	}
	floor := "auth:session:min_epoch:" + account.String()
	require.NoError(t, rdb.Set(ctx, floor, "7", 0).Err())
	serverTLS, caFile, certFile, keyFile, clientCert, clientKey := be041TLSFixture(t, dir)
	keys := map[string]*rsa.PrivateKey{}
	document := []map[string]string{}
	for _, kid := range []string{"current", "next"} {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		keys[kid] = key
		document = append(document, map[string]string{"kty": "RSA", "kid": kid, "alg": "RS256", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": "AQAB"})
	}
	signingKey := filepath.Join(dir, "signing.pem")
	require.NoError(t, os.WriteFile(signingKey, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(keys["current"])}), 0600))
	jwks := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"keys": document}))
	}))
	jwks.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{serverTLS}}
	jwks.StartTLS()
	defer jwks.Close()
	runtime, err := lifecycleprincipal.New(ctx, lifecycleprincipal.Config{Listen: "127.0.0.1:0", CertFile: certFile, KeyFile: keyFile, ClientCAFile: caFile, JWKSURL: jwks.URL, JWKSCAFile: caFile, RedisAddr: redisAddr})
	require.NoError(t, err)
	defer runtime.Close()
	roles := &auditPermissionRoleStub{allowed: true}
	service := &PublicLifecycleSpace{SpaceGRPC: &SpaceGRPC{Store: st, Roles: roles, AuditCursorKey: []byte(strings.Repeat("a", 32))}}
	var admitted atomic.Int64
	var permissionMu sync.RWMutex
	options := runtime.ServerOptions()
	options = append(options, grpc.ChainUnaryInterceptor(func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		for key := range md {
			if strings.HasPrefix(key, "x-voice-") {
				return nil, status.Error(codes.Internal, "synthetic identity reached handler")
			}
		}
		if _, err := principal.RequireDelegatedUser(ctx, "space", info.FullMethod, req.(*spacev1.GetAuditLogRequest)); err != nil {
			return nil, status.Error(codes.Internal, "typed identity missing")
		}
		permissionMu.RLock()
		defer permissionMu.RUnlock()
		admitted.Add(1)
		return handler(ctx, req)
	}))
	server := grpc.NewServer(options...)
	spacev1.RegisterSpaceServiceServer(server, service)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { _ = server.Serve(listener) }()
	defer server.Stop()
	binary := filepath.Join(dir, "gateway.test")
	compile := exec.CommandContext(ctx, "go", "test", "-mod=readonly", "-p=1", "-c", "-o", binary, ".")
	compile.Dir = filepath.Join(repoRoot(t), "src/backend/gateway")
	output, err := compile.CombinedOutput()
	require.NoError(t, err, string(output))
	run := func(t *testing.T, mode string, expectedAdmissions int64) {
		t.Helper()
		before := admitted.Load()
		command := exec.CommandContext(ctx, binary, "-test.run=^TestBE041GatewayAuditRoute$", "-test.v", "-test.timeout=30s")
		command.Env = append(os.Environ(), "BE041_AUDIT_ADDR="+listener.Addr().String(), "BE041_CA="+caFile, "BE041_CLIENT_CERT="+clientCert, "BE041_CLIENT_KEY="+clientKey, "BE041_SIGNING_KEY="+signingKey, "BE041_ACCOUNT="+account.String(), "BE041_OWNER="+owner.String(), "BE041_MEMBER="+member.String(), "BE041_OUTSIDER="+outsider.String(), "BE041_SPACE="+row.ID.String(), "BE041_MODE="+mode)
		output, err := command.CombinedOutput()
		t.Logf("Gateway mode %s: %s", mode, output)
		require.NoError(t, err, string(output))
		require.Equal(t, expectedAdmissions, admitted.Load()-before, "invalid credentials must never invoke handler/store/Role")
	}
	t.Run("owner member pagination and viewer binding", func(t *testing.T) { run(t, "normal", 5) })
	t.Run("real transport admission negatives", func(t *testing.T) { run(t, "admission", 1) })
	t.Run("owner only without Roles", func(t *testing.T) {
		permissionMu.Lock()
		service.Roles = nil
		permissionMu.Unlock()
		defer func() { permissionMu.Lock(); service.Roles = roles; permissionMu.Unlock() }()
		run(t, "owner-only", 2)
	})
	t.Run("explicit Role deny", func(t *testing.T) {
		permissionMu.Lock()
		roles.allowed = false
		permissionMu.Unlock()
		defer func() { permissionMu.Lock(); roles.allowed = true; permissionMu.Unlock() }()
		run(t, "role-deny", 1)
	})
	t.Run("Role dependency failure", func(t *testing.T) {
		permissionMu.Lock()
		roles.permissionErr = status.Error(codes.Unavailable, "private dependency error")
		permissionMu.Unlock()
		defer func() { permissionMu.Lock(); roles.permissionErr = nil; permissionMu.Unlock() }()
		run(t, "role-outage", 1)
	})
	for _, tc := range []struct{ mode, value string }{{"stale", "8"}, {"floor-missing", ""}, {"floor-corrupt", "07"}} {
		t.Run(tc.mode, func(t *testing.T) {
			if tc.value == "" {
				require.NoError(t, rdb.Del(ctx, floor).Err())
			} else {
				require.NoError(t, rdb.Set(ctx, floor, tc.value, 0).Err())
			}
			run(t, tc.mode, 0)
			require.NoError(t, rdb.Set(ctx, floor, "7", 0).Err())
		})
	}
	t.Run("freeze disclosure", func(t *testing.T) {
		_, err := st.ReserveLifecycleSchedule(ctx, account, owner, 7, &spacev1.DeleteSpaceRequest{SpaceId: row.ID.String(), OperationId: uuid.NewString(), ConfirmationName: row.Name, Proof: "test-only-proof"})
		require.NoError(t, err)
		run(t, "frozen", 3)
	})
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE space_id=$1 AND action='member_banned'`, row.ID).Scan(&count))
	require.Equal(t, 2, count, "read and admission cases never mutate existing audit")
}
