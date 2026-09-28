package s2s

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"voice/backend/messaging/internal/gameprotocol"
)

var (
	testMappingApplication = uuid.MustParse("10000000-0000-4000-8000-000000000001")
	testMappingEnvironment = uuid.MustParse("20000000-0000-4000-8000-000000000002")
	testMappingBinding     = uuid.MustParse("30000000-0000-4000-8000-000000000003")
	testMappingChat        = uuid.MustParse("40000000-0000-4000-8000-000000000004")
	testMappingKey         = []byte("0123456789abcdef0123456789abcdef")
)

func TestGameResourceMappingClientSignsExactTupleAndVerifiesExactGISResponse(t *testing.T) {
	var requestBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		requestBody = string(body)
		if r.Method != http.MethodPost || r.URL.EscapedPath() != gameResourceMappingAuthorizationPath {
			t.Errorf("request path/method = %s %s", r.Method, r.URL.EscapedPath())
		}
		if r.Header.Get("X-Voice-Workload") != "messaging" || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("workload/content-type headers = %v", r.Header)
		}
		timestamp, nonce := r.Header.Get("X-Voice-Timestamp"), r.Header.Get("X-Voice-Nonce")
		if _, err := uuid.Parse(nonce); err != nil || uuid.MustParse(nonce).String() != nonce {
			t.Errorf("nonce is not a canonical UUID: %q", nonce)
		}
		if got, want := r.Header.Get("X-Voice-Signature"), expectedWorkloadBodySignature(testMappingKey,
			http.MethodPost, gameResourceMappingAuthorizationPath, timestamp, nonce, body); got != want {
			t.Errorf("request signature = %q, want %q", got, want)
		}
		writeMappingResponse(w, http.StatusOK, `{"allowed":true,"mapping_revision":9}`+"\n", timestamp, nonce,
			expectedWorkloadResponseSignature(testMappingKey, http.StatusOK, gameResourceMappingAuthorizationPath,
				timestamp, nonce, []byte(`{"allowed":true,"mapping_revision":9}`+"\n")))
	}))
	defer server.Close()

	client := newTestResourceMappingClient(t, server, testMappingKey)
	if err := client.AuthorizeAppBindingChat(t.Context(), testMappingAuthority(), testMappingMessage()); err != nil {
		t.Fatalf("AuthorizeAppBindingChat() error = %v", err)
	}
	wantBody := `{"application_id":"10000000-0000-4000-8000-000000000001","environment_id":"20000000-0000-4000-8000-000000000002","binding_id":"30000000-0000-4000-8000-000000000003","chat_id":"40000000-0000-4000-8000-000000000004"}`
	if requestBody != wantBody {
		t.Fatalf("request body = %s, want %s", requestBody, wantBody)
	}
}

func TestGameResourceMappingClientFailsClosedOnDenyAndInvalidResponses(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		mutate func(http.Header, string, string)
	}{
		{name: "GIS deny", status: http.StatusOK, body: `{"allowed":false}` + "\n"},
		{name: "unknown response field", status: http.StatusOK, body: `{"allowed":true,"mapping_revision":9,"resource_id":"private"}` + "\n"},
		{name: "noncanonical response bytes", status: http.StatusOK, body: `{ "allowed":true,"mapping_revision":9}` + "\n"},
		{name: "zero mapping revision", status: http.StatusOK, body: `{"allowed":true,"mapping_revision":0}` + "\n"},
		{name: "response timestamp mismatch", status: http.StatusOK, body: `{"allowed":true,"mapping_revision":9}` + "\n",
			mutate: func(header http.Header, timestamp, nonce string) { header.Set("X-Voice-Response-Timestamp", "1") }},
		{name: "duplicate signature header", status: http.StatusOK, body: `{"allowed":true,"mapping_revision":9}` + "\n",
			mutate: func(header http.Header, timestamp, nonce string) {
				header.Add("X-Voice-Response-Signature", "duplicate")
			}},
		{name: "non-200 response", status: http.StatusServiceUnavailable, body: `{"allowed":true,"mapping_revision":9}` + "\n"},
		{name: "trailing JSON", status: http.StatusOK, body: `{"allowed":true,"mapping_revision":9} {}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				timestamp, nonce := r.Header.Get("X-Voice-Timestamp"), r.Header.Get("X-Voice-Nonce")
				signature := expectedWorkloadResponseSignature(testMappingKey, test.status,
					gameResourceMappingAuthorizationPath, timestamp, nonce, []byte(test.body))
				writeMappingResponse(w, test.status, test.body, timestamp, nonce, signature, test.mutate)
			}))
			defer server.Close()
			client := newTestResourceMappingClient(t, server, testMappingKey)
			if err := client.AuthorizeAppBindingChat(t.Context(), testMappingAuthority(), testMappingMessage()); err == nil {
				t.Fatal("AuthorizeAppBindingChat() unexpectedly allowed invalid mapping response")
			}
		})
	}
}

func TestGameResourceMappingClientRejectsMismatchedAuthAndMessageTupleBeforeNetwork(t *testing.T) {
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits++ }))
	defer server.Close()
	client := newTestResourceMappingClient(t, server, testMappingKey)
	authority := testMappingAuthority()
	message := testMappingMessage()
	message.ApplicationID = uuid.New()
	if err := client.AuthorizeAppBindingChat(t.Context(), authority, message); err == nil {
		t.Fatal("mismatched app authority was allowed")
	}
	if hits != 0 {
		t.Fatalf("mismatched tuple reached GIS: %d requests", hits)
	}
}

func TestNewGameResourceMappingClientRejectsIncompleteOrUnpinnedConfiguration(t *testing.T) {
	for _, config := range []GameResourceMappingAuthorizationConfig{
		{},
		{Endpoint: "http://game-integration:8444" + gameResourceMappingAuthorizationPath},
		{Endpoint: "https://game-integration:8444/wrong", TLSCertFile: "cert", TLSKeyFile: "key", CAFile: "ca", WorkloadKeyBase64: base64.StdEncoding.EncodeToString(testMappingKey)},
		{Endpoint: "https://user:password@game-integration:8444" + gameResourceMappingAuthorizationPath, TLSCertFile: "cert", TLSKeyFile: "key", CAFile: "ca", WorkloadKeyBase64: base64.StdEncoding.EncodeToString(testMappingKey)},
		{Endpoint: "https://game-integration:8444" + gameResourceMappingAuthorizationPath, TLSCertFile: "cert", TLSKeyFile: "key", CAFile: "ca"},
	} {
		if _, err := NewGameResourceMappingAuthorizationClient(config); err == nil {
			t.Fatalf("NewGameResourceMappingAuthorizationClient(%+v) unexpectedly succeeded", config)
		}
	}
}

func TestGameResourceMappingClientUsesPinnedHTTPSAndMessagingMTLSIdentity(t *testing.T) {
	ca, serverCert, clientCert := createMappingTestCertificates(t, messagingServiceURISAN)
	clientCAs := x509.NewCertPool()
	clientCAs.AddCert(ca)
	var sawVerifiedMessagingIdentity bool
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawVerifiedMessagingIdentity = r.TLS != nil && len(r.TLS.VerifiedChains) > 0 &&
			len(r.TLS.PeerCertificates) == 1 && containsMessagingURISAN(r.TLS.PeerCertificates[0])
		timestamp, nonce := r.Header.Get("X-Voice-Timestamp"), r.Header.Get("X-Voice-Nonce")
		body, _ := io.ReadAll(r.Body)
		if got, want := r.Header.Get("X-Voice-Signature"), expectedWorkloadBodySignature(testMappingKey,
			http.MethodPost, gameResourceMappingAuthorizationPath, timestamp, nonce, body); got != want {
			t.Errorf("mTLS request signature = %s, want %s", got, want)
		}
		responseBody := `{"allowed":true,"mapping_revision":9}` + "\n"
		writeMappingResponse(w, http.StatusOK, responseBody, timestamp, nonce,
			expectedWorkloadResponseSignature(testMappingKey, http.StatusOK, gameResourceMappingAuthorizationPath,
				timestamp, nonce, []byte(responseBody)))
	}))
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{serverCert},
		ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clientCAs}
	server.StartTLS()
	defer server.Close()
	endpoint, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	endpoint.Host = "localhost:" + strings.Split(server.Listener.Addr().String(), ":")[1]
	endpoint.Path = gameResourceMappingAuthorizationPath
	caFile := writeMappingPEM(t, "ca.pem", "CERTIFICATE", ca.Raw)
	clientCertFile := writeMappingPEM(t, "client.pem", "CERTIFICATE", clientCert.Certificate[0])
	clientKeyFile := writeMappingPEM(t, "client-key.pem", "EC PRIVATE KEY", mustMappingECPrivateKey(t, clientCert))
	client, err := NewGameResourceMappingAuthorizationClient(GameResourceMappingAuthorizationConfig{
		Endpoint: endpoint.String(), TLSCertFile: clientCertFile, TLSKeyFile: clientKeyFile,
		CAFile: caFile, WorkloadKeyBase64: base64.StdEncoding.EncodeToString(testMappingKey),
	})
	if err != nil {
		t.Fatalf("NewGameResourceMappingAuthorizationClient() error = %v", err)
	}
	defer client.Close()
	transport := client.client.Transport.(*http.Transport)
	if transport.TLSClientConfig.ServerName != "localhost" || client.client.Timeout != 2*time.Second {
		t.Fatalf("TLS name/timeout = %q/%s", transport.TLSClientConfig.ServerName, client.client.Timeout)
	}
	if err := client.AuthorizeAppBindingChat(t.Context(), testMappingAuthority(), testMappingMessage()); err != nil {
		t.Fatalf("AuthorizeAppBindingChat() with verified GIS TLS failed: %v", err)
	}
	if !sawVerifiedMessagingIdentity {
		t.Fatal("GIS did not receive the verified Messaging client URI SAN over mTLS")
	}

	wrongNameEndpoint := *endpoint
	wrongNameEndpoint.Host = "127.0.0.1:" + strings.Split(server.Listener.Addr().String(), ":")[1]
	wrongNameClient, err := NewGameResourceMappingAuthorizationClient(GameResourceMappingAuthorizationConfig{
		Endpoint: wrongNameEndpoint.String(), TLSCertFile: clientCertFile, TLSKeyFile: clientKeyFile,
		CAFile: caFile, WorkloadKeyBase64: base64.StdEncoding.EncodeToString(testMappingKey),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer wrongNameClient.Close()
	if err := wrongNameClient.AuthorizeAppBindingChat(t.Context(), testMappingAuthority(), testMappingMessage()); err == nil {
		t.Fatal("GIS certificate with a nonmatching endpoint name was trusted")
	}
}

func TestNewGameResourceMappingClientRequiresMessagingURIIdentity(t *testing.T) {
	ca, _, wrongClientCert := createMappingTestCertificates(t, "spiffe://voice/service/auth")
	caFile := writeMappingPEM(t, "wrong-ca.pem", "CERTIFICATE", ca.Raw)
	certFile := writeMappingPEM(t, "wrong-client.pem", "CERTIFICATE", wrongClientCert.Certificate[0])
	keyFile := writeMappingPEM(t, "wrong-client-key.pem", "EC PRIVATE KEY", mustMappingECPrivateKey(t, wrongClientCert))
	_, err := NewGameResourceMappingAuthorizationClient(GameResourceMappingAuthorizationConfig{
		Endpoint:    "https://localhost:8444" + gameResourceMappingAuthorizationPath,
		TLSCertFile: certFile, TLSKeyFile: keyFile, CAFile: caFile,
		WorkloadKeyBase64: base64.StdEncoding.EncodeToString(testMappingKey),
	})
	if err == nil {
		t.Fatal("client certificate without the Messaging URI SAN was accepted")
	}
}

func newTestResourceMappingClient(t *testing.T, server *httptest.Server, key []byte) *GameResourceMappingAuthorizationClient {
	t.Helper()
	endpoint, err := url.Parse(server.URL + gameResourceMappingAuthorizationPath)
	if err != nil {
		t.Fatal(err)
	}
	return &GameResourceMappingAuthorizationClient{
		endpoint:    endpoint,
		client:      server.Client(),
		workloadKey: key,
		now:         func() time.Time { return time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC) },
	}
}

func testMappingAuthority() gameprotocol.DeviceAuthority {
	return gameprotocol.DeviceAuthority{ApplicationID: testMappingApplication, EnvironmentID: testMappingEnvironment, BindingID: testMappingBinding}
}

func testMappingMessage() gameprotocol.Message {
	return gameprotocol.Message{ApplicationID: testMappingApplication, EnvironmentID: testMappingEnvironment,
		BindingID: testMappingBinding, ChatID: testMappingChat}
}

func writeMappingResponse(w http.ResponseWriter, status int, body, timestamp, nonce, signature string,
	mutations ...func(http.Header, string, string)) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Voice-Response-Timestamp", timestamp)
	w.Header().Set("X-Voice-Response-Nonce", nonce)
	w.Header().Set("X-Voice-Response-Signature", signature)
	for _, mutate := range mutations {
		if mutate != nil {
			mutate(w.Header(), timestamp, nonce)
		}
	}
	w.WriteHeader(status)
	_, _ = io.Copy(w, strings.NewReader(body))
}

func expectedWorkloadBodySignature(key []byte, method, path, timestamp, nonce string, body []byte) string {
	digest := sha256.Sum256(body)
	message := "v1\n" + method + "\n" + path + "\n" + timestamp + "\n" + nonce + "\n" + hex.EncodeToString(digest[:])
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(message))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func expectedWorkloadResponseSignature(key []byte, status int, path, timestamp, nonce string, body []byte) string {
	digest := sha256.Sum256(body)
	message := "v1\n" + fmt.Sprint(status) + "\n" + path + "\n" + timestamp + "\n" + nonce + "\n" + hex.EncodeToString(digest[:])
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(message))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func createMappingTestCertificates(t *testing.T, clientSAN string) (*x509.Certificate, tls.Certificate, tls.Certificate) {
	t.Helper()
	rootKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Add(-time.Minute)
	rootTemplate := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "mapping test CA"},
		NotBefore: now, NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	rootDER, err := x509.CreateCertificate(rand.Reader, rootTemplate, rootTemplate, &rootKey.PublicKey, rootKey)
	if err != nil {
		t.Fatal(err)
	}
	rootCert, err := x509.ParseCertificate(rootDER)
	if err != nil {
		t.Fatal(err)
	}
	makeLeaf := func(serial int64, commonName string, server bool, uri string) tls.Certificate {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		usage := []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
		dnsNames := []string(nil)
		if server {
			usage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
			dnsNames = []string{"localhost"}
		}
		template := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: commonName},
			NotBefore: now, NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature,
			ExtKeyUsage: usage, DNSNames: dnsNames, BasicConstraintsValid: true}
		if uri != "" {
			parsed, err := url.Parse(uri)
			if err != nil {
				t.Fatal(err)
			}
			template.URIs = []*url.URL{parsed}
		}
		der, err := x509.CreateCertificate(rand.Reader, template, rootCert, &key.PublicKey, rootKey)
		if err != nil {
			t.Fatal(err)
		}
		certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
		keyDER, err := x509.MarshalPKCS8PrivateKey(key)
		if err != nil {
			t.Fatal(err)
		}
		keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
		certificate, err := tls.X509KeyPair(certPEM, keyPEM)
		if err != nil {
			t.Fatal(err)
		}
		return certificate
	}
	return rootCert, makeLeaf(2, "localhost", true, ""), makeLeaf(3, "messaging", false, clientSAN)
}

func writeMappingPEM(t *testing.T, name, blockType string, der []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	block := pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der})
	if err := os.WriteFile(path, block, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func mustMappingECPrivateKey(t *testing.T, certificate tls.Certificate) []byte {
	t.Helper()
	key, ok := certificate.PrivateKey.(*ecdsa.PrivateKey)
	if !ok {
		t.Fatal("test certificate private key is not ECDSA")
	}
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return der
}
