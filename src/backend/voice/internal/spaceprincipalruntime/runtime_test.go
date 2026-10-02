package spaceprincipalruntime

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/pkg/principal"
)

func TestRuntimeVerifiesSpaceLifecycleRequestsAndRejectsReplayAndOtherMethods(t *testing.T) {
	current, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	jwk := func(kid string, key *rsa.PrivateKey) map[string]string {
		return map[string]string{
			"kid": kid, "kty": "RSA", "use": "sig", "alg": "RS256",
			"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{jwk("current", current), jwk("next", next)}})
	}))
	t.Cleanup(server.Close)
	dir := t.TempDir()
	certPath, keyPath := filepath.Join(dir, "principal.crt"), filepath.Join(dir, "principal.key")
	cert := server.TLS.Certificates[0]
	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]})
	keyDER, err := x509.MarshalPKCS8PrivateKey(cert.PrivateKey)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(certPath, certificatePEM, 0o600))
	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600))
	replay := miniredis.RunT(t)
	runtime, err := New(context.Background(), Config{
		JWKSURLs: map[string]string{"space": server.URL}, JWKSCAFile: certPath, ReplayAddr: replay.Addr(),
		TLSCertFile: certPath, TLSKeyFile: keyPath, ClientCAFile: certPath,
		ListenAddr:   ":0",
		RefreshAfter: time.Minute, HardExpiry: 2 * time.Minute, UnknownKIDCooldown: time.Second,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, runtime.Close()) })

	request := &callsv1.ApplySpaceLifecycleFenceRequest{}
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	method := callsv1.VoiceService_ApplySpaceLifecycleFence_FullMethodName
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: current})
	require.NoError(t, err)
	token, err := issuer.IssueService(principal.ServiceInput{Audience: "voice", RPC: method, RequestID: "operation-1", RequestHash: hash})
	require.NoError(t, err)
	verified, err := runtime.Verify(context.Background(), token, method, "operation-1", hash)
	require.NoError(t, err)
	require.Equal(t, "service:space", verified.Subject)
	require.Equal(t, "voice", verified.Audience)
	_, err = runtime.Verify(context.Background(), token, method, "operation-1", hash)
	require.Equal(t, codes.Unauthenticated, status.Code(err))

	otherMethod := callsv1.VoiceService_GetActiveCall_FullMethodName
	otherToken, err := issuer.IssueService(principal.ServiceInput{Audience: "voice", RPC: otherMethod, RequestID: "operation-2", RequestHash: hash})
	require.NoError(t, err)
	_, err = runtime.Verify(context.Background(), otherToken, otherMethod, "operation-2", hash)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
