package gameprincipal

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadServerTLS_RequiresVerifiedGISClientCertificate(t *testing.T) {
	dir := t.TempDir()
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	caTemplate := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "GIS test CA"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	require.NoError(t, err)
	caPath := filepath.Join(dir, "client-ca.pem")
	require.NoError(t, os.WriteFile(caPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), 0600))
	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	serverTemplate := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "voice"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), DNSNames: []string{"localhost"}, KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	serverDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, caTemplate, &serverKey.PublicKey, caKey)
	require.NoError(t, err)
	certPath := filepath.Join(dir, "server.pem")
	keyPath := filepath.Join(dir, "server-key.pem")
	require.NoError(t, os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverDER}), 0600))
	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(serverKey)}), 0600))

	serverTLS, err := loadServerTLS(Config{TLSCertFile: certPath, TLSKeyFile: keyPath, ClientCAFile: caPath})
	require.NoError(t, err)
	require.Equal(t, tls.RequireAndVerifyClientCert, serverTLS.ClientAuth)
	require.NotNil(t, serverTLS.ClientCAs)
	require.GreaterOrEqual(t, serverTLS.MinVersion, uint16(tls.VersionTLS12))
	_, err = loadServerTLS(Config{TLSCertFile: certPath, TLSKeyFile: keyPath, ClientCAFile: filepath.Join(dir, "missing.pem")})
	require.Error(t, err, "missing GIS client CA must prevent private listener startup")
}
