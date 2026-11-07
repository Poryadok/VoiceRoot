package sourcecustody

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// This fixture isolates wire authentication. Actual source/control health is
// covered by the separate production-process PostgreSQL tests.
func TestSupervisorTransportRequiresPinnedMutualTLSAndIndependentSigner(t *testing.T) {
	dir := t.TempDir()
	pki := processTLS(t, dir, "supervisor-auth")
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	custodian, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	publicDER, err := x509.MarshalPKIXPublicKey(public)
	require.NoError(t, err)
	privateDER, err := x509.MarshalPKCS8PrivateKey(pki.client.PrivateKey)
	require.NoError(t, err)
	publicPath, clientPath, keyPath := filepath.Join(dir, "monitor-public.pem"), filepath.Join(dir, "controller.pem"), filepath.Join(dir, "controller.key")
	require.NoError(t, os.WriteFile(publicPath, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}), 0600))
	require.NoError(t, os.WriteFile(clientPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: pki.client.Certificate[0]}), 0600))
	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER}), 0600))
	certificate, err := tls.LoadX509KeyPair(pki.serverCert, pki.serverKey)
	require.NoError(t, err)
	incarnation := uuid.New()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var probe supervisionProbe
		if json.NewDecoder(r.Body).Decode(&probe) != nil || !validProbe(probe) {
			http.Error(w, "unavailable", 400)
			return
		}
		response, err := SignMonitorResponse(MonitorResponse{Challenge: probe.Challenge, DecisionDigest: probe.DecisionDigest, Incarnation: incarnation, Sequence: probe.Sequence, Nonce: probe.Nonce, Kind: MonitorArmed}, private)
		if err != nil {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(w).Encode(response)
	})
	server := httptest.NewUnstartedServer(handler)
	pin := sha256.Sum256(pki.client.Certificate[0])
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{certificate}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pki.roots, VerifyConnection: func(state tls.ConnectionState) error {
		if !validOperatorPeer(state, pin) {
			return ErrUnavailable
		}
		return nil
	}}
	server.StartTLS()
	defer server.Close()
	config := &SupervisorClientConfig{URL: server.URL, CAFile: pki.caFile, CertFile: clientPath, KeyFile: keyPath, PublicKeyFile: publicPath, ServerCertSHA256: sha256.Sum256(certificate.Certificate[0])}
	client, err := newSupervisorClient(config, custodian)
	require.NoError(t, err)
	defer client.client.CloseIdleConnections()
	probe := supervisionProbe{Challenge: [32]byte{1}, DecisionDigest: [32]byte{2}, Sequence: 1, Nonce: [32]byte{3}}
	response, err := client.probe(context.Background(), probe)
	require.NoError(t, err)
	require.NoError(t, VerifyMonitorResponse(response, public))
	wrong := *config
	wrong.ServerCertSHA256[0] ^= 1
	rejected, err := newSupervisorClient(&wrong, custodian)
	require.NoError(t, err)
	defer rejected.client.CloseIdleConnections()
	_, err = rejected.probe(context.Background(), probe)
	require.Error(t, err, "ordinary CA trust cannot replace the leaf pin")
	unauthenticated := &http.Client{Timeout: time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: pki.roots}}}
	defer unauthenticated.CloseIdleConnections()
	body, err := json.Marshal(probe)
	require.NoError(t, err)
	_, err = unauthenticated.Post(server.URL, "application/json", bytes.NewReader(body))
	require.Error(t, err, "the monitor must require the controller client certificate")
	_, err = newSupervisorClient(config, public)
	require.Error(t, err, "the custodian key cannot also be the monitor signing key")
}

func TestSupervisorConfigRequiresCompletePinnedScope(t *testing.T) {
	values := map[string]string{}
	names := []string{"CUSTODY_SUPERVISOR_LISTEN", "CUSTODY_SUPERVISOR_TLS_CERT_FILE", "CUSTODY_SUPERVISOR_TLS_KEY_FILE", "CUSTODY_SUPERVISOR_CLIENT_CA_FILE", "CUSTODY_SUPERVISOR_DECISION_FILE", "CUSTODY_SUPERVISOR_SIGNER_PRIVATE_KEY_FILE", "CUSTODY_SUPERVISOR_CUSTODIAN_PUBLIC_KEY_FILE", "CUSTODY_SUPERVISOR_CONTROL_DATABASE_URL", "CUSTODY_SUPERVISOR_SOURCE_ADMIN_DATABASE_URL", "CUSTODY_SUPERVISOR_CLIENT_CERT_SHA256"}
	for _, name := range names {
		values[name] = "fixture"
	}
	values[names[0]] = "127.0.0.1:4445"
	values[names[7]] = "postgres://custodian:fixture@localhost:5432/control?sslmode=verify-full&sslrootcert=fixture"
	values[names[8]] = "postgres://custodian:fixture@localhost:5433/postgres?sslmode=verify-full&sslrootcert=fixture"
	values[names[9]] = strings.Repeat("ab", 32)
	lookup := func(name string) (string, bool) { value, ok := values[name]; return value, ok }
	_, err := LoadSupervisorConfig(lookup)
	require.NoError(t, err)
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			saved := values[name]
			delete(values, name)
			_, err := LoadSupervisorConfig(lookup)
			require.Error(t, err)
			values[name] = saved
		})
	}
	values[names[8]] = "postgres://custodian:fixture@localhost:5433/postgres?sslmode=disable"
	_, err = LoadSupervisorConfig(lookup)
	require.Error(t, err)
}
