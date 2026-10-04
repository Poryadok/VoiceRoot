package matchsquadmemberprincipal

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/pkg/principal"
)

type verifierStub struct {
	value principal.Principal
	calls int
}

func (v *verifierStub) Verify(_ context.Context, _, method, requestID, requestHash string) (principal.Principal, error) {
	v.calls++
	v.value.RPC, v.value.RequestID, v.value.RequestHash = method, requestID, requestHash
	return v.value, nil
}

func TestStrictUnaryInterceptorAllowsOnlyBoundDelegatedMemberMethods(t *testing.T) {
	t.Parallel()
	operation := uuid.MustParse("00000000-0000-4000-8000-000000000001").String()
	request := &callsv1.JoinMatchSquadRoomRequest{ProtocolVersion: 1, OperationId: operation}
	hash, err := principal.RequestHash(request)
	if err != nil {
		t.Fatal(err)
	}
	verifier := &verifierStub{value: principal.Principal{
		Kind: "delegated_user", Issuer: issuer, Audience: audience,
		Subject: "00000000-0000-4000-8000-000000000010", AccountID: "00000000-0000-4000-8000-000000000010",
		ProfileID: "00000000-0000-4000-8000-000000000020", SessionEpoch: 9, ExpiresAt: time.Now().Add(time.Minute),
	}}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer opaque", "x-request-id", operation))
	called := false
	_, err = StrictUnaryInterceptor(verifier)(ctx, request, &grpc.UnaryServerInfo{FullMethod: JoinMethod}, func(ctx context.Context, _ any) (any, error) {
		called = true
		value, ok := principal.FromContext(ctx)
		if !ok || value.RequestHash != hash || value.RequestID != operation {
			t.Fatal("verified principal was not bound to the request")
		}
		return "ok", nil
	})
	if err != nil || !called || verifier.calls != 1 {
		t.Fatalf("join interceptor called=%v verifier calls=%d err=%v", called, verifier.calls, err)
	}
	_, err = StrictUnaryInterceptor(verifier)(ctx, request, &grpc.UnaryServerInfo{FullMethod: "/voice.calls.v1.MatchSquadMemberService/EndMatchSquadRoom"}, func(context.Context, any) (any, error) { return nil, nil })
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("unlisted method code=%v, want PermissionDenied", status.Code(err))
	}
	if verifier.calls != 1 {
		t.Fatalf("unlisted method reached verifier %d times", verifier.calls)
	}
}

func TestStrictUnaryInterceptorRejectsRawIdentityMetadataBeforeVerification(t *testing.T) {
	t.Parallel()
	operation := uuid.MustParse("00000000-0000-4000-8000-000000000001").String()
	verifier := &verifierStub{}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer opaque", "x-request-id", operation, "x-voice-profile-id", "spoofed"))
	_, err := StrictUnaryInterceptor(verifier)(ctx, &callsv1.JoinMatchSquadRoomRequest{OperationId: operation}, &grpc.UnaryServerInfo{FullMethod: JoinMethod}, func(context.Context, any) (any, error) { return nil, nil })
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("raw identity metadata code=%v, want Unauthenticated", status.Code(err))
	}
	if verifier.calls != 0 {
		t.Fatalf("raw identity metadata reached verifier %d times", verifier.calls)
	}
}
