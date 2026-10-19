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

func TestNativeCapturedTimeoutKeepsExactWireAndRequiresPubAck(t *testing.T) {
	id := uuid.New()
	event := &eventsv1.MatchmakingStreamEvent{EventId: id.String(), OccurredAt: timestamppb.Now(), Payload: &eventsv1.MatchmakingStreamEvent_MatchTimeout{MatchTimeout: &eventsv1.MatchTimeout{SessionId: uuid.NewString(), ProfileId: uuid.NewString(), GameId: uuid.NewString(), Mode: "ranked"}}}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(event)
	require.NoError(t, err)
	js := &rosterPublication{}
	pub := &JetStreamPublisher{js: js}
	for range 2 {
		require.NoError(t, pub.PublishCapturedTimeout(context.Background(), id, raw))
	}
	require.Len(t, js.messages, 2)
	for _, message := range js.messages {
		require.Equal(t, raw, message.Data)
		require.Equal(t, id.String(), message.Header.Get(nats.MsgIdHdr))
		require.Equal(t, "mm.search_timeout", message.Subject)
	}
	js.noAck = true
	require.Error(t, pub.PublishCapturedTimeout(context.Background(), id, raw))
}

func TestNativeCapturedTimeoutRejectsOtherOneofAndNoncanonicalFacts(t *testing.T) {
	for _, kind := range []string{"cancel", "unknown", "duplicate", "identity", "mode", "event", "timestamp"} {
		t.Run(kind, func(t *testing.T) {
			id := uuid.New()
			fact := &eventsv1.MatchTimeout{SessionId: uuid.NewString(), ProfileId: uuid.NewString(), GameId: uuid.NewString(), Mode: "ranked"}
			event := &eventsv1.MatchmakingStreamEvent{EventId: id.String(), OccurredAt: timestamppb.Now(), Payload: &eventsv1.MatchmakingStreamEvent_MatchTimeout{MatchTimeout: fact}}
			switch kind {
			case "cancel":
				event.Payload = &eventsv1.MatchmakingStreamEvent_SearchCancelled{SearchCancelled: &eventsv1.SearchCancelled{SessionId: fact.SessionId, ProfileId: fact.ProfileId}}
			case "unknown":
				fact.ProtoReflect().SetUnknown([]byte{0xa0, 6, 1})
			case "identity":
				fact.GameId = "not-a-uuid"
			case "mode":
				fact.Mode = " ranked "
			case "event":
				event.EventId = uuid.NewString()
			case "timestamp":
				event.OccurredAt = nil
			}
			raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(event)
			require.NoError(t, err)
			if kind == "duplicate" {
				raw = append(raw, 10, 36)
				raw = append(raw, []byte(event.EventId)...)
			}
			js := &rosterPublication{}
			require.Error(t, (&JetStreamPublisher{js: js}).PublishCapturedTimeout(context.Background(), id, raw))
			require.Empty(t, js.messages)
		})
	}
}
