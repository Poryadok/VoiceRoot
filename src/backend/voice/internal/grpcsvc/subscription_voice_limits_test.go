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
	voicestore "voice/backend/voice/internal/store"
)

const freeVoiceRoomCap = 32

// TestVoiceGRPCVoiceRoom_freeRejects33rdParticipant documents free tier voice room cap (32).
func TestVoiceGRPCVoiceRoom_freeRejects33rdParticipant(t *testing.T) {
	prepared := make([]voicestore.SpaceMediaAdmission, 0, freeVoiceRoomCap+1)
	aborting := make([]admissionTransition, 0, 1)
	released := make([]voicestore.SpaceMediaAdmission, 0, 1)
	cleaned := make([]admissionTransition, 0, 1)
	admissions := readySpaceMediaAdmission{
		preparedAdmissions: &prepared,
		abortingAdmissions: &aborting,
		releasedFences:     &released,
		cleanupAdmissions:  &cleaned,
	}
	svc, events, spaceID, _, profiles, join := freeVoiceRoomAtCapacity(t, admissions)
	startedBefore, joinedBefore := len(events.startedCall), len(events.memberJoined)

	_, err := joinSpaceVoiceUser(t, svc, profiles[freeVoiceRoomCap], join)
	require.Equal(t, codes.ResourceExhausted, status.Code(err))
	require.Len(t, prepared, freeVoiceRoomCap+1)
	failed := prepared[freeVoiceRoomCap]
	require.Len(t, aborting, 1)
	require.Equal(t, admissionTransition{operationID: failed.OperationID, generation: failed.Generation}, aborting[0])
	require.Len(t, released, 1)
	require.Equal(t, failed.OperationID, released[0].OperationID)
	require.Equal(t, failed.Generation, released[0].Generation)
	require.Len(t, cleaned, 1)
	require.Equal(t, aborting[0], cleaned[0])
	require.Len(t, events.startedCall, startedBefore, "denied cap operation emits no new call-start event")
	require.Len(t, events.memberJoined, joinedBefore, "denied cap operation emits no member-joined event")
	call, err := svc.Calls.GetCall(context.Background(), failed.RoomID)
	require.NoError(t, err)
	require.Len(t, call.SpaceMedia, freeVoiceRoomCap)
	require.NotContains(t, call.SpaceMedia, profiles[freeVoiceRoomCap])
	require.Equal(t, spaceID, call.SpaceID)
}

func TestVoiceGRPCVoiceRoom_freeCapCleanupFailureRemainsUnavailable(t *testing.T) {
	failures := 1
	prepared := make([]voicestore.SpaceMediaAdmission, 0, freeVoiceRoomCap+1)
	aborting := make([]admissionTransition, 0, 1)
	released := make([]voicestore.SpaceMediaAdmission, 0, 1)
	cleaned := make([]admissionTransition, 0, 2)
	admissions := readySpaceMediaAdmission{
		preparedAdmissions:      &prepared,
		abortingAdmissions:      &aborting,
		releasedFences:          &released,
		cleanupAdmissions:       &cleaned,
		cleanupFailuresToInject: &failures,
	}
	svc, events, _, _, profiles, join := freeVoiceRoomAtCapacity(t, admissions)
	startedBefore, joinedBefore := len(events.startedCall), len(events.memberJoined)

	_, err := joinSpaceVoiceUser(t, svc, profiles[freeVoiceRoomCap], join)
	require.Equal(t, codes.Unavailable, status.Code(err))
	failed := prepared[freeVoiceRoomCap]
	require.Len(t, aborting, 1)
	require.Len(t, released, 1)
	require.Len(t, cleaned, 1)
	require.Equal(t, failed.OperationID, cleaned[0].operationID)
	require.Equal(t, failed.Generation, cleaned[0].generation)
	require.Len(t, events.startedCall, startedBefore, "denied cap operation emits no new call-start event")
	require.Len(t, events.memberJoined, joinedBefore, "denied cap operation emits no member-joined event")
	call, err := svc.Calls.GetCall(context.Background(), failed.RoomID)
	require.NoError(t, err)
	require.Len(t, call.SpaceMedia, freeVoiceRoomCap)
	require.NotContains(t, call.SpaceMedia, profiles[freeVoiceRoomCap])

}

func freeVoiceRoomAtCapacity(t *testing.T, admissions readySpaceMediaAdmission) (*VoiceGRPC, *recordingEvents, string, string, []string, *callsv1.JoinVoiceRoomRequest) {
	t.Helper()
	spaceID := uuid.New().String()
	voiceRoomID := uuid.New().String()
	owner := fixtureProfileOwner
	members := map[string]map[string]bool{spaceID: {owner: true}}
	profiles := make([]string, freeVoiceRoomCap+1)
	for i := 1; i <= freeVoiceRoomCap; i++ {
		profiles[i] = fixtureProfileOrdinal(i)
		members[spaceID][profiles[i]] = true
	}
	events := &recordingEvents{}
	svc := newTestVoiceService(fixedVoiceNow(), events)
	svc.SpaceMembers = &mapSpaceMembers{members: members}
	configureReadySpaceMediaFixture(svc, fixtureSpaceMediaAccessResolver{rooms: map[string]string{voiceRoomID: spaceID}, members: members, accessEpoch: 11})
	svc.SpaceMediaAdmissions = admissions
	svc.Roles = &canonicalRolePermissions{}
	join := &callsv1.JoinVoiceRoomRequest{VoiceRoomId: voiceRoomID, Space: &spacev1.SpaceRef{Id: spaceID}}
	_, err := joinSpaceVoiceUser(t, svc, owner, join)
	require.NoError(t, err)
	for i := 1; i < freeVoiceRoomCap; i++ {
		_, err = joinSpaceVoiceUser(t, svc, profiles[i], join)
		require.NoError(t, err, "participant %d", i)
	}
	return svc, events, spaceID, voiceRoomID, profiles, join
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
