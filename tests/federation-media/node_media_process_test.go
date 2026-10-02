//go:build linux

package federationmedia

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/federation/mediaauthority"
)

func startNodeMediaProcess(t *testing.T, directory string, parameters fixtureParameters, publisher *projectionPublisher) func(mediaauthority.Grant, time.Duration) (string, int) {
	t.Helper()
	privateDirectory := t.TempDir()
	require.NoError(t, os.Chmod(filepath.Dir(privateDirectory), 0755))
	require.NoError(t, os.Chown(privateDirectory, 10001, 10001))
	require.NoError(t, os.Chmod(privateDirectory, 0750))
	write := func(name string, raw []byte) string {
		path := filepath.Join(privateDirectory, name)
		require.NoError(t, os.WriteFile(path, raw, 0640))
		require.NoError(t, os.Chown(path, 10001, 10001))
		return path
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	now := time.Now()
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "owned-node-media-edge"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	require.NoError(t, err)
	privateDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().String()
	require.NoError(t, listener.Close())
	creds, err := json.Marshal(map[string]string{"api_key": parameters.APIKey, "api_secret": parameters.APISecret})
	require.NoError(t, err)
	config, err := json.Marshal(map[string]string{"listen_address": address, "trust_file": filepath.Join(directory, "trust.json"), "authority_directory": publisher.directory, "credentials_file": write("livekit.json", creds), "tls_cert_file": write("tls.crt", certPEM), "tls_key_file": write("tls.key", pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER})), "media_url": "wss://fixture-sfu.invalid"})
	require.NoError(t, err)
	log, err := os.Create(filepath.Join(privateDirectory, "node-media.log"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = log.Close() })
	command := exec.Command("/usr/local/bin/node-media", "--config", write("media.json", config))
	command.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 10001, Gid: 10001}}
	command.Stdout, command.Stderr = log, log
	require.NoError(t, command.Start())
	t.Cleanup(func() { _ = command.Process.Kill(); _ = command.Wait() })
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(certPEM))
	transport := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS13}}
	t.Cleanup(transport.CloseIdleConnections)
	client := &http.Client{Transport: transport, Timeout: 2 * time.Second}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	endpoint := "https://" + address + "/v1/media/token"
	call := func(input mediaauthority.ExchangeRequest) (string, int) {
		raw, e := json.Marshal(input)
		require.NoError(t, e)
		response, e := client.Post(endpoint, "application/json", bytes.NewReader(raw))
		if e != nil {
			return "", 0
		}
		defer response.Body.Close()
		require.Equal(t, "no-store", response.Header.Get("Cache-Control"))
		if response.StatusCode != http.StatusOK {
			_, _ = io.Copy(io.Discard, response.Body)
			return "", response.StatusCode
		}
		var result mediaauthority.ExchangeResult
		d := json.NewDecoder(io.LimitReader(response.Body, 65536))
		d.DisallowUnknownFields()
		require.NoError(t, d.Decode(&result))
		require.Equal(t, "wss://fixture-sfu.invalid", result.LivekitURL)
		// Only the fixture's internal media transport is substituted. The actual
		// HTTPS exchange, production JWT, private claim and SFU verification are used.
		return result.JWT, response.StatusCode
	}
	require.Eventually(t, func() bool {
		_, status := call(mediaauthority.ExchangeRequest{})
		return status == http.StatusForbidden
	}, 5*time.Second, 20*time.Millisecond, "production node media HTTPS edge must start")
	return func(grant mediaauthority.Grant, validity time.Duration) (string, int) {
		now := time.Now()
		grant.IssuedAt = now.UnixMilli()
		grant.ExpiresAt = now.Add(validity).UnixMilli()
		grant.Nonce = uuid.NewString()
		credential, err := mediaauthority.Sign(publisher.private, "fixture-1", grant, now)
		require.NoError(t, err)
		return call(mediaauthority.ExchangeRequest{Credential: credential, RoomName: grant.RoomName, ProfileID: grant.ProfileID})
	}
}
