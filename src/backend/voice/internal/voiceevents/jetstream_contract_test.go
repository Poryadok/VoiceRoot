package voiceevents

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
)

type recordingJetStream struct {
	messages []*nats.Msg
}

func (r *recordingJetStream) StreamInfo(string, ...nats.JSOpt) (*nats.StreamInfo, error) {
	return &nats.StreamInfo{Config: nats.StreamConfig{Name: streamName, Subjects: []string{"voice.>"}}}, nil
}

func (r *recordingJetStream) AddStream(*nats.StreamConfig, ...nats.JSOpt) (*nats.StreamInfo, error) {
	return nil, nil
}

func (r *recordingJetStream) PublishMsg(message *nats.Msg, _ ...nats.PubOpt) (*nats.PubAck, error) {
	r.messages = append(r.messages, message)
	return &nats.PubAck{Stream: streamName, Sequence: uint64(len(r.messages))}, nil
}

func TestJetStreamPublisher_CompatibilityEventsUseExactSubjectsAndEnvelopes(t *testing.T) {
	stream := &recordingJetStream{}
	publisher := &JetStreamPublisher{js: stream}
	roomID, chatID := uuid.NewString(), uuid.NewString()
	owner, member := uuid.NewString(), uuid.NewString()
	voiceRoomID, spaceID := uuid.NewString(), uuid.NewString()

	require.NoError(t, publisher.PublishCallStarted(t.Context(), &eventsv1.CallStarted{
		RoomId: roomID, ChatId: chatID, InitiatorProfileId: owner,
		ProfileIds: []string{owner, member}, MediaKind: "audio", LivekitRoomName: "voice-room-" + voiceRoomID,
		VoiceRoomId: proto.String(voiceRoomID), SpaceId: proto.String(spaceID), RoomType: proto.String("voice_room"),
	}))
	require.NoError(t, publisher.PublishVoiceMemberJoined(t.Context(), &eventsv1.VoiceMemberJoined{
		RoomId: roomID, VoiceRoomId: voiceRoomID, SpaceId: spaceID, JoinedProfileId: member,
		NotifyProfileIds: []string{owner},
	}))

	require.Len(t, stream.messages, 2)
	for index, want := range []struct {
		subject string
		assert  func(*testing.T, *eventsv1.VoiceStreamEvent)
	}{
		{
			subject: "voice.call_started",
			assert: func(t *testing.T, event *eventsv1.VoiceStreamEvent) {
				t.Helper()
				got := event.GetCallStarted()
				require.NotNil(t, got)
				require.Equal(t, roomID, got.GetRoomId())
				require.Equal(t, []string{owner, member}, got.GetProfileIds())
				require.Equal(t, chatID, got.GetChatId())
				require.Equal(t, owner, got.GetInitiatorProfileId())
				require.Equal(t, "audio", got.GetMediaKind())
				require.Equal(t, "voice-room-"+voiceRoomID, got.GetLivekitRoomName())
				require.Equal(t, voiceRoomID, got.GetVoiceRoomId())
				require.Equal(t, spaceID, got.GetSpaceId())
				require.Equal(t, "voice_room", got.GetRoomType())
			},
		},
		{
			subject: "voice.member_joined",
			assert: func(t *testing.T, event *eventsv1.VoiceStreamEvent) {
				t.Helper()
				got := event.GetVoiceMemberJoined()
				require.NotNil(t, got)
				require.Equal(t, roomID, got.GetRoomId())
				require.Equal(t, voiceRoomID, got.GetVoiceRoomId())
				require.Equal(t, spaceID, got.GetSpaceId())
				require.Equal(t, member, got.GetJoinedProfileId())
				require.Equal(t, []string{owner}, got.GetNotifyProfileIds())
			},
		},
	} {
		message := stream.messages[index]
		require.Equal(t, want.subject, message.Subject)
		var envelope eventsv1.VoiceStreamEvent
		require.NoError(t, proto.Unmarshal(message.Data, &envelope))
		require.NoError(t, uuid.Validate(envelope.GetEventId()))
		require.NotNil(t, envelope.GetOccurredAt())
		require.True(t, envelope.GetOccurredAt().IsValid())
		want.assert(t, &envelope)
	}
}

func TestJetStreamPublisher_CompatibilityPublishFailureIsReturnedAfterLifecycleCommit(t *testing.T) {
	// The caller has already committed its lifecycle mutation before invoking
	// this best-effort compatibility publisher; errors are intentionally not a
	// transaction rollback signal.
	publisher := &JetStreamPublisher{js: &failingJetStream{}}
	err := publisher.PublishCallStarted(context.Background(), &eventsv1.CallStarted{RoomId: uuid.NewString()})
	require.Error(t, err)
}

type failingJetStream struct{}

func (failingJetStream) StreamInfo(string, ...nats.JSOpt) (*nats.StreamInfo, error) {
	return &nats.StreamInfo{Config: nats.StreamConfig{Name: streamName, Subjects: []string{"voice.>"}}}, nil
}

func (failingJetStream) AddStream(*nats.StreamConfig, ...nats.JSOpt) (*nats.StreamInfo, error) {
	return nil, nil
}

func (failingJetStream) PublishMsg(*nats.Msg, ...nats.PubOpt) (*nats.PubAck, error) {
	return nil, nats.ErrDisconnected
}
