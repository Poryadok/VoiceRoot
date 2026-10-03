package principalgrpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	callsv1 "voice.app/voice/calls/v1"
)

func TestOrdinaryListenerFailsClosedForSpaceLifecycleMethods(t *testing.T) {
	interceptor := OrdinaryUnaryInterceptor()
	called := false
	_, err := interceptor(context.Background(), &callsv1.PurgeSpaceRequest{}, &grpc.UnaryServerInfo{FullMethod: callsv1.VoiceService_PurgeSpace_FullMethodName}, func(context.Context, any) (any, error) {
		called = true
		return &callsv1.PurgeSpaceResponse{}, nil
	})
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.False(t, called)
}
