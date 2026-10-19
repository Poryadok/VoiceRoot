package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	messagingv1 "voice.app/voice/messaging/v1"
	grpcsvc "voice/backend/bot/internal/grpcsvc"
	"voice/backend/pkg/principal"
)

type cleanupMessagingTLSReceiver struct {
	messagingv1.UnimplementedMessagingServiceServer
}

func (cleanupMessagingTLSReceiver) ApplyBotChatPinRemoval(ctx context.Context, _ *messagingv1.ApplyBotChatPinRemovalRequest) (*messagingv1.ApplyBotChatPinRemovalResponse, error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return nil, context.Canceled
	}
	auth, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok || len(auth.State.VerifiedChains) == 0 || auth.State.PeerCertificates[0].Subject.CommonName != "bot" {
		return nil, context.Canceled
	}
	return &messagingv1.ApplyBotChatPinRemovalResponse{}, nil
}

func TestCleanupMessagingDedicatedMutualTLSClient(t *testing.T) {
	// This test opens only a loopback listener in the isolated Linux test runner.
	dir := t.TempDir()
	now := time.Now()
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "native-test-ca"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	require.NoError(t, err)
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	caPath := filepath.Join(dir, "ca.pem")
	require.NoError(t, os.WriteFile(caPath, caPEM, 0600))
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(caPEM))
	mint := func(serial int64, name string, usage x509.ExtKeyUsage) (tls.Certificate, string, string) {
		key, e := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, e)
		cert := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: name}, DNSNames: []string{name}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage}}
		der, e := x509.CreateCertificate(rand.Reader, cert, ca, &key.PublicKey, caKey)
		require.NoError(t, e)
		certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
		keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
		certPath, keyPath := filepath.Join(dir, name+".pem"), filepath.Join(dir, name+".key")
		require.NoError(t, os.WriteFile(certPath, certPEM, 0600))
		require.NoError(t, os.WriteFile(keyPath, keyPEM, 0600))
		pair, e := tls.X509KeyPair(certPEM, keyPEM)
		require.NoError(t, e)
		return pair, certPath, keyPath
	}
	serverPair, _, _ := mint(2, "messaging-cleanup.test", x509.ExtKeyUsageServerAuth)
	_, certPath, keyPath := mint(3, "bot", x509.ExtKeyUsageClientAuth)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{serverPair}, ClientCAs: roots, ClientAuth: tls.RequireAndVerifyClientCert})))
	messagingv1.RegisterMessagingServiceServer(server, cleanupMessagingTLSReceiver{})
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: "current", PrivateKey: caKey})
	require.NoError(t, err)
	values := map[string]string{"BOT_MESSAGING_CLEANUP_GRPC_ADDR": listener.Addr().String(), "BOT_MESSAGING_CLEANUP_TLS_CA_FILE": caPath, "BOT_MESSAGING_CLEANUP_TLS_SERVER_NAME": "messaging-cleanup.test", "BOT_MESSAGING_CLEANUP_TLS_CERT_FILE": certPath, "BOT_MESSAGING_CLEANUP_TLS_KEY_FILE": keyPath}
	svc := &grpcsvc.BotGRPC{PrincipalIssuer: issuer}
	conn, err := wireCleanupMessaging(svc, func(k string) string { return values[k] })
	require.NoError(t, err)
	require.NotNil(t, conn)
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err = svc.CleanupMessaging.ApplyBotChatPinRemoval(ctx, &messagingv1.ApplyBotChatPinRemovalRequest{})
	require.NoError(t, err, "real server requires and observes Bot client certificate")
	// A configured destination is never accepted under another server identity.
	values["BOT_MESSAGING_CLEANUP_TLS_SERVER_NAME"] = "wrong-messaging.test"
	bad, err := wireCleanupMessaging(svc, func(k string) string { return values[k] })
	require.NoError(t, err)
	defer bad.Close()
	_, err = svc.CleanupMessaging.ApplyBotChatPinRemoval(ctx, &messagingv1.ApplyBotChatPinRemovalRequest{})
	require.Error(t, err)
}

func TestCleanupMessagingConfigurationFailsClosed(t *testing.T) {
	svc := &grpcsvc.BotGRPC{}
	conn, err := wireCleanupMessaging(svc, func(string) string { return "" })
	require.NoError(t, err)
	require.Nil(t, conn)
	require.Nil(t, svc.CleanupMessaging)
	_, err = wireCleanupMessaging(svc, func(k string) string {
		if k == "BOT_MESSAGING_CLEANUP_GRPC_ADDR" {
			return "messaging:9099"
		}
		return ""
	})
	require.ErrorContains(t, err, "incomplete")
	require.Nil(t, svc.CleanupMessaging)
	_, err = wireCleanupMessaging(svc, func(string) string { return "configured" })
	require.ErrorContains(t, err, "issuer")
	require.Nil(t, svc.CleanupMessaging)
}
