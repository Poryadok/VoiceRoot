package storyconsume

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/matchmaking/internal/store"
)

const streamName = "story_events"

const subjectStoryLfpCreated = "story.lfp_created"
const subjectStoryLfpResponse = "story.lfp_response"
const defaultDurable = "matchmaking_story_lfp_v2"
const storyDeliverySubject = "_INBOX.voice.matchmaking.matchmaking_story_lfp_v2"

var errInvalidStoryEvent = errors.New("invalid story LFP event")

type Consumer struct {
	nc  *nats.Conn
	sub *nats.Subscription
}

func validateStoryDurable(info *nats.ConsumerInfo) error {
	if info == nil || info.Stream != streamName || info.Name != defaultDurable ||
		info.Config.Durable != defaultDurable || info.Config.FilterSubject != "" ||
		len(info.Config.FilterSubjects) != 2 || info.Config.DeliverSubject != storyDeliverySubject ||
		info.Config.DeliverGroup != "" || info.Config.DeliverPolicy != nats.DeliverAllPolicy ||
		info.Config.AckPolicy != nats.AckExplicitPolicy {
		return fmt.Errorf("story LFP durable %q has incompatible configuration", defaultDurable)
	}
	seen := map[string]bool{}
	for _, subject := range info.Config.FilterSubjects {
		seen[subject] = true
	}
	if !seen[subjectStoryLfpCreated] || !seen[subjectStoryLfpResponse] || len(seen) != 2 {
		return fmt.Errorf("story LFP durable %q has incompatible subjects", defaultDurable)
	}
	return nil
}

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
			return fmt.Errorf("%w: invalid story_id: %v", errInvalidStoryEvent, err)
		}
		authorID, err := uuid.Parse(strings.TrimSpace(ev.GetAuthorProfileId()))
		if err != nil {
			return fmt.Errorf("%w: invalid author_profile_id: %v", errInvalidStoryEvent, err)
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
			return fmt.Errorf("%w: invalid story_id: %v", errInvalidStoryEvent, err)
		}
		authorID, err := uuid.Parse(strings.TrimSpace(ev.GetAuthorProfileId()))
		if err != nil {
			return fmt.Errorf("%w: invalid author_profile_id: %v", errInvalidStoryEvent, err)
		}
		responderID, err := uuid.Parse(strings.TrimSpace(ev.GetResponderProfileId()))
		if err != nil {
			return fmt.Errorf("%w: invalid responder_profile_id: %v", errInvalidStoryEvent, err)
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

// Start validates and binds the preprovisioned story LFP durable.
func Start(ctx context.Context, natsURL, durable string, lfp LfpStore) (*Consumer, error) {
	if lfp == nil {
		return nil, fmt.Errorf("lfp store required")
	}
	url := strings.TrimSpace(natsURL)
	if url == "" {
		return nil, fmt.Errorf("missing NATS_URL")
	}
	if strings.TrimSpace(durable) == "" {
		durable = defaultDurable
	}
	if durable != defaultDurable {
		return nil, fmt.Errorf("unsupported story LFP durable %q", durable)
	}
	nc, err := nats.Connect(url,
		nats.Name("voice-matchmaking-story-lfp"),
		nats.Timeout(10*time.Second),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}

	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("jetstream: %w", err)
	}
	info, err := js.ConsumerInfo(streamName, durable)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("inspect story LFP durable: %w", err)
	}
	if err := validateStoryDurable(info); err != nil {
		nc.Close()
		return nil, err
	}

	msgHandler := func(msg *nats.Msg) {
		var env eventsv1.StoryStreamEvent
		if err := proto.Unmarshal(msg.Data, &env); err != nil {
			_ = msg.Term()
			return
		}
		if err := ApplyStoryEvent(lfp, &env); err != nil {
			if errors.Is(err, errInvalidStoryEvent) {
				_ = msg.Term()
				return
			}
			_ = msg.Nak()
			return
		}
		_ = msg.Ack()
	}

	sub, err := js.Subscribe("", msgHandler, nats.Bind(streamName, durable), nats.ManualAck())
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("bind story LFP durable: %w", err)
	}
	return &Consumer{nc: nc, sub: sub}, nil
}

func (c *Consumer) Close() {
	if c != nil && c.nc != nil {
		_ = c.nc.Drain()
	}
}
func (c *Consumer) Run(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

// Run binds and processes story LFP events until cancellation.
func Run(ctx context.Context, natsURL, durable string, lfp LfpStore) error {
	c, err := Start(ctx, natsURL, durable, lfp)
	if err != nil {
		return err
	}
	defer c.Close()
	return c.Run(ctx)
}
