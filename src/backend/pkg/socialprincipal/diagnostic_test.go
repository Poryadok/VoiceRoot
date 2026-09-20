package socialprincipal

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestClassifyVerificationError(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want VerificationReason
	}{
		{"issuer", errors.New("untrusted issuer"), ReasonIssuer},
		{"unknown kid", errors.New("resolve signing key: jwks kid is unknown"), ReasonUnknownKID},
		{"tls", errors.New("fetch jwks: tls: failed to verify certificate"), ReasonJWKSTLS},
		{"document", errors.New("invalid principal JWKS response"), ReasonJWKSDocument},
		{"signature", errors.New("crypto/rsa: verification error"), ReasonSignature},
		{"binding", errors.New("credential binding mismatch"), ReasonBinding},
		{"temporal", errors.New("credential temporal claims invalid"), ReasonTemporal},
		{"replay", errors.New("replay guard: principal replay detected"), ReasonReplay},
		{"unknown", errors.New("malformed credential"), ReasonUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) { require.Equal(t, tc.want, classifyVerificationError(tc.err)) })
	}
}

func TestStrictUnaryInterceptorKeepsDiagnosticInternal(t *testing.T) {
	var got VerificationReason
	verifier := &Verifier{Diagnostic: func(reason VerificationReason) { got = reason }}
	_, err := StrictUnaryInterceptor(verifier)(context.Background(), nil,
		&grpc.UnaryServerInfo{FullMethod: Method("file")}, func(context.Context, any) (any, error) { t.Fatal("handler called"); return nil, nil })
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Equal(t, "invalid principal", status.Convert(err).Message())
	require.Equal(t, ReasonMetadata, got)
}
