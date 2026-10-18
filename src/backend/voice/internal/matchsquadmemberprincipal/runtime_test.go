package matchsquadmemberprincipal

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/pkg/principal"
)

func TestNewStartsWithColdJWKSAndProtectedCallRecoversWhenGatewayReturns(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	document, err := json.Marshal(map[string]any{"keys": []any{map[string]any{
		"kty": "RSA", "kid": "current", "use": "sig", "alg": "RS256",
		"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
		"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
	}}})
	if err != nil {
		t.Fatal(err)
	}
	available := atomic.Bool{}
	var jwksRequests atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		jwksRequests.Add(1)
		if !available.Load() {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(document)
	}))
	defer server.Close()

	redisFixture := startPrincipalRedis(t)
	certPath, keyPath, clientCAPath := writePrincipalTLSFiles(t)
	jwksCAPath := filepath.Join(t.TempDir(), "jwks-ca.pem")
	jwksCertificate := server.TLS.Certificates[0].Certificate[0]
	if err := os.WriteFile(jwksCAPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: jwksCertificate}), 0o600); err != nil {
		t.Fatal(err)
	}
	config := Config{
		ListenerAddr: "127.0.0.1:0", TLSCertFile: certPath, TLSKeyFile: keyPath,
		ClientCAFile: clientCAPath, JWKSURL: server.URL + "/jwks",
		JWKSCAFile: jwksCAPath, RedisAddr: redisFixture.addr,
	}
	runtime, err := New(context.Background(), config)
	if err != nil {
		t.Fatalf("member verifier startup required unavailable JWKS: %v", err)
	}
	defer func() { _ = runtime.Close() }()
	if got := jwksRequests.Load(); got != 0 {
		t.Fatalf("constructor fetched Gateway JWKS %d times; want empty-cache startup", got)
	}
	if got := redisFixture.pings.Load(); got != 1 {
		t.Fatalf("Redis readiness pings=%d, want one", got)
	}

	accountID := "00000000-0000-4000-8000-000000000010"
	profileID := "00000000-0000-4000-8000-000000000020"
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key})
	if err != nil {
		t.Fatal(err)
	}
	invoke := func(runtime *Runtime, keyIssuer *principal.Issuer, operation string) (bool, error) {
		t.Helper()
		request := &callsv1.JoinMatchSquadRoomRequest{ProtocolVersion: 1, OperationId: operation}
		hash, err := principal.RequestHash(request)
		if err != nil {
			return false, err
		}
		token, err := keyIssuer.IssueDelegatedUser(principal.DelegatedUserInput{
			Audience: audience, RPC: JoinMethod, RequestID: operation, RequestHash: hash,
			AccountID: accountID, ProfileID: profileID, SessionEpoch: 9, ClientExpiresAt: time.Now().Add(25 * time.Second),
		})
		if err != nil {
			return false, err
		}
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
			"authorization", "Bearer "+token, "x-request-id", operation,
		))
		called := false
		_, err = StrictUnaryInterceptor(runtime)(ctx, request, &grpc.UnaryServerInfo{FullMethod: JoinMethod}, func(ctx context.Context, _ any) (any, error) {
			called = true
			verified, ok := principal.FromContext(ctx)
			if !ok || verified.AccountID != accountID || verified.ProfileID != profileID || verified.SessionEpoch != 9 || verified.RequestID != operation || verified.RequestHash != hash {
				return nil, errors.New("handler received an unbound principal")
			}
			return nil, nil
		})
		return called, err
	}

	called, err := invoke(runtime, issuer, uuid.MustParse("00000000-0000-4000-8000-000000000001").String())
	if status.Code(err) != codes.Unavailable || called {
		t.Fatalf("cold-cache call code=%v handler called=%v; want unavailable before effects", status.Code(err), called)
	}
	if redisFixture.sets.Load() != 0 || redisFixture.gets.Load() != 0 {
		t.Fatalf("cold-cache denial reached Redis side effects: SET=%d GET=%d", redisFixture.sets.Load(), redisFixture.gets.Load())
	}

	available.Store(true)
	// This is the same Runtime that denied the cold-cache request. Refresh is
	// the operation used by its normal background loop; doing it directly here
	// avoids waiting for the production five-second failure cooldown.
	if err := runtime.resolver.Refresh(context.Background(), "gateway"); err != nil {
		t.Fatalf("resolver did not recover after Gateway returned: %v", err)
	}
	called, err = invoke(runtime, issuer, uuid.MustParse("00000000-0000-4000-8000-000000000002").String())
	if err != nil || !called {
		t.Fatalf("valid bound call after JWKS recovery called=%v err=%v", called, err)
	}
	if got := jwksRequests.Load(); got != 2 {
		t.Fatalf("expected one cold failure and one recovered refresh, got %d JWKS requests", got)
	}
	if redisFixture.sets.Load() != 1 || redisFixture.gets.Load() != 1 {
		t.Fatalf("verified call replay/epoch checks SET=%d GET=%d; want one each", redisFixture.sets.Load(), redisFixture.gets.Load())
	}

	// A new runtime with an available Gateway must fetch lazily from the first
	// protected request, without relying on a direct resolver.Refresh call.
	if err := runtime.Close(); err != nil {
		t.Fatalf("close recovered runtime before cold-start assertions: %v", err)
	}
	requestsBeforeColdStart := jwksRequests.Load()
	coldRuntime, err := New(context.Background(), config)
	if err != nil {
		t.Fatalf("member verifier startup with available JWKS failed: %v", err)
	}
	defer func() { _ = coldRuntime.Close() }()
	if got := jwksRequests.Load(); got != requestsBeforeColdStart {
		t.Fatalf("constructor fetched Gateway JWKS %d times; want empty-cache startup", got-requestsBeforeColdStart)
	}
	setsBeforeColdRequest, getsBeforeColdRequest := redisFixture.sets.Load(), redisFixture.gets.Load()
	called, err = invoke(coldRuntime, issuer, uuid.MustParse("00000000-0000-4000-8000-000000000004").String())
	if err != nil || !called {
		t.Fatalf("first protected call with available Gateway called=%v err=%v", called, err)
	}
	if got := jwksRequests.Load(); got != requestsBeforeColdStart+1 {
		t.Fatalf("first protected call fetched Gateway JWKS %d times; want exactly one", got-requestsBeforeColdStart)
	}
	if got := redisFixture.sets.Load(); got != setsBeforeColdRequest+1 {
		t.Fatalf("first protected call replay SETs=%d; want one", got-setsBeforeColdRequest)
	}
	if got := redisFixture.gets.Load(); got != getsBeforeColdRequest+1 {
		t.Fatalf("first protected call epoch GETs=%d; want one", got-getsBeforeColdRequest)
	}

	unknownKidIssuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "unknown", PrivateKey: key})
	if err != nil {
		t.Fatal(err)
	}
	called, err = invoke(coldRuntime, unknownKidIssuer, uuid.MustParse("00000000-0000-4000-8000-000000000003").String())
	if status.Code(err) != codes.Unavailable || called {
		t.Fatalf("unknown kid call code=%v handler called=%v; want denial before handler", status.Code(err), called)
	}
	if _, err := coldRuntime.resolveKey(context.Background(), "untrusted", "current"); err == nil {
		t.Fatal("untrusted issuer resolved")
	}
}

type principalRedisFixture struct {
	addr  string
	pings atomic.Int32
	sets  atomic.Int32
	gets  atomic.Int32
}

func startPrincipalRedis(t *testing.T) *principalRedisFixture {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	fixture := &principalRedisFixture{addr: listener.Addr().String()}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go fixture.serve(conn)
		}
	}()
	return fixture
}

func (f *principalRedisFixture) serve(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	for {
		command, err := readPrincipalRedisCommand(reader)
		if err != nil {
			return
		}
		if len(command) == 0 {
			_, _ = io.WriteString(conn, "-ERR empty command\r\n")
			continue
		}
		switch strings.ToUpper(command[0]) {
		case "HELLO":
			_, _ = io.WriteString(conn, "%7\r\n+server\r\n+redis\r\n+version\r\n+7.0.0\r\n+proto\r\n:3\r\n+id\r\n:1\r\n+mode\r\n+standalone\r\n+role\r\n+master\r\n+modules\r\n*0\r\n")
		case "PING":
			f.pings.Add(1)
			_, _ = io.WriteString(conn, "+PONG\r\n")
		case "SET":
			f.sets.Add(1)
			_, _ = io.WriteString(conn, "+OK\r\n")
		case "GET":
			f.gets.Add(1)
			_, _ = io.WriteString(conn, "$1\r\n9\r\n")
		case "CLIENT":
			_, _ = io.WriteString(conn, "+OK\r\n")
		default:
			_, _ = io.WriteString(conn, "+OK\r\n")
		}
	}
}

func readPrincipalRedisCommand(reader *bufio.Reader) ([]string, error) {
	header, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	if len(header) < 4 || header[0] != '*' {
		return nil, errors.New("invalid Redis command frame")
	}
	count, err := strconv.Atoi(strings.TrimSpace(header[1:]))
	if err != nil || count < 1 || count > 32 {
		return nil, errors.New("invalid Redis command count")
	}
	command := make([]string, count)
	for i := range command {
		bulk, err := reader.ReadString('\n')
		if err != nil || len(bulk) < 4 || bulk[0] != '$' {
			return nil, errors.New("invalid Redis bulk header")
		}
		length, err := strconv.Atoi(strings.TrimSpace(bulk[1:]))
		if err != nil || length < 0 || length > 4096 {
			return nil, errors.New("invalid Redis bulk length")
		}
		value := make([]byte, length+2)
		if _, err := io.ReadFull(reader, value); err != nil || string(value[length:]) != "\r\n" {
			return nil, errors.New("invalid Redis bulk value")
		}
		command[i] = string(value[:length])
	}
	return command, nil
}

func writePrincipalTLSFiles(t *testing.T) (string, string, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	dir := t.TempDir()
	certPath, keyPath, caPath := filepath.Join(dir, "voice-cert.pem"), filepath.Join(dir, "voice-key.pem"), filepath.Join(dir, "client-ca.pem")
	for path, value := range map[string][]byte{certPath: certPEM, keyPath: keyPEM, caPath: certPEM} {
		if err := os.WriteFile(path, value, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return certPath, keyPath, caPath
}
