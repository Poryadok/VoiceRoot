package authbinding

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
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

func TestClientClaimAndCompletionUseMTLSPrivateRoutes(t *testing.T) {
	files := writeTLSFiles(t)
	operation := uuid.New()
	claim := ClaimReceipt{ClaimID: uuid.New(), OperationID: operation, AssertionJTI: uuid.New(),
		ExpiresAt: time.Now().Add(3 * time.Second).UTC().Truncate(time.Millisecond)}
	completion := CompletionReceipt{ClaimID: claim.ClaimID, OperationID: operation, Outcome: "succeeded",
		BindingID: uuid.New(), Status: "completed"}
	revocation := RevokeReceipt{OperationID: operation, Status: "revoking", AuthorityRevision: 3}
	challenge := uuid.New()
	handoff := ExchangeResponse{HandoffJWS: "header.payload.signature"}
	server := newMTLSServer(t, files, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.Equal(t, "application/json", r.Header.Get("Accept"))
		require.NotNil(t, r.TLS)
		require.Len(t, r.TLS.PeerCertificates, 1)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		switch r.URL.Path {
		case exchangePath:
			var got ExchangeRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
			require.Equal(t, ExchangeRequest{ChallengeID: challenge, OperationID: operation,
				Code: strings.Repeat("c", 43), CodeVerifier: strings.Repeat("v", 43), DeviceProof: "device-proof"}, got)
			_ = json.NewEncoder(w).Encode(handoff)
		case claimPath:
			var got ClaimRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
			require.Equal(t, operation, got.OperationID)
			require.Equal(t, strings.Repeat("a", 64), got.RequestSHA256)
			require.Equal(t, "signed-handoff", got.HandoffJWS)
			require.Equal(t, "device-proof", got.DeviceProof)
			_ = json.NewEncoder(w).Encode(claim)
		case completionPath:
			var got CompletionRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
			require.Equal(t, CompletionRequest{ClaimID: claim.ClaimID, OperationID: operation,
				Outcome: "succeeded", BindingID: completion.BindingID}, got)
			_ = json.NewEncoder(w).Encode(completion)
		case revokePath:
			var got RevokeRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
			require.Equal(t, RevokeRequest{OperationID: operation}, got)
			_ = json.NewEncoder(w).Encode(revocation)
		default:
			http.NotFound(w, r)
		}
	})
	defer server.Close()

	client, err := NewClient(server.URL, files.ca, files.cert, files.key)
	require.NoError(t, err)
	gotHandoff, err := client.Exchange(context.Background(), ExchangeRequest{ChallengeID: challenge, OperationID: operation,
		Code: strings.Repeat("c", 43), CodeVerifier: strings.Repeat("v", 43), DeviceProof: "device-proof"})
	require.NoError(t, err)
	require.Equal(t, handoff, gotHandoff)
	gotClaim, err := client.Claim(context.Background(), ClaimRequest{OperationID: operation,
		RequestSHA256: strings.Repeat("a", 64), HandoffJWS: "signed-handoff", DeviceProof: "device-proof"})
	require.NoError(t, err)
	require.Equal(t, claim, gotClaim)
	gotCompletion, err := client.Complete(context.Background(), CompletionRequest{ClaimID: claim.ClaimID,
		OperationID: operation, Outcome: "succeeded", BindingID: completion.BindingID})
	require.NoError(t, err)
	require.Equal(t, completion, gotCompletion)
	gotRevocation, err := client.Revoke(context.Background(), operation)
	require.NoError(t, err)
	require.Equal(t, revocation, gotRevocation)
}

func TestClientFailsClosedForMissingCredentialsRedirectAndBadReceipt(t *testing.T) {
	_, err := NewClient("https://auth.example", "", "", "")
	require.ErrorIs(t, err, ErrUnavailable)
	files := writeTLSFiles(t)
	redirect := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://example.invalid", http.StatusFound)
	}))
	defer redirect.Close()
	client, err := NewClient(redirect.URL, files.ca, files.cert, files.key)
	require.NoError(t, err)
	_, err = client.Claim(context.Background(), ClaimRequest{OperationID: uuid.New(), RequestSHA256: strings.Repeat("a", 64),
		HandoffJWS: "signed-handoff", DeviceProof: "device-proof"})
	require.ErrorIs(t, err, ErrUnavailable)

	badReceipt := newMTLSServer(t, files, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(`{"claim_id":"not-a-uuid","operation_id":"00000000-0000-0000-0000-000000000000","assertion_jti":"00000000-0000-0000-0000-000000000000","expires_at":"2026-09-28T00:00:00Z"}`))
	})
	defer badReceipt.Close()
	client, err = NewClient(badReceipt.URL, files.ca, files.cert, files.key)
	require.NoError(t, err)
	_, err = client.Claim(context.Background(), ClaimRequest{OperationID: uuid.New(), RequestSHA256: strings.Repeat("a", 64),
		HandoffJWS: "signed-handoff", DeviceProof: "device-proof"})
	require.ErrorIs(t, err, ErrUnavailable)
}

func TestFailedCompletionOmitsBindingIDFromStrictAuthRequest(t *testing.T) {
	body, err := json.Marshal(CompletionRequest{ClaimID: uuid.New(), OperationID: uuid.New(), Outcome: "failed"})
	require.NoError(t, err)
	require.NotContains(t, string(body), "binding_id")
	require.Contains(t, string(body), `"outcome":"failed"`)
}

type tlsFiles struct{ ca, cert, key, serverCert, serverKey string }

func writeTLSFiles(t *testing.T) tlsFiles {
	t.Helper()
	dir := t.TempDir()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	caTemplate := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test CA"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true,
		BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	require.NoError(t, err)
	ca := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	makeLeaf := func(serial int64, server bool) ([]byte, []byte) {
		key, keyErr := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, keyErr)
		usage := []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
		template := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: "test peer"},
			NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature,
			ExtKeyUsage: usage}
		if server {
			template.Subject.CommonName = "localhost"
			template.DNSNames = []string{"localhost"}
			template.IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}
			template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
		}
		der, certErr := x509.CreateCertificate(rand.Reader, template, caTemplate, &key.PublicKey, caKey)
		require.NoError(t, certErr)
		privateDER, keyErr := x509.MarshalPKCS8PrivateKey(key)
		require.NoError(t, keyErr)
		return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
			pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER})
	}
	clientCert, clientKey := makeLeaf(2, false)
	serverCert, serverKey := makeLeaf(3, true)
	certPath, keyPath, caPath := filepath.Join(dir, "client.pem"), filepath.Join(dir, "client-key.pem"), filepath.Join(dir, "ca.pem")
	serverCertPath, serverKeyPath := filepath.Join(dir, "server.pem"), filepath.Join(dir, "server-key.pem")
	require.NoError(t, os.WriteFile(certPath, clientCert, 0600))
	require.NoError(t, os.WriteFile(keyPath, clientKey, 0600))
	require.NoError(t, os.WriteFile(caPath, ca, 0600))
	require.NoError(t, os.WriteFile(serverCertPath, serverCert, 0600))
	require.NoError(t, os.WriteFile(serverKeyPath, serverKey, 0600))
	return tlsFiles{ca: caPath, cert: certPath, key: keyPath, serverCert: serverCertPath, serverKey: serverKeyPath}
}

func newMTLSServer(t *testing.T, files tlsFiles, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	caPEM, err := os.ReadFile(files.ca)
	require.NoError(t, err)
	clientCAs := x509.NewCertPool()
	require.True(t, clientCAs.AppendCertsFromPEM(caPEM))
	serverCertificate, err := tls.LoadX509KeyPair(files.serverCert, files.serverKey)
	require.NoError(t, err)
	server := httptest.NewUnstartedServer(handler)
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS12, ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs: clientCAs, Certificates: []tls.Certificate{serverCertificate}}
	server.StartTLS()
	return server
}
