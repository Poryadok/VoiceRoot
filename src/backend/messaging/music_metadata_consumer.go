package main

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"log/slog"
	"time"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/messaging/internal/store"
)

const musicMetadataStream = "file_events"
const musicMetadataDurable = "messaging_music_metadata"
const musicMetadataSubject = "file.processed"

type musicMetadataStore interface {
	PersistMusicSnapshot(context.Context, *eventsv1.FileProcessed) (*eventsv1.FileProcessed, error)
	MusicRefreshChats(context.Context, *eventsv1.FileProcessed) ([]uuid.UUID, error)
	RefreshMusicMetadata(context.Context, uuid.UUID, *uuid.UUID, *eventsv1.FileProcessed) (int, error)
}
type musicChatPolicy interface {
	Load(context.Context, uuid.UUID) (*store.ChatThreadPolicy, error)
}

func musicMetadataConsumerConfig() *nats.ConsumerConfig {
	return &nats.ConsumerConfig{
		Durable: musicMetadataDurable, DeliverSubject: "_INBOX.voice.messaging." + musicMetadataDurable,
		DeliverGroup: musicMetadataDurable, DeliverPolicy: nats.DeliverNewPolicy, AckPolicy: nats.AckExplicitPolicy, FilterSubject: musicMetadataSubject,
	}
}

func decodeMusicSnapshot(data []byte) (*eventsv1.FileProcessed, error) {
	if len(data) > 8192 {
		return nil, errors.New("music event exceeds limit")
	}
	var envelope eventsv1.FileStreamEvent
	if proto.Unmarshal(data, &envelope) != nil {
		return nil, errors.New("music event malformed")
	}
	event := envelope.GetFileProcessed()
	if event.GetMusic() == nil {
		return nil, nil
	} // Existing image/document events.
	id, err := uuid.Parse(envelope.GetEventId())
	if err != nil || id == uuid.Nil {
		return nil, errors.New("music event identity invalid")
	}
	if err = store.ValidateMusicSnapshot(event); err != nil {
		return nil, errors.New("music event authority invalid")
	}
	return event, nil
}

func applyMusicSnapshot(ctx context.Context, messages musicMetadataStore, policy musicChatPolicy, event *eventsv1.FileProcessed) error {
	current, err := messages.PersistMusicSnapshot(ctx, event)
	if err != nil {
		return err
	}
	chats, err := messages.MusicRefreshChats(ctx, current)
	if err != nil {
		return err
	}
	for _, chat := range chats {
		scope, err := policy.Load(ctx, chat)
		if errors.Is(err, store.ErrChatNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		if scope == nil {
			return errors.New("music destination policy unavailable")
		}
		for {
			n, err := messages.RefreshMusicMetadata(ctx, chat, scope.SpaceID, current)
			if err != nil {
				return err
			}
			if n < 100 {
				break
			}
		}
	}
	return nil
}

func runMusicMetadataConsumer(ctx context.Context, url string, messages musicMetadataStore, policy musicChatPolicy, logger *slog.Logger) error {
	for {
		err := runMusicMetadataConsumerOnce(ctx, url, messages, policy, logger)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			logger.WarnContext(ctx, "music metadata consumer unavailable")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func runMusicMetadataConsumerOnce(ctx context.Context, url string, messages musicMetadataStore, policy musicChatPolicy, logger *slog.Logger) error {
	nc, err := nats.Connect(url, nats.Name("voice-messaging-music-metadata"), nats.CustomInboxPrefix("_INBOX.voice.messaging"), nats.Timeout(10*time.Second), nats.MaxReconnects(-1), nats.ReconnectWait(time.Second))
	if err != nil {
		return err
	}
	defer nc.Close()
	js, err := nc.JetStream()
	if err != nil {
		return err
	}
	want := musicMetadataConsumerConfig()
	actual, err := js.ConsumerInfo(musicMetadataStream, musicMetadataDurable)
	if err != nil {
		return err
	}
	if actual == nil || actual.Stream != musicMetadataStream || actual.Name != want.Durable || actual.Config.Durable != want.Durable || actual.Config.DeliverSubject != want.DeliverSubject || actual.Config.DeliverGroup != want.DeliverGroup || actual.Config.DeliverPolicy != want.DeliverPolicy || actual.Config.AckPolicy != want.AckPolicy || actual.Config.FilterSubject != want.FilterSubject || len(actual.Config.FilterSubjects) != 0 {
		return errors.New("music metadata durable mismatch")
	}
	sub, err := js.QueueSubscribe(musicMetadataSubject, musicMetadataDurable, func(msg *nats.Msg) {
		event, err := decodeMusicSnapshot(msg.Data)
		if err != nil {
			logger.WarnContext(ctx, "music metadata event rejected")
			_ = msg.Term()
			return
		}
		if event == nil {
			_ = msg.Ack()
			return
		}
		attempt, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		if err = applyMusicSnapshot(attempt, messages, policy, event); err != nil {
			logger.WarnContext(ctx, "music metadata refresh pending")
			_ = msg.NakWithDelay(time.Second)
			return
		}
		_ = msg.Ack()
	}, nats.Bind(musicMetadataStream, musicMetadataDurable), nats.ManualAck())
	if err != nil {
		return err
	}
	defer func() { _ = sub.Unsubscribe() }()
	<-ctx.Done()
	return ctx.Err()
}
