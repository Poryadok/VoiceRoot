package voiceuserprincipalruntime

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	callsv1 "voice.app/voice/calls/v1"
)

func TestAllowsOnlySpaceMediaAdmissionAndLeaveRPCs(t *testing.T) {
	for _, method := range []string{
		callsv1.VoiceService_JoinVoiceRoom_FullMethodName,
		callsv1.VoiceService_GetJoinToken_FullMethodName,
		callsv1.VoiceService_LeaveVoiceRoom_FullMethodName,
	} {
		require.True(t, AllowsMethod(method), method)
	}
	for _, method := range []string{
		callsv1.VoiceService_StartCall_FullMethodName,
		callsv1.VoiceService_GetVoiceStates_FullMethodName,
		callsv1.VoiceService_ApplySpaceLifecycleFence_FullMethodName,
	} {
		require.False(t, AllowsMethod(method), method)
	}
}

func TestMissingRuntimeFailsClosed(t *testing.T) {
	_, err := (*Runtime)(nil).Verify(context.Background(), "", callsv1.VoiceService_JoinVoiceRoom_FullMethodName, "request", "sha256:"+strings.Repeat("0", 64))
	require.Equal(t, codes.Unavailable, status.Code(err))
}
