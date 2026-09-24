package consumer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
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
	messageDurable        = "bot_message_events"
	messageDeliverSubject = "_INBOX.voice.bot.bot_message_events"
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
	messageID, err := uuid.Parse(strings.TrimSpace(ms.GetMessageId()))
	if err != nil {
		return nil
	}
	payload := map[string]any{
		"type":              "message",
		"chat_id":           ms.GetChatId(),
		"message_id":        ms.GetMessageId(),
		"sender_profile_id": ms.GetSenderProfileId(),
		"thread_parent_id":  ms.GetThreadParentId(),
	}
	return h.Store.QueueMessageRecipients(ctx, chatID, messageID, ms.GetSenderProfileId(), payload)
}

// ProcessNext advances one recipient without holding up other recipients.
func (h *MessageHandler) ProcessNext(ctx context.Context) (bool, error) {
	d, err := h.Store.ClaimDueMessageDelivery(ctx)
	if err != nil || d == nil {
		return false, err
	}
	if d.IsPollingMode {
		err = h.Store.CompletePollingMessage(ctx, d)
	} else {
		options := make(map[string]any, len(d.Payload)+1)
		for k, v := range d.Payload {
			options[k] = v
		}
		options["delivery_id"] = d.ID.String()
		p := webhook.InteractionPayload{Type: "message", ChatID: d.ChatID.String(), InvokerProfileID: fmt.Sprint(d.Payload["sender_profile_id"]), Options: options}
		timeout := h.Timeout
		if timeout <= 0 {
			timeout = dispatch.DefaultTimeout()
		}
		// Finish the HTTP attempt well before the database lease expires.
		attemptCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
		_, err = webhook.DeliverPOST(attemptCtx, h.Client, ptrStr(d.WebhookURL), ptrStr(d.WebhookSecret), p, timeout)
		cancel()
		if err == nil {
			err = h.Store.CompleteWebhookMessage(ctx, d.ID)
		}
	}
	if err != nil {
		if h.Logger != nil {
			h.Logger.Warn("bot message delivery failed", slog.String("bot_id", d.BotID.String()), slog.Any("error", err))
		}
		return true, errors.Join(err, h.Store.RetryMessageDelivery(ctx, d.ID, d.Attempts))
	}
	return true, nil
}

func ptrStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// StartMessageEventsConsumer validates and binds the centrally provisioned durable.
func StartMessageEventsConsumer(ctx context.Context, h *MessageHandler, natsURL string, logger *slog.Logger) (func(), error) {
	if h == nil || h.Store == nil {
		return nil, fmt.Errorf("message handler not configured")
	}
	if strings.TrimSpace(natsURL) == "" {
		return nil, fmt.Errorf("empty NATS URL")
	}
	opts := []nats.Option{nats.Name("voice-bot-message-events"), nats.CustomInboxPrefix("_INBOX.voice.bot")}
	if creds := strings.TrimSpace(os.Getenv("BOT_NATS_CREDS_FILE")); creds != "" {
		opts = append(opts, nats.UserCredentials(creds))
	}
	nc, err := nats.Connect(natsURL, opts...)
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("jetstream: %w", err)
	}
	info, err := js.ConsumerInfo(jsStreamMessageEvents, messageDurable)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("consumer info: %w", err)
	}
	if err := validateMessageConsumer(info); err != nil {
		nc.Close()
		return nil, err
	}
	_, err = js.Subscribe(subjectMessageSent, func(msg *nats.Msg) {
		if err := h.HandleMessageSent(ctx, msg.Data); err != nil {
			if logger != nil {
				logger.Warn("bot message consumer handler failed", slog.Any("error", err))
			}
			_ = msg.Nak()
			return
		}
		_ = msg.Ack()
	}, nats.ManualAck(), nats.Bind(jsStreamMessageEvents, messageDurable))
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("subscribe: %w", err)
	}
	workerCtx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-workerCtx.Done():
				return
			case <-ticker.C:
			}
			for i := 0; i < 32; i++ {
				worked, err := h.ProcessNext(workerCtx)
				if err != nil && logger != nil {
					logger.Warn("bot message outbox failed", slog.Any("error", err))
				}
				if !worked || workerCtx.Err() != nil {
					break
				}
			}
		}
	}()
	return func() { cancel(); _ = nc.Drain() }, nil
}

func validateMessageConsumer(info *nats.ConsumerInfo) error {
	if info == nil || info.Stream != jsStreamMessageEvents || info.Name != messageDurable ||
		info.Config.Durable != messageDurable || info.Config.FilterSubject != subjectMessageSent ||
		info.Config.DeliverSubject != messageDeliverSubject || info.Config.DeliverPolicy != nats.DeliverAllPolicy ||
		info.Config.AckPolicy != nats.AckExplicitPolicy {
		return fmt.Errorf("bot message durable configuration mismatch")
	}
	return nil
}
