package principalgrpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"log/slog"
	"net"
	"syscall"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/pkg/correlation"
)

// DependencyFailure records only closed diagnostic labels. Never pass through
// error text: transport errors can contain URLs, addresses or credentials.
// The caller retains its existing error mapping and fail-closed policy.
func DependencyFailure(ctx context.Context, stage string, err error, started time.Time) {
	dependencyFailure(ctx, stage, err, started, 0)
}

// DependencyHTTPFailure retains only a bounded response status, never headers or body.
func DependencyHTTPFailure(ctx context.Context, code int, started time.Time) {
	if code < 100 || code > 599 {
		code = 0
	}
	dependencyFailure(ctx, "jwks_status", nil, started, code)
}

func dependencyFailure(ctx context.Context, stage string, err error, started time.Time, httpStatus int) {
	switch stage {
	case "jwks_request", "jwks_status", "jwks_body", "jwks_parse", "jwks_keys",
		"replay_write", "auth_floor", "auth_floor_call", "auth_floor_sign", "auth_floor_response":
	default:
		stage = "unknown"
	}
	slog.WarnContext(ctx, "Voice trust dependency failed",
		"request_id", correlation.FromGRPC(ctx),
		"event", "voice_trust_dependency_failure", "stage", stage,
		"cause", dependencyCause(err), "duration_ms", time.Since(started).Milliseconds(),
		"http_status", httpStatus)
}

func dependencyCause(err error) string {
	var unknown x509.UnknownAuthorityError
	var hostname x509.HostnameError
	var invalid x509.CertificateInvalidError
	var tlsFailure *tls.CertificateVerificationError
	var dns *net.DNSError
	var network net.Error
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.As(err, &unknown):
		return "tls_unknown_authority"
	case errors.As(err, &hostname):
		return "tls_hostname"
	case errors.As(err, &invalid), errors.As(err, &tlsFailure):
		return "tls_certificate"
	case errors.As(err, &network) && network.Timeout():
		return "timeout"
	case errors.As(err, &dns):
		return "dns"
	case errors.Is(err, syscall.ECONNREFUSED):
		return "connection_refused"
	case errors.Is(err, syscall.ECONNRESET):
		return "connection_reset"
	}
	switch status.Code(err) {
	case codes.DeadlineExceeded:
		return "timeout"
	case codes.Canceled:
		return "canceled"
	case codes.Unavailable:
		return "grpc_unavailable"
	case codes.Unauthenticated:
		return "grpc_unauthenticated"
	case codes.PermissionDenied:
		return "grpc_permission_denied"
	}
	return "other"
}
