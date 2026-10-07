package grpcsvc

import (
	"context"
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
	owner := fixtureProfileOwner
	members := map[string]map[string]bool{spaceID: {owner: true}}
	profiles := make([]string, freeVoiceRoomCap+1)
	for i := 1; i <= freeVoiceRoomCap; i++ {
		profiles[i] = fixtureProfileOrdinal(i)
		members[spaceID][profiles[i]] = true
	}
	svc := newTestVoiceService(fixedVoiceNow(), &recordingEvents{})
	svc.SpaceMembers = &mapSpaceMembers{members: members}
	configureReadySpaceMediaFixture(svc, fixtureSpaceMediaAccessResolver{rooms: map[string]string{voiceRoomID: spaceID}, members: members, accessEpoch: 11})
	svc.Roles = &canonicalRolePermissions{}
	join := &callsv1.JoinVoiceRoomRequest{
		VoiceRoomId: voiceRoomID,
		Space:       &spacev1.SpaceRef{Id: spaceID},
	}

	_, err := joinSpaceVoiceUser(t, svc, owner, join)
	require.NoError(t, err)
	for i := 1; i < freeVoiceRoomCap; i++ {
		_, err = joinSpaceVoiceUser(t, svc, profiles[i], join)
		require.NoError(t, err, "participant %d", i)
	}
	_, err = joinSpaceVoiceUser(t, svc, profiles[freeVoiceRoomCap], join)
	require.Equal(t, codes.ResourceExhausted, status.Code(err))
}

// TestVoiceGRPCVoiceRoom_spaceProAllows33rdParticipant documents Space Pro voice cap (128).
func TestVoiceGRPCVoiceRoom_spaceProAllows33rdParticipant(t *testing.T) {
	spaceID := uuid.New().String()
	voiceRoomID := uuid.New().String()
	owner := fixtureProfileOwner
	members := map[string]map[string]bool{spaceID: {owner: true}}
	profiles := make([]string, 33)
	for i := 1; i <= 32; i++ {
		profiles[i] = fixtureProfileOrdinal(i)
		members[spaceID][profiles[i]] = true
	}
	svc := newTestVoiceService(fixedVoiceNow(), &recordingEvents{})
	svc.SpaceMembers = &mapSpaceMembers{members: members}
	configureReadySpaceMediaFixture(svc, fixtureSpaceMediaAccessResolver{rooms: map[string]string{voiceRoomID: spaceID}, members: members, accessEpoch: 11})
	svc.Roles = &canonicalRolePermissions{}
	svc.SpacePro = staticSpacePro{spaces: map[string]bool{spaceID: true}}
	join := &callsv1.JoinVoiceRoomRequest{
		VoiceRoomId: voiceRoomID,
		Space:       &spacev1.SpaceRef{Id: spaceID},
	}

	_, err := joinSpaceVoiceUser(t, svc, owner, join)
	require.NoError(t, err)
	for i := 1; i < 32; i++ {
		_, err = joinSpaceVoiceUser(t, svc, profiles[i], join)
		require.NoError(t, err, "participant %d", i)
	}
	_, err = joinSpaceVoiceUser(t, svc, profiles[32], join)
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
