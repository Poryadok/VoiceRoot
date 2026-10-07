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
		_, err := joinSpaceVoiceUser(t, svc, profileID, &callsv1.JoinVoiceRoomRequest{VoiceRoomId: source, Space: &spacev1.SpaceRef{Id: spaceID}})
		require.NoError(t, err)
	}
	return voiceRoomMoveFixture{svc: svc, events: events, roles: roles, spaceID: spaceID, source: source, dest: dest, actor: actor, target: target}
}

func (f voiceRoomMoveFixture) moderatorRequest(operationID string) *callsv1.MoveVoiceRoomParticipantRequest {
	return &callsv1.MoveVoiceRoomParticipantRequest{FromVoiceRoomId: f.source, ToVoiceRoomId: f.dest, Space: &spacev1.SpaceRef{Id: f.spaceID}, ParticipantProfileId: f.target, OperationId: operationID}
}

func TestSpaceMoveVoiceRoomParticipant_FailsClosedBeforeEffects(t *testing.T) {
	f := newVoiceRoomMoveFixture(t)
	require.Len(t, f.events.startedCall, 1)
	require.Len(t, f.events.memberJoined, 2)
	_, err := f.svc.MoveVoiceRoomParticipant(voiceTestCtx(f.actor), f.moderatorRequest(uuid.NewString()))
	require.Equal(t, codes.Unavailable, status.Code(err))
	source, err := f.svc.Calls.GetCallByVoiceRoomID(t.Context(), f.source)
	require.NoError(t, err)
	require.True(t, source.IsParticipant(f.actor))
	require.True(t, source.IsParticipant(f.target))
	_, err = f.svc.Calls.GetCallByVoiceRoomID(t.Context(), f.dest)
	require.ErrorIs(t, err, voicestore.ErrNotFound)
	require.Len(t, f.events.memberJoined, 2)
	require.Empty(t, f.roles.moveOthersChecks)
	require.Empty(t, f.roles.voiceJoinChecks)
	selfRequest := &callsv1.MoveToVoiceRoomRequest{FromVoiceRoomId: f.source, ToVoiceRoomId: f.dest, Space: &spacev1.SpaceRef{Id: f.spaceID}, OperationId: uuid.NewString()}
	_, err = f.svc.MoveToVoiceRoom(voiceTestCtx(f.target), selfRequest)
	require.Equal(t, codes.Unavailable, status.Code(err))
	_, err = f.svc.Calls.GetCallByVoiceRoomID(t.Context(), f.dest)
	require.ErrorIs(t, err, voicestore.ErrNotFound)
	require.Len(t, f.events.memberJoined, 2)
}

func TestMoveVoiceRoomParticipant_ReplayAndConflictingReplayDoNotMutateRoster(t *testing.T) {
	f := newVoiceRoomMoveFixture(t)
	op := uuid.NewString()
	_, err := f.svc.MoveVoiceRoomParticipant(voiceTestCtx(f.actor), f.moderatorRequest(op))
	require.Equal(t, codes.Unavailable, status.Code(err))
	_, err = f.svc.MoveVoiceRoomParticipant(voiceTestCtx(f.actor), f.moderatorRequest(op))
	require.Equal(t, codes.Unavailable, status.Code(err))
	conflict := f.moderatorRequest(op)
	conflict.ParticipantProfileId = f.actor
	_, err = f.svc.MoveVoiceRoomParticipant(voiceTestCtx(f.actor), conflict)
	require.Equal(t, codes.Unavailable, status.Code(err))
	_, err = f.svc.Calls.GetCallByVoiceRoomID(t.Context(), f.dest)
	require.ErrorIs(t, err, voicestore.ErrNotFound)
}

func TestMoveVoiceRoomParticipant_DenialsLeaveBothRostersUnchanged(t *testing.T) {
	for _, tc := range []struct {
		name      string
		configure func(*recordingVoiceRolePermissions)
	}{
		{"moderator permission", func(r *recordingVoiceRolePermissions) { r.moveOthersErr = ErrVoiceMoveOthersDenied }},
		{"target destination join", func(r *recordingVoiceRolePermissions) { r.voiceJoinErr = ErrVoiceJoinDenied }},
		{"target destination role unavailable", func(r *recordingVoiceRolePermissions) {
			r.voiceJoinErr = status.Error(codes.Unavailable, "role unavailable")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newVoiceRoomMoveFixture(t)
			tc.configure(f.roles)
			_, err := f.svc.MoveVoiceRoomParticipant(voiceTestCtx(f.actor), f.moderatorRequest(uuid.NewString()))
			require.Equal(t, codes.Unavailable, status.Code(err))
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
	_, err := f.svc.MoveToVoiceRoom(voiceTestCtx(f.target), &callsv1.MoveToVoiceRoomRequest{FromVoiceRoomId: f.source, ToVoiceRoomId: f.dest, Space: &spacev1.SpaceRef{Id: f.spaceID}, OperationId: uuid.NewString()})
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.Empty(t, f.roles.moveOthersChecks)
}

func TestMoveVoiceRoomParticipant_OperationConflictIsFailedPrecondition(t *testing.T) {
	f := newVoiceRoomMoveFixture(t)
	op := uuid.NewString()
	_, err := f.svc.MoveVoiceRoomParticipant(voiceTestCtx(f.actor), f.moderatorRequest(op))
	require.Equal(t, codes.Unavailable, status.Code(err))
	changed := f.moderatorRequest(op)
	changed.ToVoiceRoomId = uuid.NewString()
	_, err = f.svc.MoveVoiceRoomParticipant(voiceTestCtx(f.actor), changed)
	require.Equal(t, codes.Unavailable, status.Code(err))
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
