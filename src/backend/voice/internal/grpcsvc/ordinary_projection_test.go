package grpcsvc

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/voice/internal/ordinarycall"
)

func TestOrdinaryProjectionExposesOnlyCommittedMembership(t *testing.T) {
	caller, callee := ordinarycall.Identity{AccountID: uuid.New(), ProfileID: uuid.New()}, ordinarycall.Identity{AccountID: uuid.New(), ProfileID: uuid.New()}
	intent := ordinarycall.CreateIntent{OperationID: uuid.New(), RoomID: uuid.New(), ChatID: uuid.New(), Initiator: caller, Callee: &callee, MediaKind: 1}
	now := time.Now().UTC()
	room, err := ordinarycall.NewPendingRoom(intent, now)
	require.NoError(t, err)
	_, err = ordinaryCallProjection(room)
	require.ErrorIs(t, err, ordinarycall.ErrEffectPending)
	require.NoError(t, room.RecordEnsureResult(intent.OperationID, true, now))
	require.NoError(t, room.CommitAdmission(intent.OperationID, now))
	call, err := ordinaryCallProjection(room)
	require.NoError(t, err)
	require.Equal(t, callsv1.CallStatus_CALL_STATUS_RINGING, call.Status)
	require.Len(t, call.States, 2)
	require.False(t, room.CanIssueToken(caller.ProfileID), "ring projection is not token authority")
	op := uuid.New()
	require.NoError(t, room.BeginAccept(op, callee.ProfileID, now))
	require.NoError(t, room.CommitAdmission(op, now))
	call, err = ordinaryCallProjection(room)
	require.NoError(t, err)
	require.Equal(t, callsv1.CallStatus_CALL_STATUS_ACTIVE, call.Status)
	require.NoError(t, room.DecideEnd(caller.ProfileID, "hangup", now))
	call, err = ordinaryCallProjection(room)
	require.NoError(t, err)
	require.Equal(t, callsv1.CallStatus_CALL_STATUS_ENDED, call.Status)
	require.Empty(t, call.States)
	require.Len(t, call.OrdinaryMediaIdentities, 2, "retired identities remain available for exact index cleanup")
}

func TestOrdinaryProjectionPreservesDeclinedAndMissedPublicStatus(t *testing.T) {
	for _, decline := range []bool{true, false} {
		caller, callee := ordinarycall.Identity{AccountID: uuid.New(), ProfileID: uuid.New()}, ordinarycall.Identity{AccountID: uuid.New(), ProfileID: uuid.New()}
		intent := ordinarycall.CreateIntent{OperationID: uuid.New(), RoomID: uuid.New(), ChatID: uuid.New(), Initiator: caller, Callee: &callee, MediaKind: 1}
		now := time.Now().UTC()
		room, err := ordinarycall.NewPendingRoom(intent, now)
		require.NoError(t, err)
		require.NoError(t, room.RecordEnsureResult(intent.OperationID, true, now))
		require.NoError(t, room.CommitAdmission(intent.OperationID, now))
		want := callsv1.CallStatus_CALL_STATUS_MISSED
		if decline {
			require.NoError(t, room.DecideDecline(callee.ProfileID, now))
			want = callsv1.CallStatus_CALL_STATUS_DECLINED
		} else {
			require.NoError(t, room.DecideTimeout(room.ExpiresAt))
		}
		call, err := ordinaryCallProjection(room)
		require.NoError(t, err)
		require.Equal(t, want, call.Status)
	}
}
