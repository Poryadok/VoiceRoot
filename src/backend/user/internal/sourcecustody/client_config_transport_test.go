package sourcecustody

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/principal"
)

type custodyTransportFixture struct {
	cfg        ClientConfig
	binding    Binding
	key1, key2 []byte
}

// Private synthetic OPEN and a replay map only exercise the actual transport
// and signing path. They do not qualify a source or deployed custody.
func newCustodyTransportFixture(t *testing.T) custodyTransportFixture {
	t.Helper()
	_, _, _, binding, controller := custodyFixture(t)
	dir := t.TempDir()
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	caTemplate := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	require.NoError(t, err)
	ca, err := x509.ParseCertificate(caDER)
	require.NoError(t, err)
	issue := func(serial int64, usage x509.ExtKeyUsage) (tls.Certificate, []byte, []byte) {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		leaf := &x509.Certificate{SerialNumber: big.NewInt(serial), DNSNames: []string{"localhost"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{usage}}
		der, err := x509.CreateCertificate(rand.Reader, leaf, ca, &key.PublicKey, caKey)
		require.NoError(t, err)
		private, err := x509.MarshalPKCS8PrivateKey(key)
		require.NoError(t, err)
		certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
		keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private})
		cert, err := tls.X509KeyPair(certPEM, keyPEM)
		require.NoError(t, err)
		return cert, certPEM, keyPEM
	}
	serverCert, _, _ := issue(2, x509.ExtKeyUsageServerAuth)
	_, clientPEM, clientKey := issue(3, x509.ExtKeyUsageClientAuth)
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	for name, data := range map[string][]byte{"ca.pem": caPEM, "client.pem": clientPEM, "client.key": clientKey} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), data, 0600))
	}
	first, firstPEM := testPKCS8Key(t, 2048)
	next, nextPEM := testPKCS8Key(t, 2048)
	keys := filepath.Join(dir, "keys")
	require.NoError(t, os.Mkdir(keys, 0700))
	writeCustodyKey(t, keys, "current.pem", firstPEM)
	writeCustodyKey(t, keys, "next.pem", nextPEM)
	seen := map[string]bool{}
	var mu sync.Mutex
	resolve := func(_ context.Context, issuer, kid string) (*rsa.PublicKey, error) {
		if issuer != "user" {
			return nil, errors.New("unknown issuer")
		}
		switch kid {
		case "current":
			return &first.PublicKey, nil
		case "next":
			return &next.PublicKey, nil
		}
		return nil, errors.New("unknown kid")
	}
	replay := func(_ context.Context, issuer, id string, _ time.Time) error {
		mu.Lock()
		defer mu.Unlock()
		key := issuer + ":" + id
		if seen[key] {
			return errors.New("replay")
		}
		seen[key] = true
		return nil
	}
	handler, err := NewStreamServer(controller, StreamConfig{KeyResolver: resolve, ReplayGuard: replay, Clock: time.Now, Heartbeat: 100 * time.Millisecond})
	require.NoError(t, err)
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{serverCert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots})))
	userv1.RegisterUserSourceCustodyServiceServer(server, handler)
	go func() { _ = server.Serve(lis) }()
	t.Cleanup(server.Stop)
	return custodyTransportFixture{cfg: ClientConfig{Address: lis.Addr().String(), CAFile: filepath.Join(dir, "ca.pem"), ServerName: "localhost", CertFile: filepath.Join(dir, "client.pem"), KeyFile: filepath.Join(dir, "client.key"), SigningKeysDir: keys, ActiveKID: "current", HeartbeatTimeout: 500 * time.Millisecond}, binding: binding, key1: firstPEM, key2: nextPEM}
}

func TestCustodyClientConstructorAuthenticatesSignedGrant(t *testing.T) {
	f := newCustodyTransportFixture(t)
	runtime, err := OpenClient(context.Background(), f.cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, runtime.Close()) })
	require.NotNil(t, runtime.Client())
	public := runtime.JWKS()
	require.NotEmpty(t, public)
	keys, err := principal.ParseJWKS(public)
	require.NoError(t, err)
	require.Len(t, keys, 2)
	require.Contains(t, keys, "current")
	require.Contains(t, keys, "next")
	signer := runtime.Issuer()
	require.NotNil(t, signer)
	const authRPC = "/voice.identityerasure.v1.AuthUserIdentityAuthorityService/ResolveAccountHandle"
	hash := "sha256:" + strings.Repeat("a", 64)
	token, err := signer.IssueService(principal.ServiceInput{Audience: "auth", RPC: authRPC, RequestID: f.binding.SourceID, RequestHash: hash})
	require.NoError(t, err)
	_, err = principal.VerifyService(context.Background(), token, principal.VerifyConfig{ExpectedIssuer: "user", ExpectedAudience: "auth", ExpectedRPC: authRPC, ExpectedRequestID: f.binding.SourceID, ExpectedRequestHash: hash, KeyResolver: func(_ context.Context, _, kid string) (*rsa.PublicKey, error) { return keys[kid], nil }})
	require.NoError(t, err)
	require.NotContains(t, strings.ToUpper(string(public)), "PRIVATE")
	require.Equal(t, public, runtime.JWKS(), "public set is stable")
	public[0] ^= 0xff
	require.NotEqual(t, public, runtime.JWKS(), "caller cannot mutate exported public key document")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	grant, err := runtime.Client().Acquire(ctx, f.binding)
	require.NoError(t, err)
	require.NoError(t, grant.Context().Err())
	grant.Release()
}

func TestCustodyClientSignerRejectsMalformedKeysets(t *testing.T) {
	f := newCustodyTransportFixture(t)
	valid, err := OpenClient(context.Background(), f.cfg)
	require.NoError(t, err)
	require.NoError(t, valid.Close())
	_, weak := testPKCS8Key(t, 1024)
	nonRSA, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	nonRSADER, err := x509.MarshalPKCS8PrivateKey(nonRSA)
	require.NoError(t, err)
	nonRSAPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: nonRSADER})
	for _, tc := range []struct {
		name  string
		files map[string][]byte
		kid   string
	}{
		{"missing", map[string][]byte{"current.pem": f.key1}, "current"},
		{"duplicate", map[string][]byte{"current.pem": f.key1, "next.pem": f.key1}, "current"},
		{"unknown-active", map[string][]byte{"current.pem": f.key1, "next.pem": f.key2}, "unknown"},
		{"weak", map[string][]byte{"current.pem": weak, "next.pem": f.key2}, "current"},
		{"non-rsa", map[string][]byte{"current.pem": nonRSAPEM, "next.pem": f.key2}, "current"},
		{"bad-pem", map[string][]byte{"current.pem": []byte("not a key"), "next.pem": f.key2}, "current"},
		{"leading-data", map[string][]byte{"current.pem": append([]byte("junk\n"), f.key1...), "next.pem": f.key2}, "current"},
		{"forged-pem-prefix", map[string][]byte{"current.pem": append([]byte("-----BEGIN PRIVATE KEY-----junk\n"), f.key1...), "next.pem": f.key2}, "current"},
		{"trailing-data", map[string][]byte{"current.pem": append(append([]byte{}, f.key1...), []byte("junk")...), "next.pem": f.key2}, "current"},
		{"multiple-pem", map[string][]byte{"current.pem": append(append([]byte{}, f.key1...), f.key2...), "next.pem": f.key2}, "current"},
		{"extra-file", map[string][]byte{"current.pem": f.key1, "next.pem": f.key2, "extra.txt": []byte("x")}, "current"},
		{"unexpected-hidden", map[string][]byte{"current.pem": f.key1, "next.pem": f.key2, "..unexpected": []byte("x")}, "current"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, data := range tc.files {
				writeCustodyKey(t, dir, name, data)
			}
			cfg := f.cfg
			cfg.SigningKeysDir = dir
			cfg.ActiveKID = tc.kid
			runtime, err := OpenClient(context.Background(), cfg)
			if runtime != nil {
				_ = runtime.Close()
			}
			require.Error(t, err)
		})
	}
}

func TestCustodyClientSignerAcceptsProjectedSymlinksWithinRoot(t *testing.T) {
	f := newCustodyTransportFixture(t)
	dir := t.TempDir()
	version := "..2026_10_30_20_00_00.123456789"
	require.NoError(t, os.Mkdir(filepath.Join(dir, version), 0700))
	writeCustodyKey(t, filepath.Join(dir, version), "current.pem", f.key1)
	writeCustodyKey(t, filepath.Join(dir, version), "next.pem", f.key2)
	if err := os.Symlink(version, filepath.Join(dir, "..data")); err != nil {
		t.Skipf("platform cannot create projected-volume symlink: %v", err)
	}
	for _, name := range []string{"current.pem", "next.pem"} {
		if err := os.Symlink(filepath.Join("..data", name), filepath.Join(dir, name)); err != nil {
			t.Skipf("platform cannot create projected-volume symlink: %v", err)
		}
	}
	cfg := f.cfg
	cfg.SigningKeysDir = dir
	runtime, err := OpenClient(context.Background(), cfg)
	require.NoError(t, err)
	defer runtime.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	grant, err := runtime.Client().Acquire(ctx, f.binding)
	require.NoError(t, err)
	grant.Release()
}

func TestCustodyClientSignerRejectsSymlinkEscape(t *testing.T) {
	f := newCustodyTransportFixture(t)
	dir, outside := t.TempDir(), t.TempDir()
	writeCustodyKey(t, outside, "current.pem", f.key1)
	writeCustodyKey(t, dir, "next.pem", f.key2)
	if err := os.Symlink(filepath.Join(outside, "current.pem"), filepath.Join(dir, "current.pem")); err != nil {
		t.Skipf("platform cannot create key symlink: %v", err)
	}
	cfg := f.cfg
	cfg.SigningKeysDir = dir
	runtime, err := OpenClient(context.Background(), cfg)
	if runtime != nil {
		_ = runtime.Close()
	}
	require.Error(t, err, "keys must resolve inside configured directory")
}

func TestCustodyClientConstructorDeniesBadPeerTLS(t *testing.T) {
	f := newCustodyTransportFixture(t)
	_, wrongClient, _ := streamTransportTLS(t)
	private, err := x509.MarshalPKCS8PrivateKey(wrongClient.PrivateKey)
	require.NoError(t, err)
	dir := t.TempDir()
	wrongCert := filepath.Join(dir, "wrong.pem")
	wrongKey := filepath.Join(dir, "wrong.key")
	require.NoError(t, os.WriteFile(wrongCert, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: wrongClient.Certificate[0]}), 0600))
	require.NoError(t, os.WriteFile(wrongKey, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private}), 0600))
	t.Run("mismatched-client-key", func(t *testing.T) {
		cfg := f.cfg
		cfg.KeyFile = wrongKey
		runtime, err := OpenClient(context.Background(), cfg)
		if runtime != nil {
			_ = runtime.Close()
		}
		require.Error(t, err)
	})
	for _, tc := range []struct {
		name   string
		change func(*ClientConfig)
	}{
		{"wrong-name", func(c *ClientConfig) { c.ServerName = "wrong.invalid" }},
		{"wrong-server-ca", func(c *ClientConfig) { c.CAFile = wrongCert }},
		{"wrong-client-ca", func(c *ClientConfig) { c.CertFile = wrongCert; c.KeyFile = wrongKey }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := f.cfg
			tc.change(&cfg)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			runtime, err := OpenClient(ctx, cfg)
			require.NoError(t, err, "syntactically valid but untrusted material must reach handshake")
			defer runtime.Close()
			grant, err := runtime.Client().Acquire(ctx, f.binding)
			if grant != nil {
				grant.Release()
			}
			require.Error(t, err, "actual TLS handshake must deny this peer")
		})
	}
}
