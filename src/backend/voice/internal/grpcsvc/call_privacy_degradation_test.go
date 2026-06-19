package grpcsvc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/pkg/privacy"

	callsv1 "voice.app/voice/calls/v1"
	chatv1 "voice.app/voice/chat/v1"
)

type failingCallPrivacyStub struct{}

func (failingCallPrivacyStub) AllowCallsAudience(context.Context, uuid.UUID) (privacy.Audience, error) {
	return privacy.Audience{}, errors.New("user grpc unavailable")
}

// TestStartCall_PrivacyDepsUnavailable_Internal documents OPERATIONS.md fail-closed on privacy S2S errors.
func TestStartCall_PrivacyDepsUnavailable_Internal(t *testing.T) {
	callerID := uuid.New().String()
	calleeID := uuid.New().String()

	now := time.Unix(1700000000, 0).UTC()
	svc := newTestVoiceService(now, &recordingEvents{})
	svc.Privacy = failingCallPrivacyStub{}

	_, err := svc.StartCall(voiceTestCtx(callerID), &callsv1.StartCallRequest{
		LinkedChat:      &chatv1.ChatRef{Id: uuid.NewString()},
		CalleeProfileId: strPtr(calleeID),
		MediaKind:       mediaPtr(callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO),
	})
	require.Error(t, err)
	require.Equal(t, codes.Internal, status.Code(err))
}
