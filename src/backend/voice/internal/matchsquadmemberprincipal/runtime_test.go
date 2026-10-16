package matchsquadmemberprincipal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/pkg/principal"
)

func TestColdJWKSFailureRejectsMemberCallAndLaterKeyCanResolve(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	e := big.NewInt(int64(privateKey.PublicKey.E)).Bytes()
	document, err := json.Marshal(map[string]any{"keys": []any{map[string]any{
		"kty": "RSA", "kid": "current", "use": "sig", "alg": "RS256",
		"n": base64.RawURLEncoding.EncodeToString(privateKey.PublicKey.N.Bytes()),
		"e": base64.RawURLEncoding.EncodeToString(e),
	}}})
	if err != nil {
		t.Fatal(err)
	}

	available := false
	resolver, err := principal.NewJWKSResolverWithConfig(principal.JWKSResolverConfig{
		Fetch: func(context.Context, string) ([]byte, error) {
			if !available {
				return nil, errors.New("fixture JWKS unavailable")
			}
			return document, nil
		},
		RefreshAfter: time.Minute, HardExpiry: 2 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	runtime := &Runtime{resolver: resolver, redis: redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", MaxRetries: -1})}
	defer func() { _ = runtime.redis.Close() }()

	requestID := "00000000-0000-4000-8000-000000000001"
	request := &callsv1.JoinMatchSquadRoomRequest{ProtocolVersion: 1, OperationId: requestID}
	// This token has a parseable issuer and kid so key resolution is reached;
	// the unavailable cold cache must reject it before signature/replay/handler work.
	token := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","kid":"current"}`)) + "." +
		base64.RawURLEncoding.EncodeToString([]byte(`{"iss":"gateway"}`)) + ".invalid"
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"authorization", "Bearer "+token, "x-request-id", requestID,
	))
	called := false
	_, err = StrictUnaryInterceptor(runtime)(ctx, request, &grpc.UnaryServerInfo{FullMethod: JoinMethod}, func(context.Context, any) (any, error) {
		called = true
		return nil, nil
	})
	if status.Code(err) != codes.Unavailable || called {
		t.Fatalf("cold JWKS call code=%v handler called=%v; want unavailable before handler", status.Code(err), called)
	}

	available = true
	key, err := runtime.resolveKey(context.Background(), issuer, "current")
	if err != nil || key == nil || key.N.Cmp(privateKey.PublicKey.N) != 0 {
		t.Fatalf("resolver did not recover after JWKS became available: key=%v err=%v", key != nil, err)
	}
	if _, err := runtime.resolveKey(context.Background(), "untrusted", "current"); err == nil {
		t.Fatal("untrusted issuer resolved after recovery")
	}
}
