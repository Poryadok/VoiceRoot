package userevents

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/pkg/correlation"
	"voice/backend/pkg/natslog"
	"voice/backend/user/internal/store"
)

const (
	streamName             = "user_events"
	subjectAccountDeleted  = "user.account_deleted"
	subjectProfileCreated  = "user.profile_created"
	subjectProfileUpdated  = "user.profile_updated"
	subjectProfileSwitched = "user.profile_switched"
	subjectProfileVerified = "user.verified"
	subjectPresenceChanged = "user.presence_changed"
	subjectGameDetected    = "user.game_detected"
	subjectSettingsChanged = "user.settings_changed"
)

// JetStreamPublisher publishes UserStreamEvent payloads to NATS JetStream.
type JetStreamPublisher struct {
	nc     *nats.Conn
	js     nats.JetStreamContext
	Logger *slog.Logger

	ensureOnce sync.Once
	ensureErr  error
}

// AccountDeletionConsumer is the only User consumer of Auth's account-state
// event. It ACKs only after the User transaction has committed its inbox,
// lifecycle overlay and Search delete journal/outbox records.
type AccountDeletionConsumer struct {
	js         nats.JetStreamContext
	profiles   *store.ProfileStore
	connection *JetStreamPublisher
	sub        *nats.Subscription
	bind       func() (*nats.Subscription, error)
}

func NewAccountDeletionConsumer(natsURL, credentialsFile string, profiles *store.ProfileStore) (*AccountDeletionConsumer, error) {
	if profiles == nil {
		return nil, fmt.Errorf("account deletion consumer not initialized")
	}
	connection, err := NewJetStreamConsumerConnection(natsURL, credentialsFile)
	if err != nil {
		return nil, err
	}
	return &AccountDeletionConsumer{js: connection.js, profiles: profiles, connection: connection}, nil
}

func (c *AccountDeletionConsumer) Close() error {
	if c == nil || c.connection == nil {
		return nil
	}
	return c.connection.Close()
}

// Start establishes the durable, broker-authorized subscription before the
// process serves traffic. Missing streams or denied consumer permissions are a
// startup failure, never a background warning.
func (c *AccountDeletionConsumer) Start() error {
	if c == nil || c.profiles == nil {
		return fmt.Errorf("account deletion consumer not initialized")
	}
	if c.sub != nil {
		return nil
	}
	bind := c.bind
	if bind == nil {
		if c.js == nil {
			return fmt.Errorf("account deletion consumer not initialized")
		}
		bind = func() (*nats.Subscription, error) {
			return c.js.PullSubscribe(subjectAccountDeleted, "user-account-deletion-v1", nats.BindStream(streamName))
		}
	}
	sub, err := bind()
	if err != nil {
		return fmt.Errorf("bind account deletion consumer: %w", err)
	}
	c.sub = sub
	return nil
}

func (c *AccountDeletionConsumer) Run(ctx context.Context) error {
	if c == nil || c.js == nil || c.profiles == nil {
		return fmt.Errorf("account deletion consumer not initialized")
	}
	if err := c.Start(); err != nil {
		return err
	}
	sub := c.sub
	for ctx.Err() == nil {
		messages, err := sub.Fetch(1, nats.MaxWait(time.Second))
		if err != nil {
			if errors.Is(err, nats.ErrTimeout) {
				continue
			}
			return err
		}
		for _, message := range messages {
			if err := c.handle(ctx, message); err != nil {
				continue
			}
		}
	}
	return ctx.Err()
}

func (c *AccountDeletionConsumer) handle(ctx context.Context, message *nats.Msg) error {
	if message == nil {
		return fmt.Errorf("empty account deletion message")
	}
	var envelope eventsv1.UserStreamEvent
	if err := proto.Unmarshal(message.Data, &envelope); err != nil {
		return fmt.Errorf("malformed account deletion envelope: %w", err)
	}
	eventID, err := uuid.Parse(envelope.GetEventId())
	if err != nil || eventID == uuid.Nil {
		return fmt.Errorf("invalid account deletion event_id")
	}
	deleted := envelope.GetUserAccountDeleted()
	accountID, err := uuid.Parse(deleted.GetAccountId())
	if deleted == nil || err != nil || accountID == uuid.Nil {
		return fmt.Errorf("invalid account deletion account_id")
	}
	occurredAt := envelope.GetOccurredAt()
	if occurredAt == nil || !occurredAt.IsValid() {
		return fmt.Errorf("invalid account deletion occurred_at")
	}
	when := occurredAt.AsTime()
	if err := c.profiles.ApplyAccountDeleted(ctx, eventID, accountID, when); err != nil {
		return err
	}
	return message.Ack()
}

// NewJetStreamPublisher connects to NATS_URL and prepares JetStream for user.events.
func NewJetStreamPublisher(natsURL string) (*JetStreamPublisher, error) {
	return newJetStreamPublisher(natsURL)
}

// NewJetStreamConsumerConnection uses the dedicated credentials mounted for a
// least-privilege account-deletion consumer. It deliberately does not call
// ensureStream: stream administration is not a consumer permission.
func NewJetStreamConsumerConnection(natsURL, credentialsFile string) (*JetStreamPublisher, error) {
	credentialsFile = strings.TrimSpace(credentialsFile)
	if credentialsFile == "" {
		return nil, fmt.Errorf("account deletion NATS credentials file is required")
	}
	return newJetStreamPublisher(natsURL, nats.UserCredentials(credentialsFile))
}

func newJetStreamPublisher(natsURL string, options ...nats.Option) (*JetStreamPublisher, error) {
	if natsURL == "" {
		return nil, fmt.Errorf("empty NATS URL")
	}
	options = append(options,
		nats.Name("voice-user-user-events"),
		nats.Timeout(10*time.Second),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Second),
	)
	nc, err := nats.Connect(natsURL, options...)
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}
	js, err := nc.JetStream()
	if err != nil {
		_ = nc.Drain()
		return nil, fmt.Errorf("jetstream: %w", err)
	}
	return &JetStreamPublisher{nc: nc, js: js}, nil
}

func (p *JetStreamPublisher) ensureStream() error {
	if p == nil || p.js == nil {
		return fmt.Errorf("jetstream publisher not initialized")
	}
	p.ensureOnce.Do(func() {
		subjects := []string{
			subjectAccountDeleted,
			subjectProfileCreated,
			subjectProfileUpdated,
			subjectProfileSwitched,
			subjectProfileVerified,
			subjectPresenceChanged,
			subjectGameDetected,
			subjectSettingsChanged,
		}
		if info, err := p.js.StreamInfo(streamName); err == nil {
			for _, subj := range subjects {
				if !streamHasSubject(info, subj) {
					cfg := info.Config
					cfg.Subjects = append(cfg.Subjects, subj)
					_, p.ensureErr = p.js.UpdateStream(&cfg)
					if p.ensureErr != nil {
						return
					}
					info, p.ensureErr = p.js.StreamInfo(streamName)
					if p.ensureErr != nil {
						return
					}
				}
			}
			return
		}
		_, p.ensureErr = p.js.AddStream(&nats.StreamConfig{
			Name:      streamName,
			Subjects:  subjects,
			Retention: nats.LimitsPolicy,
			MaxAge:    7 * 24 * time.Hour,
			Storage:   nats.FileStorage,
		})
	})
	return p.ensureErr
}

func streamHasSubject(info *nats.StreamInfo, subject string) bool {
	if info == nil {
		return false
	}
	for _, s := range info.Config.Subjects {
		if s == subject {
			return true
		}
	}
	return false
}

func (p *JetStreamPublisher) publishProto(ctx context.Context, subject string, env *eventsv1.UserStreamEvent) error {
	if err := p.ensureStream(); err != nil {
		return err
	}
	b, err := proto.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal UserStreamEvent: %w", err)
	}
	requestID := correlation.FromGRPC(ctx)
	msg := &nats.Msg{Subject: subject, Data: b, Header: nats.Header{}}
	natslog.SetRequestIDHeader(msg.Header, requestID)
	if _, err := p.js.PublishMsg(msg); err != nil {
		return fmt.Errorf("jetstream publish %s: %w", subject, err)
	}
	natslog.LogPublish(p.Logger, subject, requestID, "user event published",
		slog.String("event_id", env.GetEventId()))
	return nil
}

// PublishProfileCreated emits user.profile_created.
func (p *JetStreamPublisher) PublishProfileCreated(ctx context.Context, profileID, accountID string) error {
	env := &eventsv1.UserStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
		Payload: &eventsv1.UserStreamEvent_ProfileCreated{
			ProfileCreated: &eventsv1.ProfileCreated{
				ProfileId: profileID,
				AccountId: accountID,
			},
		},
	}
	return p.publishProto(ctx, subjectProfileCreated, env)
}

// PublishProfileUpdated emits user.profile_updated after a committed profile mutation.
func (p *JetStreamPublisher) PublishProfileUpdated(ctx context.Context, profileID string, changedFields []string) error {
	env := &eventsv1.UserStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
		Payload: &eventsv1.UserStreamEvent_ProfileUpdated{
			ProfileUpdated: &eventsv1.ProfileUpdated{
				ProfileId:     profileID,
				ChangedFields: changedFields,
			},
		},
	}
	return p.publishProto(ctx, subjectProfileUpdated, env)
}

// PublishProfileSwitched emits user.profile_switched.
func (p *JetStreamPublisher) PublishProfileSwitched(ctx context.Context, accountID, oldProfileID, newProfileID string) error {
	env := &eventsv1.UserStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
		Payload: &eventsv1.UserStreamEvent_ProfileSwitched{
			ProfileSwitched: &eventsv1.ProfileSwitched{
				AccountId:    accountID,
				ProfileId:    newProfileID,
				OldProfileId: oldProfileID,
				NewProfileId: newProfileID,
			},
		},
	}
	return p.publishProto(ctx, subjectProfileSwitched, env)
}

// PublishVerified emits user.verified.
func (p *JetStreamPublisher) PublishVerified(ctx context.Context, profileID, verificationType string) error {
	env := &eventsv1.UserStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
		Payload: &eventsv1.UserStreamEvent_ProfileVerified{
			ProfileVerified: &eventsv1.ProfileVerified{
				ProfileId:        profileID,
				VerificationType: verificationType,
			},
		},
	}
	return p.publishProto(ctx, subjectProfileVerified, env)
}

func newPresenceChangedEvent(profileID, oldStatus, newStatus string) *eventsv1.UserStreamEvent {
	return &eventsv1.UserStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
		Payload: &eventsv1.UserStreamEvent_PresenceChange{
			PresenceChange: &eventsv1.PresenceChange{
				ProfileId: profileID,
				Status:    newStatus,
				OldStatus: oldStatus,
				NewStatus: newStatus,
			},
		},
	}
}

// PublishPresenceChanged emits user.presence_changed (docs/microservices/user-service.md).
func (p *JetStreamPublisher) PublishPresenceChanged(ctx context.Context, profileID, oldStatus, newStatus string) error {
	return p.publishProto(ctx, subjectPresenceChanged, newPresenceChangedEvent(profileID, oldStatus, newStatus))
}

// PublishGameDetected emits user.game_detected after a new non-empty game title is persisted.
func (p *JetStreamPublisher) PublishGameDetected(ctx context.Context, profileID, gameName string) error {
	env := &eventsv1.UserStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
		Payload: &eventsv1.UserStreamEvent_GameDetected{
			GameDetected: &eventsv1.GameDetected{ProfileId: profileID, GameName: gameName},
		},
	}
	return p.publishProto(ctx, subjectGameDetected, env)
}

// PublishSettingsChanged emits user.settings_changed after a committed settings update.
func (p *JetStreamPublisher) PublishSettingsChanged(ctx context.Context, profileID string, changedKeys []string, changedKeysJSON string) error {
	env := &eventsv1.UserStreamEvent{
		EventId: uuid.NewString(), OccurredAt: timestamppb.New(time.Now().UTC()),
		Payload: &eventsv1.UserStreamEvent_SettingsChanged{SettingsChanged: &eventsv1.SettingsChanged{
			ProfileId:       profileID,
			ChangedKeys:     changedKeys,
			ChangedKeysJson: changedKeysJSON,
		}},
	}
	return p.publishProto(ctx, subjectSettingsChanged, env)
}

// Close drains the underlying NATS connection.
func (p *JetStreamPublisher) Close() error {
	if p == nil || p.nc == nil {
		return nil
	}
	return p.nc.Drain()
}
