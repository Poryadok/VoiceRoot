package messageevents

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type outboxStoreSpy struct {
	events  []OutboxEvent
	acked   []uuid.UUID
	retried []uuid.UUID
	next    time.Time
}

func (s *outboxStoreSpy) ClaimMessageEventOutbox(context.Context, int, time.Duration) ([]OutboxEvent, error) {
	return append([]OutboxEvent(nil), s.events...), nil
}
func (s *outboxStoreSpy) RecordMessageEventPubAck(_ context.Context, id, _ uuid.UUID, sequence uint64) (bool, error) {
	if sequence == 0 {
		return false, errors.New("missing sequence")
	}
	s.acked = append(s.acked, id)
	return true, nil
}
func (s *outboxStoreSpy) RetryMessageEventOutbox(_ context.Context, id, _ uuid.UUID, next time.Time) (bool, error) {
	s.retried = append(s.retried, id)
	s.next = next
	return true, nil
}

type outboxPublisherSpy struct {
	event OutboxEvent
	seq   uint64
	err   error
}

func (p *outboxPublisherSpy) PublishOutboxEvent(_ context.Context, event OutboxEvent) (uint64, error) {
	p.event = event
	return p.seq, p.err
}

func TestOutboxDispatcherPublishesPersistedBytesAndRecordsPositivePubAck(t *testing.T) {
	id, lease := uuid.New(), uuid.New()
	payload := []byte{0x0a, 0x03, 0x01, 0x02, 0x03}
	store := &outboxStoreSpy{events: []OutboxEvent{{EventID: id, LeaseToken: lease, Subject: subjectMessageSent, Payload: payload, Attempts: 1}}}
	publisher := &outboxPublisherSpy{seq: 17}
	dispatcher := &OutboxDispatcher{Store: store, Publisher: publisher}

	require.NoError(t, dispatcher.DispatchBatch(context.Background()))
	require.Equal(t, id, publisher.event.EventID)
	require.True(t, bytes.Equal(payload, publisher.event.Payload), "dispatcher must pass persisted bytes without reconstruction")
	require.Equal(t, []uuid.UUID{id}, store.acked)
	require.Empty(t, store.retried)
}

func TestOutboxDispatcherSchedulesRetryOnUncertainPublish(t *testing.T) {
	id, lease := uuid.New(), uuid.New()
	store := &outboxStoreSpy{events: []OutboxEvent{{EventID: id, LeaseToken: lease, Subject: subjectMessageSent, Payload: []byte{1}, Attempts: 3}}}
	now := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	dispatcher := &OutboxDispatcher{
		Store: store, Publisher: &outboxPublisherSpy{err: errors.New("ack uncertain")},
		Now: func() time.Time { return now },
	}

	require.NoError(t, dispatcher.DispatchBatch(context.Background()))
	require.Empty(t, store.acked)
	require.Equal(t, []uuid.UUID{id}, store.retried)
	require.Equal(t, now.Add(4*time.Second), store.next)
}
