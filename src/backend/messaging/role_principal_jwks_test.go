package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"voice/backend/pkg/principal"
)

func TestRolePrincipalJWKSRequiresSigner(t *testing.T) {
	for _, name := range []string{managedChatPurgeSigningKeysDirEnv, managedChatPurgeActiveKIDEnv} {
		t.Setenv(name, "")
		require.NoError(t, os.Unsetenv(name))
	}
	for _, suffix := range []string{"LISTEN", "TLS_CERT_FILE", "TLS_KEY_FILE", "CLIENT_CA_FILE"} {
		t.Setenv("MESSAGING_ROLE_JWKS_"+suffix, "configured")
	}
	t.Setenv("MESSAGING_ROLE_JWKS_LISTEN", "127.0.0.1:0")
	runtime, err := newManagedChatPurgeRuntime(context.Background(), nil)
	if runtime != nil {
		defer runtime.Close()
	}
	require.Error(t, err, "Role JWKS configuration without existing signer must fail startup")
	require.ErrorContains(t, err, "signing-key configuration")
}

func TestRolePrincipalJWKSConfigIsOptionalAndRejectsPartial(t *testing.T) {
	_, enabled, err := loadRolePrincipalJWKSConfig(func(string) (string, bool) { return "", false })
	require.NoError(t, err)
	require.False(t, enabled)
	for _, missing := range []string{"LISTEN", "TLS_CERT_FILE", "TLS_KEY_FILE", "CLIENT_CA_FILE"} {
		t.Run(missing, func(t *testing.T) {
			_, enabled, err := loadRolePrincipalJWKSConfig(func(name string) (string, bool) {
				if name == "MESSAGING_ROLE_JWKS_"+missing {
					return "", true
				}
				return "configured", true
			})
			require.True(t, enabled)
			require.ErrorContains(t, err, "MESSAGING_ROLE_JWKS_"+missing)
		})
	}
	_, enabled, err = loadRolePrincipalJWKSConfig(func(string) (string, bool) { return "", true })
	require.True(t, enabled)
	require.Error(t, err)
}

func TestRolePrincipalJWKSServesSameRotationSetWithIndependentMTLS(t *testing.T) {
	dir := t.TempDir()
	for _, kid := range []string{"current", "next"} {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		encoded, err := x509.MarshalPKCS8PrivateKey(key)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, kid+".pem"), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded}), 0600))
	}
	t.Setenv(managedChatPurgeSigningKeysDirEnv, dir)
	t.Setenv(managedChatPurgeActiveKIDEnv, "current")
	base := roleJWKSFixture(t, "base")
	role := roleJWKSFixture(t, "role")
	baseAddr, roleAddr := roleJWKSAddress(t), roleJWKSAddress(t)
	for name, value := range map[string]string{
		"MESSAGING_PURGE_PRINCIPAL_JWKS_LISTEN": baseAddr, "MESSAGING_PURGE_PRINCIPAL_TLS_CERT_FILE": base.cert, "MESSAGING_PURGE_PRINCIPAL_TLS_KEY_FILE": base.key, "MESSAGING_PURGE_PRINCIPAL_CLIENT_CA_FILE": base.ca,
		"MESSAGING_ROLE_JWKS_LISTEN": roleAddr, "MESSAGING_ROLE_JWKS_TLS_CERT_FILE": role.cert, "MESSAGING_ROLE_JWKS_TLS_KEY_FILE": role.key, "MESSAGING_ROLE_JWKS_CLIENT_CA_FILE": role.ca,
	} {
		t.Setenv(name, value)
	}
	runtime, err := newManagedChatPurgeRuntime(context.Background(), nil)
	require.NoError(t, err)
	defer runtime.Close()
	fetch := func(addr string, roots *x509.CertPool, certs []tls.Certificate) ([]byte, error) {
		transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "messaging", Certificates: certs}}
		defer transport.CloseIdleConnections()
		client := &http.Client{Transport: transport, Timeout: 2 * time.Second}
		resp, err := client.Get("https://" + addr + "/.well-known/principal-jwks.json")
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		return io.ReadAll(resp.Body)
	}
	baseDoc, err := fetch(baseAddr, base.roots, []tls.Certificate{base.client})
	require.NoError(t, err)
	roleDoc, err := fetch(roleAddr, role.roots, []tls.Certificate{role.client})
	require.NoError(t, err, "Role client must reach separate trust-domain publisher")
	require.JSONEq(t, string(baseDoc), string(roleDoc))
	keys, err := principal.ParseJWKS(roleDoc)
	require.NoError(t, err)
	require.Len(t, keys, 2)
	require.NotNil(t, keys["next"])
	hash := "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	token, err := runtime.Issuer.IssueService(principal.ServiceInput{Audience: "role", RPC: "/voice.role.v1.RoleService/CheckPermission", RequestID: "test-role", RequestHash: hash})
	require.NoError(t, err)
	_, err = principal.VerifyService(context.Background(), token, principal.VerifyConfig{ExpectedIssuer: "messaging", ExpectedAudience: "role", ExpectedRPC: "/voice.role.v1.RoleService/CheckPermission", ExpectedRequestID: "test-role", ExpectedRequestHash: hash, KeyResolver: func(_ context.Context, _, kid string) (*rsa.PublicKey, error) { return keys[kid], nil }, ReplayGuard: func(context.Context, string, string, time.Time) error { return nil }})
	require.NoError(t, err)
	_, err = fetch(roleAddr, role.roots, nil)
	require.Error(t, err, "anonymous client rejected")
	_, err = fetch(roleAddr, role.roots, []tls.Certificate{base.client})
	require.Error(t, err, "base client CA not trusted on Role listener")
	_, err = fetch(baseAddr, base.roots, []tls.Certificate{role.client})
	require.Error(t, err, "existing base listener trust unchanged")
	runtime.Close()
	_, err = fetch(roleAddr, role.roots, []tls.Certificate{role.client})
	require.Error(t, err, "runtime Close must stop secondary publisher")
}

type roleJWKSTLSFixture struct {
	ca, cert, key string
	roots         *x509.CertPool
	client        tls.Certificate
}

func roleJWKSFixture(t *testing.T, label string) roleJWKSTLSFixture {
	t.Helper()
	dir := t.TempDir()
	now := time.Now()
	pub, caKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	caTemplate := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, pub, caKey)
	require.NoError(t, err)
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	caPath := filepath.Join(dir, "ca.pem")
	require.NoError(t, os.WriteFile(caPath, caPEM, 0600))
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(caPEM))
	makeCert := func(name string, usage x509.ExtKeyUsage, serial int64) (string, string, tls.Certificate) {
		pub, key, err := ed25519.GenerateKey(rand.Reader)
		require.NoError(t, err)
		template := &x509.Certificate{SerialNumber: big.NewInt(serial), NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage}, DNSNames: []string{"messaging"}}
		der, err := x509.CreateCertificate(rand.Reader, template, caTemplate, pub, caKey)
		require.NoError(t, err)
		encoded, err := x509.MarshalPKCS8PrivateKey(key)
		require.NoError(t, err)
		certPEM, keyPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded})
		certPath, keyPath := filepath.Join(dir, name+".crt"), filepath.Join(dir, name+".key")
		require.NoError(t, os.WriteFile(certPath, certPEM, 0600))
		require.NoError(t, os.WriteFile(keyPath, keyPEM, 0600))
		cert, err := tls.X509KeyPair(certPEM, keyPEM)
		require.NoError(t, err)
		return certPath, keyPath, cert
	}
	cert, key, _ := makeCert("messaging", x509.ExtKeyUsageServerAuth, 2)
	_, _, client := makeCert(label+"-client", x509.ExtKeyUsageClientAuth, 3)
	return roleJWKSTLSFixture{ca: caPath, cert: cert, key: key, roots: roots, client: client}
}
func roleJWKSAddress(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	require.NoError(t, l.Close())
	return addr
}
