package inventory

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/pkg/principal"
)

type messageInventoryFixture struct {
	messagingv1.UnimplementedMessagingReferenceInventoryServiceServer
	headers  metadata.MD
	request  *messagingv1.BeginReferenceInventoryRequest
	response *messagingv1.BeginReferenceInventoryResponse
}

func (s *messageInventoryFixture) BeginReferenceInventory(ctx context.Context, r *messagingv1.BeginReferenceInventoryRequest) (*messagingv1.BeginReferenceInventoryResponse, error) {
	s.headers, _ = metadata.FromIncomingContext(ctx)
	s.request = r
	return s.response, nil
}

func TestMessagingSourceSignsEnvelopeAndRejectsMissingPayload(t *testing.T) {
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
	fixture := &messageInventoryFixture{response: &messagingv1.BeginReferenceInventoryResponse{Inventory: &commonv1.BeginReferenceInventoryResponse{}}}
	messagingv1.RegisterMessagingReferenceInventoryServiceServer(server, fixture)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "file", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	client, err := DialMessagingSource(listener.Addr().String(), &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "localhost", Certificates: []tls.Certificate{cert}}, issuer)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	r := &commonv1.BeginReferenceInventoryRequest{OperationId: "operation"}
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer forged", "x-voice-profile-id", "forged"))
	_, err = client.BeginReferenceInventory(ctx, r)
	require.NoError(t, err)
	require.Empty(t, fixture.headers.Get("x-voice-profile-id"))
	parts := strings.Split(strings.TrimPrefix(fixture.headers.Get("authorization")[0], "Bearer "), ".")
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var claims map[string]any
	require.NoError(t, json.Unmarshal(payload, &claims))
	hash, err := principal.RequestHash(fixture.request)
	require.NoError(t, err)
	require.Equal(t, "messaging", claims["aud"])
	require.Equal(t, hash, claims["request_hash"])
	require.Equal(t, messagingv1.MessagingReferenceInventoryService_BeginReferenceInventory_FullMethodName, claims["rpc"])
	fixture.response = &messagingv1.BeginReferenceInventoryResponse{}
	_, err = client.BeginReferenceInventory(ctx, r)
	require.Error(t, err)
	fixture.response = &messagingv1.BeginReferenceInventoryResponse{Inventory: &commonv1.BeginReferenceInventoryResponse{}}
	fixture.response.Inventory.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
	_, err = client.BeginReferenceInventory(ctx, r)
	require.Error(t, err)
}
