package socialprincipal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"voice/backend/pkg/principal"
)

// Exercise the env loader and actual HTTPS verifiers in one process with two
// independent roots. Redis is explicitly miniredis; this is transport proof,
// not a complete Space service or live-Redis acceptance run.
func TestListenerLocalCATwoTrustDomains(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, data []byte) string {
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, data, 0600))
		return path
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	document := jwksDocument(t, map[string]*rsa.PublicKey{"current": &key.PublicKey, "next": &next.PublicKey})
	makeJWKS := func(name string) (*httptest.Server, string, string, *x509.CertPool) {
		cert, certPEM, privatePEM := ephemeralTLS(t)
		server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(document) }))
		server.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}}
		server.StartTLS()
		t.Cleanup(server.Close)
		roots := x509.NewCertPool()
		require.True(t, roots.AppendCertsFromPEM(certPEM))
		return server, write(name+".pem", certPEM), write(name+".key", privatePEM), roots
	}
	social, socialCert, socialKey, socialRoots := makeJWKS("social")
	voice, voiceCert, voiceKey, voiceRoots := makeJWKS("voice")
	ca, caKey, caPEM := testMessagingClientCA(t)
	clientCert := testMessagingClientCertificate(t, ca, caKey)
	private, err := x509.MarshalPKCS8PrivateKey(clientCert.PrivateKey)
	require.NoError(t, err)
	clientPath := write("client.pem", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientCert.Certificate[0]}))
	clientKey := write("client.key", pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private}))
	clientCA := write("client-ca.pem", caPEM)
	replay := miniredis.RunT(t)
	t.Setenv("S2S_JWKS_URLS_JSON", `{"social":"`+social.URL+`","voice":"`+voice.URL+`"}`)
	t.Setenv("S2S_JWKS_CA_FILE", socialCert)
	for _, values := range []struct{ prefix, cert, key string }{
		{"SPACE_PRINCIPAL_", socialCert, socialKey},
		{"SPACE_VOICE_COMMUNICATION_", voiceCert, voiceKey},
	} {
		t.Setenv(values.prefix+"TLS_CERT_FILE", values.cert)
		t.Setenv(values.prefix+"TLS_KEY_FILE", values.key)
		t.Setenv(values.prefix+"REPLAY_REDIS_ADDR", replay.Addr())
	}
	const prefix = "SPACE_VOICE_COMMUNICATION_"
	t.Setenv(prefix+"JWKS_CA_FILE", voiceCert)
	t.Setenv(prefix+"CLIENT_CA_FILE", clientCA)
	t.Setenv(prefix+"JWKS_CLIENT_CERT_FILE", clientPath)
	t.Setenv(prefix+"JWKS_CLIENT_KEY_FILE", clientKey)
	socialCfg, enabled, err := LoadFromEnv("space")
	require.NoError(t, err)
	require.True(t, enabled)
	voiceCfg, enabled, err := LoadFromEnvWithAudience("space", "space-voice-communication", prefix, ":9094")
	require.NoError(t, err)
	require.True(t, enabled)
	require.Equal(t, socialCert, socialCfg.JWKSCAFile)
	require.Equal(t, voiceCert, voiceCfg.JWKSCAFile)
	socialRuntime, err := New(t.Context(), socialCfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = socialRuntime.Close() })
	voiceRuntime, err := New(t.Context(), voiceCfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = voiceRuntime.Close() })
	var socialCalls, intercepted, voiceCalls atomic.Int32
	socialConn := testPrivacyConnection(t, socialRuntime, socialRoots, &socialCalls, &intercepted)
	voiceConn := communicationRuntimeConnection(t, voiceRuntime, &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: voiceRoots, ServerName: "privacy.test", Certificates: []tls.Certificate{clientCert}}, &voiceCalls)
	request := wrapperspb.String("same profile")
	for _, tc := range []struct{ issuer, capability string }{{"social", "space"}, {"voice", "space-voice-communication"}} {
		signer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: tc.issuer, KeyID: "current", PrivateKey: key})
		require.NoError(t, err)
		hash, err := principal.RequestHash(request)
		require.NoError(t, err)
		token, err := signer.IssueService(principal.ServiceInput{Audience: "space", RPC: Method(tc.capability), RequestID: tc.issuer, RequestHash: hash})
		require.NoError(t, err)
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", tc.issuer))
		if tc.issuer == "social" {
			err = socialConn.Invoke(ctx, Method(tc.capability), request, new(wrapperspb.StringValue))
		} else {
			err = voiceConn.Invoke(ctx, Method(tc.capability), request, new(wrapperspb.StringValue))
		}
		cancel()
		require.NoError(t, err)
	}
	require.EqualValues(t, 1, socialCalls.Load())
	require.EqualValues(t, 1, voiceCalls.Load())
	for _, path := range []string{filepath.Join(dir, "absent.pem"), write("corrupt.pem", []byte("invalid CA"))} {
		cfg := voiceCfg
		cfg.JWKSCAFile = path
		_, err := New(t.Context(), cfg)
		require.Error(t, err, "bad selected CA must not use shared CA")
	}
	wrong := voiceCfg
	wrong.JWKSCAFile = socialCert
	wrongRuntime, err := New(t.Context(), wrong)
	require.NoError(t, err)
	t.Cleanup(func() { _ = wrongRuntime.Close() })
	_, err = wrongRuntime.resolver.Resolve(t.Context(), "voice", "current")
	require.Error(t, err, "Social trust cannot validate Voice TLS")
}
