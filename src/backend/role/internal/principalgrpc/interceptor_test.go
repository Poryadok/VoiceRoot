package principalgrpc

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"voice/backend/pkg/principal"
)

func TestStrictUnaryInterceptor_OnlyAcceptsStrictMetadataAndBindsDeterministicRequestHash(t *testing.T) {
	var got tokenVerification
	interceptor := StrictUnaryInterceptor(verifierFunc(func(_ context.Context, token, method, requestID, requestHash string) (principal.Principal, error) {
		got = tokenVerification{token, method, requestID, requestHash}
		return principal.Principal{Issuer: "gateway", ProfileID: "profile-1"}, nil
	}))
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer signed-token", "x-request-id", "request-1"))
	_, err := interceptor(ctx, &emptypb.Empty{}, &grpc.UnaryServerInfo{FullMethod: "/voice.role.v1.RoleService/ListRoles"}, func(ctx context.Context, _ any) (any, error) {
		if p, ok := principal.FromContext(ctx); !ok || p.ProfileID != "profile-1" {
			t.Fatal("verified principal missing from handler context")
		}
		return &emptypb.Empty{}, nil
	})
	if err != nil {
		t.Fatalf("interceptor error = %v", err)
	}
	if got.token != "signed-token" || got.method != "/voice.role.v1.RoleService/ListRoles" || got.requestID != "request-1" || got.requestHash == "" {
		t.Fatalf("verification binding = %#v", got)
	}
}

func TestStrictUnaryInterceptor_RejectsUnsafeMetadataAndNeverCallsVerifier(t *testing.T) {
	called := false
	interceptor := StrictUnaryInterceptor(verifierFunc(func(context.Context, string, string, string, string) (principal.Principal, error) {
		called = true
		return principal.Principal{}, nil
	}))
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer signed-token", "x-request-id", "request-1", "x-voice-profile-id", "forged"))
	_, err := interceptor(ctx, &emptypb.Empty{}, &grpc.UnaryServerInfo{FullMethod: "/voice.role.v1.RoleService/ListRoles"}, func(context.Context, any) (any, error) { return nil, nil })
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("code = %v, error=%v", status.Code(err), err)
	}
	if called {
		t.Fatal("verifier called after strict metadata rejection")
	}
}

func TestStrictUnaryInterceptor_MapsVerificationFailureWithoutCredentialDetails(t *testing.T) {
	interceptor := StrictUnaryInterceptor(verifierFunc(func(context.Context, string, string, string, string) (principal.Principal, error) {
		return principal.Principal{}, errors.New("token secret should not leak")
	}))
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer signed-token", "x-request-id", "request-1"))
	_, err := interceptor(ctx, &emptypb.Empty{}, &grpc.UnaryServerInfo{FullMethod: "/voice.role.v1.RoleService/ListRoles"}, func(context.Context, any) (any, error) { return nil, nil })
	if status.Code(err) != codes.Unauthenticated || status.Convert(err).Message() != "invalid principal" {
		t.Fatalf("generic auth failure = %v", err)
	}
}

type tokenVerification struct{ token, method, requestID, requestHash string }
type verifierFunc func(context.Context, string, string, string, string) (principal.Principal, error)

func (f verifierFunc) Verify(ctx context.Context, token, method, requestID, requestHash string) (principal.Principal, error) {
	return f(ctx, token, method, requestID, requestHash)
}
