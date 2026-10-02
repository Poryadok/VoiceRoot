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
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/federation/mediaauthority"
	"voice/backend/federation/protocol"
)

// The controlled HTTPS signer is only a protocol fixture. The separate process
// under test is the unmodified production node-authority executable, including
// its mTLS transport, complete-policy ACK and atomic file publication.
func startControllerProcess(t *testing.T, directory string, parameters fixtureParameters, publisher *projectionPublisher) func() time.Time {
	t.Helper()
	privateDirectory := t.TempDir()
	require.NoError(t, os.Chmod(filepath.Dir(privateDirectory), 0755))
	require.NoError(t, os.Chown(privateDirectory, 10001, 10001))
	require.NoError(t, os.Chmod(privateDirectory, 0750))
	require.NoError(t, os.Chown(publisher.directory, 10001, 10001))
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	now := time.Now()
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "owned-media-fixture-ca"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	require.NoError(t, err)
	ca, err = x509.ParseCertificate(caDER)
	require.NoError(t, err)
	leaf := func(serial int64, usage x509.ExtKeyUsage) (tls.Certificate, []byte, []byte) {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)
		cert := &x509.Certificate{SerialNumber: big.NewInt(serial), NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}}
		der, err := x509.CreateCertificate(rand.Reader, cert, ca, &key.PublicKey, caKey)
		require.NoError(t, err)
		keyDER, err := x509.MarshalPKCS8PrivateKey(key)
		require.NoError(t, err)
		certPEM, keyPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
		pair, err := tls.X509KeyPair(certPEM, keyPEM)
		require.NoError(t, err)
		return pair, certPEM, keyPEM
	}
	serverCertificate, _, _ := leaf(2, x509.ExtKeyUsageServerAuth)
	clientCertificate, clientPEM, clientKeyPEM := leaf(3, x509.ExtKeyUsageClientAuth)
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(caPEM))
	credentialBytes := make([]byte, 32)
	_, err = rand.Read(credentialBytes)
	require.NoError(t, err)
	credential := base64.RawURLEncoding.EncodeToString(credentialBytes)
	var mutex sync.Mutex
	bundles := map[string]mediaauthority.Bundle{}
	acks := map[string]int{}
	publisher.sink = func(space string, bundle mediaauthority.Bundle) error {
		mutex.Lock()
		defer mutex.Unlock()
		bundles[space] = bundle
		return nil
	}
	require.NoError(t, publisher.publish())
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.TLS == nil || len(request.TLS.PeerCertificates) != 1 || !bytes.Equal(request.TLS.PeerCertificates[0].Raw, clientCertificate.Certificate[0]) || request.Header.Get("Authorization") != "Bearer "+credential {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		prefix := "/v1/nodes/" + parameters.NodeID + "/spaces/"
		if !strings.HasPrefix(request.URL.Path, prefix) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		parts := strings.Split(strings.TrimPrefix(request.URL.Path, prefix), "/")
		mutex.Lock()
		defer mutex.Unlock()
		bundle, ok := bundles[parts[0]]
		if !ok || len(parts) < 2 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var envelope protocol.Envelope
		switch {
		case request.Method == http.MethodGet && len(parts) == 2 && parts[1] == "snapshot":
			envelope = bundle.Manifest
		case request.Method == http.MethodGet && len(parts) == 4 && parts[1] == "snapshot" && parts[2] == "pages":
			page, err := strconv.Atoi(parts[3])
			if err != nil || page < 0 || page >= len(bundle.Pages) {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			envelope = bundle.Pages[page]
		case request.Method == http.MethodPost && len(parts) == 2 && parts[1] == "lease":
			var ack protocol.AppliedRevisionAck
			decoder := json.NewDecoder(io.LimitReader(request.Body, 4096))
			decoder.DisallowUnknownFields()
			if decoder.Decode(&ack) != nil || decoder.Decode(new(any)) != io.EOF || uuid.Validate(ack.Nonce) != nil || len(ack.ReceiverBootNonces) == 0 || !protocol.ValidReceiverBootNonces(ack.ReceiverBootNonces) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			now := time.Now()
			claims, err := protocol.VerifyEnvelope(publisher.private.Public().(ed25519.PublicKey), bundle.Manifest, now)
			if err != nil || ack.Revision != claims.Revision || ack.Hash != claims.Hash {
				w.WriteHeader(http.StatusConflict)
				return
			}
			lease, err := protocol.VerifyEnvelope(publisher.private.Public().(ed25519.PublicKey), bundle.Lease, now)
			if err != nil {
				w.WriteHeader(http.StatusConflict)
				return
			}
			lease.IssuedAt = now.UnixMilli()
			lease.ReceiverBootNonces = slices.Clone(ack.ReceiverBootNonces)
			envelope, err = protocol.SignEnvelope(publisher.private, "fixture-1", lease)
			if err != nil {
				w.WriteHeader(http.StatusConflict)
				return
			}
			acks[parts[0]]++
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(envelope)
	}))
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{serverCertificate}, ClientCAs: roots, ClientAuth: tls.RequireAndVerifyClientCert}
	server.StartTLS()
	t.Cleanup(server.Close)
	write := func(name string, raw []byte) string {
		path := filepath.Join(privateDirectory, name)
		require.NoError(t, os.WriteFile(path, raw, 0640))
		require.NoError(t, os.Chown(path, 10001, 10001))
		return path
	}
	spaces := []string{publisher.grants[0][0].SpaceID, publisher.grants[1][0].SpaceID}
	config, err := json.Marshal(map[string]any{"master_url": server.URL, "trust_file": filepath.Join(directory, "trust.json"), "credential_file": write("credential", []byte(credential)), "ca_cert_file": write("ca.pem", caPEM), "client_cert_file": write("client.pem", clientPEM), "client_key_file": write("client-key.pem", clientKeyPEM), "authority_directory": publisher.directory, "boot_request_directory": os.Getenv("VOICE_SFU_BOOT_REQUEST_DIR"), "spaces": spaces, "interval_milliseconds": 100})
	require.NoError(t, err)
	log, err := os.Create(filepath.Join(privateDirectory, "controller.log"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = log.Close() })
	command := exec.Command("/usr/local/bin/node-authority", "--config", write("controller.json", config))
	command.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 10001, Gid: 10001}}
	command.Stdout, command.Stderr = log, log
	require.NoError(t, command.Start())
	stopped := false
	t.Cleanup(func() {
		if !stopped {
			_ = command.Process.Kill()
			_ = command.Wait()
		}
	})
	require.Eventually(t, func() bool {
		mutex.Lock()
		defer mutex.Unlock()
		for _, space := range spaces {
			raw, err := os.ReadFile(filepath.Join(publisher.directory, space+".json"))
			var bundle mediaauthority.Bundle
			if err != nil || json.Unmarshal(raw, &bundle) != nil || bundle.Scope.SpaceID != space || acks[space] == 0 {
				return false
			}
		}
		return true
	}, 5*time.Second, 20*time.Millisecond, "production controller must materialize both signed policies")
	return func() time.Time {
		require.False(t, stopped)
		killed := time.Now()
		require.NoError(t, command.Process.Kill())
		require.Error(t, command.Wait())
		stopped = true
		require.NotNil(t, command.ProcessState)
		files := map[string][]byte{}
		mutex.Lock()
		lastACKs := map[string]int{}
		for _, space := range spaces {
			lastACKs[space] = acks[space]
			files[space], err = os.ReadFile(filepath.Join(publisher.directory, space+".json"))
			require.NoError(t, err)
		}
		mutex.Unlock()
		t.Cleanup(func() {
			mutex.Lock()
			defer mutex.Unlock()
			for _, space := range spaces {
				raw, err := os.ReadFile(filepath.Join(publisher.directory, space+".json"))
				require.NoError(t, err)
				require.True(t, bytes.Equal(files[space], raw), "no authority file refresh after controller death")
				require.Equal(t, lastACKs[space], acks[space], "no policy ACK after controller death")
			}
		})
		t.Log("production controller process killed and reaped; controlled signer and SFU remain running")
		return killed
	}
}
