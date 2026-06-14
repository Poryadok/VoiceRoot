package grpcsvc

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	callsv1 "voice.app/voice/calls/v1"
	spacev1 "voice.app/voice/space/v1"
)

const freeVoiceRoomCap = 32

// TestVoiceGRPCVoiceRoom_freeRejects33rdParticipant documents free tier voice room cap (32).
func TestVoiceGRPCVoiceRoom_freeRejects33rdParticipant(t *testing.T) {
	spaceID := uuid.New().String()
	voiceRoomID := uuid.New().String()
	members := map[string]map[string]bool{spaceID: {"profile-owner": true}}
	for i := 1; i <= freeVoiceRoomCap; i++ {
		members[spaceID][fmt.Sprintf("profile-%02d", i)] = true
	}
	svc := newTestVoiceService(fixedVoiceNow(), &recordingEvents{})
	svc.SpaceMembers = &mapSpaceMembers{members: members}
	join := &callsv1.JoinVoiceRoomRequest{
		VoiceRoomId: voiceRoomID,
		Space:       &spacev1.SpaceRef{Id: spaceID},
	}

	_, err := svc.JoinVoiceRoom(voiceTestCtx("profile-owner"), join)
	require.NoError(t, err)
	for i := 1; i < freeVoiceRoomCap; i++ {
		_, err = svc.JoinVoiceRoom(voiceTestCtx(fmt.Sprintf("profile-%02d", i)), join)
		require.NoError(t, err, "participant %d", i)
	}
	_, err = svc.JoinVoiceRoom(voiceTestCtx(fmt.Sprintf("profile-%02d", freeVoiceRoomCap)), join)
	require.Equal(t, codes.ResourceExhausted, status.Code(err))
}

// TestVoiceGRPCVoiceRoom_spaceProAllows33rdParticipant documents Space Pro voice cap (128).
func TestVoiceGRPCVoiceRoom_spaceProAllows33rdParticipant(t *testing.T) {
	spaceID := uuid.New().String()
	voiceRoomID := uuid.New().String()
	members := map[string]map[string]bool{spaceID: {"profile-owner": true}}
	for i := 1; i <= 32; i++ {
		members[spaceID][fmt.Sprintf("profile-%02d", i)] = true
	}
	svc := newTestVoiceService(fixedVoiceNow(), &recordingEvents{})
	svc.SpaceMembers = &mapSpaceMembers{members: members}
	svc.SpacePro = staticSpacePro{spaces: map[string]bool{spaceID: true}}
	join := &callsv1.JoinVoiceRoomRequest{
		VoiceRoomId: voiceRoomID,
		Space:       &spacev1.SpaceRef{Id: spaceID},
	}

	_, err := svc.JoinVoiceRoom(voiceTestCtx("profile-owner"), join)
	require.NoError(t, err)
	for i := 1; i < 32; i++ {
		_, err = svc.JoinVoiceRoom(voiceTestCtx(fmt.Sprintf("profile-%02d", i)), join)
		require.NoError(t, err, "participant %d", i)
	}
	_, err = svc.JoinVoiceRoom(voiceTestCtx("profile-32"), join)
	require.NoError(t, err)
}

func fixedVoiceNow() time.Time {
	return time.Unix(1700000000, 0).UTC()
}

type staticSpacePro struct {
	spaces map[string]bool
}

func (s staticSpacePro) HasSpacePro(_ context.Context, spaceID string) (bool, error) {
	return s.spaces[spaceID], nil
}
