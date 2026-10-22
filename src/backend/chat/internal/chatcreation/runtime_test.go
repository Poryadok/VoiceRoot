package chatcreation

import (
	"context"
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
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/pkg/principal"
)

type transportCA struct {
	certificate *x509.Certificate
	key         *rsa.PrivateKey
	pem         []byte
}

func newTransportCA(t *testing.T, serial int64) transportCA {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	template := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: "composite test CA"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return transportCA{cert, key, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})}
}
func (ca transportCA) leaf(t *testing.T, serial int64, usage x509.ExtKeyUsage) (tls.Certificate, string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	template := &x509.Certificate{SerialNumber: big.NewInt(serial), NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), BasicConstraintsValid: true, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage}, DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}}
	der, err := x509.CreateCertificate(rand.Reader, template, ca.certificate, &key.PublicKey, ca.key)
	require.NoError(t, err)
	dir := t.TempDir()
	certFile, keyFile := filepath.Join(dir, "leaf.crt"), filepath.Join(dir, "leaf.key")
	require.NoError(t, os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600))
	require.NoError(t, os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0600))
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	require.NoError(t, err)
	return cert, certFile, keyFile
}

type createTransportServer struct {
	chatv1.UnimplementedChatServiceServer
	calls atomic.Int32
}

func (s *createTransportServer) CreateChat(ctx context.Context, req *chatv1.CreateChatRequest) (*chatv1.CreateChatResponse, error) {
	if _, err := Creator(ctx, req); err != nil {
		return nil, status.Error(codes.Unauthenticated, "verified creator absent")
	}
	s.calls.Add(1)
	return &chatv1.CreateChatResponse{}, nil
}

func TestRealCreateTLSJWKSReplayAndRevocation(t *testing.T) {
	serverCA, clientCA, foreignCA := newTransportCA(t, 1), newTransportCA(t, 2), newTransportCA(t, 3)
	serverCert, certFile, keyFile := serverCA.leaf(t, 10, x509.ExtKeyUsageServerAuth)
	clientCert, clientCertFile, clientKeyFile := clientCA.leaf(t, 11, x509.ExtKeyUsageClientAuth)
	foreignCert, _, _ := foreignCA.leaf(t, 12, x509.ExtKeyUsageClientAuth)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(serverCA.pem))
	dir := t.TempDir()
	serverCAFile, clientCAFile := filepath.Join(dir, "server-ca.crt"), filepath.Join(dir, "client-ca.crt")
	require.NoError(t, os.WriteFile(serverCAFile, serverCA.pem, 0600))
	require.NoError(t, os.WriteFile(clientCAFile, clientCA.pem, 0600))
	keys := map[string]*rsa.PrivateKey{}
	for _, kid := range []string{"current", "next"} {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		keys[kid] = key
	}
	document := []map[string]string{}
	for kid, key := range keys {
		document = append(document, map[string]string{"kty": "RSA", "kid": kid, "alg": "RS256", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": "AQAB"})
	}
	jwks := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": document})
	}))
	jwksClients := x509.NewCertPool()
	require.True(t, jwksClients.AppendCertsFromPEM(clientCA.pem))
	jwks.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{serverCert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: jwksClients}
	jwks.StartTLS()
	defer jwks.Close()
	replay := miniredis.RunT(t)
	cfg := CreateConfig{Listen: "127.0.0.1:0", CertFile: certFile, KeyFile: keyFile, ClientCAFile: clientCAFile, JWKSURL: jwks.URL, JWKSCAFile: serverCAFile, RedisAddr: replay.Addr()}
	cfg.BotJWKSURL = jwks.URL
	cfg.JWKSClientCertFile, cfg.JWKSClientKeyFile = clientCertFile, clientKeyFile
	runtime, err := NewCreateRuntime(context.Background(), cfg, func(context.Context, string, int64) error { return nil })
	require.NoError(t, err)
	defer runtime.Close()
	listener, err := net.Listen("tcp", cfg.Listen)
	require.NoError(t, err)
	service := &createTransportServer{}
	server := grpc.NewServer(runtime.ServerOptions()...)
	chatv1.RegisterChatServiceServer(server, service)
	go func() { _ = server.Serve(listener) }()
	defer server.Stop()
	connect := func(certs []tls.Certificate) chatv1.ChatServiceClient {
		conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: certs})))
		require.NoError(t, err)
		t.Cleanup(func() { _ = conn.Close() })
		return chatv1.NewChatServiceClient(conn)
	}
	client := connect([]tls.Certificate{clientCert})
	req := &chatv1.CreateChatRequest{SpaceId: proto.String(uuid.NewString()), Type: chatv1.ChatType_CHAT_TYPE_GROUP, Name: proto.String("runtime"), RequestId: uuid.NewString()}
	profile := uuid.NewString()

	sign := func(kid string) context.Context {
		hash, err := principal.RequestHash(req)
		require.NoError(t, err)
		issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: kid, PrivateKey: keys[kid]})
		require.NoError(t, err)
		token, err := issuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "chat", RPC: chatv1.ChatService_CreateChat_FullMethodName, RequestID: req.RequestId, RequestHash: hash, AccountID: uuid.NewString(), ProfileID: profile, AccountType: "regular", SessionEpoch: 7, ClientExpiresAt: time.Now().Add(time.Minute)})
		require.NoError(t, err)
		return metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", req.RequestId))
	}
	for _, kid := range []string{"current", "next"} {
		ctx := sign(kid)
		_, err := client.CreateChat(ctx, req)
		require.NoError(t, err)
		_, err = client.CreateChat(ctx, req)
		require.Equal(t, codes.Unauthenticated, status.Code(err))
	}
	require.Equal(t, int32(2), service.calls.Load())
	// Exact business retries use fresh credentials; a changed request is denied.
	ctx := sign("current")
	changed := proto.Clone(req).(*chatv1.CreateChatRequest)
	changed.Name = proto.String("changed")
	_, err = client.CreateChat(ctx, changed)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	ctx = metadata.AppendToOutgoingContext(sign("current"), "x-voice-profile-id", profile)
	_, err = client.CreateChat(ctx, req)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = client.GetChat(sign("current"), &chatv1.GetChatRequest{ChatId: uuid.NewString()})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	for _, certs := range [][]tls.Certificate{nil, {foreignCert}} {
		ctx, cancel := context.WithTimeout(sign("current"), time.Second)
		_, err = connect(certs).CreateChat(ctx, req)
		cancel()
		require.Error(t, err)
	}
	// Another runtime after restart still sees the permanent shared replay mark.
	ctx = sign("current")
	_, err = client.CreateChat(ctx, req)
	require.NoError(t, err)
	second, err := NewCreateRuntime(context.Background(), cfg, func(context.Context, string, int64) error { return nil })
	require.NoError(t, err)
	defer second.Close()
	secondListener, err := net.Listen("tcp", cfg.Listen)
	require.NoError(t, err)
	secondService := &createTransportServer{}
	secondServer := grpc.NewServer(second.ServerOptions()...)
	chatv1.RegisterChatServiceServer(secondServer, secondService)
	go func() { _ = secondServer.Serve(secondListener) }()
	defer secondServer.Stop()
	secondConn, err := grpc.NewClient(secondListener.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: []tls.Certificate{clientCert}})))
	require.NoError(t, err)
	defer secondConn.Close()
	secondClient := chatv1.NewChatServiceClient(secondConn)
	_, err = secondClient.CreateChat(ctx, req)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Zero(t, secondService.calls.Load(), "previously consumed token is denied across real TLS restart")
	_, err = secondClient.CreateChat(sign("next"), req)
	require.NoError(t, err, "fresh token reaches the same real second listener")
	require.Equal(t, int32(1), secondService.calls.Load())
	replay.Close()
	_, err = client.CreateChat(sign("current"), req)
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.Equal(t, int32(3), service.calls.Load())
}
