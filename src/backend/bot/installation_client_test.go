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
	spacev1 "voice.app/voice/space/v1"
	grpcsvc "voice/backend/bot/internal/grpcsvc"
	"voice/backend/pkg/principal"
)

type installationTLSReceiver struct {
	spacev1.UnimplementedSpaceServiceServer
}

func (installationTLSReceiver) AddBotMember(ctx context.Context, _ *spacev1.AddBotMemberRequest) (*spacev1.AddBotMemberResponse, error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return nil, context.Canceled
	}
	auth, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok || len(auth.State.VerifiedChains) == 0 || auth.State.PeerCertificates[0].Subject.CommonName != "bot" {
		return nil, context.Canceled
	}
	return &spacev1.AddBotMemberResponse{}, nil
}

func TestInstallationSpaceDedicatedMutualTLSClient(t *testing.T) {
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
	serverPair, _, _ := mint(2, "space-native.test", x509.ExtKeyUsageServerAuth)
	_, certPath, keyPath := mint(3, "bot", x509.ExtKeyUsageClientAuth)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{serverPair}, ClientCAs: roots, ClientAuth: tls.RequireAndVerifyClientCert})))
	spacev1.RegisterSpaceServiceServer(server, installationTLSReceiver{})
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: "current", PrivateKey: caKey})
	require.NoError(t, err)
	values := map[string]string{"BOT_SPACE_INSTALLATION_GRPC_ADDR": listener.Addr().String(), "BOT_SPACE_INSTALLATION_TLS_CA_FILE": caPath, "BOT_SPACE_INSTALLATION_TLS_SERVER_NAME": "space-native.test", "BOT_SPACE_INSTALLATION_TLS_CERT_FILE": certPath, "BOT_SPACE_INSTALLATION_TLS_KEY_FILE": keyPath}
	svc := &grpcsvc.BotGRPC{PrincipalIssuer: issuer}
	conn, err := wireInstallationSpace(svc, func(k string) string { return values[k] })
	require.NoError(t, err)
	require.NotNil(t, conn)
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err = svc.InstallationSpace.AddBotMember(ctx, &spacev1.AddBotMemberRequest{})
	require.NoError(t, err, "real server requires and observes Bot client certificate")
	// A configured destination is never accepted under another server identity.
	values["BOT_SPACE_INSTALLATION_TLS_SERVER_NAME"] = "wrong-space.test"
	bad, err := wireInstallationSpace(svc, func(k string) string { return values[k] })
	require.NoError(t, err)
	defer bad.Close()
	_, err = svc.InstallationSpace.AddBotMember(ctx, &spacev1.AddBotMemberRequest{})
	require.Error(t, err)
}

func TestInstallationSpaceConfigurationFailsClosed(t *testing.T) {
	svc := &grpcsvc.BotGRPC{}
	conn, err := wireInstallationSpace(svc, func(string) string { return "" })
	require.NoError(t, err)
	require.Nil(t, conn)
	require.Nil(t, svc.InstallationSpace)
	_, err = wireInstallationSpace(svc, func(k string) string {
		if k == "BOT_SPACE_INSTALLATION_GRPC_ADDR" {
			return "space:9098"
		}
		return ""
	})
	require.ErrorContains(t, err, "incomplete")
	require.Nil(t, svc.InstallationSpace)
	_, err = wireInstallationSpace(svc, func(string) string { return "configured" })
	require.ErrorContains(t, err, "issuer")
	require.Nil(t, svc.InstallationSpace)
}
