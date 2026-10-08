package spacemedia

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type admissionOutboxStoreFake struct {
	item      *AdmissionOutboxItem
	delivered int
	released  int
}

func (s *admissionOutboxStoreFake) ClaimOutbox(context.Context, time.Duration) (*AdmissionOutboxItem, error) {
	return s.item, nil
}
func (s *admissionOutboxStoreFake) MarkOutboxDelivered(context.Context, AdmissionOutboxItem) error {
	s.delivered++
	return nil
}
func (s *admissionOutboxStoreFake) ReleaseOutboxLease(context.Context, AdmissionOutboxItem) error {
	s.released++
	return nil
}

type admissionOutboxPublisherFake struct {
	id      uuid.UUID
	subject string
	payload []byte
	err     error
}

func (p *admissionOutboxPublisherFake) PublishAdmissionEvent(_ context.Context, id uuid.UUID, subject string, payload []byte) error {
	p.id, p.subject, p.payload = id, subject, append([]byte(nil), payload...)
	return p.err
}

func TestAdmissionOutboxRelayMarksDeliveredOnlyAfterPublishAck(t *testing.T) {
	item := &AdmissionOutboxItem{ID: uuid.New(), OperationID: uuid.New(), Subject: "voice.member_joined", Payload: []byte("immutable")}
	store := &admissionOutboxStoreFake{item: item}
	publisher := &admissionOutboxPublisherFake{err: errors.New("ack unavailable")}
	relay := &AdmissionOutboxRelay{Store: store, Publisher: publisher}
	require.Error(t, relay.DispatchOnce(context.Background()))
	require.Equal(t, item.ID, publisher.id)
	require.Equal(t, item.Subject, publisher.subject)
	require.Equal(t, item.Payload, publisher.payload)
	require.Zero(t, store.delivered)
	require.Equal(t, 1, store.released)

	publisher.err = nil
	require.NoError(t, relay.DispatchOnce(context.Background()))
	require.Equal(t, 1, store.delivered)
	require.Equal(t, 1, store.released)
}
