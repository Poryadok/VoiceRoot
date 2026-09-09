package consumer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	analyticsv1 "voice.app/voice/analytics/v1"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/analytics/internal/adapters"
	"voice/backend/analytics/internal/buffer"
	"voice/backend/analytics/internal/metrics"
	"voice/backend/pkg/natslog"
)

// Runner subscribes to domain and analytics JetStream subjects.
type Runner struct {
	Mapper adapters.Mapper
	Buffer *buffer.Accumulator
	Logger *slog.Logger
}

func (r *Runner) Start(ctx context.Context, natsURL, instanceID string) error {
	if r == nil || r.Buffer == nil {
		return fmt.Errorf("analytics consumer: missing buffer")
	}
	url := strings.TrimSpace(natsURL)
	if url == "" {
		return fmt.Errorf("analytics consumer: missing NATS_URL")
	}
	nc, err := nats.Connect(url,
		nats.Name("voice-analytics"),
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
	if err := ensureAnalyticsStream(js); err != nil {
		return err
	}

	inst := strings.ReplaceAll(strings.TrimSpace(instanceID), "-", "")
	if inst == "" {
		inst = "default"
	}

	type subSpec struct {
		stream  string
		subject string
		name    string
		handler func(*nats.Msg) error
	}
	specs := []subSpec{
		{"message_events", ">", "msg", r.wrapProto(r.handleMessageProto)},
		{"user_events", "user.>", "user", r.wrapProto(r.handleUserProto)},
		{"chat_events", ">", "chat", r.wrapProto(r.handleChatProto)},
		{"matchmaking_events", "mm.>", "mm", r.wrapProto(r.handleMatchmakingProto)},
		{"voice_events", "voice.>", "voice", r.wrapProto(r.handleVoiceProto)},
		{"story_events", "story.>", "story", r.wrapProto(r.handleStoryProto)},
		{"bot_events", "bot.>", "bot", r.wrapProto(r.handleBotProto)},
		{"social_events", "social.>", "social", r.wrapProto(r.handleSocialProto)},
		{"role_events", "role.>", "role", r.handleRoleMsg},
		{"file_events", "file.>", "file", r.wrapProto(r.handleFileProto)},
		{"subscription_events", "subscription.>", "subscription", r.wrapProto(r.handleSubscriptionProto)},
		{"moderation_events", "moderation.>", "moderation", r.wrapProto(r.handleModerationProto)},
		{"analytics_events", "analytics.>", "telemetry", r.handleAnalyticsMsg},
	}

	for _, spec := range specs {
		spec := spec
		durable := "analytics_" + spec.name + "_" + inst
		handler := func(msg *nats.Msg) {
			if err := spec.handler(msg); err != nil {
				jetStreamConsumeAck(msg, err)
				if r.Logger != nil {
					r.Logger.Warn("analytics consume failed", slog.String("stream", spec.stream), slog.Any("error", err))
					natslog.LogConsume(r.Logger, msg, slog.LevelWarn, "analytics consume error")
				}
				return
			}
			natslog.LogConsume(r.Logger, msg, slog.LevelInfo, "analytics event consumed")
		}
		sub, err := r.subscribe(ctx, js, spec.stream, spec.subject, durable, handler)
		if err != nil {
			return err
		}
		go func(s *nats.Subscription) {
			<-ctx.Done()
			_ = s.Unsubscribe()
		}(sub)
	}
	<-ctx.Done()
	return ctx.Err()
}

// subscribe waits for a publisher-owned stream instead of permanently stopping the
// analytics consumer when Analytics starts before that publisher. Existing durable
// consumers still bind by name after a restart.
func (r *Runner) subscribe(ctx context.Context, js nats.JetStreamContext, stream, subject, durable string, handler nats.MsgHandler) (*nats.Subscription, error) {
	return subscribeJetStreamWithRetry(ctx, r.Logger, stream, func() (*nats.Subscription, error) {
		return subscribeCreateOrBind(
			func() (*nats.Subscription, error) {
				return js.Subscribe(subject, handler,
					nats.Durable(durable),
					nats.BindStream(stream),
					nats.DeliverNew(),
					nats.ManualAck(),
				)
			},
			func() (*nats.Subscription, error) {
				return js.Subscribe("", handler, nats.Bind(stream, durable), nats.ManualAck())
			},
		)
	})
}

func isJetStreamNotFound(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, nats.ErrStreamNotFound) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "stream not found")
}

// subscribeCreateOrBind creates a durable consumer, or binds to the durable one
// created by an earlier Analytics process. A missing publisher stream must reach
// the caller unchanged so it is the only retryable startup condition.
func subscribeCreateOrBind(create, bind func() (*nats.Subscription, error)) (*nats.Subscription, error) {
	sub, createErr := create()
	if createErr == nil {
		return sub, nil
	}
	if isJetStreamNotFound(createErr) {
		return nil, createErr
	}
	sub, bindErr := bind()
	if bindErr == nil {
		return sub, nil
	}
	return nil, fmt.Errorf("create durable: %w; bind existing durable: %v", createErr, bindErr)
}

func subscribeJetStreamWithRetry(ctx context.Context, logger *slog.Logger, stream string, subscribe func() (*nats.Subscription, error)) (*nats.Subscription, error) {
	delay := time.Second
	for {
		sub, err := subscribe()
		if err == nil {
			return sub, nil
		}
		if !isJetStreamNotFound(err) {
			return nil, fmt.Errorf("subscribe %s: %w", stream, err)
		}
		if logger != nil {
			logger.Info("analytics JetStream stream not ready, retrying",
				slog.String("stream", stream),
				slog.Duration("retry_in", delay),
				slog.String("error", err.Error()),
			)
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("subscribe %s: %w", stream, ctx.Err())
		case <-time.After(delay):
		}
		if delay < 30*time.Second {
			delay *= 2
		}
	}
}

func ensureAnalyticsStream(js nats.JetStreamContext) error {
	if _, err := js.StreamInfo("analytics_events"); err == nil {
		return nil
	}
	_, err := js.AddStream(&nats.StreamConfig{
		Name:      "analytics_events",
		Subjects:  []string{"analytics.>"},
		Retention: nats.LimitsPolicy,
		MaxAge:    7 * 24 * time.Hour,
	})
	return err
}

func (r *Runner) wrapProto(fn func([]byte, *nats.Msg) error) func(*nats.Msg) error {
	return func(msg *nats.Msg) error {
		return fn(msg.Data, msg)
	}
}

func (r *Runner) appendWithAck(ev *analyticsv1.AnalyticsEvent, msg *nats.Msg) {
	if ev == nil || ev.GetEventId() == "" {
		if msg != nil {
			_ = msg.Ack()
		}
		return
	}
	var ack buffer.MsgAck
	if msg != nil {
		ack = msg
	}
	r.Buffer.AppendWithAck(ev, ack)
	metrics.EventsIngested.Inc()
	if ev.GetTimestamp() != nil {
		lag := time.Since(ev.GetTimestamp().AsTime()).Seconds()
		if lag >= 0 {
			metrics.IngestLag.Observe(lag)
		}
	}
	metrics.BufferDepth.Set(float64(r.Buffer.PendingCount()))
}

func (r *Runner) handleUserProto(data []byte, msg *nats.Msg) error {
	var env eventsv1.UserStreamEvent
	if err := proto.Unmarshal(data, &env); err != nil {
		return err
	}
	r.appendWithAck(r.Mapper.FromUser(&env), msg)
	return nil
}

func (r *Runner) handleMessageProto(data []byte, msg *nats.Msg) error {
	var env eventsv1.MessageStreamEvent
	if err := proto.Unmarshal(data, &env); err != nil {
		return err
	}
	r.appendWithAck(r.Mapper.FromMessage(&env), msg)
	return nil
}

func (r *Runner) handleChatProto(data []byte, msg *nats.Msg) error {
	var env eventsv1.ChatStreamEvent
	if err := proto.Unmarshal(data, &env); err != nil {
		return err
	}
	r.appendWithAck(r.Mapper.FromChat(&env), msg)
	return nil
}

func (r *Runner) handleMatchmakingProto(data []byte, msg *nats.Msg) error {
	var env eventsv1.MatchmakingStreamEvent
	if err := proto.Unmarshal(data, &env); err != nil {
		return err
	}
	r.appendWithAck(r.Mapper.FromMatchmaking(&env), msg)
	return nil
}

func (r *Runner) handleVoiceProto(data []byte, msg *nats.Msg) error {
	var env eventsv1.VoiceStreamEvent
	if err := proto.Unmarshal(data, &env); err != nil {
		return err
	}
	r.appendWithAck(r.Mapper.FromVoice(&env), msg)
	return nil
}

func (r *Runner) handleStoryProto(data []byte, msg *nats.Msg) error {
	var env eventsv1.StoryStreamEvent
	if err := proto.Unmarshal(data, &env); err != nil {
		return err
	}
	r.appendWithAck(r.Mapper.FromStory(&env), msg)
	return nil
}

func (r *Runner) handleBotProto(data []byte, msg *nats.Msg) error {
	var env eventsv1.BotStreamEvent
	if err := proto.Unmarshal(data, &env); err != nil {
		return err
	}
	r.appendWithAck(r.Mapper.FromBot(&env), msg)
	return nil
}

func (r *Runner) handleSocialProto(data []byte, msg *nats.Msg) error {
	var env eventsv1.SocialStreamEvent
	if err := proto.Unmarshal(data, &env); err != nil {
		return err
	}
	r.appendWithAck(r.Mapper.FromSocial(&env), msg)
	return nil
}

func (r *Runner) handleFileProto(data []byte, msg *nats.Msg) error {
	var env eventsv1.FileStreamEvent
	if err := proto.Unmarshal(data, &env); err != nil {
		return err
	}
	r.appendWithAck(r.Mapper.FromFile(&env), msg)
	return nil
}

func (r *Runner) handleSubscriptionProto(data []byte, msg *nats.Msg) error {
	var env eventsv1.SubscriptionStreamEvent
	if err := proto.Unmarshal(data, &env); err != nil {
		return err
	}
	r.appendWithAck(r.Mapper.FromSubscription(&env), msg)
	return nil
}

func (r *Runner) handleModerationProto(data []byte, msg *nats.Msg) error {
	var env eventsv1.ModerationStreamEvent
	if err := proto.Unmarshal(data, &env); err != nil {
		return err
	}
	r.appendWithAck(r.Mapper.FromModeration(&env), msg)
	return nil
}

func (r *Runner) handleRoleMsg(msg *nats.Msg) error {
	r.appendWithAck(r.Mapper.FromRoleSubject(msg.Subject, msg.Data), msg)
	return nil
}

func (r *Runner) handleAnalyticsMsg(msg *nats.Msg) error {
	ev := r.Mapper.FromAnalyticsSubject(msg.Subject, msg.Data)
	if ev == nil {
		var parsed analyticsv1.AnalyticsEvent
		if err := protojson.Unmarshal(msg.Data, &parsed); err != nil {
			if err2 := proto.Unmarshal(msg.Data, &parsed); err2 != nil {
				return err
			}
		}
		ev = &parsed
	}
	r.appendWithAck(ev, msg)
	return nil
}
