package main

import (
	"context"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"testing"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/messaging/internal/store"
)

func TestMusicConsumerContractAndBoundedEventAdmission(t *testing.T) {
	config := musicMetadataConsumerConfig()
	require.Equal(t, "messaging_music_metadata", config.Durable)
	require.Equal(t, "_INBOX.voice.messaging.messaging_music_metadata", config.DeliverSubject)
	require.Equal(t, config.Durable, config.DeliverGroup)
	require.Equal(t, "file.processed", config.FilterSubject)
	require.Equal(t, nats.DeliverNewPolicy, config.DeliverPolicy)
	require.Equal(t, nats.AckExplicitPolicy, config.AckPolicy)
	_, err := decodeMusicSnapshot(make([]byte, 8193))
	require.Error(t, err)
	envelope := &eventsv1.FileStreamEvent{EventId: uuid.NewString(), Payload: &eventsv1.FileStreamEvent_FileProcessed{FileProcessed: &eventsv1.FileProcessed{FileId: uuid.NewString(), Status: "ready"}}}
	body, err := proto.Marshal(envelope)
	require.NoError(t, err)
	event, err := decodeMusicSnapshot(body)
	require.NoError(t, err)
	require.Nil(t, event, "legacy image processing remains unrelated")
	envelope.GetFileProcessed().Music = &eventsv1.FileMusicMetadata{ChatId: uuid.NewString(), MetadataRevision: 1, SizeBytes: 40, ScanResult: "skipped", DurationSeconds: 2}
	body, err = proto.Marshal(envelope)
	require.NoError(t, err)
	event, err = decodeMusicSnapshot(body)
	require.NoError(t, err)
	require.NotNil(t, event)
	envelope.GetFileProcessed().Music.ScanResult = "infected"
	body, err = proto.Marshal(envelope)
	require.NoError(t, err)
	_, err = decodeMusicSnapshot(body)
	require.Error(t, err)
}

type musicRefreshFake struct {
	chat, scope uuid.UUID
	calls       int
}

func (f *musicRefreshFake) MusicRefreshChats(context.Context, *eventsv1.FileProcessed) ([]uuid.UUID, error) {
	return []uuid.UUID{f.chat}, nil
}
func (f *musicRefreshFake) RefreshMusicMetadata(_ context.Context, chat uuid.UUID, scope *uuid.UUID, _ *eventsv1.FileProcessed) (int, error) {
	if chat != f.chat || scope == nil || *scope != f.scope {
		panic("wrong destination fence")
	}
	f.calls++
	if f.calls == 1 {
		return 100, nil
	}
	return 1, nil
}

type musicPolicyFake struct{ scope uuid.UUID }

func (f musicPolicyFake) Load(context.Context, uuid.UUID) (*store.ChatThreadPolicy, error) {
	return &store.ChatThreadPolicy{SpaceID: &f.scope}, nil
}
func TestMusicConsumerUsesDestinationScopeForEveryBatch(t *testing.T) {
	fake := &musicRefreshFake{chat: uuid.New(), scope: uuid.New()}
	require.NoError(t, applyMusicSnapshot(context.Background(), fake, musicPolicyFake{scope: fake.scope}, &eventsv1.FileProcessed{}))
	require.Equal(t, 2, fake.calls)
}
