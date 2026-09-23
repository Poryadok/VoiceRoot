package searchprojection

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"

	"voice/backend/user/internal/store"
)

type fakeProjectionOutboxStore struct {
	record   *store.SearchProjectionOutboxRecord
	released uuid.UUID
	marked   uuid.UUID
}

func (s *fakeProjectionOutboxStore) ClaimSearchProjectionOutbox(context.Context, string, time.Time) (*store.SearchProjectionOutboxRecord, error) {
	return s.record, nil
}
func (s *fakeProjectionOutboxStore) MarkSearchProjectionOutboxDelivered(_ context.Context, id uuid.UUID, _ string) error {
	s.marked = id
	return nil
}
func (s *fakeProjectionOutboxStore) ReleaseSearchProjectionOutboxLease(_ context.Context, id uuid.UUID, _ string) error {
	s.released = id
	return nil
}

type fakeProjectionPublisher struct{ ack *nats.PubAck }

func (p fakeProjectionPublisher) PublishMsg(*nats.Msg, ...nats.PubOpt) (*nats.PubAck, error) {
	return p.ack, nil
}

func TestOutboxDispatcher_MissingPubAckLeavesRecordRetryable(t *testing.T) {
	eventID := uuid.New()
	outbox := &fakeProjectionOutboxStore{record: &store.SearchProjectionOutboxRecord{EventID: eventID, JournalOffset: 7, Payload: []byte("stable")}}
	dispatcher := &OutboxDispatcher{store: outbox, js: fakeProjectionPublisher{}, owner: "test"}
	found, err := dispatcher.DispatchOnce(context.Background())
	require.True(t, found)
	require.Error(t, err)
	require.Equal(t, eventID, outbox.released)
	require.Equal(t, uuid.Nil, outbox.marked)
}

func TestOutboxDispatcher_BadPubAckLeavesRecordRetryable(t *testing.T) {
	eventID := uuid.New()
	outbox := &fakeProjectionOutboxStore{record: &store.SearchProjectionOutboxRecord{EventID: eventID, JournalOffset: 7, Payload: []byte("stable")}}
	dispatcher := &OutboxDispatcher{store: outbox, js: fakeProjectionPublisher{ack: &nats.PubAck{Stream: "wrong", Sequence: 1}}, owner: "test"}
	found, err := dispatcher.DispatchOnce(context.Background())
	require.True(t, found)
	require.Error(t, err)
	require.Equal(t, eventID, outbox.released)
	require.Equal(t, uuid.Nil, outbox.marked)
}

func TestValidateProjectionStreamRequiresDeploymentContract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  nats.StreamConfig
		wantErr bool
	}{
		{
			name: "exact deployment stream",
			config: nats.StreamConfig{
				Name:      projectionStream,
				Subjects:  []string{projectionSubject},
				Retention: nats.LimitsPolicy,
				Storage:   nats.FileStorage,
			},
		},
		{
			name: "wrong subject",
			config: nats.StreamConfig{
				Name:      projectionStream,
				Subjects:  []string{"user.>"},
				Retention: nats.LimitsPolicy,
				Storage:   nats.FileStorage,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateProjectionStream(&nats.StreamInfo{Config: tt.config})
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestValidateProjectionStreamRejectsMissingStream(t *testing.T) {
	t.Parallel()
	require.Error(t, validateProjectionStream(nil))
}
