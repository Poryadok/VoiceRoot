package socialprincipal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"sync"
	"testing"
	"time"
	"voice/backend/pkg/principal"
)

func TestMessagingDeletionStatesCapabilityIsExplicit(t *testing.T) {
	method := "/voice.user.v1.UserService/GetProfileDeletionStates"
	require.True(t, AllowsMethod("messaging", method), "Messaging compactor needs exact deletion-state authority")
	for _, other := range []string{"user", "auth", "auth-admission", "notification", "search", "file"} {
		require.False(t, AllowsMethod(other, method), other)
	}
}

func TestMessagingDeletionStatesVerifiesSignedBindingAndReplay(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	method := "/voice.user.v1.UserService/GetProfileDeletionStates"
	req := wrapperspb.String("profile-batch")
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	issue := func(issuer, audience, rpc, binding string) string {
		signer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: issuer, KeyID: "current", PrivateKey: key})
		require.NoError(t, err)
		token, err := signer.IssueService(principal.ServiceInput{Audience: audience, RPC: rpc, RequestID: "request", RequestHash: binding})
		require.NoError(t, err)
		return token
	}
	valid := issue("messaging", "user", method, hash)
	for _, tc := range []struct {
		name, token, id string
		extra           []string
		code            codes.Code
	}{
		{name: "valid", token: valid, id: "request", code: codes.OK},
		{name: "missing", code: codes.Unauthenticated},
		{name: "raw marker", token: valid, id: "request", extra: []string{"x-internal-caller", "messaging"}, code: codes.Unauthenticated},
		{name: "duplicate bearer", token: valid, id: "request", extra: []string{"authorization", "Bearer " + valid}, code: codes.Unauthenticated},
		{name: "duplicate request", token: valid, id: "request", extra: []string{"x-request-id", "request"}, code: codes.Unauthenticated},
		{name: "wrong request", token: valid, id: "other", code: codes.Unauthenticated},
		{name: "wrong hash", token: issue("messaging", "user", method, "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), id: "request", code: codes.Unauthenticated},
		{name: "wrong rpc", token: issue("messaging", "user", "/other", hash), id: "request", code: codes.Unauthenticated},
		{name: "wrong audience", token: issue("messaging", "elsewhere", method, hash), id: "request", code: codes.Unauthenticated},
		{name: "wrong issuer", token: issue("social", "user", method, hash), id: "request", code: codes.PermissionDenied},
	} {
		t.Run(tc.name, func(t *testing.T) {
			verifier := &Verifier{Target: "user", Capability: "messaging", Issuers: map[string]bool{"messaging": true, "social": true}, Resolve: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }}
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
				require.NoError(t, RequireMessagingPresence(ctx, method, req))
				return req, nil
			})
			require.Equal(t, tc.code, status.Code(err))
			require.Equal(t, tc.code == codes.OK, called)
		})
	}
	seen := sync.Map{}
	verifier := &Verifier{Target: "user", Capability: "messaging", Issuers: map[string]bool{"messaging": true}, Resolve: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(_ context.Context, issuer, jti string, _ time.Time) error {
		if _, exists := seen.LoadOrStore(issuer+jti, true); exists {
			return errors.New("replay")
		}
		return nil
	}}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+valid, "x-request-id", "request"))
	calls := 0
	handler := func(context.Context, any) (any, error) { calls++; return req, nil }
	_, err = StrictUnaryInterceptor(verifier)(ctx, req, &grpc.UnaryServerInfo{FullMethod: method}, handler)
	require.NoError(t, err)
	_, err = StrictUnaryInterceptor(verifier)(ctx, req, &grpc.UnaryServerInfo{FullMethod: method}, handler)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Equal(t, 1, calls)
}
