package grpcsvc

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/voice/internal/livekit"
	voicestore "voice/backend/voice/internal/store"

	spacev1 "voice.app/voice/space/v1"
	callsv1 "voice.app/voice/calls/v1"
)

func TestVoiceGRPC_StartStopScreenShare(t *testing.T) {
	t.Parallel()
	now := time.Unix(1700000000, 0).UTC()
	events := &recordingEvents{}
	svc := newTestVoiceService(now, events)
	store := svc.Calls.(*voicestore.MemoryCallStore)
	ctx := voiceTestCtx("profile-a")

	_, err := store.CreateCall(ctx, voicestore.Call{
		RoomID:             "room-ss",
		LivekitRoomName:    "voice-dm-room-ss",
		ChatID:             "chat-1",
		InitiatorProfileID: "profile-a",
		CalleeProfileID:    "profile-b",
		MediaKind:          callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO,
		Status:             callsv1.CallStatus_CALL_STATUS_ACTIVE,
		StartedAt:          now,
	})
	require.NoError(t, err)

	startResp, err := svc.StartScreenShare(ctx, &callsv1.StartScreenShareRequest{RoomId: "room-ss"})
	require.NoError(t, err)
	require.NotEmpty(t, startResp.GetScreenShareSession().GetStreamId())
	require.Len(t, events.started, 1)
	require.Equal(t, "room-ss", events.started[0].GetRoomId())
	require.Equal(t, "profile-a", events.started[0].GetProfileId())

	states, err := svc.GetVoiceStates(ctx, &callsv1.GetVoiceStatesRequest{RoomId: "room-ss"})
	require.NoError(t, err)
	require.True(t, states.GetParticipants()[0].GetIsScreenSharing())

	_, err = svc.StopScreenShare(ctx, &callsv1.StopScreenShareRequest{RoomId: "room-ss"})
	require.NoError(t, err)
	require.Len(t, events.stopped, 1)

	states, err = svc.GetVoiceStates(ctx, &callsv1.GetVoiceStatesRequest{RoomId: "room-ss"})
	require.NoError(t, err)
	for _, p := range states.GetParticipants() {
		if p.GetProfileId() == "profile-a" {
			require.False(t, p.GetIsScreenSharing())
		}
	}
}

func TestVoiceGRPC_StartScreenShare_LimitAndPermission(t *testing.T) {
	t.Parallel()
	now := time.Unix(1700000000, 0).UTC()
	events := &recordingEvents{}
	svc := newTestVoiceService(now, events)
	store := svc.Calls.(*voicestore.MemoryCallStore)
	ctx := voiceTestCtx("profile-owner")

	_, err := store.CreateCall(ctx, voicestore.Call{
		RoomID:             "room-limit",
		LivekitRoomName:    "voice-group-limit",
		ChatID:             "group-1",
		SessionKind:        callsv1.VoiceSessionKind_VOICE_SESSION_KIND_GROUP_VOICE,
		InitiatorProfileID: "profile-owner",
		MediaKind:          callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO,
		Status:             callsv1.CallStatus_CALL_STATUS_ACTIVE,
		StartedAt:          now,
		States: map[string]voicestore.ParticipantState{
			"profile-owner": {ProfileID: "profile-owner"},
			"profile-2":     {ProfileID: "profile-2"},
			"profile-3":     {ProfileID: "profile-3"},
			"profile-4":     {ProfileID: "profile-4"},
		},
	})
	require.NoError(t, err)

	for _, id := range []string{"profile-owner", "profile-2", "profile-3"} {
		pctx := voiceTestCtx(id)
		_, err := svc.StartScreenShare(pctx, &callsv1.StartScreenShareRequest{RoomId: "room-limit"})
		require.NoError(t, err)
	}

	_, err = svc.StartScreenShare(voiceTestCtx("profile-4"), &callsv1.StartScreenShareRequest{RoomId: "room-limit"})
	require.Equal(t, codes.ResourceExhausted, status.Code(err))

	_, err = svc.StartScreenShare(voiceTestCtx("outsider"), &callsv1.StartScreenShareRequest{RoomId: "room-limit"})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestVoiceGRPC_StartScreenShare_VoiceRoomRoleCheck(t *testing.T) {
	t.Parallel()
	now := time.Unix(1700000000, 0).UTC()
	events := &recordingEvents{}
	svc := &VoiceGRPC{
		Calls:  voicestore.NewMemoryCallStore(),
		Tokens: livekit.NewHS256TokenIssuer("dev-key", "dev-secret", "ws://livekit:7880", time.Hour),
		Events: events,
		Now:    func() time.Time { return now },
		Roles: &mapRolePermissions{allowed: map[string]map[string]bool{
			"space-1": {"profile-allowed": true},
		}},
	}
	store := svc.Calls.(*voicestore.MemoryCallStore)
	_, err := store.CreateCall(context.Background(), voicestore.Call{
		RoomID:             "room-vr",
		LivekitRoomName:    "voice-room-vr",
		VoiceRoomID:        "vr-1",
		SpaceID:            "space-1",
		SessionKind:        callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM,
		InitiatorProfileID: "profile-allowed",
		MediaKind:          callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO,
		Status:             callsv1.CallStatus_CALL_STATUS_ACTIVE,
		StartedAt:          now,
	})
	require.NoError(t, err)

	_, err = svc.StartScreenShare(voiceTestCtx("profile-allowed"), &callsv1.StartScreenShareRequest{RoomId: "room-vr"})
	require.NoError(t, err)

	_, err = store.AddParticipant(context.Background(), "room-vr", "profile-denied", voicestore.MaxVoiceRoomParticipants)
	require.NoError(t, err)
	_, err = svc.StartScreenShare(voiceTestCtx("profile-denied"), &callsv1.StartScreenShareRequest{RoomId: "room-vr"})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

type screenShareLeaveAdmission struct {
	readySpaceMediaAdmission
	beginOperations  []uuid.UUID
	beginGenerations []string
	completed        []voicestore.SpaceMediaAdmission
}

func (a *screenShareLeaveAdmission) BeginRoomLeave(_ context.Context, operationID uuid.UUID, generation string) error {
	a.beginOperations = append(a.beginOperations, operationID)
	a.beginGenerations = append(a.beginGenerations, generation)
	return nil
}

func (a *screenShareLeaveAdmission) CompleteRoomLeave(_ context.Context, admission voicestore.SpaceMediaAdmission) error {
	a.completed = append(a.completed, admission)
	return nil
}

type screenShareLeaveRevocationStore interface {
	voicestore.CallStore
	BeginSpaceMediaRevocation(context.Context, string, string, string, string) (voicestore.Call, bool, error)
	CompleteSpaceMediaRevocation(context.Context, string, string, string, string) (voicestore.Call, bool, error)
}

type screenShareLeaveMedia struct {
	roomName            string
	identity            string
	roomID              string
	spaceID             string
	voiceRoomID         string
	participant         voicestore.SpaceMediaParticipant
	calls               screenShareLeaveRevocationStore
	admissions          *screenShareLeaveAdmission
	participantRevoking bool
	removedIdentities   []string
}

func (m *screenShareLeaveMedia) RemoveParticipant(ctx context.Context, roomName, identity string) error {
	if roomName != m.roomName || identity != m.identity || len(m.admissions.beginOperations) != 1 || len(m.admissions.completed) != 0 {
		return status.Error(codes.FailedPrecondition, "test media removal requires the exact room identity")
	}
	currentCall, err := m.calls.GetCall(ctx, m.roomID)
	if err != nil {
		return err
	}
	current, ok := currentCall.SpaceMedia[m.participant.ProfileID]
	if currentCall.SpaceID != m.spaceID || currentCall.VoiceRoomID != m.voiceRoomID || !ok ||
		current.ProfileID != m.participant.ProfileID || current.AccountID != m.participant.AccountID ||
		current.Identity != m.participant.Identity || current.AdmissionOperationID != m.participant.AdmissionOperationID ||
		current.RoomGeneration != m.participant.RoomGeneration || current.Generation != m.participant.Generation || !current.Revoking {
		return status.Error(codes.FailedPrecondition, "test media removal requires the exact revoking participant")
	}
	m.participantRevoking = true
	m.removedIdentities = append(m.removedIdentities, identity)
	return nil
}

type screenShareLeaveRevoker struct {
	calls           screenShareLeaveRevocationStore
	media           *screenShareLeaveMedia
	spaceID         string
	voiceRoomID     string
	profileID       string
	removedProfiles []string
}

func (r *screenShareLeaveRevoker) RevokeSpaceMediaParticipant(ctx context.Context, call voicestore.Call, participant voicestore.SpaceMediaParticipant) (voicestore.Call, bool, error) {
	current, ok := call.SpaceMedia[r.profileID]
	if call.SpaceID != r.spaceID || call.VoiceRoomID != r.voiceRoomID || participant.ProfileID != r.profileID || !ok ||
		current.ProfileID != participant.ProfileID || current.AccountID != participant.AccountID ||
		current.Identity != participant.Identity || current.AdmissionOperationID != participant.AdmissionOperationID || current.RoomGeneration != participant.RoomGeneration ||
		current.Generation != participant.Generation || participant.AccountID == "" || participant.AdmissionOperationID == "" ||
		participant.Identity == "" || participant.RoomGeneration == 0 || participant.Generation == "" {
		return voicestore.Call{}, false, status.Error(codes.FailedPrecondition, "test revoker requires the exact confirmed admission")
	}
	_, matched, err := r.calls.BeginSpaceMediaRevocation(ctx, call.RoomID, participant.ProfileID, participant.Identity, participant.Generation)
	if err != nil {
		return call, false, err
	}
	if !matched {
		return call, false, status.Error(codes.FailedPrecondition, "test revoker requires the matching current media participant")
	}
	if err := r.media.RemoveParticipant(ctx, call.LivekitRoomName, participant.Identity); err != nil {
		return call, false, err
	}
	updated, completed, err := r.calls.CompleteSpaceMediaRevocation(ctx, call.RoomID, participant.ProfileID, participant.Identity, participant.Generation)
	if err != nil {
		return updated, false, err
	}
	if !completed {
		return updated, false, status.Error(codes.FailedPrecondition, "test revoker could not complete the matching media participant")
	}
	r.removedProfiles = append(r.removedProfiles, participant.ProfileID)
	return updated, true, nil
}

func TestVoiceGRPC_LeaveVoiceRoom_ClearsScreenShare(t *testing.T) {
	t.Parallel()
	now := time.Unix(1700000000, 0).UTC()
	events := &recordingEvents{}
	spaceID, voiceRoomID, profileID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	svc := newTestVoiceService(now, events)
	configureReadySpaceMediaFixture(svc, &canonicalAccessResolver{result: CanonicalVoiceRoomAccess{
		SpaceID: spaceID, Member: true, Active: true, AccessEpoch: 11,
	}})
	var prepared []voicestore.SpaceMediaAdmission
	admissions := &screenShareLeaveAdmission{readySpaceMediaAdmission: readySpaceMediaAdmission{preparedAdmissions: &prepared}}
	svc.SpaceMediaAdmissions = admissions
	calls := svc.Calls.(*voicestore.MemoryCallStore)
	revoker := &screenShareLeaveRevoker{
		calls: calls, spaceID: spaceID, voiceRoomID: voiceRoomID, profileID: profileID,
	}
	revoker.media = &screenShareLeaveMedia{
		calls: calls, spaceID: spaceID, voiceRoomID: voiceRoomID, admissions: admissions,
	}
	svc.SpaceMediaRevoker = revoker
	svc.Roles = &canonicalRolePermissions{}
	join := &callsv1.JoinVoiceRoomRequest{VoiceRoomId: voiceRoomID, Space: &spacev1.SpaceRef{Id: spaceID}}
	joined, err := joinSpaceVoiceUser(t, svc, profileID, join)
	require.NoError(t, err)
	roomID := joined.GetVoiceSession().GetRoomId()

	_, err = svc.StartScreenShare(voiceTestCtx(profileID), &callsv1.StartScreenShareRequest{RoomId: roomID})
	require.NoError(t, err)
	callBeforeLeave, err := svc.Calls.GetCall(context.Background(), roomID)
	require.NoError(t, err)
	participant, admitted := callBeforeLeave.SpaceMedia[profileID]
	require.True(t, admitted)
	require.Len(t, prepared, 1)
	confirmed := prepared[0]
	require.Equal(t, confirmed.OperationID.String(), participant.AdmissionOperationID)
	require.Equal(t, confirmed.Generation, participant.Generation)
	require.Equal(t, confirmed.RoomGeneration, participant.RoomGeneration)
	require.Equal(t, confirmed.AccountID.String(), participant.AccountID)
	require.Equal(t, confirmed.ProfileID.String(), participant.ProfileID)
	require.Equal(t, confirmed.SpaceID.String(), callBeforeLeave.SpaceID)
	require.Equal(t, confirmed.RoomID, callBeforeLeave.RoomID)
	require.Equal(t, confirmed.VoiceRoomID, callBeforeLeave.VoiceRoomID)
	revoker.media.roomName = callBeforeLeave.LivekitRoomName
	revoker.media.identity = participant.Identity
	revoker.media.roomID = callBeforeLeave.RoomID
	revoker.media.participant = participant
	operationID, err := uuid.Parse(participant.AdmissionOperationID)
	require.NoError(t, err)

	_, err = svc.LeaveVoiceRoom(voiceTestCtx(profileID), &callsv1.LeaveVoiceRoomRequest{VoiceRoomId: voiceRoomID})
	require.NoError(t, err)
	require.Len(t, events.stopped, 1)
	require.Equal(t, []string{profileID}, revoker.removedProfiles)
	require.Equal(t, []uuid.UUID{operationID}, admissions.beginOperations)
	require.Equal(t, []string{participant.Generation}, admissions.beginGenerations)
	require.Len(t, admissions.completed, 1)
	completed := admissions.completed[0]
	require.Equal(t, confirmed.OperationID, completed.OperationID)
	require.Equal(t, confirmed.Generation, completed.Generation)
	require.Equal(t, confirmed.AccountID, completed.AccountID)
	require.Equal(t, confirmed.ProfileID, completed.ProfileID)
	require.Equal(t, confirmed.SpaceID, completed.SpaceID)
	require.Equal(t, confirmed.RoomID, completed.RoomID)
	require.True(t, revoker.media.participantRevoking)
	require.Equal(t, []string{participant.Identity}, revoker.media.removedIdentities)
	callAfterLeave, err := svc.Calls.GetCall(context.Background(), roomID)
	require.NoError(t, err)
	require.NotContains(t, callAfterLeave.ProfileIDs(), profileID)
	require.NotContains(t, callAfterLeave.SpaceMedia, profileID)
	require.Empty(t, callAfterLeave.ScreenShares)
}
