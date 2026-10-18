package store

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
	callsv1 "voice.app/voice/calls/v1"
)

// Existing projection tests supply the new authority fixture explicitly. The
// regression below invokes the production writer without that preparation.
func createLifecycleCallForTest(ctx context.Context, s *RedisCallStore, call Call) (Call, error) {
	if call.SpaceID != "" {
		if err := s.SetSpaceLifecycle(ctx, call.SpaceID, call.SpaceLifecycleGeneration, true); err != nil {
			return Call{}, err
		}
	}
	return s.CreateCall(ctx, call)
}

func TestRedisSpaceLifecycleRejectsMissingDenyAndStaleGeneration(t *testing.T) {
	ctx := t.Context()
	s, client := newRedisCallStoreForTest(t, "p3:")
	call := Call{RoomID: "call", SpaceID: "space", VoiceRoomID: "voice", SessionKind: callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM, States: map[string]ParticipantState{}, Status: callsv1.CallStatus_CALL_STATUS_UNSPECIFIED}
	_, err := s.CreateCall(ctx, call)
	require.ErrorIs(t, err, ErrSpaceMediaStaleGrant, "missing watermark is not initial LIVE")
	require.NoError(t, s.SetSpaceLifecycle(ctx, "space", 0, true))
	require.NoError(t, s.SetSpaceLifecycle(ctx, "space", 1, false))
	require.ErrorIs(t, s.SetSpaceLifecycle(ctx, "space", 0, true), ErrSpaceMediaStaleGrant)
	_, err = s.CreateCall(ctx, call)
	require.ErrorIs(t, err, ErrSpaceMediaStaleGrant, "delayed pre-freeze effect cannot recreate shell")
	require.NoError(t, s.SetSpaceLifecycle(ctx, "space", 2, true))
	call.SpaceLifecycleGeneration = 2
	_, err = s.CreateCall(ctx, call)
	require.NoError(t, err)
	require.NoError(t, client.Del(ctx, s.spaceLifecycleKey("space")).Err())
	p := testSpaceMediaParticipant("member", "identity", "generation", 1, 1)
	p.LifecycleGeneration = 2
	_, err = s.AdmitSpaceMediaParticipant(ctx, call.RoomID, p, MaxVoiceRoomParticipants)
	require.ErrorIs(t, err, ErrSpaceMediaStaleGrant, "Redis loss requires authority restoration")
}
