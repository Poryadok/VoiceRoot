// Package testcreate provides isolated signed CreateChat clients for service tests.
// Fixture identity headers are inputs to its test Gateway signer, never authority
// at the Chat handler. Production construction does not import this package.
package testcreate

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/chat/internal/authctx"
	"voice/backend/chat/internal/chatcreation"
	"voice/backend/pkg/principal"
)

// NewClient keeps the ordinary fixture transport for other RPCs and routes only
// CreateChat through a separate real mTLS listener and production CreateVerifier.
func NewClient(t *testing.T, service chatv1.ChatServiceServer) (chatv1.ChatServiceClient, func()) {
	t.Helper()
	client, _, _, cleanup := newClient(t, service)
	return client, cleanup
}

type createSigner struct{ issuer *principal.Issuer }

func (s createSigner) context(ctx context.Context, req *chatv1.CreateChatRequest) (context.Context, error) {
	md, _ := metadata.FromOutgoingContext(ctx)
	account, profile := md.Get(authctx.HeaderUserID), md.Get(authctx.HeaderProfileID)
	// Missing/ambiguous fixture identity remains unsigned and must be denied by
	// the real verifier. A test cannot obtain a synthetic verified context.
	if len(account) != 1 || len(profile) != 1 {
		return ctx, nil
	}
	accountType := "regular"
	if types := md.Get(authctx.HeaderAccountType); len(types) == 1 {
		accountType = types[0]
	}
	requestID := req.GetRequestId()
	if requestID == "" {
		requestID = uuid.NewString()
	}
	hash, err := principal.RequestHash(req)
	if err != nil {
		return nil, err
	}
	token, err := s.issuer.IssueDelegatedUser(principal.DelegatedUserInput{
		Audience: "chat", RPC: chatv1.ChatService_CreateChat_FullMethodName,
		RequestID: requestID, RequestHash: hash, AccountID: account[0], ProfileID: profile[0],
		AccountType: accountType, SessionEpoch: 1, ClientExpiresAt: time.Now().Add(time.Minute),
	})
	if err != nil {
		return nil, err
	}
	// No raw identity is forwarded to the protected listener.
	return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID)), nil
}

type routedConn struct {
	ordinary, create *grpc.ClientConn
	signer           createSigner
}

func (c routedConn) Invoke(ctx context.Context, method string, req, reply any, options ...grpc.CallOption) error {
	if method != chatv1.ChatService_CreateChat_FullMethodName {
		return c.ordinary.Invoke(ctx, method, req, reply, options...)
	}
	signed, err := c.signer.context(ctx, req.(*chatv1.CreateChatRequest))
	if err != nil {
		return err
	}
	return c.create.Invoke(signed, method, req, reply, options...)
}

func (c routedConn) NewStream(ctx context.Context, description *grpc.StreamDesc, method string, options ...grpc.CallOption) (grpc.ClientStream, error) {
	return c.ordinary.NewStream(ctx, description, method, options...)
}

func newClient(t *testing.T, service chatv1.ChatServiceServer) (chatv1.ChatServiceClient, *grpc.ClientConn, createSigner, func()) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "fixture-gateway", PrivateKey: key})
	require.NoError(t, err)
	serverTLS, clientTLS := transportTLS(t)
	var mu sync.Mutex
	consumed := map[string]bool{}
	verifier := chatcreation.CreateVerifier{
		Keys: func(_ context.Context, issuer, kid string) (*rsa.PublicKey, error) {
			if issuer != "gateway" || kid != "fixture-gateway" {
				return nil, errors.New("unknown fixture issuer/key")
			}
			return &key.PublicKey, nil
		},
		Replay: func(_ context.Context, issuer, jti string, _ time.Time) error {
			mu.Lock()
			defer mu.Unlock()
			id := issuer + ":" + jti
			if consumed[id] {
				return errors.New("fixture credential replay")
			}
			consumed[id] = true
			return nil
		},
		Epoch: func(_ context.Context, _ string, epoch int64) error {
			if epoch != 1 {
				return errors.New("stale fixture session epoch")
			}
			return nil
		},
	}
	ordinaryListener, createListener := bufconn.Listen(1<<20), bufconn.Listen(1<<20)
	ordinaryServer := grpc.NewServer(grpc.UnaryInterceptor(chatcreation.OrdinaryCreateUnary()))
	createServer := grpc.NewServer(grpc.Creds(credentials.NewTLS(serverTLS)), grpc.UnaryInterceptor(verifier.Unary()))
	chatv1.RegisterChatServiceServer(ordinaryServer, service)
	chatv1.RegisterChatServiceServer(createServer, service)
	go func() { _ = ordinaryServer.Serve(ordinaryListener) }()
	go func() { _ = createServer.Serve(createListener) }()
	dial := func(listener *bufconn.Listener, secure bool) *grpc.ClientConn {
		options := []grpc.DialOption{grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() })}
		if secure {
			options = append(options, grpc.WithTransportCredentials(credentials.NewTLS(clientTLS)))
		} else {
			options = append(options, grpc.WithTransportCredentials(insecure.NewCredentials()))
		}
		conn, err := grpc.NewClient("passthrough:///chat-fixture", options...)
		require.NoError(t, err)
		return conn
	}
	ordinary, create := dial(ordinaryListener, false), dial(createListener, true)
	signer := createSigner{issuer: issuer}
	cleanup := func() {
		_ = ordinary.Close()
		_ = create.Close()
		ordinaryServer.Stop()
		createServer.Stop()
		_ = ordinaryListener.Close()
		_ = createListener.Close()
	}
	return chatv1.NewChatServiceClient(routedConn{ordinary: ordinary, create: create, signer: signer}), create, signer, cleanup
}

func transportTLS(t *testing.T) (*tls.Config, *tls.Config) {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	now := time.Now()
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "isolated Chat test CA"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	require.NoError(t, err)
	parsedCA, err := x509.ParseCertificate(caDER)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	roots.AddCert(parsedCA)
	leaf := func(serial int64, usage x509.ExtKeyUsage) tls.Certificate {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)
		certificate := &x509.Certificate{SerialNumber: big.NewInt(serial), DNSNames: []string{"chat-fixture"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), ExtKeyUsage: []x509.ExtKeyUsage{usage}, KeyUsage: x509.KeyUsageDigitalSignature}
		der, err := x509.CreateCertificate(rand.Reader, certificate, ca, &key.PublicKey, caKey)
		require.NoError(t, err)
		return tls.Certificate{Certificate: [][]byte{der, caDER}, PrivateKey: key}
	}
	server := &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{leaf(2, x509.ExtKeyUsageServerAuth)}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots}
	client := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: "chat-fixture", RootCAs: roots, Certificates: []tls.Certificate{leaf(3, x509.ExtKeyUsageClientAuth)}}
	return server, client
}
