package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
)

func TestCommunicationBootstrapAbsentAndPartialConfiguration(t *testing.T) {
	configs, err := loadCommunicationConfigs()
	require.NoError(t, err)
	require.Empty(t, configs)
	for _, prefix := range []string{"SPACE_VOICE_COMMUNICATION_", "SPACE_MESSAGING_COMMUNICATION_"} {
		t.Run(prefix, func(t *testing.T) {
			t.Setenv(prefix+"JWKS_CLIENT_CERT_FILE", "only-one-setting.pem")
			_, err := loadCommunicationConfigs()
			require.Error(t, err, "partial owned config must fail before opening a listener")
		})
	}
}

func TestOrdinaryCommunicationBoundaryRejectsBothProtectedMethods(t *testing.T) {
	for _, method := range []string{spacev1.SpaceService_ResolveVoiceRoomAccess_FullMethodName, spacev1.SpaceService_ResolveMemberCommunicationState_FullMethodName} {
		called := false
		_, err := ordinaryCommunicationUnary()(t.Context(), nil, &grpc.UnaryServerInfo{FullMethod: method}, func(context.Context, any) (any, error) { called = true; return nil, nil })
		require.Equal(t, codes.Unauthenticated, status.Code(err))
		require.False(t, called)
	}
}
