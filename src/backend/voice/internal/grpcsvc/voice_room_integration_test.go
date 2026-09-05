package grpcsvc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	voicestore "voice/backend/voice/internal/store"

	callsv1 "voice.app/voice/calls/v1"
	chatv1 "voice.app/voice/chat/v1"
	spacev1 "voice.app/voice/space/v1"
)

type voiceRoomFixture struct {
	svc         *VoiceGRPC
	spaceID     string
	voiceRoomID string
}

type voiceRolePermissionCheck struct {
	spaceID     string
	profileID   string
	voiceRoomID string
}

// recordingVoiceRolePermissions is a test double for the complete Space voice
// permission boundary, including the RolePermissionChecker speak check.
type recordingVoiceRolePermissions struct {
	voiceSpeakErr error
	muteOthersErr error

	voiceSpeakChecks []voiceRolePermissionCheck
	muteOthersChecks []voiceRolePermissionCheck
}

func (r *recordingVoiceRolePermissions) EnsureScreenShare(context.Context, string, string, string) error {
	return nil
}

func (r *recordingVoiceRolePermissions) EnsureVoiceJoin(context.Context, string, string, string) error {
	return nil
}

func (r *recordingVoiceRolePermissions) EnsureVoiceSpeak(_ context.Context, spaceID, profileID, voiceRoomID string) error {
	r.voiceSpeakChecks = append(r.voiceSpeakChecks, voiceRolePermissionCheck{
		spaceID:     spaceID,
		profileID:   profileID,
		voiceRoomID: voiceRoomID,
	})
	return r.voiceSpeakErr
}

func (r *recordingVoiceRolePermissions) EnsureMuteOthers(_ context.Context, spaceID, profileID, voiceRoomID string) error {
	r.muteOthersChecks = append(r.muteOthersChecks, voiceRolePermissionCheck{
		spaceID:     spaceID,
		profileID:   profileID,
		voiceRoomID: voiceRoomID,
	})
	return r.muteOthersErr
}

func startVoiceRoomFixture(t *testing.T) voiceRoomFixture {
	t.Helper()
	spaceID := uuid.New().String()
	voiceRoomID := uuid.New().String()
	members := map[string]map[string]bool{
		spaceID: {
			"profile-owner":  true,
			"profile-member": true,
		},
	}
	svc := newTestVoiceService(time.Unix(1700000000, 0).UTC(), &recordingEvents{})
	svc.SpaceMembers = &mapSpaceMembers{members: members}
	return voiceRoomFixture{svc: svc, spaceID: spaceID, voiceRoomID: voiceRoomID}
}

func (f voiceRoomFixture) joinReq(profileID string) *callsv1.JoinVoiceRoomRequest {
	return &callsv1.JoinVoiceRoomRequest{
		VoiceRoomId: f.voiceRoomID,
		Space:       &spacev1.SpaceRef{Id: f.spaceID},
	}
}

func (f voiceRoomFixture) joinParticipants(t *testing.T) string {
	t.Helper()
	joined, err := f.svc.JoinVoiceRoom(voiceTestCtx("profile-owner"), f.joinReq("profile-owner"))
	require.NoError(t, err)
	_, err = f.svc.JoinVoiceRoom(voiceTestCtx("profile-member"), f.joinReq("profile-member"))
	require.NoError(t, err)
	return joined.GetVoiceSession().GetRoomId()
}

func (f voiceRoomFixture) participantState(t *testing.T, roomID, profileID string) *callsv1.VoiceParticipantState {
	t.Helper()
	states, err := f.svc.GetVoiceStates(voiceTestCtx("profile-owner"), &callsv1.GetVoiceStatesRequest{RoomId: roomID})
	require.NoError(t, err)
	for _, state := range states.GetParticipants() {
		if state.GetProfileId() == profileID {
			return state
		}
	}
	require.Failf(t, "participant state", "profile %q is missing from room %q", profileID, roomID)
	return nil
}

func (f voiceRoomFixture) setParticipantState(t *testing.T, roomID, profileID string, patch voicestore.VoiceStatePatch) {
	t.Helper()
	_, _, err := f.svc.Calls.UpdateVoiceState(context.Background(), roomID, profileID, patch)
	require.NoError(t, err)
}

func requireVoiceRoleCheck(t *testing.T, checks []voiceRolePermissionCheck, spaceID, profileID, voiceRoomID string) {
	t.Helper()
	require.Len(t, checks, 1)
	require.Equal(t, voiceRolePermissionCheck{
		spaceID:     spaceID,
		profileID:   profileID,
		voiceRoomID: voiceRoomID,
	}, checks[0])
}

func liveKitCanPublish(t *testing.T, token string) (allowed, present bool) {
	t.Helper()
	parts := strings.Split(token, ".")
	require.Len(t, parts, 3)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var claims struct {
		Video map[string]json.RawMessage `json:"video"`
	}
	require.NoError(t, json.Unmarshal(payload, &claims))
	grant, present := claims.Video["canPublish"]
	if !present {
		return false, false
	}
	require.NoError(t, json.Unmarshal(grant, &allowed))
	return allowed, true
}

func TestVoiceGRPCJoinVoiceRoom_createsActiveSession(t *testing.T) {
	f := startVoiceRoomFixture(t)

	joined, err := f.svc.JoinVoiceRoom(voiceTestCtx("profile-owner"), f.joinReq("profile-owner"))
	require.NoError(t, err)
	session := joined.GetVoiceSession()
	require.NotEmpty(t, session.GetRoomId())
	require.Equal(t, f.voiceRoomID, session.GetVoiceRoomId())
	require.Contains(t, session.GetLivekitRoomName(), "voice-room-")
}

func TestVoiceGRPCVoiceRoom_memberJoinsExistingRoom(t *testing.T) {
	f := startVoiceRoomFixture(t)

	_, err := f.svc.JoinVoiceRoom(voiceTestCtx("profile-owner"), f.joinReq("profile-owner"))
	require.NoError(t, err)

	_, err = f.svc.JoinVoiceRoom(voiceTestCtx("profile-member"), f.joinReq("profile-member"))
	require.NoError(t, err)

	states, err := f.svc.GetVoiceStates(voiceTestCtx("profile-member"), &callsv1.GetVoiceStatesRequest{
		VoiceRoomId: &f.voiceRoomID,
	})
	require.NoError(t, err)
	require.Len(t, states.GetParticipants(), 2)
}

func TestVoiceGRPCVoiceRoom_joinTokenPublishGrantFollowsVoiceSpeakPermission(t *testing.T) {
	tests := []struct {
		name         string
		profileID    string
		roles        RolePermissionChecker
		wantCode     codes.Code
		wantPublish  bool
		wantRoleCall bool
	}{
		{
			name:         "denied member receives listen-only token",
			profileID:    "profile-member",
			roles:        &recordingVoiceRolePermissions{voiceSpeakErr: ErrVoiceSpeakDenied},
			wantCode:     codes.OK,
			wantPublish:  false,
			wantRoleCall: true,
		},
		{
			name:         "allowed member receives publish token",
			profileID:    "profile-member",
			roles:        &recordingVoiceRolePermissions{},
			wantCode:     codes.OK,
			wantPublish:  true,
			wantRoleCall: true,
		},
		{
			name:         "owner-shaped role allow receives publish token",
			profileID:    "profile-owner",
			roles:        &recordingVoiceRolePermissions{},
			wantCode:     codes.OK,
			wantPublish:  true,
			wantRoleCall: true,
		},
		{
			name:         "role unavailable fails closed",
			profileID:    "profile-member",
			roles:        &recordingVoiceRolePermissions{voiceSpeakErr: errors.New("role service unavailable")},
			wantCode:     codes.PermissionDenied,
			wantRoleCall: true,
		},
		{
			name:      "role checker missing fails closed",
			profileID: "profile-member",
			wantCode:  codes.PermissionDenied,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := startVoiceRoomFixture(t)
			roomID := f.joinParticipants(t)
			f.svc.Roles = tc.roles

			response, err := f.svc.GetJoinToken(voiceTestCtx(tc.profileID), &callsv1.GetJoinTokenRequest{RoomId: roomID})
			require.Equal(t, tc.wantCode, status.Code(err))
			if tc.wantRoleCall {
				roles := tc.roles.(*recordingVoiceRolePermissions)
				requireVoiceRoleCheck(t, roles.voiceSpeakChecks, f.spaceID, tc.profileID, f.voiceRoomID)
			}
			if tc.wantCode != codes.OK {
				require.Nil(t, response)
				return
			}

			allowed, present := liveKitCanPublish(t, response.GetJwt())
			require.True(t, present, "Space voice token must carry an explicit publish grant")
			require.Equal(t, tc.wantPublish, allowed)
		})
	}
}

func TestVoiceGRPCVoiceRoom_nonMemberDenied(t *testing.T) {
	f := startVoiceRoomFixture(t)

	_, err := f.svc.JoinVoiceRoom(voiceTestCtx("profile-stranger"), f.joinReq("profile-stranger"))
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

// TestVoiceGRPCJoinVoiceRoom_roleDenyPermissionDenied documents roles.md VOICE_JOIN:
// Role Service deny → JoinVoiceRoom PermissionDenied before session create.
func TestVoiceGRPCJoinVoiceRoom_roleDenyPermissionDenied(t *testing.T) {
	t.Parallel()
	f := startVoiceRoomFixture(t)
	f.svc.Roles = &mapRolePermissions{allowed: map[string]map[string]bool{
		f.spaceID: {"profile-owner": true},
	}}

	_, err := f.svc.JoinVoiceRoom(voiceTestCtx("profile-member"), f.joinReq("profile-member"))
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Contains(t, status.Convert(err).Message(), "voice join not permitted")

	_, err = f.svc.JoinVoiceRoom(voiceTestCtx("profile-owner"), f.joinReq("profile-owner"))
	require.NoError(t, err)
}

// TestVoiceGRPCJoinVoiceRoom_voiceRoomOverrideDeny documents roles.md room overrides:
// space-level VOICE_JOIN allow + voice_room_overrides deny → PermissionDenied.
func TestVoiceGRPCJoinVoiceRoom_voiceRoomOverrideDeny(t *testing.T) {
	t.Parallel()
	f := startVoiceRoomFixture(t)
	f.svc.Roles = &mapRolePermissions{
		allowed: map[string]map[string]bool{
			f.spaceID: {
				"profile-owner":  true,
				"profile-member": true,
			},
		},
		deniedRooms: map[string]map[string]bool{
			f.voiceRoomID: {"profile-member": true},
		},
	}

	_, err := f.svc.JoinVoiceRoom(voiceTestCtx("profile-owner"), f.joinReq("profile-owner"))
	require.NoError(t, err)

	_, err = f.svc.JoinVoiceRoom(voiceTestCtx("profile-member"), f.joinReq("profile-member"))
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Contains(t, status.Convert(err).Message(), "voice join not permitted")
}

func TestVoiceGRPCVoiceRoom_spaceMemberViewsRosterWithoutJoining(t *testing.T) {
	f := startVoiceRoomFixture(t)

	_, err := f.svc.JoinVoiceRoom(voiceTestCtx("profile-owner"), f.joinReq("profile-owner"))
	require.NoError(t, err)

	states, err := f.svc.GetVoiceStates(voiceTestCtx("profile-member"), &callsv1.GetVoiceStatesRequest{
		VoiceRoomId: &f.voiceRoomID,
	})
	require.NoError(t, err)
	require.Len(t, states.GetParticipants(), 1)
}

func TestVoiceGRPCVoiceRoom_leaveRemovesParticipant(t *testing.T) {
	f := startVoiceRoomFixture(t)

	_, err := f.svc.JoinVoiceRoom(voiceTestCtx("profile-owner"), f.joinReq("profile-owner"))
	require.NoError(t, err)
	_, err = f.svc.JoinVoiceRoom(voiceTestCtx("profile-member"), f.joinReq("profile-member"))
	require.NoError(t, err)

	_, err = f.svc.LeaveVoiceRoom(voiceTestCtx("profile-member"), &callsv1.LeaveVoiceRoomRequest{
		VoiceRoomId: f.voiceRoomID,
	})
	require.NoError(t, err)

	states, err := f.svc.GetVoiceStates(voiceTestCtx("profile-owner"), &callsv1.GetVoiceStatesRequest{
		VoiceRoomId: &f.voiceRoomID,
	})
	require.NoError(t, err)
	require.Len(t, states.GetParticipants(), 1)
}

func TestVoiceGRPCVoiceRoom_max32Participants(t *testing.T) {
	spaceID := uuid.New().String()
	voiceRoomID := uuid.New().String()
	members := map[string]map[string]bool{spaceID: {"profile-owner": true}}
	for i := 1; i <= 32; i++ {
		members[spaceID][fmt.Sprintf("profile-%02d", i)] = true
	}
	svc := newTestVoiceService(time.Unix(1700000000, 0).UTC(), &recordingEvents{})
	svc.SpaceMembers = &mapSpaceMembers{members: members}
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
	require.Equal(t, codes.ResourceExhausted, status.Code(err))
}

func TestVoiceGRPCVoiceRoom_voiceSpeakDenialBlocksUnmuteButAllowsSelfMute(t *testing.T) {
	f := startVoiceRoomFixture(t)
	roles := &recordingVoiceRolePermissions{voiceSpeakErr: errors.New("VOICE_SPEAK denied")}
	f.svc.Roles = roles
	roomID := f.joinParticipants(t)

	muted := true
	_, err := f.svc.UpdateVoiceState(voiceTestCtx("profile-owner"), &callsv1.UpdateVoiceStateRequest{
		RoomId:  roomID,
		IsMuted: &muted,
	})
	require.NoError(t, err, "self-mute must not require VOICE_SPEAK")
	require.Empty(t, roles.voiceSpeakChecks, "self-mute must not call Role Service")
	require.True(t, f.participantState(t, roomID, "profile-owner").GetIsMuted())

	unmuted := false
	_, err = f.svc.UpdateVoiceState(voiceTestCtx("profile-owner"), &callsv1.UpdateVoiceStateRequest{
		RoomId:  roomID,
		IsMuted: &unmuted,
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	requireVoiceRoleCheck(t, roles.voiceSpeakChecks, f.spaceID, "profile-owner", f.voiceRoomID)
	require.True(t, f.participantState(t, roomID, "profile-owner").GetIsMuted(), "denied unmute must not mutate state")
}

func TestVoiceGRPCVoiceRoom_unmuteFailsClosedWithoutOrWithUnavailableRoleChecker(t *testing.T) {
	cases := []struct {
		name  string
		roles RolePermissionChecker
	}{
		{name: "missing"},
		{name: "unavailable", roles: &recordingVoiceRolePermissions{voiceSpeakErr: errors.New("role service unavailable")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := startVoiceRoomFixture(t)
			f.svc.Roles = tc.roles
			roomID := f.joinParticipants(t)
			muted := true
			_, err := f.svc.UpdateVoiceState(voiceTestCtx("profile-owner"), &callsv1.UpdateVoiceStateRequest{
				RoomId:  roomID,
				IsMuted: &muted,
			})
			require.NoError(t, err, "self-mute remains available during Role outage")

			unmuted := false
			_, err = f.svc.UpdateVoiceState(voiceTestCtx("profile-owner"), &callsv1.UpdateVoiceStateRequest{
				RoomId:  roomID,
				IsMuted: &unmuted,
			})
			require.Equal(t, codes.PermissionDenied, status.Code(err))
			require.True(t, f.participantState(t, roomID, "profile-owner").GetIsMuted(), "failed-closed unmute must not mutate state")
		})
	}
}

func TestVoiceGRPCVoiceRoom_nonMutePatchDoesNotRequireVoiceSpeak(t *testing.T) {
	f := startVoiceRoomFixture(t)
	roles := &recordingVoiceRolePermissions{voiceSpeakErr: errors.New("VOICE_SPEAK denied")}
	f.svc.Roles = roles
	roomID := f.joinParticipants(t)
	deafened := true

	_, err := f.svc.UpdateVoiceState(voiceTestCtx("profile-owner"), &callsv1.UpdateVoiceStateRequest{
		RoomId:     roomID,
		IsDeafened: &deafened,
	})
	require.NoError(t, err, "an absent is_muted patch is not an unmute")
	require.True(t, f.participantState(t, roomID, "profile-owner").GetIsDeafened())
	require.Empty(t, roles.voiceSpeakChecks, "a non-mute patch must not call VOICE_SPEAK")
}

func TestVoiceGRPCUpdateVoiceState_groupNonMutePatchAllowsMissingRoleChecker(t *testing.T) {
	svc := newTestGroupVoiceService(time.Unix(1700000000, 0).UTC(), &recordingEvents{})
	var noRoles RolePermissionChecker
	svc.Roles = noRoles
	group := chatv1.ChatType_CHAT_TYPE_GROUP
	started, err := svc.StartCall(voiceTestCtx("profile-owner"), &callsv1.StartCallRequest{
		RoomTypeEnum: callsv1.VoiceSessionKind_VOICE_SESSION_KIND_GROUP_VOICE.Enum(),
		LinkedChat:   &chatv1.ChatRef{Id: "group-chat-1", Type: &group},
		MediaKind:    mediaPtr(callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO),
	})
	require.NoError(t, err)
	roomID := started.GetCallSession().GetRoomId()
	deafened := true

	_, err = svc.UpdateVoiceState(voiceTestCtx("profile-owner"), &callsv1.UpdateVoiceStateRequest{
		RoomId:     roomID,
		IsDeafened: &deafened,
	})
	require.NoError(t, err, "group voice has no Space Role scope")
	states, err := svc.GetVoiceStates(voiceTestCtx("profile-owner"), &callsv1.GetVoiceStatesRequest{RoomId: roomID})
	require.NoError(t, err)
	require.True(t, findParticipantState(states.GetParticipants(), "profile-owner").GetIsDeafened())
}

func TestVoiceGRPCVoiceRoom_protectedActionsFailClosedWithoutRoleChecker(t *testing.T) {
	t.Run("unmute", func(t *testing.T) {
		f := startVoiceRoomFixture(t)
		roomID := f.joinParticipants(t)
		muted := true
		_, err := f.svc.UpdateVoiceState(voiceTestCtx("profile-owner"), &callsv1.UpdateVoiceStateRequest{RoomId: roomID, IsMuted: &muted})
		require.NoError(t, err)
		unmuted := false
		_, err = f.svc.UpdateVoiceState(voiceTestCtx("profile-owner"), &callsv1.UpdateVoiceStateRequest{RoomId: roomID, IsMuted: &unmuted})
		require.Equal(t, codes.PermissionDenied, status.Code(err))
		require.True(t, f.participantState(t, roomID, "profile-owner").GetIsMuted())
	})

	t.Run("enable commander mode", func(t *testing.T) {
		f := startVoiceRoomFixture(t)
		roomID := f.joinParticipants(t)
		_, err := f.svc.SetCommanderMode(voiceTestCtx("profile-owner"), &callsv1.SetCommanderModeRequest{RoomId: roomID, Enabled: true})
		require.Equal(t, codes.PermissionDenied, status.Code(err))
		require.False(t, f.participantState(t, roomID, "profile-owner").GetIsCommander())
	})

	t.Run("begin broadcasting", func(t *testing.T) {
		f := startVoiceRoomFixture(t)
		roomID := f.joinParticipants(t)
		commander := true
		f.setParticipantState(t, roomID, "profile-owner", voicestore.VoiceStatePatch{IsCommander: &commander})
		_, err := f.svc.SetBroadcasting(voiceTestCtx("profile-owner"), &callsv1.SetBroadcastingRequest{RoomId: roomID, Enabled: true})
		require.Equal(t, codes.PermissionDenied, status.Code(err))
		require.False(t, f.participantState(t, roomID, "profile-owner").GetIsBroadcasting())
	})

	t.Run("grant floor", func(t *testing.T) {
		f := startVoiceRoomFixture(t)
		roomID := f.joinParticipants(t)
		_, err := f.svc.GrantFloor(voiceTestCtx("profile-owner"), &callsv1.GrantFloorRequest{RoomId: roomID, ProfileId: "profile-member"})
		require.Equal(t, codes.PermissionDenied, status.Code(err))
		require.False(t, f.participantState(t, roomID, "profile-member").GetHasFloor())
	})

	t.Run("revoke floor", func(t *testing.T) {
		f := startVoiceRoomFixture(t)
		roomID := f.joinParticipants(t)
		hasFloor := true
		f.setParticipantState(t, roomID, "profile-member", voicestore.VoiceStatePatch{HasFloor: &hasFloor})
		_, err := f.svc.RevokeFloor(voiceTestCtx("profile-owner"), &callsv1.RevokeFloorRequest{RoomId: roomID, ProfileId: "profile-member"})
		require.Equal(t, codes.PermissionDenied, status.Code(err))
		require.True(t, f.participantState(t, roomID, "profile-member").GetHasFloor())
	})
}
