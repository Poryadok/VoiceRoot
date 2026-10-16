package principalgrpc

import (
	"bytes"
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log/slog"
	"net"
	"net/url"
	"syscall"
	"testing"
	"time"
)

func TestDependencyFailureUsesOnlyClosedLabels(t *testing.T) {
	var out bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&out, nil)))
	defer slog.SetDefault(old)
	cases := []struct {
		err   error
		cause string
	}{
		{fmt.Errorf("secret-canary: %w", context.DeadlineExceeded), "timeout"},
		{&url.Error{Op: "GET", URL: "https://secret-canary", Err: x509.UnknownAuthorityError{}}, "tls_unknown_authority"},
		{x509.HostnameError{Host: "secret-canary"}, "tls_hostname"},
		{&net.DNSError{Err: "secret-canary", Name: "private-host"}, "dns"},
		{syscall.ECONNREFUSED, "connection_refused"},
		{status.Error(codes.Unavailable, "secret-canary"), "grpc_unavailable"},
		{errors.New("secret-canary"), "other"},
	}
	for _, tc := range cases {
		out.Reset()
		DependencyFailure(context.Background(), "jwks_request", tc.err, time.Now())
		require.Contains(t, out.String(), `"cause":"`+tc.cause+`"`)
		require.NotContains(t, out.String(), "secret-canary")
		require.NotContains(t, out.String(), "private-host")
	}
	out.Reset()
	DependencyFailure(context.Background(), "secret-canary", errors.New("secret-canary"), time.Now())
	require.Contains(t, out.String(), `"stage":"unknown"`)
	require.NotContains(t, out.String(), "secret-canary")
}

func TestDependencyHTTPStatusIsBoundedAndDistinct(t *testing.T) {
	var out bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&out, nil)))
	defer slog.SetDefault(old)
	for _, test := range []struct{ input, want int }{{503, 503}, {403, 403}, {99, 0}, {600, 0}, {-1, 0}} {
		out.Reset()
		DependencyHTTPFailure(context.Background(), test.input, time.Now())
		require.Contains(t, out.String(), `"stage":"jwks_status"`)
		require.Contains(t, out.String(), fmt.Sprintf(`"http_status":%d`, test.want))
	}
}
