package consumer

import (
	"context"
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
		{"analytics_events", "analytics.>", "telemetry", r.handleAnalyticsMsg},
	}

	for _, spec := range specs {
		spec := spec
		durable := "analytics_" + spec.name + "_" + inst
		handler := func(msg *nats.Msg) {
			if err := spec.handler(msg); err != nil && r.Logger != nil {
				r.Logger.Warn("analytics consume failed", slog.String("stream", spec.stream), slog.Any("error", err))
				natslog.LogConsume(r.Logger, msg, slog.LevelWarn, "analytics consume error")
				return
			}
			natslog.LogConsume(r.Logger, msg, slog.LevelInfo, "analytics event consumed")
		}
		sub, err := js.Subscribe(spec.subject, handler, nats.Durable(durable), nats.BindStream(spec.stream), nats.DeliverNew())
		if err != nil {
			sub, err = js.Subscribe("", handler, nats.Bind(spec.stream, durable))
			if err != nil {
				return fmt.Errorf("subscribe %s: %w", spec.stream, err)
			}
		}
		go func(s *nats.Subscription) {
			<-ctx.Done()
			_ = s.Unsubscribe()
		}(sub)
	}
	<-ctx.Done()
	return ctx.Err()
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

func (r *Runner) wrapProto(fn func([]byte) error) func(*nats.Msg) error {
	return func(msg *nats.Msg) error {
		return fn(msg.Data)
	}
}

func (r *Runner) append(ev *analyticsv1.AnalyticsEvent) {
	if ev == nil || ev.GetEventId() == "" {
		return
	}
	r.Buffer.AppendProto(ev)
	metrics.EventsIngested.Inc()
	if ev.GetTimestamp() != nil {
		lag := time.Since(ev.GetTimestamp().AsTime()).Seconds()
		if lag >= 0 {
			metrics.IngestLag.Observe(lag)
		}
	}
	metrics.BufferDepth.Set(float64(r.Buffer.PendingCount()))
}

func (r *Runner) handleUserProto(data []byte) error {
	var env eventsv1.UserStreamEvent
	if err := proto.Unmarshal(data, &env); err != nil {
		return err
	}
	r.append(r.Mapper.FromUser(&env))
	return nil
}

func (r *Runner) handleMessageProto(data []byte) error {
	var env eventsv1.MessageStreamEvent
	if err := proto.Unmarshal(data, &env); err != nil {
		return err
	}
	r.append(r.Mapper.FromMessage(&env))
	return nil
}

func (r *Runner) handleChatProto(data []byte) error {
	var env eventsv1.ChatStreamEvent
	if err := proto.Unmarshal(data, &env); err != nil {
		return err
	}
	r.append(r.Mapper.FromChat(&env))
	return nil
}

func (r *Runner) handleMatchmakingProto(data []byte) error {
	var env eventsv1.MatchmakingStreamEvent
	if err := proto.Unmarshal(data, &env); err != nil {
		return err
	}
	r.append(r.Mapper.FromMatchmaking(&env))
	return nil
}

func (r *Runner) handleVoiceProto(data []byte) error {
	var env eventsv1.VoiceStreamEvent
	if err := proto.Unmarshal(data, &env); err != nil {
		return err
	}
	r.append(r.Mapper.FromVoice(&env))
	return nil
}

func (r *Runner) handleStoryProto(data []byte) error {
	var env eventsv1.StoryStreamEvent
	if err := proto.Unmarshal(data, &env); err != nil {
		return err
	}
	r.append(r.Mapper.FromStory(&env))
	return nil
}

func (r *Runner) handleBotProto(data []byte) error {
	var env eventsv1.BotStreamEvent
	if err := proto.Unmarshal(data, &env); err != nil {
		return err
	}
	r.append(r.Mapper.FromBot(&env))
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
	r.append(ev)
	return nil
}