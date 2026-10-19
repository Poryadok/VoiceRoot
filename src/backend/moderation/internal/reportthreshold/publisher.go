package reportthreshold

import (
	"context"
	"errors"
	"time"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/moderation/internal/store"
)

type Publisher struct {
	connection *nats.Conn
	js         nats.JetStreamContext
}

func NewPublisher(url string) (*Publisher, error) {
	if url == "" {
		return nil, errors.New("report threshold NATS URL required")
	}
	nc, err := nats.Connect(url, nats.Name("voice-moderation-report-threshold"), nats.CustomInboxPrefix("_INBOX.voice.moderation"), nats.Timeout(5*time.Second), nats.MaxReconnects(-1))
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

func (p *Publisher) Publish(ctx context.Context, event store.ReportThresholdEvent) error {
	if p == nil || p.js == nil {
		return ErrAuthority
	}
	if event.Kind != "domain.sanction_applied" || event.Subject != "moderation.sanction_applied" {
		return errors.New("invalid threshold event subject")
	}
	var envelope eventsv1.ModerationStreamEvent
	if err := proto.Unmarshal(event.Payload, &envelope); err != nil {
		return err
	}
	sanction := envelope.GetSanctionApplied()
	if envelope.GetEventId() != event.ID.String() || envelope.GetOccurredAt() == nil || envelope.GetOccurredAt().CheckValid() != nil || sanction == nil || sanction.GetType() != "shadow_ban" {
		return errors.New("invalid threshold event body")
	}
	if _, err := id(sanction.GetSanctionId()); err != nil {
		return err
	}
	if _, err := id(sanction.GetTargetAccountId()); err != nil {
		return err
	}
	info, err := p.js.StreamInfo("moderation_events", nats.Context(ctx))
	if err != nil {
		return err
	}
	if info == nil || info.Config.Name != "moderation_events" || info.Config.Storage != nats.FileStorage || info.Config.Retention != nats.LimitsPolicy || info.Config.MaxAge != 7*24*time.Hour || len(info.Config.Subjects) != 3 {
		return errors.New("moderation stream contract mismatch")
	}
	seen := make(map[string]bool, len(info.Config.Subjects))
	for _, subject := range info.Config.Subjects {
		seen[subject] = true
	}
	for _, expected := range []string{"moderation.report_created", "moderation.sanction_applied", "moderation.appeal_submitted"} {
		if !seen[expected] {
			return errors.New("moderation stream subjects mismatch")
		}
	}
	message := &nats.Msg{Subject: event.Subject, Data: event.Payload, Header: nats.Header{}}
	message.Header.Set(nats.MsgIdHdr, event.ID.String())
	ack, err := p.js.PublishMsg(message, nats.Context(ctx))
	if err != nil {
		return err
	}
	if ack == nil || ack.Stream != "moderation_events" || ack.Sequence == 0 {
		return errors.New("invalid threshold publication acknowledgement")
	}
	return nil
}
