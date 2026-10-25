package usercapture

import (
	"bytes"
	"context"
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"net"
	"strings"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/principal"
)

// TransportConfig contains caller-owned, already acquired workload credentials.
// It supplies no deployment key bootstrap, environment defaults or grants.
type TransportConfig struct {
	Address    string
	ServerName string
	// Roots must be explicitly supplied and nonempty. Caller-provided system
	// pools are allowed; the constructor never acquires fallback system trust.
	Roots       *x509.CertPool
	Certificate tls.Certificate
}

// New constructs an exclusively mutual-TLS capture client. The caller owns the
// returned connection and must close it during shutdown.
func New(config TransportConfig, issuer *principal.Issuer) (*Client, *grpc.ClientConn, error) {
	return newTransport(config, issuer, nil)
}

func newTransport(config TransportConfig, issuer *principal.Issuer, dialer func(context.Context, string) (net.Conn, error)) (*Client, *grpc.ClientConn, error) {
	if issuer == nil || config.Address == "" || config.Address != strings.TrimSpace(config.Address) || config.ServerName == "" || config.ServerName != strings.TrimSpace(config.ServerName) || config.Roots == nil || config.Roots.Equal(x509.NewCertPool()) || len(config.Certificate.Certificate) == 0 || config.Certificate.PrivateKey == nil {
		return nil, nil, ErrUnavailable
	}
	signer, ok := config.Certificate.PrivateKey.(crypto.Signer)
	if !ok {
		return nil, nil, ErrUnavailable
	}
	leaf, err := x509.ParseCertificate(config.Certificate.Certificate[0])
	if err != nil {
		return nil, nil, ErrUnavailable
	}
	certPublic, err := x509.MarshalPKIXPublicKey(leaf.PublicKey)
	if err != nil {
		return nil, nil, ErrUnavailable
	}
	keyPublic, err := x509.MarshalPKIXPublicKey(signer.Public())
	if err != nil || !bytes.Equal(certPublic, keyPublic) {
		return nil, nil, ErrUnavailable
	}
	certificate := config.Certificate
	certificate.Certificate = make([][]byte, len(config.Certificate.Certificate))
	for i, der := range config.Certificate.Certificate {
		certificate.Certificate[i] = append([]byte(nil), der...)
	}
	certificate.Leaf = nil
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: config.Roots.Clone(), ServerName: config.ServerName, Certificates: []tls.Certificate{certificate}}
	options := []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)), grpc.WithDisableRetry()}
	// Only package tests can replace socket transport with a private in-memory
	// connection. They still execute the same real TLS handshake and RPC client.
	if dialer != nil {
		options = append(options, grpc.WithContextDialer(dialer))
	}
	conn, err := grpc.NewClient(grpcclient.DialTarget(config.Address), options...)
	if err != nil {
		return nil, nil, ErrUnavailable
	}
	return &Client{rpc: userv1.NewUserServiceClient(conn), issuer: issuer}, conn, nil
}
