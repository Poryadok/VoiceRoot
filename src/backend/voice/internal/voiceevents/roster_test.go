package voiceevents

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"testing"
	"time"
	eventsv1 "voice.app/voice/events/v1"
)

func TestRosterDurablePublicationPreservesBytesAndRejectsInvalidBinding(t *testing.T) {
	id := uuid.New()
	fact := &eventsv1.VoiceRosterMembershipChanged{ProtocolVersion: 1, RoomId: uuid.NewString(), RosterVersion: 2, Change: eventsv1.VoiceRosterMembershipChanged_CHANGE_JOINED, ProfileId: uuid.NewString()}
	env := &eventsv1.VoiceStreamEvent{EventId: id.String(), OccurredAt: timestamppb.New(time.Now()), Payload: &eventsv1.VoiceStreamEvent_VoiceRosterMembershipChanged{VoiceRosterMembershipChanged: fact}}
	js := &recordingJetStream{}
	p := &JetStreamPublisher{js: js}
	b, err := proto.MarshalOptions{Deterministic: true}.Marshal(env)
	require.NoError(t, err)
	for range 2 {
		require.NoError(t, p.PublishAdmissionEvent(context.Background(), id, "voice.roster_membership_changed", b))
	}
	require.Len(t, js.messages, 2)
	for _, m := range js.messages {
		require.Equal(t, b, m.Data)
		require.Equal(t, id.String(), m.Header.Get("Nats-Msg-Id"))
	}
	for _, mutate := range []func(){func() { fact.ProtocolVersion = 2 }, func() { fact.RosterVersion = 0 }, func() { fact.Change = eventsv1.VoiceRosterMembershipChanged_CHANGE_UNSPECIFIED }, func() { fact.ProfileId = "invalid" }} {
		fact.ProtocolVersion = 1
		fact.RosterVersion = 2
		fact.Change = eventsv1.VoiceRosterMembershipChanged_CHANGE_JOINED
		fact.ProfileId = uuid.NewString()
		mutate()
		invalid, err := proto.Marshal(env)
		require.NoError(t, err)
		require.Error(t, p.PublishAdmissionEvent(context.Background(), id, "voice.roster_membership_changed", invalid))
	}
	require.Error(t, p.PublishAdmissionEvent(context.Background(), uuid.New(), "voice.roster_membership_changed", b))
	require.Error(t, p.PublishAdmissionEvent(context.Background(), id, "voice.participant_joined", b))
	require.Len(t, js.messages, 2, "invalid facts never reach JetStream")
}
