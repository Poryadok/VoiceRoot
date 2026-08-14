package consumer

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/bot/internal/dispatch"
	"voice/backend/bot/internal/store"
	"voice/backend/bot/internal/webhook"
)

const (
	jsStreamMessageEvents = "message_events"
	subjectMessageSent    = "message.sent"
)

// MessageHandler delivers inbound chat messages to installed bots.
type MessageHandler struct {
	Store   *store.BotStore
	Client  *http.Client
	Logger  *slog.Logger
	Timeout time.Duration
}

// HandleMessageSent processes a message.sent JetStream payload.
func (h *MessageHandler) HandleMessageSent(ctx context.Context, data []byte) error {
	if h == nil || h.Store == nil {
		return nil
	}
	var ev eventsv1.MessageStreamEvent
	if err := proto.Unmarshal(data, &ev); err != nil {
		return nil
	}
	ms := ev.GetMessageSent()
	if ms == nil || ms.GetChatId() == "" || ms.GetMessageId() == "" {
		return nil
	}
	chatID, err := uuid.Parse(strings.TrimSpace(ms.GetChatId()))
	if err != nil {
		return nil
	}
	bots, err := h.Store.ListLiveBotsForChat(ctx, chatID)
	if err != nil {
		return err
	}
	payload := map[string]any{
		"type":                "message",
		"chat_id":             ms.GetChatId(),
		"message_id":          ms.GetMessageId(),
		"sender_profile_id":   ms.GetSenderProfileId(),
		"thread_parent_id":    ms.GetThreadParentId(),
	}
	for _, bot := range bots {
		if bot.ActorProfileID.String() == strings.TrimSpace(ms.GetSenderProfileId()) {
			continue
		}
		if bot.IsPollingMode {
			_, _ = h.Store.EnqueueEvent(ctx, bot.ID, "message", payload, "")
			continue
		}
		url := strings.TrimSpace(ptrStr(bot.WebhookURL))
		if url == "" {
			continue
		}
		whPayload := webhook.InteractionPayload{
			Type:             "message",
			ChatID:           ms.GetChatId(),
			InvokerProfileID: ms.GetSenderProfileId(),
			Options:          payload,
		}
		timeout := h.Timeout
		if timeout <= 0 {
			timeout = dispatch.DefaultTimeout()
		}
		_, err := webhook.DeliverPOST(ctx, h.Client, url, bot.WebhookSecret, whPayload, timeout)
		if err != nil && h.Logger != nil {
			h.Logger.Warn("bot message webhook failed",
				slog.String("bot_id", bot.ID.String()),
				slog.String("chat_id", ms.GetChatId()),
				slog.Any("error", err))
		}
	}
	return nil
}

func ptrStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// RunMessageEventsConsumer subscribes to message_events and delivers to bots.
func RunMessageEventsConsumer(ctx context.Context, h *MessageHandler, natsURL string, logger *slog.Logger) error {
	if h == nil || h.Store == nil {
		return fmt.Errorf("message handler not configured")
	}
	if strings.TrimSpace(natsURL) == "" {
		return fmt.Errorf("empty NATS URL")
	}
	nc, err := nats.Connect(natsURL, nats.Name("voice-bot-message-events"))
	if err != nil {
		return fmt.Errorf("nats connect: %w", err)
	}
	defer func() { _ = nc.Drain() }()
	js, err := nc.JetStream()
	if err != nil {
		return fmt.Errorf("jetstream: %w", err)
	}
	durable := "bot_message_events"
	_, err = js.Subscribe(subjectMessageSent, func(msg *nats.Msg) {
		if err := h.HandleMessageSent(ctx, msg.Data); err != nil && logger != nil {
			logger.Warn("bot message consumer handler failed", slog.Any("error", err))
		}
		_ = msg.Ack()
	}, nats.Durable(durable), nats.ManualAck(), nats.Bind(jsStreamMessageEvents, durable))
	if err != nil {
		return fmt.Errorf("subscribe: %w", err)
	}
	<-ctx.Done()
	return ctx.Err()
}
