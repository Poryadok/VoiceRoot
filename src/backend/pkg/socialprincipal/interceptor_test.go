package socialprincipal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"voice/backend/pkg/principal"
)

func TestProtectedBoundary(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	for _, target := range []string{"user", "space"} {
		t.Run(target, func(t *testing.T) {
			req := wrapperspb.String("profile")
			hash, err := principal.RequestHash(req)
			require.NoError(t, err)
			method := Method(target)
			issue := func(issuer, audience, rpc, requestID, binding string, signingKey *rsa.PrivateKey) string {
				signer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: issuer, KeyID: "current", PrivateKey: signingKey})
				require.NoError(t, err)
				token, err := signer.IssueService(principal.ServiceInput{Audience: audience, RPC: rpc, RequestID: requestID, RequestHash: binding})
				require.NoError(t, err)
				return token
			}
			token := issue("social", target, method, "request", hash, key)
			for _, tc := range []struct {
				name, token, id string
				extra           []string
				code            codes.Code
			}{
				{name: "valid", token: token, id: "request", code: codes.OK},
				{name: "missing", code: codes.Unauthenticated},
				{name: "forged", token: issue("social", target, method, "request", hash, other), id: "request", code: codes.Unauthenticated},
				{name: "wrong audience", token: issue("social", "elsewhere", method, "request", hash, key), id: "request", code: codes.Unauthenticated},
				{name: "wrong rpc", token: issue("social", target, "/other", "request", hash, key), id: "request", code: codes.Unauthenticated},
				{name: "wrong request id", token: token, id: "other", code: codes.Unauthenticated},
				{name: "wrong hash", token: issue("social", target, method, "request", "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", key), id: "request", code: codes.Unauthenticated},
				{name: "raw", token: token, id: "request", extra: []string{"x-voice-internal-caller", "social"}, code: codes.Unauthenticated},
				{name: "duplicate bearer", token: token, id: "request", extra: []string{"authorization", "Bearer " + token}, code: codes.Unauthenticated},
				{name: "duplicate request", token: token, id: "request", extra: []string{"x-request-id", "request"}, code: codes.Unauthenticated},
				{name: "verified nonSocial", token: issue("chat", target, method, "request", hash, key), id: "request", code: codes.PermissionDenied},
			} {
				t.Run(tc.name, func(t *testing.T) {
					verifier := &Verifier{Target: target, Issuers: map[string]bool{"social": true, "chat": true}, Resolve: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }}
					md := metadata.MD{}
					if tc.token != "" {
						md.Set("authorization", "Bearer "+tc.token)
					}
					if tc.id != "" {
						md.Set("x-request-id", tc.id)
					}
					md = metadata.Join(md, metadata.Pairs(tc.extra...))
					called := false
					_, err := StrictUnaryInterceptor(verifier)(metadata.NewIncomingContext(context.Background(), md), req, &grpc.UnaryServerInfo{FullMethod: method}, func(ctx context.Context, _ any) (any, error) {
						called = true
						require.NoError(t, RequireSocial(ctx, target, req))
						return req, nil
					})
					require.Equal(t, tc.code, status.Code(err))
					require.Equal(t, tc.code == codes.OK, called)
				})
			}
			seen := sync.Map{}
			verifier := &Verifier{Target: target, Issuers: map[string]bool{"social": true}, Resolve: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(_ context.Context, issuer, jti string, _ time.Time) error {
				if _, exists := seen.LoadOrStore(issuer+jti, true); exists {
					return errors.New("replay")
				}
				return nil
			}}
			ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", "request"))
			calls := 0
			handler := func(context.Context, any) (any, error) { calls++; return req, nil }
			_, err = StrictUnaryInterceptor(verifier)(ctx, req, &grpc.UnaryServerInfo{FullMethod: method}, handler)
			require.NoError(t, err)
			_, err = StrictUnaryInterceptor(verifier)(ctx, req, &grpc.UnaryServerInfo{FullMethod: method}, handler)
			require.Equal(t, codes.Unauthenticated, status.Code(err))
			require.Equal(t, 1, calls)
		})
	}
}

func TestOrdinaryRejectsSocialOnlyOnPrivacy(t *testing.T) {
	for _, target := range []string{"user", "space"} {
		for _, marker := range []string{"social", " social ", "SOCIAL"} {
			ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-internal-caller", marker))
			calls := 0
			handler := func(context.Context, any) (any, error) { calls++; return nil, nil }
			_, err := OrdinaryUnaryInterceptor(target)(ctx, nil, &grpc.UnaryServerInfo{FullMethod: Method(target)}, handler)
			require.Equal(t, codes.Unauthenticated, status.Code(err))
			require.Zero(t, calls)
			_, err = OrdinaryUnaryInterceptor(target)(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/voice.user.v1.UserService/GetProfile"}, handler)
			require.NoError(t, err)
			require.Equal(t, 1, calls)
		}
	}
}
