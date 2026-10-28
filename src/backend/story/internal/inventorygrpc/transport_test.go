package inventorygrpc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
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

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	commonv1 "voice.app/voice/common/v1"
	storyv1 "voice.app/voice/story/v1"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/referenceinventory"
)

func TestInventoryConfigCompleteOrAbsent(t *testing.T) {
	cfg, enabled, err := LoadFromEnv(func(string) (string, bool) { return "", false })
	if enabled || err != nil || cfg.ListenAddr != "" {
		t.Fatalf("absent: %v %v", enabled, err)
	}
	for _, name := range configNames {
		_, enabled, err := LoadFromEnv(func(key string) (string, bool) { return "", key == envPrefix+name })
		if !enabled || err == nil {
			t.Fatalf("partial %s: %v %v", name, enabled, err)
		}
	}
	values := map[string]string{"GRPC_LISTEN": "127.0.0.1:0", "TLS_CERT_FILE": "server.pem", "TLS_KEY_FILE": "key.pem", "CLIENT_CA_FILE": "ca.pem", "FILE_JWKS_URL": "https://file.internal/.well-known/jwks.json", "REPLAY_REDIS_ADDR": "127.0.0.1:6379"}
	lookup := func(k string) (string, bool) { v, ok := values[strings.TrimPrefix(k, envPrefix)]; return v, ok }
	if _, enabled, err = LoadFromEnv(lookup); !enabled || err != nil {
		t.Fatalf("complete: %v %v", enabled, err)
	}
	for _, bad := range []string{"http://file.internal/jwks", "https://user@file.internal/jwks", "https://file.internal/jwks#fragment", "https://file.internal/jwks?key=x"} {
		values["FILE_JWKS_URL"] = bad
		if _, _, err = LoadFromEnv(lookup); err == nil {
			t.Fatalf("accepted %s", bad)
		}
	}
}

type tlsFixture struct {
	caFile, certFile, keyFile string
	cert                      tls.Certificate
	roots                     *x509.CertPool
}

func makeTLS(t *testing.T) tlsFixture {
	t.Helper()
	dir := t.TempDir()
	now := time.Now()
	caKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "inventory test CA"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	leaf := &x509.Certificate{SerialNumber: big.NewInt(2), DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}}
	der, err := x509.CreateCertificate(rand.Reader, leaf, ca, &key.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	f := tlsFixture{caFile: filepath.Join(dir, "ca.pem"), certFile: filepath.Join(dir, "cert.pem"), keyFile: filepath.Join(dir, "key.pem"), roots: x509.NewCertPool()}
	f.roots.AppendCertsFromPEM(caPEM)
	for path, data := range map[string][]byte{f.caFile: caPEM, f.certFile: certPEM, f.keyFile: keyPEM} {
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	f.cert, err = tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func jwksBytes(keys ...*rsa.PrivateKey) []byte {
	entries := []map[string]string{}
	for i, key := range keys {
		entries = append(entries, map[string]string{"kty": "RSA", "kid": []string{"current", "next"}[i], "alg": "RS256", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())})
	}
	raw, _ := json.Marshal(map[string]any{"keys": entries})
	return raw
}

func TestInventorySignedMTLSTransportRejectsInvalidAuthorityBeforeStore(t *testing.T) {
	f := makeTLS(t)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwks := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(jwksBytes(key, next)) }))
	jwks.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{f.cert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: f.roots}
	jwks.StartTLS()
	defer jwks.Close()
	replay := miniredis.RunT(t)
	cfg := Config{ListenAddr: "127.0.0.1:0", TLSCertFile: f.certFile, TLSKeyFile: f.keyFile, ClientCAFile: f.caFile, JWKSCAFile: f.caFile, JWKSClientCertFile: f.certFile, JWKSClientKeyFile: f.keyFile, FileJWKSURL: jwks.URL, ReplayAddr: replay.Addr()}
	runtime, err := NewRuntime(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	id := identity()
	d, _ := referenceinventory.Describe(nil, id, 0)
	st := &memoryStore{descriptor: d}
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer(runtime.ServerOptions()...)
	storyv1.RegisterStoryReferenceInventoryServiceServer(server, New(st))
	go func() { _ = server.Serve(lis) }()
	defer server.Stop()
	dial := func(cert *tls.Certificate, roots *x509.CertPool) *grpc.ClientConn {
		conf := &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "localhost"}
		if cert != nil {
			conf.Certificates = []tls.Certificate{*cert}
		}
		conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(conf)))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = conn.Close() })
		return conn
	}
	client := storyv1.NewStoryReferenceInventoryServiceClient(dial(&f.cert, f.roots))
	request := &commonv1.BeginReferenceInventoryRequest{OperationId: uuid.NewString(), Identity: id}
	signed := func(issuer, audience, method string, body proto.Message, clock func() time.Time) context.Context {
		t.Helper()
		iss, err := principal.NewIssuer(principal.IssuerConfig{Issuer: issuer, KeyID: "current", PrivateKey: key, Clock: clock})
		if err != nil {
			t.Fatal(err)
		}
		hash, _ := principal.RequestHash(body)
		rid := uuid.NewString()
		token, err := iss.IssueService(principal.ServiceInput{Audience: audience, RPC: method, RequestID: rid, RequestHash: hash})
		if err != nil {
			t.Fatal(err)
		}
		return metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", rid))
	}
	valid := signed("file", "story", BeginMethod, request, nil)
	if got, err := client.BeginReferenceInventory(valid, request); err != nil || !proto.Equal(got.GetDescriptor_(), d) {
		t.Fatalf("valid: %v %v", got, err)
	}
	if _, err := client.BeginReferenceInventory(valid, request); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("replay: %v", err)
	}
	for _, tc := range []struct {
		name, issuer, aud, rpc string
		mutate                 bool
		clock                  func() time.Time
	}{
		{name: "issuer", issuer: "gateway", aud: "story", rpc: BeginMethod},
		{name: "audience", issuer: "file", aud: "file", rpc: BeginMethod},
		{name: "rpc", issuer: "file", aud: "story", rpc: PageMethod},
		{name: "hash", issuer: "file", aud: "story", rpc: BeginMethod, mutate: true},
		{name: "expired", issuer: "file", aud: "story", rpc: BeginMethod, clock: func() time.Time { return time.Now().Add(-time.Minute) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := signed(tc.issuer, tc.aud, tc.rpc, request, tc.clock)
			req := proto.Clone(request).(*commonv1.BeginReferenceInventoryRequest)
			if tc.mutate {
				req.OperationId = uuid.NewString()
			}
			if _, err := client.BeginReferenceInventory(ctx, req); status.Code(err) != codes.Unauthenticated {
				t.Fatalf("%v", err)
			}
		})
	}
	for _, raw := range []string{"x-profile-id", "x-account-id", "x-voice-principal", "x-internal-caller"} {
		ctx := signed("file", "story", BeginMethod, request, nil)
		ctx = metadata.AppendToOutgoingContext(ctx, raw, uuid.NewString())
		if _, err := client.BeginReferenceInventory(ctx, request); status.Code(err) != codes.Unauthenticated {
			t.Fatalf("raw %s: %v", raw, err)
		}
	}
	if _, err := client.BeginReferenceInventory(context.Background(), request); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("missing: %v", err)
	}
	unknown := proto.Clone(request).(*commonv1.BeginReferenceInventoryRequest)
	unknown.Identity.ProtoReflect().SetUnknown([]byte{0x98, 0x06, 1})
	if _, err := client.BeginReferenceInventory(signed("file", "story", BeginMethod, unknown, nil), unknown); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("unknown: %v", err)
	}
	foreign := makeTLS(t)
	for _, conn := range []*grpc.ClientConn{dial(nil, f.roots), dial(&foreign.cert, f.roots), dial(&f.cert, x509.NewCertPool())} {
		ctx, cancel := context.WithTimeout(signed("file", "story", BeginMethod, request, nil), time.Second)
		_, err := storyv1.NewStoryReferenceInventoryServiceClient(conn).BeginReferenceInventory(ctx, request)
		cancel()
		if err == nil {
			t.Fatal("invalid TLS succeeded")
		}
	}
	if st.calls != 1 {
		t.Fatalf("invalid requests reached Store, calls=%d", st.calls)
	}
}
