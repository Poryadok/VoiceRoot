package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/socialprincipal"
	grpcsvc "voice/backend/user/internal/grpcsvc"
)

func TestStartupAdmissionDeniesHandlersAndHealthUntilRelease(t *testing.T) {
	o := newUserStartup(context.Background())
	t.Cleanup(o.Close)
	var calls atomic.Int32
	handler := func(context.Context, any) (any, error) { calls.Add(1); return nil, nil }
	_, err := o.unary(context.Background(), nil, &grpc.UnaryServerInfo{}, handler)
	require.Equal(t, codes.Unavailable, status.Code(err))
	stream := func(any, grpc.ServerStream) error { calls.Add(1); return nil }
	require.Equal(t, codes.Unavailable, status.Code(o.stream(nil, nil, &grpc.StreamServerInfo{}, stream)))
	protected := o.http(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls.Add(1); w.WriteHeader(200) }))
	w := httptest.NewRecorder()
	protected.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	require.Equal(t, 503, w.Code)
	require.Zero(t, calls.Load(), "no handler/store side effect before admission")
	require.NoError(t, o.Release())
	_, err = o.unary(context.Background(), nil, &grpc.UnaryServerInfo{}, handler)
	require.NoError(t, err)
	require.NoError(t, o.stream(nil, nil, &grpc.StreamServerInfo{}, stream))
	w = httptest.NewRecorder()
	protected.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	require.Equal(t, 200, w.Code)
	require.EqualValues(t, 3, calls.Load())
}

func TestStartupServeFailureRevokesAndCannotRaceRelease(t *testing.T) {
	o := newUserStartup(context.Background())
	t.Cleanup(o.Close)
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := grpc.NewServer(o.options()...)
	require.NoError(t, o.Serve(srv, lis))
	require.NoError(t, lis.Close())
	select {
	case <-o.ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("Serve failure did not cancel startup")
	}
	require.Error(t, o.Release())
	require.False(t, o.admitted())
}

func TestStartupCancellationClosesEarlyListenerAndAdmission(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	o := newUserStartup(ctx)
	t.Cleanup(o.Close)
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	require.NoError(t, o.Serve(grpc.NewServer(o.options()...), lis))
	cancel()
	o.Close()
	require.Error(t, o.Release())
	conn, err := net.DialTimeout("tcp", lis.Addr().String(), time.Second)
	if conn != nil {
		_ = conn.Close()
	}
	require.Error(t, err)
}

func TestStartupFailureAfterReleaseRevokesWithoutHandlerCall(t *testing.T) {
	o := newUserStartup(context.Background())
	t.Cleanup(o.Close)
	require.NoError(t, o.Release())
	o.fail(errors.New("selected listener stopped"))
	called := false
	_, err := o.unary(context.Background(), nil, &grpc.UnaryServerInfo{}, func(context.Context, any) (any, error) { called = true; return nil, nil })
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.False(t, called)
}

func TestStartupNotificationPrimaryPartialConfigReturnsError(t *testing.T) {
	if os.Getenv("VOICE_USER_STARTUP_CONFIG_CHILD") == "1" {
		t.Setenv("VOICE_RELEASE_PROFILE", "development")
		t.Setenv("USER_NOTIFICATION_PRIMARY_PRINCIPAL_TLS_CERT_FILE", "/missing")
		require.ErrorContains(t, runUser(context.Background()), "user Notification primary principal config")
		return
	}
	// A fatal exit would skip runUser's cleanup. Isolate that regression so its
	// RED is a failed assertion instead of terminating the whole component suite.
	cmd := exec.Command(os.Args[0], "-test.run=^TestStartupNotificationPrimaryPartialConfigReturnsError$")
	cmd.Env = append(os.Environ(), "VOICE_USER_STARTUP_CONFIG_CHILD=1")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", output)
}

// Actual mTLS transport readiness must neither bypass startup admission nor
// confer protected RPC authority once admission opens. The ordinary profile
// deliberately has no protected authority; this test supplies no allow facts.
func TestStartupMutualTLSKeepsAdmissionAndOrdinaryAuthoritySeparate(t *testing.T) {
	root := repoRootFromComposeTest(t)
	certPath := filepath.Join(root, "src/backend/auth/src/test/resources/principal-tls/server-cert.pem")
	keyPath := filepath.Join(root, "src/backend/auth/src/test/resources/principal-tls/server-key.pem")
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	require.NoError(t, err)
	pem, err := os.ReadFile(certPath)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(pem))
	o := newUserStartup(context.Background())
	t.Cleanup(o.Close)
	options := o.options()
	options = append(options, grpc.Creds(credentials.NewTLS(&tls.Config{
		MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert},
		ClientCAs: roots, ClientAuth: tls.RequireAndVerifyClientCert,
	})), grpc.ChainUnaryInterceptor(socialprincipal.OrdinaryUnaryInterceptor("user")))
	srv := grpc.NewServer(options...)
	userv1.RegisterUserServiceServer(srv, &grpcsvc.UserGRPC{})
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	require.NoError(t, o.Serve(srv, lis))
	valid := &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "localhost", Certificates: []tls.Certificate{cert}}
	connect := func(t *testing.T, config *tls.Config, ready bool) *grpc.ClientConn {
		t.Helper()
		conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(config)))
		require.NoError(t, err)
		t.Cleanup(func() { _ = conn.Close() })
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		err = waitForRequiredGRPCReady(ctx, conn)
		if ready {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
		}
		return conn
	}
	client := userv1.NewUserServiceClient(connect(t, valid, true))
	call := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_, err := client.CaptureSpaceMemberAccountBindings(ctx, &userv1.CaptureSpaceMemberAccountBindingsRequest{})
		return err
	}
	require.Equal(t, codes.Unavailable, status.Code(call()))
	require.NoError(t, o.Release())
	require.Equal(t, codes.Unimplemented, status.Code(call()), "ordinary listener does not expose capture authority")
	for _, name := range []string{"wrong-ca", "wrong-name", "no-client-identity"} {
		t.Run(name, func(t *testing.T) {
			config := valid.Clone()
			switch name {
			case "wrong-ca":
				config.RootCAs = x509.NewCertPool()
			case "wrong-name":
				config.ServerName = "wrong.invalid"
			case "no-client-identity":
				config.Certificates = nil
			}
			connect(t, config, false)
		})
	}
}
