package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/test/bufconn"
)

func TestSocialPrincipalJWKSInMemoryTLSLifecycle(t *testing.T) {
	socialConfiguredRuntime(t)
	runtime, err := loadSocialPrincipalRuntime()
	require.NoError(t, err)
	t.Cleanup(runtime.Close)
	listener := bufconn.Listen(1 << 20)
	result := make(chan error, 1)
	go func() { result <- runtime.JWKS.ServeTLS(listener, "", "") }()
	ca, err := os.ReadFile(os.Getenv("SOCIAL_PRINCIPAL_TLS_CERT_FILE"))
	require.NoError(t, err)
	trusted := x509.NewCertPool()
	require.True(t, trusted.AppendCertsFromPEM(ca))
	for _, tc := range []struct {
		name       string
		roots      *x509.CertPool
		serverName string
		valid      bool
	}{
		{"trusted", trusted, "social", true},
		{"wrong CA", x509.NewCertPool(), "social", false},
		{"wrong name", trusted, "attacker", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			transport := &http.Transport{DialTLSContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				raw, dialErr := listener.DialContext(ctx)
				if dialErr != nil {
					return nil, dialErr
				}
				conn := tls.Client(raw, &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: tc.roots, ServerName: tc.serverName})
				if handshakeErr := conn.HandshakeContext(ctx); handshakeErr != nil {
					_ = raw.Close()
					return nil, handshakeErr
				}
				return conn, nil
			}}
			t.Cleanup(transport.CloseIdleConnections)
			client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
			response, getErr := client.Get("https://social/.well-known/jwks.json")
			if !tc.valid {
				require.Error(t, getErr)
				return
			}
			require.NoError(t, getErr)
			require.Equal(t, http.StatusOK, response.StatusCode)
			require.NoError(t, response.Body.Close())
		})
	}
	runtime.Close()
	select {
	case serveErr := <-result:
		require.ErrorIs(t, serveErr, http.ErrServerClosed)
	case <-time.After(5 * time.Second):
		t.Fatal("JWKS server did not stop")
	}
}
