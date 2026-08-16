package storyconsume

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	"voice/backend/matchmaking/internal/store"
	eventsv1 "voice.app/voice/events/v1"
)

const streamName = "story_events"

// LfpStore applies story.lfp_* events into matchmaking_db.
type LfpStore interface {
	UpsertListing(ctx context.Context, storyID, authorID uuid.UUID, criteriaJSON string) (store.LfpListing, error)
	UpsertRequest(ctx context.Context, storyID, authorID, responderID uuid.UUID, responseType string) (store.LfpRequest, error)
	GetListing(ctx context.Context, storyID uuid.UUID) (store.LfpListing, error)
}

// ApplyStoryEvent mirrors LFP story events into matchmaking listings/requests.
func ApplyStoryEvent(lfp LfpStore, env *eventsv1.StoryStreamEvent) error {
	if lfp == nil || env == nil {
		return nil
	}
	switch p := env.GetPayload().(type) {
	case *eventsv1.StoryStreamEvent_StoryLfpCreated:
		ev := p.StoryLfpCreated
		if ev == nil {
			return nil
		}
		storyID, err := uuid.Parse(strings.TrimSpace(ev.GetStoryId()))
		if err != nil {
			return fmt.Errorf("invalid story_id: %w", err)
		}
		authorID, err := uuid.Parse(strings.TrimSpace(ev.GetAuthorProfileId()))
		if err != nil {
			return fmt.Errorf("invalid author_profile_id: %w", err)
		}
		_, err = lfp.UpsertListing(context.Background(), storyID, authorID, ev.GetCriteriaJson())
		return err
	case *eventsv1.StoryStreamEvent_StoryLfpResponse:
		ev := p.StoryLfpResponse
		if ev == nil {
			return nil
		}
		storyID, err := uuid.Parse(strings.TrimSpace(ev.GetStoryId()))
		if err != nil {
			return fmt.Errorf("invalid story_id: %w", err)
		}
		authorID, err := uuid.Parse(strings.TrimSpace(ev.GetAuthorProfileId()))
		if err != nil {
			return fmt.Errorf("invalid author_profile_id: %w", err)
		}
		responderID, err := uuid.Parse(strings.TrimSpace(ev.GetResponderProfileId()))
		if err != nil {
			return fmt.Errorf("invalid responder_profile_id: %w", err)
		}
		if _, err := lfp.GetListing(context.Background(), storyID); err != nil {
			// Ensure listing exists so FK succeeds even if lfp_created was missed.
			if _, upsertErr := lfp.UpsertListing(context.Background(), storyID, authorID, `{}`); upsertErr != nil {
				return upsertErr
			}
		}
		_, err = lfp.UpsertRequest(context.Background(), storyID, authorID, responderID, ev.GetResponseType())
		return err
	default:
		return nil
	}
}

// Run starts a durable JetStream consumer for story.lfp_* subjects.
func Run(ctx context.Context, natsURL, durable string, lfp LfpStore) error {
	if lfp == nil {
		return fmt.Errorf("lfp store required")
	}
	url := strings.TrimSpace(natsURL)
	if url == "" {
		return fmt.Errorf("missing NATS_URL")
	}
	if strings.TrimSpace(durable) == "" {
		durable = "matchmaking_story_lfp"
	}
	nc, err := nats.Connect(url,
		nats.Name("voice-matchmaking-story-lfp"),
		nats.Timeout(10*time.Second),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Second),
	)
	if err != nil {
		return fmt.Errorf("nats connect: %w", err)
	}
	go func() {
		<-ctx.Done()
		_ = nc.Drain()
	}()

	js, err := nc.JetStream()
	if err != nil {
		return fmt.Errorf("jetstream: %w", err)
	}

	msgHandler := func(msg *nats.Msg) {
		var env eventsv1.StoryStreamEvent
		if err := proto.Unmarshal(msg.Data, &env); err != nil {
			_ = msg.Term()
			return
		}
		if err := ApplyStoryEvent(lfp, &env); err != nil {
			_ = msg.Nak()
			return
		}
		_ = msg.Ack()
	}

	sub, err := js.Subscribe("story.lfp_>", msgHandler,
		nats.Durable(durable),
		nats.BindStream(streamName),
		nats.ManualAck(),
	)
	if err != nil {
		sub, err = js.Subscribe("", msgHandler, nats.Bind(streamName, durable), nats.ManualAck())
		if err != nil {
			return fmt.Errorf("jetstream subscribe story.lfp_*: %w", err)
		}
	}
	defer func() { _ = sub.Unsubscribe() }()

	<-ctx.Done()
	return ctx.Err()
}
