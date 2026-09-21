package searchprojection

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"

	"voice/backend/user/internal/store"
)

const (
	projectionStream  = "user_profile_projection"
	projectionSubject = "user.search_profile_projection"
)

// OutboxDispatcher publishes the exact bytes committed with User's authority
// event. A durable PubAck is required before the row may be marked delivered.
type OutboxDispatcher struct {
	store projectionOutboxStore
	js    projectionPublisher
	owner string
}

type projectionOutboxStore interface {
	ClaimSearchProjectionOutbox(context.Context, string, time.Time) (*store.SearchProjectionOutboxRecord, error)
	MarkSearchProjectionOutboxDelivered(context.Context, uuid.UUID, string) error
	ReleaseSearchProjectionOutboxLease(context.Context, uuid.UUID, string) error
}

type projectionPublisher interface {
	PublishMsg(*nats.Msg, ...nats.PubOpt) (*nats.PubAck, error)
}

func NewOutboxDispatcher(profiles *store.ProfileStore, natsURL, owner string) (*OutboxDispatcher, *nats.Conn, error) {
	if profiles == nil || natsURL == "" || owner == "" {
		return nil, nil, errors.New("projection outbox dispatcher requires store, NATS URL, and owner")
	}
	nc, err := nats.Connect(natsURL, nats.Name("voice-user-search-projection"), nats.RetryOnFailedConnect(true), nats.MaxReconnects(-1))
	if err != nil {
		return nil, nil, err
	}
	js, err := nc.JetStream()
	if err != nil {
		_ = nc.Drain()
		return nil, nil, err
	}
	if _, err := js.StreamInfo(projectionStream); err != nil {
		if _, err := js.AddStream(&nats.StreamConfig{Name: projectionStream, Subjects: []string{projectionSubject}, Retention: nats.LimitsPolicy, Storage: nats.FileStorage}); err != nil {
			_ = nc.Drain()
			return nil, nil, err
		}
	}
	return &OutboxDispatcher{store: profiles, js: js, owner: owner}, nc, nil
}

// DispatchOnce preserves ordering by claiming one offset at a time. On any
// known publish failure the lease is released for retry; an ambiguous crash
// after PubAck intentionally leaves the lease for redelivery by message ID.
func (d *OutboxDispatcher) DispatchOnce(ctx context.Context) (bool, error) {
	record, err := d.store.ClaimSearchProjectionOutbox(ctx, d.owner, time.Now().Add(30*time.Second))
	if err != nil || record == nil {
		return record != nil, err
	}
	msg := &nats.Msg{Subject: projectionSubject, Data: record.Payload, Header: nats.Header{}}
	msg.Header.Set(nats.MsgIdHdr, record.EventID.String())
	ack, err := d.js.PublishMsg(msg, nats.Context(ctx))
	if err != nil || ack == nil || ack.Stream != projectionStream || ack.Sequence == 0 {
		_ = d.store.ReleaseSearchProjectionOutboxLease(ctx, record.EventID, d.owner)
		if err == nil {
			err = errors.New("missing or invalid projection PubAck")
		}
		return true, fmt.Errorf("publish projection outbox: %w", err)
	}
	if err := d.store.MarkSearchProjectionOutboxDelivered(ctx, record.EventID, d.owner); err != nil {
		return true, fmt.Errorf("mark projection outbox delivered after PubAck: %w", err)
	}
	return true, nil
}

func (d *OutboxDispatcher) Run(ctx context.Context) {
	for ctx.Err() == nil {
		found, err := d.DispatchOnce(ctx)
		if err != nil || !found {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
		}
	}
}
