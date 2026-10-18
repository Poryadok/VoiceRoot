package rosterconsume

import (
	"context"
	"errors"
	"fmt"
	"github.com/nats-io/nats.go"
	"log/slog"
	"time"
	"voice/backend/matchmaking/internal/store"
)

const Stream = "voice_events"
const Durable = "matchmaking_voice_roster_v1"
const Subject = "voice.roster_membership_changed"

type Inbox interface {
	ApplyVoiceRosterEvent(context.Context, []byte) (store.VoiceRosterResetResult, error)
}
type Consumer struct {
	nc                 *nats.Conn
	sub                *nats.Subscription
	Store              Inbox
	Logger             *slog.Logger
	OnContractMismatch func()
}

func ValidateConsumer(info *nats.ConsumerInfo) error {
	if info == nil || info.Stream != Stream || info.Name != Durable || info.Config.Durable != Durable || info.Config.FilterSubject != Subject || len(info.Config.FilterSubjects) != 0 || info.Config.DeliverSubject != "" || info.Config.DeliverGroup != "" || info.Config.DeliverPolicy != nats.DeliverAllPolicy || info.Config.AckPolicy != nats.AckExplicitPolicy {
		return errors.New("incompatible Voice roster pull durable")
	}
	return nil
}

func Start(url string, inbox Inbox, logger *slog.Logger) (*Consumer, error) {
	if inbox == nil {
		return nil, errors.New("Voice roster inbox unavailable")
	}
	nc, err := nats.Connect(url, nats.Name("voice-matchmaking-roster"), nats.CustomInboxPrefix("_INBOX.voice.matchmaking"), nats.Timeout(10*time.Second), nats.MaxReconnects(-1), nats.ReconnectWait(time.Second))
	if err != nil {
		return nil, err
	}
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, err
	}
	info, err := js.ConsumerInfo(Stream, Durable)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("inspect Voice roster durable: %w", err)
	}
	if err = ValidateConsumer(info); err != nil {
		nc.Close()
		return nil, err
	}
	sub, err := js.PullSubscribe(Subject, Durable, nats.Bind(Stream, Durable), nats.ManualAck())
	if err != nil {
		nc.Close()
		return nil, err
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Consumer{nc: nc, sub: sub, Store: inbox, Logger: logger}, nil
}

func (c *Consumer) Run(ctx context.Context) error {
	for ctx.Err() == nil {
		messages, err := c.sub.Fetch(32, nats.MaxWait(time.Second))
		if err != nil {
			if errors.Is(err, nats.ErrTimeout) {
				continue
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			c.Logger.Warn("Voice roster fetch unavailable; durable position retained", slog.Any("error", err))
			retry := time.NewTimer(time.Second)
			select {
			case <-ctx.Done():
				retry.Stop()
				return ctx.Err()
			case <-retry.C:
				continue
			}
		}
		for _, message := range messages {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			apply, done := context.WithTimeout(ctx, 15*time.Second)
			result, err := c.Store.ApplyVoiceRosterEvent(apply, message.Data)
			done()
			if err != nil {
				c.Logger.Warn("Voice roster inbox transaction failed", slog.Any("error", err))
				if nakErr := message.NakWithDelay(time.Second); nakErr != nil {
					c.Logger.Warn("Voice roster negative ACK failed", slog.Any("error", nakErr))
				}
				continue
			}
			if result.ContractMismatch {
				c.Logger.Error("Voice roster CONTRACT_MISMATCH requires operator intervention")
				if c.OnContractMismatch != nil {
					c.OnContractMismatch()
				}
			}
			ack, cancel := context.WithTimeout(ctx, 3*time.Second)
			err = message.AckSync(nats.Context(ack))
			cancel()
			if err != nil {
				c.Logger.Warn("Voice roster ACK uncertain; durable inbox preserves replay", slog.Any("error", err))
			}
		}
	}
	return ctx.Err()
}
func (c *Consumer) Close() {
	if c != nil && c.nc != nil {
		_ = c.nc.Drain()
	}
}
