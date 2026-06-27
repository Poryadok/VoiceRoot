package roleevents

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/nats-io/nats.go"

	"voice/backend/pkg/correlation"
	"voice/backend/pkg/natslog"
)

const (
	streamName           = "role_events"
	subjectRoleCreated   = "role.created"
	subjectRoleUpdated   = "role.updated"
	subjectRoleDeleted   = "role.deleted"
	subjectRoleAssigned  = "role.assigned"
	subjectRoleRevoked   = "role.revoked"
	subjectChatOverride  = "role.chat_override_set"
	subjectVoiceOverride = "role.voice_override_set"
)

type roleEventPayload struct {
	SpaceID     string `json:"space_id,omitempty"`
	RoleID      string `json:"role_id,omitempty"`
	ProfileID   string `json:"profile_id,omitempty"`
	Name        string `json:"name,omitempty"`
	ChatID      string `json:"chat_id,omitempty"`
	VoiceRoomID string `json:"voice_room_id,omitempty"`
}

// JetStreamPublisher publishes role.events payloads to NATS JetStream.
type JetStreamPublisher struct {
	nc *nats.Conn
	js nats.JetStreamContext
	// Logger emits structured nats_publish lines; optional.
	Logger *slog.Logger

	ensureOnce sync.Once
	ensureErr  error
}

// NewJetStreamPublisher connects to NATS and prepares JetStream for role.events.
func NewJetStreamPublisher(natsURL string) (*JetStreamPublisher, error) {
	if natsURL == "" {
		return nil, fmt.Errorf("empty NATS URL")
	}
	nc, err := nats.Connect(natsURL,
		nats.Name("voice-role-events"),
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
			subjectRoleCreated, subjectRoleUpdated, subjectRoleDeleted,
			subjectRoleAssigned, subjectRoleRevoked,
			subjectChatOverride, subjectVoiceOverride,
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

// Close drains the NATS connection.
func (p *JetStreamPublisher) Close() error {
	if p == nil || p.nc == nil {
		return nil
	}
	return p.nc.Drain()
}

func (p *JetStreamPublisher) publish(ctx context.Context, subject string, payload roleEventPayload) error {
	if err := p.ensureStream(); err != nil {
		return err
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	requestID := correlation.FromGRPC(ctx)
	msg := &nats.Msg{Subject: subject, Data: b, Header: nats.Header{}}
	natslog.SetRequestIDHeader(msg.Header, requestID)
	if _, err := p.js.PublishMsg(msg); err != nil {
		return fmt.Errorf("jetstream publish %s: %w", subject, err)
	}
	natslog.LogPublish(p.Logger, subject, requestID, "role event published",
		slog.String("role_id", payload.RoleID),
		slog.String("space_id", payload.SpaceID),
	)
	return nil
}

func (p *JetStreamPublisher) PublishRoleCreated(ctx context.Context, spaceID, roleID, name string) error {
	return p.publish(ctx, subjectRoleCreated, roleEventPayload{SpaceID: spaceID, RoleID: roleID, Name: name})
}

func (p *JetStreamPublisher) PublishRoleAssigned(ctx context.Context, spaceID, profileID, roleID string) error {
	return p.publish(ctx, subjectRoleAssigned, roleEventPayload{SpaceID: spaceID, ProfileID: profileID, RoleID: roleID})
}

func (p *JetStreamPublisher) PublishRoleUpdated(ctx context.Context, spaceID, roleID string, changedFields []string) error {
	_ = changedFields
	return p.publish(ctx, subjectRoleUpdated, roleEventPayload{SpaceID: spaceID, RoleID: roleID})
}

func (p *JetStreamPublisher) PublishRoleDeleted(ctx context.Context, spaceID, roleID string) error {
	return p.publish(ctx, subjectRoleDeleted, roleEventPayload{SpaceID: spaceID, RoleID: roleID})
}

func (p *JetStreamPublisher) PublishRoleRevoked(ctx context.Context, spaceID, profileID, roleID string) error {
	return p.publish(ctx, subjectRoleRevoked, roleEventPayload{SpaceID: spaceID, ProfileID: profileID, RoleID: roleID})
}

func (p *JetStreamPublisher) PublishChatOverrideSet(ctx context.Context, chatID, roleID string) error {
	return p.publish(ctx, subjectChatOverride, roleEventPayload{ChatID: chatID, RoleID: roleID})
}

func (p *JetStreamPublisher) PublishVoiceOverrideSet(ctx context.Context, voiceRoomID, roleID string) error {
	return p.publish(ctx, subjectVoiceOverride, roleEventPayload{VoiceRoomID: voiceRoomID, RoleID: roleID})
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
