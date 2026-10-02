//go:build linux

package federationmedia

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"syscall"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/federation/mediaauthority"
	"voice/backend/federation/protocol"
)

func startNodeMediaProcess(t *testing.T, directory string, parameters fixtureParameters, publisher *projectionPublisher) func(mediaauthority.Grant, time.Duration) (string, int) {
	exchange, _ := startRestartableNodeMediaProcess(t, directory, parameters, publisher)
	return exchange
}

func startRestartableNodeMediaProcess(t *testing.T, directory string, parameters fixtureParameters, publisher *projectionPublisher) (func(mediaauthority.Grant, time.Duration) (string, int), func()) {
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
	config, err := json.Marshal(map[string]string{"listen_address": address, "trust_file": filepath.Join(directory, "trust.json"), "authority_directory": publisher.directory, "boot_request_directory": os.Getenv("VOICE_SFU_BOOT_REQUEST_DIR"), "credentials_file": write("livekit.json", creds), "tls_cert_file": write("tls.crt", certPEM), "tls_key_file": write("tls.key", pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER})), "media_url": "wss://fixture-sfu.invalid"})
	require.NoError(t, err)
	log, err := os.Create(filepath.Join(privateDirectory, "node-media.log"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = log.Close() })
	configPath := write("media.json", config)
	var command *exec.Cmd
	start := func() {
		command = exec.Command("/usr/local/bin/node-media", "--config", configPath)
		command.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 10001, Gid: 10001}}
		command.Stdout, command.Stderr = log, log
		require.NoError(t, command.Start())
	}
	stop := func() {
		require.NoError(t, command.Process.Kill())
		require.Error(t, command.Wait())
		require.NotNil(t, command.ProcessState)
	}
	start()
	t.Cleanup(stop)
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
	exchange := func(grant mediaauthority.Grant, validity time.Duration) (string, int) {
		now := time.Now()
		grant.IssuedAt = now.UnixMilli()
		grant.ExpiresAt = now.Add(validity).UnixMilli()
		grant.Nonce = uuid.NewString()
		credential, err := mediaauthority.Sign(publisher.private, "fixture-1", grant, now)
		require.NoError(t, err)
		return call(mediaauthority.ExchangeRequest{Credential: credential, RoomName: grant.RoomName, ProfileID: grant.ProfileID})
	}
	restart := func() {
		transport.CloseIdleConnections()
		stop()
		start()
		require.Eventually(t, func() bool {
			_, status := call(mediaauthority.ExchangeRequest{})
			return status == http.StatusForbidden
		}, time.Second, 10*time.Millisecond, "restarted production media edge must start")
	}
	return exchange, restart
}

func TestNodeMediaRestartRequiresFreshBootLease_live(t *testing.T) {
	directory := os.Getenv("VOICE_SFU_FIXTURE_DIR")
	if directory == "" || os.Getenv("VOICE_SFU_BOOT_REQUEST_DIR") == "" {
		t.Skip("owned Linux media fixture required")
	}
	var parameters fixtureParameters
	raw, err := os.ReadFile(filepath.Join(directory, "parameters.json"))
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &parameters))
	private, err := base64.RawURLEncoding.DecodeString(parameters.PrivateKey)
	require.NoError(t, err)
	publisher := &projectionPublisher{private: private, directory: filepath.Join(directory, "authority"), allow: [2]bool{true, true}}
	for space := range publisher.grants {
		publisher.grants[space] = []mediaauthority.Grant{{Version: 1, Issuer: "fixture-master", Audience: "voice-node-media", Environment: "sandbox", NodeID: parameters.NodeID,
			SpaceID: uuid.NewString(), Generation: 1, AuthorityEpoch: 1, AccountID: uuid.NewString(), ProfileID: uuid.NewString(), ResourceID: uuid.NewString(), SessionEpoch: 1,
			RoomName: "restart-" + uuid.NewString(), CanPublish: true, RoutingGeneration: 1, Nonce: uuid.NewString()}}
	}
	exchange, restart := startRestartableNodeMediaProcess(t, directory, parameters, publisher)
	require.NoError(t, publisher.publish())
	grant := publisher.grants[0][0]
	require.Eventually(t, func() bool {
		_, status := exchange(grant, 30*time.Second)
		return status == http.StatusOK
	}, time.Second, 10*time.Millisecond)
	// Refresh immediately before restart so expiry cannot explain the denial.
	require.NoError(t, publisher.publish())
	path := filepath.Join(publisher.directory, grant.SpaceID+".json")
	before, err := os.ReadFile(path)
	require.NoError(t, err)
	var saved mediaauthority.Bundle
	require.NoError(t, json.Unmarshal(before, &saved))
	restart()
	_, status := exchange(grant, 30*time.Second)
	require.Equal(t, http.StatusForbidden, status, "saved still-valid authority cannot admit a new process")
	lease, err := protocol.VerifyEnvelope(ed25519.PrivateKey(private).Public().(ed25519.PublicKey), saved.Lease, time.Now())
	require.NoError(t, err, "old signed lease must remain valid at the actual denied exchange")
	after, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, before, after, "restart must not rewrite saved signed evidence")
	nonces, err := mediaauthority.ActiveBootNonces(os.Getenv("VOICE_SFU_BOOT_REQUEST_DIR"), grant.Issuer, grant.Environment, grant.NodeID, time.Now())
	require.NoError(t, err)
	require.True(t, slices.ContainsFunc(nonces, func(nonce string) bool { return !slices.Contains(lease.ReceiverBootNonces, nonce) }), "restarted receiver must request a fresh process UUID")
	require.NoError(t, publisher.publish())
	require.Eventually(t, func() bool {
		_, status := exchange(grant, 30*time.Second)
		return status == http.StatusOK
	}, time.Second, 10*time.Millisecond, "fresh signed lease for current boot must restore admission")
	t.Log("production media edge restarted; still-valid saved lease denied; fresh boot lease admitted")
}
