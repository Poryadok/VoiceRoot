package inventory

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	commonv1 "voice.app/voice/common/v1"
	storyv1 "voice.app/voice/story/v1"
	"voice/backend/pkg/principal"
)

type inventoryWireFixture struct {
	storyv1.UnimplementedStoryReferenceInventoryServiceServer
	mu      sync.Mutex
	headers []metadata.MD
}

func (s *inventoryWireFixture) BeginReferenceInventory(ctx context.Context, _ *commonv1.BeginReferenceInventoryRequest) (*commonv1.BeginReferenceInventoryResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	s.mu.Lock()
	s.headers = append(s.headers, md)
	s.mu.Unlock()
	return &commonv1.BeginReferenceInventoryResponse{}, nil
}

func TestStorySourceSignsFreshFileBindingOverMutualTLS(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	template := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), DNSNames: []string{"localhost"}, IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	cert := tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
	leaf, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	roots.AddCert(leaf)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}, ClientCAs: roots, ClientAuth: tls.RequireAndVerifyClientCert})))
	fixture := &inventoryWireFixture{}
	storyv1.RegisterStoryReferenceInventoryServiceServer(server, fixture)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "file", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	trust := &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "localhost", Certificates: []tls.Certificate{cert}}
	client, err := DialStorySource(listener.Addr().String(), trust, issuer)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	req := &commonv1.BeginReferenceInventoryRequest{OperationId: "same-operation"}
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer forwarded", "x-voice-profile-id", "forged", "x-voice-internal-caller", "story"))
	for i := 0; i < 2; i++ {
		_, err = client.BeginReferenceInventory(ctx, req)
		require.NoError(t, err)
	}
	fixture.mu.Lock()
	headers := append([]metadata.MD(nil), fixture.headers...)
	fixture.mu.Unlock()
	require.Len(t, headers, 2)
	require.NotEqual(t, headers[0].Get("x-request-id"), headers[1].Get("x-request-id"))
	expectedHash, err := principal.RequestHash(req)
	require.NoError(t, err)
	for _, md := range headers {
		require.Empty(t, md.Get("x-voice-profile-id"))
		require.Empty(t, md.Get("x-voice-internal-caller"))
		token := strings.TrimPrefix(md.Get("authorization")[0], "Bearer ")
		parts := strings.Split(token, ".")
		require.Len(t, parts, 3)
		signature, err := base64.RawURLEncoding.DecodeString(parts[2])
		require.NoError(t, err)
		hash := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
		require.NoError(t, rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, hash[:], signature))
		payload, err := base64.RawURLEncoding.DecodeString(parts[1])
		require.NoError(t, err)
		var claims map[string]any
		require.NoError(t, json.Unmarshal(payload, &claims))
		require.Equal(t, "file", claims["iss"])
		require.Equal(t, "service:file", claims["sub"])
		require.Equal(t, "story", claims["aud"])
		require.Equal(t, storyv1.StoryReferenceInventoryService_BeginReferenceInventory_FullMethodName, claims["rpc"])
		require.Equal(t, md.Get("x-request-id")[0], claims["request_id"])
		require.Equal(t, expectedHash, claims["request_hash"])
	}
	foreign, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "story", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	bad, err := DialStorySource(listener.Addr().String(), trust, foreign)
	require.NoError(t, err)
	defer func() { _ = bad.Close() }()
	_, err = bad.BeginReferenceInventory(ctx, req)
	require.Error(t, err, "foreign signer cannot impersonate File")
	for _, mutate := range []func(*tls.Config){func(c *tls.Config) { c.RootCAs = nil }, func(c *tls.Config) { c.Certificates = nil }, func(c *tls.Config) { c.InsecureSkipVerify = true }, func(c *tls.Config) { c.ServerName = "" }} {
		broken := trust.Clone()
		mutate(broken)
		_, err = DialStorySource(listener.Addr().String(), broken, issuer)
		require.Error(t, err)
	}
}
