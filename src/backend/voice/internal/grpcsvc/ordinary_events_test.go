package grpcsvc

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"testing"
	"time"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/voice/internal/ordinarycall"
)

func TestOrdinaryEncoderFreezesCommittedEventIdentityTimeAndTerminalAudience(t *testing.T) {
	now := time.Now().UTC()
	caller := ordinarycall.Identity{AccountID: uuid.New(), ProfileID: uuid.New()}
	callee := ordinarycall.Identity{AccountID: uuid.New(), ProfileID: uuid.New()}
	op := uuid.New()
	r, err := ordinarycall.NewPendingRoom(ordinarycall.CreateIntent{OperationID: op, RoomID: uuid.New(), ChatID: uuid.New(), Initiator: caller, Callee: &callee, MediaKind: 1}, now)
	require.NoError(t, err)
	require.NoError(t, r.RecordEnsureResult(op, true, now))
	require.NoError(t, r.CommitAdmission(op, now))
	accept := uuid.New()
	require.NoError(t, r.BeginAccept(accept, callee.ProfileID, now))
	require.NoError(t, r.CommitAdmission(accept, now))
	require.NoError(t, r.DecideEnd(caller.ProfileID, "hangup", now.Add(time.Second)))
	for _, e := range r.Events {
		encoded, err := EncodeOrdinaryEvent(r, e)
		require.NoError(t, err)
		var env eventsv1.VoiceStreamEvent
		require.NoError(t, proto.Unmarshal(encoded.Payload, &env))
		require.Equal(t, e.ID.String(), env.EventId)
		require.True(t, e.OccurredAt.Equal(env.OccurredAt.AsTime()))
		if e.Kind == ordinarycall.EventEnded {
			require.ElementsMatch(t, []string{caller.ProfileID.String(), callee.ProfileID.String()}, env.GetCallEnded().ProfileIds)
			require.Equal(t, int32(1), env.GetCallEnded().DurationSeconds)
		}
		repeated, err := EncodeOrdinaryEvent(r, e)
		require.NoError(t, err)
		require.Equal(t, encoded, repeated)
	}
}
