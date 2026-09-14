package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func socialTLSFixture(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "social"}, DNSNames: []string{"social"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, IsCA: true, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	cert := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	require.NoError(t, os.WriteFile(cert, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600))
	private, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private}), 0600))
	return cert, keyPath
}
func socialConfiguredRuntime(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	writeSocialPrincipalKey(t, dir, "current")
	writeSocialPrincipalKey(t, dir, "next")
	cert, key := socialTLSFixture(t)
	for k, v := range map[string]string{"SOCIAL_PRINCIPAL_SIGNING_KEYS_DIR": dir, "SOCIAL_PRINCIPAL_ACTIVE_KID": "current", "SOCIAL_PRINCIPAL_TLS_CERT_FILE": cert, "SOCIAL_PRINCIPAL_TLS_KEY_FILE": key, "USER_PRINCIPAL_GRPC_ADDR": "user:9091", "USER_PRINCIPAL_TLS_CA_FILE": cert, "USER_PRINCIPAL_TLS_SERVER_NAME": "user", "SPACE_PRINCIPAL_GRPC_ADDR": "space:9091", "SPACE_PRINCIPAL_TLS_CA_FILE": cert, "SPACE_PRINCIPAL_TLS_SERVER_NAME": "space"} {
		t.Setenv(k, v)
	}
}
func TestSocialPrincipalRuntimeConfiguredTLSAndPublicJWKS(t *testing.T) {
	socialConfiguredRuntime(t)
	runtime, err := loadSocialPrincipalRuntime()
	require.NoError(t, err)
	defer runtime.Close()
	require.NotNil(t, runtime.User)
	require.NotNil(t, runtime.Space)
	require.Equal(t, ":8443", runtime.JWKS.Addr)
	require.GreaterOrEqual(t, runtime.JWKS.TLSConfig.MinVersion, uint16(0x0303))
	require.Len(t, runtime.JWKS.TLSConfig.Certificates, 1)
	req := httptest.NewRequest("GET", "https://social/.well-known/jwks.json", nil)
	res := httptest.NewRecorder()
	runtime.JWKS.Handler.ServeHTTP(res, req)
	require.Equal(t, 200, res.Code)
	require.Contains(t, res.Body.String(), `"kid":"current"`)
	require.Contains(t, res.Body.String(), `"kid":"next"`)
	require.NotContains(t, res.Body.String(), `"d":`)
}
func TestSocialPrincipalRuntimePartialConfigFailsClosed(t *testing.T) {
	for _, name := range []string{"USER_GRPC_ADDR", "SPACE_GRPC_ADDR", "USER_PRINCIPAL_GRPC_ADDR", "USER_PRINCIPAL_TLS_CA_FILE", "SOCIAL_PRINCIPAL_JWKS_LISTEN", "SOCIAL_PRINCIPAL_TLS_KEY_FILE"} {
		t.Run(name, func(t *testing.T) {
			t.Setenv(name, "configured")
			_, err := loadSocialPrincipalRuntime()
			require.Error(t, err)
		})
	}
}
func TestSocialPrincipalRuntimeRejectsInvalidTLS(t *testing.T) {
	for _, name := range []string{"SOCIAL_PRINCIPAL_TLS_CERT_FILE", "SOCIAL_PRINCIPAL_TLS_KEY_FILE", "USER_PRINCIPAL_TLS_CA_FILE", "USER_PRINCIPAL_TLS_SERVER_NAME", "SPACE_PRINCIPAL_TLS_CA_FILE", "SPACE_PRINCIPAL_TLS_SERVER_NAME"} {
		t.Run(name, func(t *testing.T) {
			socialConfiguredRuntime(t)
			t.Setenv(name, "")
			_, err := loadSocialPrincipalRuntime()
			require.Error(t, err)
		})
	}
}
