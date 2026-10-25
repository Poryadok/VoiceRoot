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

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func TestAuditInitiatingConfig_RejectsPartialAndExplicitEmptyConfiguration(t *testing.T) {
	for _, service := range []string{"ROLE", "CHAT", "BOT_ADMIN"} {
		t.Run(service, func(t *testing.T) {
			prefix := "GATEWAY_" + service + "_AUDIT_"
			for _, suffix := range []string{"GRPC_ADDR", "TLS_CA_FILE", "TLS_SERVER_NAME", "CLIENT_CERT_FILE", "CLIENT_KEY_FILE"} {
				t.Setenv(prefix+suffix, "")
			}
			if _, enabled, err := auditInitiatingClientConfigFromEnv(service); !enabled || err == nil {
				t.Fatalf("explicit empty protected configuration: enabled=%v err=%v", enabled, err)
			}
			t.Setenv(prefix+"GRPC_ADDR", "private.example:9099")
			if _, enabled, err := auditInitiatingClientConfigFromEnv(service); !enabled || err == nil {
				t.Fatalf("partial protected configuration: enabled=%v err=%v", enabled, err)
			}
		})
	}
}

func TestAuditInitiatingConfig_RejectsUnknownService(t *testing.T) {
	if _, _, err := auditInitiatingClientConfigFromEnv("SPACE"); err == nil {
		t.Fatal("unexpected initiating transport service accepted")
	}
}

func TestAuditInitiatingConfig_ProtectedClientsUseMutualTLSAndReadiness(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "audit.test"}, DNSNames: []string{"audit.test"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(certPEM)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}, ClientCAs: roots, ClientAuth: tls.RequireAndVerifyClientCert})))
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	dir := t.TempDir()
	certFile, keyFile := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certFile, certPEM, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, keyPEM, 0600); err != nil {
		t.Fatal(err)
	}
	for _, service := range auditInitiatingServices {
		prefix := "GATEWAY_" + service + "_AUDIT_"
		for suffix, value := range map[string]string{"GRPC_ADDR": listener.Addr().String(), "TLS_CA_FILE": certFile, "TLS_SERVER_NAME": "audit.test", "CLIENT_CERT_FILE": certFile, "CLIENT_KEY_FILE": keyFile} {
			t.Setenv(prefix+suffix, value)
		}
	}
	clients := &grpcClients{}
	clients.configureAuditInitiatingTransports()
	t.Cleanup(clients.close)
	if clients.roleAudit == nil || clients.chatAudit == nil || clients.botAuditAdmin == nil || len(clients.connections) != 3 {
		t.Fatal("protected transports were not constructed independently")
	}
	if clients.role != nil || clients.chat != nil || clients.bot != nil {
		t.Fatal("ordinary clients populated by protected configuration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := clients.waitAuditInitiatingReady(ctx); err != nil {
		t.Fatal(err)
	}
	clients.auditInitiatingTransports[0].conn = nil
	if err := clients.waitAuditInitiatingReady(ctx); err == nil {
		t.Fatal("missing configured protected connection accepted")
	}
}
