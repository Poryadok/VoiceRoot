package squad

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadPrincipalJWKSListenerIsDisabledOnlyWhenProfileIsAbsent(t *testing.T) {
	loaded, err := LoadPrincipalJWKSListener(func(string) string { return "" })
	require.NoError(t, err)
	require.False(t, loaded.Enabled)
	require.Nil(t, loaded.Handler)

	_, err = LoadPrincipalJWKSListener(func(name string) string {
		if name == "MATCHMAKING_PRINCIPAL_JWKS_LISTEN_ADDR" {
			return ":8443"
		}
		return ""
	})
	require.ErrorContains(t, err, "partial")
}

func TestLoadPrincipalJWKSListenerPublishesBothValidatedRotationKeys(t *testing.T) {
	keysDir := t.TempDir()
	for _, kid := range []string{"current", "next"} {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		der, err := x509.MarshalPKCS8PrivateKey(key)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(keysDir, kid+".pem"), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600))
	}
	certFile, keyFile := writeJWKSListenerCertificate(t)
	values := map[string]string{
		"MATCHMAKING_PRINCIPAL_SIGNING_KEYS_DIR":   keysDir,
		"MATCHMAKING_PRINCIPAL_ACTIVE_KID":         "current",
		"MATCHMAKING_PRINCIPAL_JWKS_LISTEN_ADDR":   ":8443",
		"MATCHMAKING_PRINCIPAL_JWKS_TLS_CERT_FILE": certFile,
		"MATCHMAKING_PRINCIPAL_JWKS_TLS_KEY_FILE":  keyFile,
	}
	loaded, err := LoadPrincipalJWKSListener(func(name string) string { return values[name] })
	require.NoError(t, err)
	require.True(t, loaded.Enabled)
	require.Equal(t, ":8443", loaded.ListenAddr)
	require.Equal(t, uint16(tls.VersionTLS12), loaded.TLSConfig.MinVersion)

	response := httptest.NewRecorder()
	loaded.Handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "https://matchmaking/internal/v1/principal/jwks.json", nil))
	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "application/jwk-set+json", response.Header().Get("Content-Type"))
	require.Contains(t, response.Body.String(), `"kid":"current"`)
	require.Contains(t, response.Body.String(), `"kid":"next"`)
	require.NotContains(t, response.Body.String(), "PRIVATE KEY")

	methodResponse := httptest.NewRecorder()
	loaded.Handler.ServeHTTP(methodResponse, httptest.NewRequest(http.MethodPost, "/internal/v1/principal/jwks.json", strings.NewReader("{}")))
	require.Equal(t, http.StatusMethodNotAllowed, methodResponse.Code)

	server := httptest.NewUnstartedServer(loaded.Handler)
	server.TLS = loaded.TLSConfig.Clone()
	server.StartTLS()
	defer server.Close()

	serverCertificate, err := tls.LoadX509KeyPair(certFile, keyFile)
	require.NoError(t, err)
	root, err := x509.ParseCertificate(serverCertificate.Certificate[0])
	require.NoError(t, err)
	trustedRoots := x509.NewCertPool()
	trustedRoots.AddCert(root)
	trustedClient := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    trustedRoots,
	}}}
	trustedResponse, err := trustedClient.Get(server.URL + "/internal/v1/principal/jwks.json")
	require.NoError(t, err)
	defer trustedResponse.Body.Close()
	require.Equal(t, http.StatusOK, trustedResponse.StatusCode)

	untrustedClient := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    x509.NewCertPool(),
	}}}
	_, err = untrustedClient.Get(server.URL + "/internal/v1/principal/jwks.json")
	require.Error(t, err)
}

func TestLoadPrincipalJWKSListenerAcceptsProjectedSecretKeyDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Kubernetes projected Secret symlink layout is exercised on Linux")
	}
	keysDir := t.TempDir()
	version := "..2026_10_17_12_34_56.123456789"
	versionDir := filepath.Join(keysDir, version)
	require.NoError(t, os.Mkdir(versionDir, 0o700))
	writePrincipalRotationKeys(t, versionDir)
	require.NoError(t, os.Symlink(version, filepath.Join(keysDir, "..data")))
	for _, kid := range []string{"current", "next"} {
		require.NoError(t, os.Symlink(filepath.Join("..data", kid+".pem"), filepath.Join(keysDir, kid+".pem")))
	}
	certFile, keyFile := writeJWKSListenerCertificate(t)
	values := map[string]string{
		"MATCHMAKING_PRINCIPAL_SIGNING_KEYS_DIR":   keysDir,
		"MATCHMAKING_PRINCIPAL_ACTIVE_KID":         "current",
		"MATCHMAKING_PRINCIPAL_JWKS_LISTEN_ADDR":   ":8443",
		"MATCHMAKING_PRINCIPAL_JWKS_TLS_CERT_FILE": certFile,
		"MATCHMAKING_PRINCIPAL_JWKS_TLS_KEY_FILE":  keyFile,
	}
	loaded, err := LoadPrincipalJWKSListener(func(name string) string { return values[name] })
	require.NoError(t, err)
	require.True(t, loaded.Enabled)

	response := httptest.NewRecorder()
	loaded.Handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/internal/v1/principal/jwks.json", nil))
	require.Equal(t, http.StatusOK, response.Code)
	require.Contains(t, response.Body.String(), `"kid":"current"`)
	require.Contains(t, response.Body.String(), `"kid":"next"`)
}

func TestLoadMatchmakingSigningKeysAcceptsAtomicWriterUpdateInProgress(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Kubernetes projected Secret symlink layout is exercised on Linux")
	}
	keysDir := t.TempDir()
	currentVersion := "..2026_10_17_12_34_56.123456789"
	nextVersion := "..2026_10_17_12_34_57.123456789"
	for _, version := range []string{currentVersion, nextVersion} {
		versionDir := filepath.Join(keysDir, version)
		require.NoError(t, os.Mkdir(versionDir, 0o700))
		writePrincipalRotationKeys(t, versionDir)
	}
	expected, err := loadMatchmakingSigningKeys(filepath.Join(keysDir, currentVersion), "current")
	require.NoError(t, err)
	require.NoError(t, os.Symlink(currentVersion, filepath.Join(keysDir, "..data")))
	require.NoError(t, os.Symlink(nextVersion, filepath.Join(keysDir, "..data_tmp")))
	for _, kid := range []string{"current", "next"} {
		require.NoError(t, os.Symlink(filepath.Join("..data", kid+".pem"), filepath.Join(keysDir, kid+".pem")))
	}

	keys, err := loadMatchmakingSigningKeys(keysDir, "current")
	require.NoError(t, err)
	require.Len(t, keys, 2)
	require.Equal(t, expected["current"].N, keys["current"].N)
	require.Equal(t, expected["next"].N, keys["next"].N)
}

func TestLoadMatchmakingSigningKeysUsesPublishedAtomicWriterVersion(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Kubernetes projected Secret symlink layout is exercised on Linux")
	}
	keysDir := t.TempDir()
	oldVersion := "..2026_10_17_12_34_56.123456789"
	newVersion := "..2026_10_17_12_34_57.123456789"
	for _, version := range []string{oldVersion, newVersion} {
		versionDir := filepath.Join(keysDir, version)
		require.NoError(t, os.Mkdir(versionDir, 0o700))
		writePrincipalRotationKeys(t, versionDir)
	}
	expected, err := loadMatchmakingSigningKeys(filepath.Join(keysDir, newVersion), "current")
	require.NoError(t, err)
	require.NoError(t, os.Symlink(newVersion, filepath.Join(keysDir, "..data")))
	for _, kid := range []string{"current", "next"} {
		require.NoError(t, os.Symlink(filepath.Join("..data", kid+".pem"), filepath.Join(keysDir, kid+".pem")))
	}

	keys, err := loadMatchmakingSigningKeys(keysDir, "current")
	require.NoError(t, err)
	require.Equal(t, expected["current"].N, keys["current"].N)
	require.Equal(t, expected["next"].N, keys["next"].N)
}

func TestLoadMatchmakingSigningKeysRejectsAtomicWriterTemporaryLinkOutsideDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Kubernetes projected Secret symlink layout is exercised on Linux")
	}
	keysDir, outsideDir := t.TempDir(), t.TempDir()
	currentVersion := "..2026_10_17_12_34_56.123456789"
	outsideVersion := "..2026_10_17_12_34_57.123456789"
	currentDir := filepath.Join(keysDir, currentVersion)
	outsideVersionDir := filepath.Join(outsideDir, outsideVersion)
	require.NoError(t, os.Mkdir(currentDir, 0o700))
	require.NoError(t, os.Mkdir(outsideVersionDir, 0o700))
	writePrincipalRotationKeys(t, currentDir)
	require.NoError(t, os.Symlink(currentVersion, filepath.Join(keysDir, "..data")))
	require.NoError(t, os.Symlink(filepath.Join(outsideDir, outsideVersion), filepath.Join(keysDir, "..data_tmp")))
	for _, kid := range []string{"current", "next"} {
		require.NoError(t, os.Symlink(filepath.Join("..data", kid+".pem"), filepath.Join(keysDir, kid+".pem")))
	}

	_, err := loadMatchmakingSigningKeys(keysDir, "current")
	require.ErrorContains(t, err, "resolves outside")
}

func TestLoadMatchmakingSigningKeysRejectsUnknownAtomicWriterEntry(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Kubernetes projected Secret symlink layout is exercised on Linux")
	}
	keysDir := t.TempDir()
	version := "..2026_10_17_12_34_56.123456789"
	versionDir := filepath.Join(keysDir, version)
	require.NoError(t, os.Mkdir(versionDir, 0o700))
	writePrincipalRotationKeys(t, versionDir)
	require.NoError(t, os.Symlink(version, filepath.Join(keysDir, "..data")))
	for _, kid := range []string{"current", "next"} {
		require.NoError(t, os.Symlink(filepath.Join("..data", kid+".pem"), filepath.Join(keysDir, kid+".pem")))
	}
	require.NoError(t, os.WriteFile(filepath.Join(keysDir, "unexpected.txt"), []byte("not a key"), 0o600))

	_, err := loadMatchmakingSigningKeys(keysDir, "current")
	require.ErrorContains(t, err, "only PEM rotation keys")
}

func TestLoadPrincipalJWKSListenerRejectsProjectedKeySymlinkOutsideDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink containment is exercised on Linux")
	}
	keysDir, outsideDir := t.TempDir(), t.TempDir()
	writePrincipalRotationKeys(t, keysDir)
	outsideKey := filepath.Join(outsideDir, "next.pem")
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(outsideKey, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600))
	require.NoError(t, os.Remove(filepath.Join(keysDir, "next.pem")))
	require.NoError(t, os.Symlink(outsideKey, filepath.Join(keysDir, "next.pem")))

	_, err = loadMatchmakingSigningKeys(keysDir, "current")
	require.ErrorContains(t, err, "resolves outside")
}

func writePrincipalRotationKeys(t *testing.T, directory string) {
	t.Helper()
	for _, kid := range []string{"current", "next"} {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		der, err := x509.MarshalPKCS8PrivateKey(key)
		require.NoError(t, err)
		path := filepath.Join(directory, kid+".pem")
		require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600))
	}
}

func writeJWKSListenerCertificate(t *testing.T) (string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	require.NoError(t, err)
	template := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "matchmaking"}, DNSNames: []string{"matchmaking"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, IsCA: true, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	certFile, keyFile := filepath.Join(t.TempDir(), "tls.crt"), filepath.Join(t.TempDir(), "tls.key")
	require.NoError(t, os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600))
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600))
	return certFile, keyFile
}
