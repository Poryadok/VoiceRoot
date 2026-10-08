package voiceuserprincipalruntime

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/pkg/principal"
	"voice/backend/voice/internal/principalgrpc"
)

type fakeVerifier struct {
	method, requestID, hash string
	calls                   int
}

func (v *fakeVerifier) Verify(_ context.Context, _, method, requestID, hash string) (principal.Principal, error) {
	v.calls++
	v.method, v.requestID, v.hash = method, requestID, hash
	return principal.Principal{Kind: "delegated_user", Issuer: "gateway", AccountID: "8a78bd68-75e9-4f21-9387-3bdc2f6116ab", ProfileID: "6e155399-76a3-4f78-9d28-1274e9b48585", SessionEpoch: 5}, nil
}

func TestStrictUnaryInterceptorBindsUserAndAllowsOnlyNamedMethod(t *testing.T) {
	verifier := &fakeVerifier{}
	request := &callsv1.JoinVoiceRoomRequest{VoiceRoomId: "room"}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer signed", "x-request-id", "request-1"))
	var got principal.Principal
	response, err := StrictUnaryInterceptor(verifier)(ctx, request, &grpc.UnaryServerInfo{FullMethod: callsv1.VoiceService_JoinVoiceRoom_FullMethodName}, func(ctx context.Context, req any) (any, error) {
		var ok bool
		got, ok = principal.FromContext(ctx)
		require.True(t, ok)
		require.Same(t, request, req)
		return "ok", nil
	})
	require.NoError(t, err)
	require.Equal(t, "ok", response)
	require.Equal(t, callsv1.VoiceService_JoinVoiceRoom_FullMethodName, verifier.method)
	require.Equal(t, "request-1", verifier.requestID)
	require.NotEmpty(t, verifier.hash)
	require.Equal(t, "8a78bd68-75e9-4f21-9387-3bdc2f6116ab", got.AccountID)
}

func TestStrictUnaryInterceptorBindsGetJoinToken(t *testing.T) {
	verifier := &fakeVerifier{}
	request := &callsv1.GetJoinTokenRequest{RoomId: "dm-room"}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer signed", "x-request-id", "token-request"))
	var got principal.Principal
	handlerCalled := false
	response, err := StrictUnaryInterceptor(verifier)(ctx, request, &grpc.UnaryServerInfo{FullMethod: callsv1.VoiceService_GetJoinToken_FullMethodName}, func(ctx context.Context, req any) (any, error) {
		handlerCalled = true
		var ok bool
		got, ok = principal.FromContext(ctx)
		require.True(t, ok)
		require.Same(t, request, req)
		return "ok", nil
	})
	require.NoError(t, err)
	require.Equal(t, "ok", response)
	require.True(t, handlerCalled)
	require.Equal(t, 1, verifier.calls)
	require.Equal(t, callsv1.VoiceService_GetJoinToken_FullMethodName, verifier.method)
	require.Equal(t, "token-request", verifier.requestID)
	expectedHash, err := principal.RequestHash(request)
	require.NoError(t, err)
	require.Equal(t, expectedHash, verifier.hash)
	require.Equal(t, "8a78bd68-75e9-4f21-9387-3bdc2f6116ab", got.AccountID)
	require.Equal(t, "6e155399-76a3-4f78-9d28-1274e9b48585", got.ProfileID)
	require.EqualValues(t, 5, got.SessionEpoch)
}

func TestStrictUnaryInterceptorRejectsMissingMetadataAndSiblingRPC(t *testing.T) {
	verifier := &fakeVerifier{}
	intercept := StrictUnaryInterceptor(verifier)
	request := &callsv1.JoinVoiceRoomRequest{}
	_, err := intercept(context.Background(), request, &grpc.UnaryServerInfo{FullMethod: callsv1.VoiceService_JoinVoiceRoom_FullMethodName}, func(context.Context, any) (any, error) { t.Fatal("missing metadata reached handler"); return nil, nil })
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = intercept(metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer signed", "x-request-id", "request-2")), request, &grpc.UnaryServerInfo{FullMethod: callsv1.VoiceService_StartCall_FullMethodName}, func(context.Context, any) (any, error) { t.Fatal("unlisted method reached handler"); return nil, nil })
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Zero(t, verifier.calls)
}

type unavailableVerifier struct{}

func (unavailableVerifier) Verify(context.Context, string, string, string, string) (principal.Principal, error) {
	return principal.Principal{}, principalgrpc.Unavailable(errors.New("fixture dependency unavailable"))
}

func TestStrictUnaryInterceptorMapsUnavailableVerifierAndDoesNotCallHandler(t *testing.T) {
	request := &callsv1.JoinVoiceRoomRequest{VoiceRoomId: "room"}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer signed", "x-request-id", "request-unavailable"))
	handlerCalled := false
	_, err := StrictUnaryInterceptor(unavailableVerifier{})(ctx, request,
		&grpc.UnaryServerInfo{FullMethod: callsv1.VoiceService_JoinVoiceRoom_FullMethodName},
		func(context.Context, any) (any, error) { handlerCalled = true; return nil, nil })
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.False(t, handlerCalled)
}
