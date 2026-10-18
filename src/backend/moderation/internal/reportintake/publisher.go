package reportintake

import (
	"context"
	"errors"
	"time"

	"github.com/nats-io/nats.go"
	"voice/backend/moderation/internal/store"
)

// Publisher sends only the two fixed intake subjects, using persisted bytes and
// Nats-Msg-Id. Deployment owns stream creation and retention configuration.
type Publisher struct {
	connection *nats.Conn
	js         nats.JetStreamContext
}

func NewPublisher(url string) (*Publisher, error) {
	if url == "" {
		return nil, errors.New("intake NATS URL required")
	}
	nc, err := nats.Connect(url, nats.Name("voice-moderation-intake"), nats.CustomInboxPrefix("_INBOX.voice.moderation"), nats.Timeout(5*time.Second), nats.MaxReconnects(-1))
	if err != nil {
		return nil, err
	}
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, err
	}
	return &Publisher{connection: nc, js: js}, nil
}

func (p *Publisher) Close() {
	if p != nil && p.connection != nil {
		p.connection.Close()
	}
}

func (p *Publisher) Publish(ctx context.Context, event store.IntakeEvent) error {
	if p == nil || p.js == nil {
		return errors.New("intake publisher unavailable")
	}
	stream := ""
	var expected []string
	switch {
	case event.Kind == "domain.report_created" && event.Subject == "moderation.report_created":
		stream = "moderation_events"
		expected = []string{"moderation.report_created", "moderation.sanction_applied", "moderation.appeal_submitted"}
	case event.Kind == "analytics.report_created" && event.Subject == "analytics.moderation.report_created":
		stream = "analytics_events"
		expected = []string{"analytics.>"}
	default:
		return errors.New("invalid intake event subject")
	}
	// Recheck instead of caching an old bootstrap failure through recovery.
	info, err := p.js.StreamInfo(stream, nats.Context(ctx))
	if err != nil {
		return err
	}
	if info == nil || info.Config.Name != stream || info.Config.Storage != nats.FileStorage || info.Config.Retention != nats.LimitsPolicy || info.Config.MaxAge != 7*24*time.Hour || len(info.Config.Subjects) != len(expected) {
		return errors.New("intake stream contract mismatch")
	}
	seen := map[string]bool{}
	for _, s := range info.Config.Subjects {
		seen[s] = true
	}
	for _, s := range expected {
		if !seen[s] {
			return errors.New("intake stream subjects mismatch")
		}
	}
	msg := &nats.Msg{Subject: event.Subject, Data: event.Payload, Header: nats.Header{}}
	msg.Header.Set(nats.MsgIdHdr, event.ID.String())
	ack, err := p.js.PublishMsg(msg, nats.Context(ctx))
	if err != nil {
		return err
	}
	if ack == nil || ack.Stream != stream || ack.Sequence == 0 {
		return errors.New("invalid intake publication acknowledgement")
	}
	return nil
}
