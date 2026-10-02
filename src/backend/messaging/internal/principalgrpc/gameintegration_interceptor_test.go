package principalgrpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/pkg/principal"
)

type gameIntegrationVerifierStub struct {
	principal principal.Principal
	err       error
	requestID string
	hash      string
	method    string
}

func (s *gameIntegrationVerifierStub) Verify(_ context.Context, _ string, method, requestID, hash string) (principal.Principal, error) {
	s.method, s.requestID, s.hash = method, requestID, hash
	return s.principal, s.err
}

func TestGameIntegrationListenerBindsExactGISPrincipalAndResultOperation(t *testing.T) {
	request := &messagingv1.ProjectGameActionResultRequest{MessageId: "m", ApplicationId: "a", EnvironmentId: "e", OperationId: "operation-1", ActionId: "act", ResultId: "result", StateVersion: "v2", Status: "succeeded", SafeSummary: "Done"}
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	method := messagingv1.MessagingService_ProjectGameActionResult_FullMethodName
	verifier := &gameIntegrationVerifierStub{principal: principal.Principal{Kind: "service", Issuer: "gameintegration", Subject: "service:gameintegration", Audience: "messaging", RPC: method, RequestID: "operation-1", RequestHash: hash}}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer token", "x-request-id", "operation-1"))
	called := false
	response, err := GameIntegrationStrictUnaryInterceptor(verifier)(ctx, request, &grpc.UnaryServerInfo{FullMethod: method}, func(ctx context.Context, _ any) (any, error) {
		called = true
		p, ok := principal.FromContext(ctx)
		require.True(t, ok)
		require.Equal(t, "gameintegration", p.Issuer)
		return "ok", nil
	})
	require.NoError(t, err)
	require.Equal(t, "ok", response)
	require.True(t, called)
	require.Equal(t, method, verifier.method)
	require.Equal(t, "operation-1", verifier.requestID)
	require.Equal(t, hash, verifier.hash)

	request.OperationId = "different-operation"
	ctx = metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer token", "x-request-id", "operation-1"))
	_, err = GameIntegrationStrictUnaryInterceptor(verifier)(ctx, request, &grpc.UnaryServerInfo{FullMethod: method}, func(context.Context, any) (any, error) {
		t.Fatal("mismatched request id reached handler")
		return nil, nil
	})
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestGameIntegrationMethodsFailClosedOutsideDedicatedListener(t *testing.T) {
	method := messagingv1.MessagingService_ResolveGameAction_FullMethodName
	_, err := GameIntegrationOrdinaryUnaryInterceptor()(context.Background(), &messagingv1.ResolveGameActionRequest{}, &grpc.UnaryServerInfo{FullMethod: method}, func(context.Context, any) (any, error) {
		t.Fatal("protected RPC reached ordinary listener")
		return nil, nil
	})
	require.Equal(t, codes.Unavailable, status.Code(err))
	purgeMethod := messagingv1.MessagingService_PurgeManagedChatContent_FullMethodName
	_, err = GameIntegrationOrdinaryUnaryInterceptor()(context.Background(), &messagingv1.PurgeManagedChatContentRequest{}, &grpc.UnaryServerInfo{FullMethod: purgeMethod}, func(context.Context, any) (any, error) {
		t.Fatal("managed chat purge reached ordinary listener")
		return nil, nil
	})
	require.Equal(t, codes.Unavailable, status.Code(err))
	_, err = GameIntegrationStrictUnaryInterceptor(&gameIntegrationVerifierStub{})(context.Background(), &messagingv1.PurgeManagedChatContentRequest{}, &grpc.UnaryServerInfo{FullMethod: purgeMethod}, func(context.Context, any) (any, error) {
		t.Fatal("managed chat purge was rejected by dedicated listener")
		return nil, nil
	})
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = GameIntegrationStrictUnaryInterceptor(&gameIntegrationVerifierStub{})(context.Background(), &messagingv1.ResolveGameActionRequest{}, &grpc.UnaryServerInfo{FullMethod: "/voice.messaging.v1.MessagingService/GetMessage"}, func(context.Context, any) (any, error) { t.Fatal("ordinary RPC reached GIS listener"); return nil, nil })
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
