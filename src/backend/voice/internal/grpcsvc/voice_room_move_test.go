package grpcsvc

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	voicestore "voice/backend/voice/internal/store"

	callsv1 "voice.app/voice/calls/v1"
	spacev1 "voice.app/voice/space/v1"
)

type voiceRoomMoveFixture struct {
	svc                   *VoiceGRPC
	events                *recordingEvents
	roles                 *recordingVoiceRolePermissions
	spaceID, source, dest string
	actor, target         string
}

func newVoiceRoomMoveFixture(t *testing.T) voiceRoomMoveFixture {
	t.Helper()
	spaceID, source, dest, actor, target := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	members := map[string]map[string]bool{spaceID: {actor: true, target: true}}
	events := &recordingEvents{}
	svc := newTestVoiceService(time.Unix(1700000000, 0).UTC(), events)
	roles := &recordingVoiceRolePermissions{}
	svc.Roles = roles
	svc.VoiceRoomAccessResolver = fixtureCanonicalVoiceRoomResolver{rooms: map[string]string{source: spaceID, dest: spaceID}, members: members}
	for _, profileID := range []string{actor, target} {
		_, err := svc.JoinVoiceRoom(voiceTestCtx(profileID), &callsv1.JoinVoiceRoomRequest{VoiceRoomId: source, Space: &spacev1.SpaceRef{Id: spaceID}})
		require.NoError(t, err)
	}
	return voiceRoomMoveFixture{svc: svc, events: events, roles: roles, spaceID: spaceID, source: source, dest: dest, actor: actor, target: target}
}

func (f voiceRoomMoveFixture) moderatorRequest(operationID string) *callsv1.MoveVoiceRoomParticipantRequest {
	return &callsv1.MoveVoiceRoomParticipantRequest{FromVoiceRoomId: f.source, ToVoiceRoomId: f.dest, Space: &spacev1.SpaceRef{Id: f.spaceID}, ParticipantProfileId: f.target, OperationId: operationID}
}

func TestMoveVoiceRoomParticipant_SeparatesModeratorPermissionFromTargetJoin(t *testing.T) {
	f := newVoiceRoomMoveFixture(t)
	// The source room's creation and target join have already committed. Only
	// existing participants are notified; the joining target is never added as
	// a recipient merely because it joined.
	require.Len(t, f.events.startedCall, 1)
	require.Len(t, f.events.memberJoined, 2)
	joined := f.events.memberJoined[1]
	require.Equal(t, f.source, joined.GetVoiceRoomId())
	require.Equal(t, f.target, joined.GetJoinedProfileId())
	require.Equal(t, []string{f.actor}, joined.GetNotifyProfileIds())

	response, err := f.svc.MoveVoiceRoomParticipant(voiceTestCtx(f.actor), f.moderatorRequest(uuid.NewString()))
	require.NoError(t, err)
	require.Equal(t, callsv1.VoiceRoomLifecycleMethod_VOICE_ROOM_LIFECYCLE_METHOD_MODERATOR_MOVE, response.GetReceipt().GetMethod())
	require.Equal(t, voiceRolePermissionCheck{spaceID: f.spaceID, profileID: f.actor, voiceRoomID: f.source}, f.roles.moveOthersChecks[len(f.roles.moveOthersChecks)-1])
	require.Equal(t, voiceRolePermissionCheck{spaceID: f.spaceID, profileID: f.target, voiceRoomID: f.dest}, f.roles.voiceJoinChecks[len(f.roles.voiceJoinChecks)-1])

	source, err := f.svc.Calls.GetCallByVoiceRoomID(t.Context(), f.source)
	require.NoError(t, err)
	require.True(t, source.IsParticipant(f.actor))
	require.False(t, source.IsParticipant(f.target))
	destination, err := f.svc.Calls.GetCallByVoiceRoomID(t.Context(), f.dest)
	require.NoError(t, err)
	require.True(t, destination.IsParticipant(f.target))
	require.False(t, destination.IsParticipant(f.actor), "actor need not join destination to move another participant")
	require.Len(t, f.events.memberJoined, 3, "only the successful destination move publishes one join")
	moved := f.events.memberJoined[2]
	require.Equal(t, destination.RoomID, moved.GetRoomId())
	require.Equal(t, f.dest, moved.GetVoiceRoomId())
	require.Equal(t, f.spaceID, moved.GetSpaceId())
	require.Equal(t, f.target, moved.GetJoinedProfileId())
	require.Empty(t, moved.GetNotifyProfileIds(), "an empty destination must not invent an audience")
}

func TestMoveVoiceRoomParticipant_ReplayAndConflictingReplayDoNotMutateRoster(t *testing.T) {
	f := newVoiceRoomMoveFixture(t)
	op := uuid.NewString()
	first, err := f.svc.MoveVoiceRoomParticipant(voiceTestCtx(f.actor), f.moderatorRequest(op))
	require.NoError(t, err)
	f.roles.moveOthersErr = ErrVoiceMoveOthersDenied
	replay, err := f.svc.MoveVoiceRoomParticipant(voiceTestCtx(f.actor), f.moderatorRequest(op))
	require.NoError(t, err)
	require.Equal(t, first.GetReceipt().GetRoomId(), replay.GetReceipt().GetRoomId())
	require.Len(t, f.events.memberJoined, 3, "identical replay must not publish or move twice")
	require.Equal(t, []string{f.actor}, f.events.memberJoined[1].GetNotifyProfileIds(), "pre-move join routing remains immutable")

	conflict := f.moderatorRequest(op)
	conflict.ParticipantProfileId = f.actor
	_, err = f.svc.MoveVoiceRoomParticipant(voiceTestCtx(f.actor), conflict)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	destination, err := f.svc.Calls.GetCallByVoiceRoomID(t.Context(), f.dest)
	require.NoError(t, err)
	require.True(t, destination.IsParticipant(f.target))
}

func TestMoveVoiceRoomParticipant_DenialsLeaveBothRostersUnchanged(t *testing.T) {
	for _, tc := range []struct {
		name      string
		configure func(*recordingVoiceRolePermissions)
		want      codes.Code
	}{
		{"moderator permission", func(r *recordingVoiceRolePermissions) { r.moveOthersErr = ErrVoiceMoveOthersDenied }, codes.PermissionDenied},
		{"target destination join", func(r *recordingVoiceRolePermissions) { r.voiceJoinErr = ErrVoiceJoinDenied }, codes.PermissionDenied},
		{"target destination role unavailable", func(r *recordingVoiceRolePermissions) {
			r.voiceJoinErr = status.Error(codes.Unavailable, "role unavailable")
		}, codes.Unavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newVoiceRoomMoveFixture(t)
			tc.configure(f.roles)
			_, err := f.svc.MoveVoiceRoomParticipant(voiceTestCtx(f.actor), f.moderatorRequest(uuid.NewString()))
			require.Equal(t, tc.want, status.Code(err))
			source, err := f.svc.Calls.GetCallByVoiceRoomID(t.Context(), f.source)
			require.NoError(t, err)
			require.True(t, source.IsParticipant(f.target))
			_, err = f.svc.Calls.GetCallByVoiceRoomID(t.Context(), f.dest)
			require.ErrorIs(t, err, voicestore.ErrNotFound)
		})
	}
}

func TestMoveToVoiceRoom_IsSelfOnlyAndDoesNotRequireMoveOthers(t *testing.T) {
	f := newVoiceRoomMoveFixture(t)
	f.roles.moveOthersErr = ErrVoiceMoveOthersDenied
	response, err := f.svc.MoveToVoiceRoom(voiceTestCtx(f.target), &callsv1.MoveToVoiceRoomRequest{FromVoiceRoomId: f.source, ToVoiceRoomId: f.dest, Space: &spacev1.SpaceRef{Id: f.spaceID}, OperationId: uuid.NewString()})
	require.NoError(t, err)
	require.Equal(t, callsv1.VoiceRoomLifecycleMethod_VOICE_ROOM_LIFECYCLE_METHOD_SELF_MOVE, response.GetReceipt().GetMethod())
	require.Empty(t, f.roles.moveOthersChecks)
}

func TestMoveVoiceRoomParticipant_OperationConflictIsFailedPrecondition(t *testing.T) {
	f := newVoiceRoomMoveFixture(t)
	op := uuid.NewString()
	_, err := f.svc.MoveVoiceRoomParticipant(voiceTestCtx(f.actor), f.moderatorRequest(op))
	require.NoError(t, err)
	changed := f.moderatorRequest(op)
	changed.ToVoiceRoomId = uuid.NewString()
	_, err = f.svc.MoveVoiceRoomParticipant(voiceTestCtx(f.actor), changed)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	require.Equal(t, "operation id conflicts with a different request", status.Convert(err).Message())
}

func TestMoveStoreErr_RetryExhaustionIsUnavailable(t *testing.T) {
	require.Equal(t, codes.Unavailable, status.Code(moveStoreErr(redis.TxFailedErr)))
}

type unavailableMoveCallStore struct{ voicestore.CallStore }

func (unavailableMoveCallStore) MoveVoiceRoomParticipant(context.Context, voicestore.VoiceRoomMoveRequest) (voicestore.VoiceRoomMoveResult, error) {
	return voicestore.VoiceRoomMoveResult{}, redis.TxFailedErr
}

func TestMoveVoiceRoomParticipant_RetryExhaustionIsUnavailableAndLeavesRosterUntouched(t *testing.T) {
	f := newVoiceRoomMoveFixture(t)
	f.svc.Calls = unavailableMoveCallStore{CallStore: f.svc.Calls}
	_, err := f.svc.MoveVoiceRoomParticipant(voiceTestCtx(f.actor), f.moderatorRequest(uuid.NewString()))
	require.Equal(t, codes.Unavailable, status.Code(err))
	source, getErr := f.svc.Calls.GetCallByVoiceRoomID(t.Context(), f.source)
	require.NoError(t, getErr)
	require.True(t, source.IsParticipant(f.target))
	_, getErr = f.svc.Calls.GetCallByVoiceRoomID(t.Context(), f.dest)
	require.ErrorIs(t, getErr, voicestore.ErrNotFound)
}
