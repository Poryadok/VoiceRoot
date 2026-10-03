package principalgrpc

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	botv1 "voice.app/voice/bot/v1"
	"voice/backend/pkg/principal"
)

type lifecycleVerifier struct {
	gotMethod, gotRequestID, gotHash string
	err                              error
}

func (v *lifecycleVerifier) Verify(_ context.Context, token, method, requestID, hash string) (principal.Principal, error) {
	return principal.Principal{}, status.Error(codes.Unauthenticated, "Game Integration verifier does not accept Space principals")
}

func (v *lifecycleVerifier) VerifySpace(_ context.Context, token, method, requestID, hash string) (principal.Principal, error) {
	if token != "space-token" {
		return principal.Principal{}, status.Error(codes.Unauthenticated, "bad token")
	}
	v.gotMethod, v.gotRequestID, v.gotHash = method, requestID, hash
	if v.err != nil {
		return principal.Principal{}, v.err
	}
	return principal.Principal{Kind: "service", Issuer: "space", Subject: "service:space", Audience: "bot", RPC: method, RequestID: requestID, RequestHash: hash}, nil
}

func TestSpaceLifecycleUnaryInterceptorBindsVerifiedPrincipalToExactRequest(t *testing.T) {
	verifier := &lifecycleVerifier{}
	request := &botv1.ApplySpaceLifecycleFenceRequest{}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer space-token", "x-request-id", "req-1"))
	called := false
	_, err := SpaceLifecycleUnaryInterceptor(verifier)(ctx, request, &grpc.UnaryServerInfo{FullMethod: botv1.BotService_ApplySpaceLifecycleFence_FullMethodName}, func(ctx context.Context, got any) (any, error) {
		called = true
		if got != request {
			t.Fatal("handler request changed")
		}
		verified, ok := principal.FromContext(ctx)
		if !ok || verified.Issuer != "space" || verified.RequestHash != verifier.gotHash {
			t.Fatalf("verified principal missing from handler context: %+v, %v", verified, ok)
		}
		return nil, nil
	})
	if err != nil || !called {
		t.Fatalf("lifecycle interceptor: called=%v err=%v", called, err)
	}
	if verifier.gotMethod != botv1.BotService_ApplySpaceLifecycleFence_FullMethodName || verifier.gotRequestID != "req-1" || verifier.gotHash == "" {
		t.Fatalf("verifier did not receive exact request binding: %+v", verifier)
	}
}

func TestSpaceLifecycleUnaryInterceptorRejectsOtherMethodsAndMissingMetadata(t *testing.T) {
	for _, tc := range []struct {
		name, method string
		ctx          context.Context
	}{
		{name: "other method", method: botv1.BotService_PublishGameEvent_FullMethodName, ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer space-token", "x-request-id", "req-1"))},
		{name: "missing metadata", method: botv1.BotService_PurgeSpace_FullMethodName, ctx: context.Background()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			_, err := SpaceLifecycleUnaryInterceptor(&lifecycleVerifier{})(tc.ctx, &botv1.PurgeSpaceRequest{}, &grpc.UnaryServerInfo{FullMethod: tc.method}, func(context.Context, any) (any, error) {
				called = true
				return nil, nil
			})
			if called || status.Code(err) != codes.Unauthenticated && status.Code(err) != codes.Unimplemented {
				t.Fatalf("unexpected result: called=%v code=%v err=%v", called, status.Code(err), err)
			}
		})
	}
}

func TestSpaceLifecycleUnaryInterceptorMapsVerifierUnavailable(t *testing.T) {
	verifier := &lifecycleVerifier{err: Unavailable(status.Error(codes.Unavailable, "jwks unavailable"))}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer space-token", "x-request-id", "req-1"))
	_, err := SpaceLifecycleUnaryInterceptor(verifier)(ctx, &botv1.PurgeSpaceRequest{}, &grpc.UnaryServerInfo{FullMethod: botv1.BotService_PurgeSpace_FullMethodName}, func(context.Context, any) (any, error) { return nil, nil })
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("unavailable verifier should remain retryable: %v", err)
	}
}

func TestOrdinaryUnaryInterceptorHidesProtectedServiceMethods(t *testing.T) {
	for _, method := range []string{botv1.BotService_PublishGameEvent_FullMethodName, botv1.BotService_ApplySpaceLifecycleFence_FullMethodName, botv1.BotService_PurgeSpace_FullMethodName} {
		called := false
		_, err := OrdinaryUnaryInterceptor()(context.Background(), &botv1.PurgeSpaceRequest{}, &grpc.UnaryServerInfo{FullMethod: method}, func(context.Context, any) (any, error) {
			called = true
			return nil, nil
		})
		if called || status.Code(err) != codes.Unavailable {
			t.Fatalf("lifecycle method %s reached ordinary listener: called=%v err=%v", method, called, err)
		}
	}
}
