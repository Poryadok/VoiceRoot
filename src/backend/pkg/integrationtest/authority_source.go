package integrationtest

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"voice/backend/pkg/authoritysource"
	authorityv1 "voice/backend/pkg/pb/voice/authority/v1"
	"voice/backend/pkg/principal"
)

type SourceTLSFixture struct {
	Config            authoritysource.RuntimeConfig
	Roots             *x509.CertPool
	ClientCertificate tls.Certificate
	Current, Next     *rsa.PrivateKey
	Redis             *miniredis.Miniredis
	JWKS              *SourceJWKSFixture
}

type sourceJWKSResponse struct {
	status int
	body   []byte
}
type SourceJWKSFixture struct {
	response atomic.Pointer[sourceJWKSResponse]
}

func (j *SourceJWKSFixture) Set(status int, body []byte) {
	j.response.Store(&sourceJWKSResponse{status, append([]byte{}, body...)})
}

func SourceJWKSDocument(t *testing.T, current, next *rsa.PrivateKey) []byte {
	t.Helper()
	keys := []map[string]string{}
	for _, entry := range []struct {
		id  string
		key *rsa.PrivateKey
	}{{"current", current}, {"next", next}} {
		keys = append(keys, map[string]string{"kid": entry.id, "kty": "RSA", "use": "sig", "alg": "RS256", "n": base64.RawURLEncoding.EncodeToString(entry.key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(entry.key.E)).Bytes())})
	}
	encoded, err := json.Marshal(map[string]any{"keys": keys})
	require.NoError(t, err)
	return encoded
}

// NewSourceTLSFixture owns ephemeral TLS/signing material and speaks actual
// HTTPS and Redis protocols. Consumers supply their real owning reader.
func NewSourceTLSFixture(t *testing.T, owner authorityv1.AuthorityOwner) SourceTLSFixture {
	t.Helper()
	current, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	jwks := &SourceJWKSFixture{}
	jwks.Set(http.StatusOK, SourceJWKSDocument(t, current, next))
	https := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		response := jwks.response.Load()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(response.status)
		_, _ = w.Write(response.body)
	}))
	t.Cleanup(https.Close)
	dir := t.TempDir()
	certFile, keyFile, clientCAFile := filepath.Join(dir, "server.crt"), filepath.Join(dir, "server.key"), filepath.Join(dir, "client-ca.pem")
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: https.TLS.Certificates[0].Certificate[0]})
	require.NoError(t, os.WriteFile(certFile, certPEM, 0600))
	keyDER, err := x509.MarshalPKCS8PrivateKey(https.TLS.Certificates[0].PrivateKey)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0600))
	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	clientTemplate := &x509.Certificate{SerialNumber: big.NewInt(9017), NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, BasicConstraintsValid: true, IsCA: true}
	clientDER, err := x509.CreateCertificate(rand.Reader, clientTemplate, clientTemplate, &clientKey.PublicKey, clientKey)
	require.NoError(t, err)
	clientPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientDER})
	clientKeyDER, err := x509.MarshalPKCS8PrivateKey(clientKey)
	require.NoError(t, err)
	clientCertificate, err := tls.X509KeyPair(clientPEM, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: clientKeyDER}))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(clientCAFile, clientPEM, 0600))
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(certPEM))
	replay := miniredis.RunT(t)
	return SourceTLSFixture{Config: authoritysource.RuntimeConfig{Owner: owner, JWKSURL: https.URL, JWKSCAFile: certFile, TLSCertFile: certFile, TLSKeyFile: keyFile, ClientCAFile: clientCAFile, ReplayAddr: replay.Addr(), ListenAddr: "127.0.0.1:0", RefreshAfter: 30 * time.Second, HardExpiry: 2 * time.Minute, UnknownKIDCooldown: 50 * time.Millisecond}, Roots: roots, ClientCertificate: clientCertificate, Current: current, Next: next, Redis: replay, JWKS: jwks}
}

func (f SourceTLSFixture) ClientCredentials() credentials.TransportCredentials {
	return credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: f.Roots, Certificates: []tls.Certificate{f.ClientCertificate}})
}

func (f SourceTLSFixture) SignedContext(t *testing.T, request proto.Message, rpc, id string) context.Context {
	t.Helper()
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "federation", KeyID: "current", PrivateKey: f.Current})
	require.NoError(t, err)
	token, err := issuer.IssueService(principal.ServiceInput{Audience: authoritysource.Audience(f.Config.Owner), RPC: rpc, RequestID: id, RequestHash: hash})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)
	return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
}
