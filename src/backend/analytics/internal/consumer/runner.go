package consumer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
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

const analyticsDurablePrefix = "analytics_v2_"

// Runner subscribes to domain and analytics JetStream subjects only while ClickHouse persistence is ready.
type Runner struct {
	Mapper           adapters.Mapper
	Buffer           *buffer.Accumulator
	Logger           *slog.Logger
	PersistenceReady bool
}

func analyticsDurableName(source string) string {
	return analyticsDurablePrefix + strings.TrimSpace(source)
}

// The queue group intentionally matches the service-wide durable so pods share one source backlog.
func analyticsQueueName(source string) string { return analyticsDurableName(source) }

func (r *Runner) Start(ctx context.Context, natsURL, _ string) error {
	if r == nil || r.Buffer == nil {
		return fmt.Errorf("analytics consumer: missing buffer")
	}
	if !r.PersistenceReady {
		return fmt.Errorf("analytics consumer: ClickHouse persistence is not ready")
	}
	url := strings.TrimSpace(natsURL)
	if url == "" {
		return fmt.Errorf("analytics consumer: missing NATS_URL")
	}
	nc, err := nats.Connect(url, nats.Name("voice-analytics"), nats.Timeout(10*time.Second), nats.RetryOnFailedConnect(true), nats.MaxReconnects(-1), nats.ReconnectWait(time.Second))
	if err != nil {
		return fmt.Errorf("nats connect: %w", err)
	}
	go func() { <-ctx.Done(); _ = nc.Drain() }()
	js, err := nc.JetStream()
	if err != nil {
		return fmt.Errorf("jetstream: %w", err)
	}
	if err := ensureAnalyticsStream(js); err != nil {
		return err
	}

	type subSpec struct {
		stream, subject, name string
		handler               func(*nats.Msg) error
	}
	specs := []subSpec{
		{"message_events", ">", "msg", r.wrapProto(r.handleMessageProto)}, {"user_events", "user.>", "user", r.wrapProto(r.handleUserProto)}, {"chat_events", ">", "chat", r.wrapProto(r.handleChatProto)}, {"matchmaking_events", "mm.>", "mm", r.wrapProto(r.handleMatchmakingProto)}, {"voice_events", "voice.>", "voice", r.wrapProto(r.handleVoiceProto)}, {"story_events", "story.>", "story", r.wrapProto(r.handleStoryProto)}, {"bot_events", "bot.>", "bot", r.wrapProto(r.handleBotProto)}, {"social_events", "social.>", "social", r.wrapProto(r.handleSocialProto)}, {"role_events", "role.>", "role", r.handleRoleMsg}, {"file_events", "file.>", "file", r.wrapProto(r.handleFileProto)}, {"subscription_events", "subscription.>", "subscription", r.wrapProto(r.handleSubscriptionProto)}, {"moderation_events", "moderation.>", "moderation", r.wrapProto(r.handleModerationProto)}, {"analytics_events", "analytics.>", "telemetry", r.handleAnalyticsMsg},
	}
	for _, spec := range specs {
		spec := spec
		durable := analyticsDurableName(spec.name)
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
		sub, err := r.subscribe(ctx, js, spec.stream, spec.subject, durable, analyticsQueueName(spec.name), handler)
		if err != nil {
			return err
		}
		go func(s *nats.Subscription) { <-ctx.Done(); _ = s.Unsubscribe() }(sub)
	}
	<-ctx.Done()
	return ctx.Err()
}

func (r *Runner) subscribe(ctx context.Context, js nats.JetStreamContext, stream, subject, durable, queue string, handler nats.MsgHandler) (*nats.Subscription, error) {
	return subscribeJetStreamWithRetry(ctx, r.Logger, stream, func() (*nats.Subscription, error) {
		return subscribeCreateOrBind(
			func() (*nats.Subscription, error) {
				return js.QueueSubscribe(subject, queue, handler, nats.Durable(durable), nats.BindStream(stream), nats.DeliverNew(), nats.ManualAck())
			},
			func() (*nats.Subscription, error) {
				return js.QueueSubscribe("", queue, handler, nats.Bind(stream, durable), nats.ManualAck())
			},
		)
	})
}

func isJetStreamNotFound(err error) bool {
	if err == nil {
		return false
	}
	var e *createBindSubscriptionError
	if errors.As(err, &e) {
		return isJetStreamNotFound(e.createErr)
	}
	return errors.Is(err, nats.ErrStreamNotFound) || strings.Contains(strings.ToLower(err.Error()), "stream not found")
}

type createBindSubscriptionError struct{ createErr, bindErr error }

func (e *createBindSubscriptionError) Error() string {
	return fmt.Sprintf("create durable: %v; bind existing durable: %v", e.createErr, e.bindErr)
}
func (e *createBindSubscriptionError) Unwrap() error { return e.createErr }
func subscribeCreateOrBind(create, bind func() (*nats.Subscription, error)) (*nats.Subscription, error) {
	sub, err := create()
	if err == nil {
		return sub, nil
	}
	if isJetStreamNotFound(err) {
		return nil, err
	}
	sub, bindErr := bind()
	if bindErr == nil {
		return sub, nil
	}
	return nil, &createBindSubscriptionError{err, bindErr}
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
			logger.Info("analytics JetStream stream not ready, retrying", slog.String("stream", stream), slog.Duration("retry_in", delay), slog.String("error", err.Error()))
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
	_, err := js.AddStream(&nats.StreamConfig{Name: "analytics_events", Subjects: []string{"analytics.>"}, Retention: nats.LimitsPolicy, MaxAge: 7 * 24 * time.Hour})
	return err
}
func (r *Runner) wrapProto(fn func([]byte, *nats.Msg) error) func(*nats.Msg) error {
	return func(msg *nats.Msg) error { return fn(msg.Data, msg) }
}

func normalizeSourceEventID(candidate string, msg *nats.Msg) string {
	if id, err := uuid.Parse(strings.TrimSpace(candidate)); err == nil {
		return id.String()
	}
	if msg == nil {
		return ""
	}
	meta, err := msg.Metadata()
	if err != nil || strings.TrimSpace(meta.Stream) == "" || meta.Sequence.Stream == 0 {
		return ""
	}
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte("voice.analytics.jetstream.v1\x00"+meta.Stream+"\x00"+strconv.FormatUint(meta.Sequence.Stream, 10))).String()
}

func (r *Runner) appendWithSourceAck(ev *analyticsv1.AnalyticsEvent, sourceEventID string, msg *nats.Msg) error {
	if ev == nil {
		if msg != nil {
			_ = msg.Ack()
		}
		return nil
	}
	ev.EventId = normalizeSourceEventID(sourceEventID, msg)
	if ev.GetEventId() == "" {
		return errors.New("analytics event has no stable source identity")
	}
	var ack buffer.MsgAck
	if msg != nil {
		ack = msg
	}
	r.Buffer.AppendWithAck(ev, ack)
	metrics.EventsIngested.Inc()
	if ev.GetTimestamp() != nil {
		if lag := time.Since(ev.GetTimestamp().AsTime()).Seconds(); lag >= 0 {
			metrics.IngestLag.Observe(lag)
		}
	}
	metrics.BufferDepth.Set(float64(r.Buffer.PendingCount()))
	return nil
}
func (r *Runner) handleUserProto(d []byte, m *nats.Msg) error {
	var e eventsv1.UserStreamEvent
	if err := proto.Unmarshal(d, &e); err != nil {
		return err
	}
	return r.appendWithSourceAck(r.Mapper.FromUser(&e), e.GetEventId(), m)
}
func (r *Runner) handleMessageProto(d []byte, m *nats.Msg) error {
	var e eventsv1.MessageStreamEvent
	if err := proto.Unmarshal(d, &e); err != nil {
		return err
	}
	return r.appendWithSourceAck(r.Mapper.FromMessage(&e), e.GetEventId(), m)
}
func (r *Runner) handleChatProto(d []byte, m *nats.Msg) error {
	var e eventsv1.ChatStreamEvent
	if err := proto.Unmarshal(d, &e); err != nil {
		return err
	}
	return r.appendWithSourceAck(r.Mapper.FromChat(&e), e.GetEventId(), m)
}
func (r *Runner) handleMatchmakingProto(d []byte, m *nats.Msg) error {
	var e eventsv1.MatchmakingStreamEvent
	if err := proto.Unmarshal(d, &e); err != nil {
		return err
	}
	return r.appendWithSourceAck(r.Mapper.FromMatchmaking(&e), e.GetEventId(), m)
}
func (r *Runner) handleVoiceProto(d []byte, m *nats.Msg) error {
	var e eventsv1.VoiceStreamEvent
	if err := proto.Unmarshal(d, &e); err != nil {
		return err
	}
	return r.appendWithSourceAck(r.Mapper.FromVoice(&e), e.GetEventId(), m)
}
func (r *Runner) handleStoryProto(d []byte, m *nats.Msg) error {
	var e eventsv1.StoryStreamEvent
	if err := proto.Unmarshal(d, &e); err != nil {
		return err
	}
	return r.appendWithSourceAck(r.Mapper.FromStory(&e), e.GetEventId(), m)
}
func (r *Runner) handleBotProto(d []byte, m *nats.Msg) error {
	var e eventsv1.BotStreamEvent
	if err := proto.Unmarshal(d, &e); err != nil {
		return err
	}
	return r.appendWithSourceAck(r.Mapper.FromBot(&e), e.GetEventId(), m)
}
func (r *Runner) handleSocialProto(d []byte, m *nats.Msg) error {
	var e eventsv1.SocialStreamEvent
	if err := proto.Unmarshal(d, &e); err != nil {
		return err
	}
	return r.appendWithSourceAck(r.Mapper.FromSocial(&e), e.GetEventId(), m)
}
func (r *Runner) handleFileProto(d []byte, m *nats.Msg) error {
	var e eventsv1.FileStreamEvent
	if err := proto.Unmarshal(d, &e); err != nil {
		return err
	}
	return r.appendWithSourceAck(r.Mapper.FromFile(&e), e.GetEventId(), m)
}
func (r *Runner) handleSubscriptionProto(d []byte, m *nats.Msg) error {
	var e eventsv1.SubscriptionStreamEvent
	if err := proto.Unmarshal(d, &e); err != nil {
		return err
	}
	return r.appendWithSourceAck(r.Mapper.FromSubscription(&e), e.GetEventId(), m)
}
func (r *Runner) handleModerationProto(d []byte, m *nats.Msg) error {
	var e eventsv1.ModerationStreamEvent
	if err := proto.Unmarshal(d, &e); err != nil {
		return err
	}
	return r.appendWithSourceAck(r.Mapper.FromModeration(&e), e.GetEventId(), m)
}
func (r *Runner) handleRoleMsg(m *nats.Msg) error {
	return r.appendWithSourceAck(r.Mapper.FromRoleSubject(m.Subject, m.Data), "", m)
}
func (r *Runner) handleAnalyticsMsg(m *nats.Msg) error {
	ev := r.Mapper.FromAnalyticsSubject(m.Subject, m.Data)
	if ev == nil {
		var parsed analyticsv1.AnalyticsEvent
		if err := protojson.Unmarshal(m.Data, &parsed); err != nil {
			if err2 := proto.Unmarshal(m.Data, &parsed); err2 != nil {
				return err
			}
		}
		ev = &parsed
	}
	return r.appendWithSourceAck(ev, ev.GetEventId(), m)
}
