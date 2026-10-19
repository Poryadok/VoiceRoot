package grpcsvc_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	botv1 "voice.app/voice/bot/v1"
	"voice/backend/bot/internal/grpcsvc"
	"voice/backend/bot/internal/principalgrpc"
	"voice/backend/bot/internal/principalruntime"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/integrationtest"
)

// TestBotGatewayHTTPFixture is an explicitly selected, bounded private-Linux
// cross-process fixture. The Gateway companion sends real HTTP and calls this
// production mTLS verifier/handler; no test capability enters the handler.
func TestBotGatewayHTTPFixture(t *testing.T) {
	dir := os.Getenv("BOT_GATEWAY_HTTP_FIXTURE_DIR")
	if dir == "" {
		t.Skip("explicit cross-process fixture only")
	}
	require.NoError(t, os.MkdirAll(dir, 0700))
	owner, floor := actualJavaInitiatingFloor(t)
	require.NotNil(t, floor, "this fixture requires the real Java Auth listener")
	ctx := t.Context()
	pool := integrationtest.StartPostgres(t, ctx, "bot_http_fixture", "")
	_, err := pool.Exec(ctx, migrationSQL(t))
	require.NoError(t, err)
	st := &store.BotStore{Pool: pool}
	bot, _, err := st.CreateBot(ctx, owner, "HTTP lifecycle", "", `[]`, uuid.New())
	require.NoError(t, err)
	profile := uuid.New()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	keyFile := filepath.Join(dir, "gateway-key.pem")
	require.NoError(t, os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0600))
	certTemplate := &x509.Certificate{SerialNumber: big.NewInt(247), NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}}
	der, err := x509.CreateCertificate(rand.Reader, certTemplate, certTemplate, &key.PublicKey, key)
	require.NoError(t, err)
	certFile := filepath.Join(dir, "transport-cert.pem")
	require.NoError(t, os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600))
	jwks := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{"kty": "RSA", "kid": "fixture", "alg": "RS256", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())}}})
	}))
	t.Cleanup(jwks.Close)
	jwksCA := filepath.Join(dir, "jwks-ca.pem")
	require.NoError(t, os.WriteFile(jwksCA, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: jwks.Certificate().Raw}), 0600))
	replay := miniredis.RunT(t)
	runtime, err := principalruntime.New(ctx, principalruntime.Config{ReplayRedisURL: "redis://" + replay.Addr(), BotPrivateKeyFile: keyFile, BotKeyID: "fixture", BotTLSCert: certFile, BotTLSKey: keyFile, BotJWKSClientCA: certFile})
	require.NoError(t, err)
	t.Cleanup(func() { _ = runtime.Close() })
	protected, err := runtime.NewGatewayRuntime(principalruntime.GatewayConfig{CertFile: certFile, KeyFile: keyFile, ClientCAFile: certFile, JWKSURL: jwks.URL, JWKSCAFile: jwksCA}, floor.RequireCurrent)
	require.NoError(t, err)
	t.Cleanup(protected.Close)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(protected.TLS)), grpc.UnaryInterceptor(principalgrpc.GatewayLifecycleUnaryInterceptor(protected.Verifier)), grpc.StreamInterceptor(principalgrpc.GatewayLifecycleStreamInterceptor()))
	botv1.RegisterBotServiceServer(server, grpcsvc.NewBotGRPC(st, nil))
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); _ = listener.Close() })
	expect := http.StatusNoContent
	if os.Getenv("BOT_AUTH_FLOOR_INTEROP_EXPECT") == "0" {
		expect = http.StatusServiceUnavailable
	}
	if os.Getenv("BOT_AUTH_FLOOR_INTEROP_EXPECT") == "20" {
		expect = http.StatusUnauthorized
	}
	ready, err := json.Marshal(map[string]any{"address": listener.Addr().String(), "bot_id": bot.ID.String(), "account_id": owner.String(), "profile_id": profile.String(), "key_file": keyFile, "cert_file": certFile, "expect": expect})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ready.tmp"), ready, 0600))
	require.NoError(t, os.Rename(filepath.Join(dir, "ready.tmp"), filepath.Join(dir, "ready.json")))
	deadline := time.NewTimer(180 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-deadline.C:
			t.Fatal("Gateway companion did not finish within bounded fixture lifetime")
		case <-tick.C:
			if _, err := os.Stat(filepath.Join(dir, "done")); err == nil {
				goto verify
			}
		}
	}
verify:
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM bot_deletion_operations WHERE bot_id=$1`, bot.ID).Scan(&count))
	live, err := st.GetBotByID(ctx, bot.ID)
	require.NoError(t, err)
	if expect != http.StatusNoContent {
		require.Zero(t, count)
		require.Equal(t, "live", live.Status)
		return
	}
	require.Equal(t, 1, count)
	require.Equal(t, "disabled", live.Status)
	var decision []byte
	var state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT decision_bytes,state FROM bot_deletion_operations WHERE bot_id=$1`, bot.ID).Scan(&decision, &state))
	require.Equal(t, "COMPLETE", state)
	var saved struct{ Admission *store.DeletionAdmission }
	require.NoError(t, json.Unmarshal(decision, &saved))
	require.NotNil(t, saved.Admission)
	require.Equal(t, owner, saved.Admission.AccountID)
	require.Equal(t, profile, saved.Admission.ProfileID)
	require.EqualValues(t, 9, saved.Admission.SessionEpoch)
}
