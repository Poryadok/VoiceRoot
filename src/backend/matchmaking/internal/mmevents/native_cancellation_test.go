package mmevents

import (
	"context"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"testing"
	eventsv1 "voice.app/voice/events/v1"
)

func TestNativeCapturedCancellationPreservesBothFrozenTerminalEnvelopes(t *testing.T) {
	for _, reason := range []string{"", "VOICE_ROSTER_CHANGED"} {
		t.Run("reason="+reason, func(t *testing.T) {
			id := uuid.New()
			event := &eventsv1.MatchmakingStreamEvent{EventId: id.String(), OccurredAt: timestamppb.Now(), Payload: &eventsv1.MatchmakingStreamEvent_SearchCancelled{SearchCancelled: &eventsv1.SearchCancelled{SessionId: uuid.NewString(), ProfileId: uuid.NewString(), Reason: reason}}}
			raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(event)
			require.NoError(t, err)
			js := &rosterPublication{}
			publisher := &JetStreamPublisher{js: js}
			for range 2 {
				require.NoError(t, publisher.PublishCapturedCancellation(context.Background(), id, raw))
			}
			require.Len(t, js.messages, 2)
			for _, message := range js.messages {
				require.Equal(t, raw, message.Data)
				require.Equal(t, id.String(), message.Header.Get(nats.MsgIdHdr))
				require.Equal(t, subjectSearchCancel, message.Subject)
			}
			if reason == "" {
				require.Error(t, publisher.PublishRosterCancellation(context.Background(), id, raw), "legacy roster-only API remains closed")
			}
			js.noAck = true
			require.Error(t, publisher.PublishCapturedCancellation(context.Background(), id, raw))
		})
	}
}
func TestNativeCapturedCancellationRejectsUnknownReasonAndNoncanonicalPayload(t *testing.T) {
	for _, kind := range []string{"reason", "unknown", "noncanonical", "identity", "wrong-event"} {
		t.Run(kind, func(t *testing.T) {
			id := uuid.New()
			fact := &eventsv1.SearchCancelled{SessionId: uuid.NewString(), ProfileId: uuid.NewString()}
			event := &eventsv1.MatchmakingStreamEvent{EventId: id.String(), OccurredAt: timestamppb.Now(), Payload: &eventsv1.MatchmakingStreamEvent_SearchCancelled{SearchCancelled: fact}}
			switch kind {
			case "reason":
				fact.Reason = "invented"
			case "unknown":
				event.ProtoReflect().SetUnknown([]byte{0xa0, 6, 1})
			case "identity":
				fact.ProfileId = "not-an-id"
			case "wrong-event":
				event.EventId = uuid.NewString()
			}
			raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(event)
			require.NoError(t, err)
			if kind == "noncanonical" {
				raw = append(raw, 10, 36)
				raw = append(raw, []byte(event.EventId)...)
			}
			js := &rosterPublication{}
			require.Error(t, (&JetStreamPublisher{js: js}).PublishCapturedCancellation(context.Background(), id, raw))
			require.Empty(t, js.messages)
		})
	}
}
