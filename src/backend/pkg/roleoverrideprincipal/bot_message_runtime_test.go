package roleoverrideprincipal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
	"voice/backend/pkg/principal"
)

func TestMessagingAuthorityRuntimeExactIssuerAndMutualTLSJWKS(t *testing.T) {
	cert, pem, keyPEM := targetTLS(t)
	dir := t.TempDir()
	certPath, keyPath := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	require.NoError(t, os.WriteFile(certPath, pem, 0600))
	require.NoError(t, os.WriteFile(keyPath, keyPEM, 0600))
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	jwks, err := principal.PublicJWKSKeys([]principal.JWKSKey{{KeyID: "active", PublicKey: &key.PublicKey}, {KeyID: "next", PublicKey: &next.PublicKey}})
	require.NoError(t, err)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(pem))
	publisher := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NotEmpty(t, r.TLS.PeerCertificates)
		_, _ = w.Write(jwks)
	}))
	publisher.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots}
	publisher.StartTLS()
	defer publisher.Close()
	replay := miniredis.RunT(t)
	cfg := Config{Target: "space", ListenAddr: "127.0.0.1:0", TLSCertFile: certPath, TLSKeyFile: keyPath, ClientCAFile: certPath, JWKSCAFile: certPath, JWKSClientCertFile: certPath, JWKSClientKeyFile: keyPath, JWKSURLs: map[string]string{"role": publisher.URL, "messaging": publisher.URL, "bot": publisher.URL}, ReplayAddr: replay.Addr(), RefreshAfter: 30 * time.Second, HardExpiry: 2 * time.Minute, UnknownKIDCooldown: time.Second}
	ctx := context.Background()
	runtime, err := New(ctx, cfg)
	require.NoError(t, err)
	defer runtime.Close()
	hash, err := principal.RequestHash(&emptypb.Empty{})
	require.NoError(t, err)
	signed := func(issuer, method string) (string, string) {
		signer, e := principal.NewIssuer(principal.IssuerConfig{Issuer: issuer, KeyID: "active", PrivateKey: key})
		require.NoError(t, e)
		id := uuid.NewString()
		token, e := signer.IssueService(principal.ServiceInput{Audience: "space", RPC: method, RequestID: id, RequestHash: hash})
		require.NoError(t, e)
		return token, id
	}
	token, id := signed("messaging", MemberAdmissionAuthorityMethod)
	p, err := runtime.Verify(ctx, token, MemberAdmissionAuthorityMethod, id, hash)
	require.NoError(t, err)
	require.Equal(t, "messaging", p.Issuer)
	_, err = runtime.Verify(ctx, token, MemberAdmissionAuthorityMethod, id, hash)
	require.Error(t, err, "exact transport replay denied")
	token, id = signed("messaging", Method("space"))
	_, err = runtime.Verify(ctx, token, Method("space"), id, hash)
	require.Error(t, err, "Messaging gets no override target authority")
	token, id = signed("bot", MemberAdmissionAuthorityMethod)
	_, err = runtime.Verify(ctx, token, MemberAdmissionAuthorityMethod, id, hash)
	require.Error(t, err, "shared map does not trust unrelated issuer")
	cfg.Target = "chat"
	chatRuntime, err := New(ctx, cfg)
	require.NoError(t, err)
	defer chatRuntime.Close()
	require.Empty(t, chatRuntime.endpoints["messaging"])
	cfg.Target = "space"
	cfg.JWKSClientKeyFile = ""
	require.Error(t, cfg.validate())
	cfg.JWKSClientCertFile = ""
	noClient, err := New(ctx, cfg)
	require.NoError(t, err)
	defer noClient.Close()
	token, id = signed("messaging", MemberAdmissionAuthorityMethod)
	_, err = noClient.Verify(ctx, token, MemberAdmissionAuthorityMethod, id, hash)
	require.Error(t, err, "JWKS mTLS cannot silently fall back")
}
