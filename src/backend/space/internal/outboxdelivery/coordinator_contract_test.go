package outboxdelivery

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
)

const (
	testChatEventsStream = "chat_events"
	testSpaceSubject     = "space.updated"
)

var (
	testEventID     = uuid.MustParse("8ec754c8-c06d-4a79-a72c-1315ef7ca573")
	testSpaceID     = uuid.MustParse("22487458-ab26-4f0d-9046-e8e79bb8f3fd")
	testLease       = uuid.MustParse("95af2daa-d6d9-4d75-8bd8-bf2ef03a68d3")
	testReplayLease = uuid.MustParse("884e77e1-d3f7-4205-8af4-f1f55fbc197b")
	testCreatedAt   = time.Date(2026, time.September, 11, 7, 8, 9, 123456000, time.UTC)
)

var _ DeliveryStore = (*fakeDeliveryStore)(nil)
var _ Transport = (*scriptedTransport)(nil)

type fakeDeliveryStore struct {
	event         ClaimedEvent
	leaseTokens   []uuid.UUID
	currentLease  uuid.UUID
	preserveLease bool
	claims        []ClaimedEvent
	markCalls     []deliveryMark
	delivered     bool
	markErrs      []error
}

type deliveryMark struct {
	eventID    uuid.UUID
	leaseToken uuid.UUID
}

func newFakeDeliveryStore() *fakeDeliveryStore {
	event := ClaimedEvent{
		EventID:    testEventID,
		SpaceID:    testSpaceID,
		EventType:  testSpaceSubject,
		CreatedAt:  testCreatedAt,
		LeaseToken: testLease,
	}
	return &fakeDeliveryStore{event: event, leaseTokens: []uuid.UUID{testLease}}
}

func (s *fakeDeliveryStore) ClaimReady(_ context.Context, limit int) ([]ClaimedEvent, error) {
	if limit != 100 {
		return nil, errors.New("coordinator must request the documented maximum batch of 100")
	}
	if s.delivered {
		return nil, nil
	}
	claim := s.event
	if len(s.claims) < len(s.leaseTokens) {
		claim.LeaseToken = s.leaseTokens[len(s.claims)]
	}
	if !s.preserveLease {
		s.currentLease = claim.LeaseToken
	}
	s.claims = append(s.claims, claim)
	return []ClaimedEvent{claim}, nil
}

func (s *fakeDeliveryStore) MarkDelivered(_ context.Context, eventID, leaseToken uuid.UUID) (bool, error) {
	s.markCalls = append(s.markCalls, deliveryMark{eventID: eventID, leaseToken: leaseToken})
	if len(s.markErrs) != 0 {
		err := s.markErrs[0]
		s.markErrs = s.markErrs[1:]
		if err != nil {
			return false, err
		}
	}
	if eventID != s.event.EventID || leaseToken != s.currentLease {
		return false, nil
	}
	s.delivered = true
	return true, nil
}

type publishOutcome struct {
	ack    *nats.PubAck
	err    error
	record bool
}

type scriptedTransport struct {
	outcomes []publishOutcome
	calls    int
	msgs     []*nats.Msg
}

func (p *scriptedTransport) Publish(_ context.Context, msg *nats.Msg) (*nats.PubAck, error) {
	if p.calls >= len(p.outcomes) {
		return nil, errors.New("unexpected publish")
	}
	outcome := p.outcomes[p.calls]
	p.calls++
	if outcome.record {
		p.msgs = append(p.msgs, cloneNATSMessage(msg))
	}
	return outcome.ack, outcome.err
}

func cloneNATSMessage(msg *nats.Msg) *nats.Msg {
	copyMsg := &nats.Msg{
		Subject: msg.Subject,
		Data:    bytes.Clone(msg.Data),
		Header:  nats.Header{},
	}
	for key, values := range msg.Header {
		copyMsg.Header[key] = append([]string(nil), values...)
	}
	return copyMsg
}

func correctAck() *nats.PubAck {
	return &nats.PubAck{Stream: testChatEventsStream, Sequence: 1}
}

func TestOwnershipOutboxDelivery_UsesStoredIdentityTimeDeterministicEnvelopeAndMessageID(t *testing.T) {
	store := newFakeDeliveryStore()
	transport := &scriptedTransport{outcomes: []publishOutcome{{ack: correctAck(), record: true}}}

	require.NoError(t, NewCoordinator(store, transport).DispatchOnce(context.Background()))
	require.True(t, store.delivered)
	require.Len(t, transport.msgs, 1)

	msg := transport.msgs[0]
	require.Equal(t, testSpaceSubject, msg.Subject)
	require.Equal(t, testEventID.String(), msg.Header.Get(nats.MsgIdHdr))

	var envelope eventsv1.ChatStreamEvent
	require.NoError(t, proto.Unmarshal(msg.Data, &envelope))
	require.Equal(t, testEventID.String(), envelope.GetEventId())
	require.NotNil(t, envelope.GetOccurredAt())
	require.NoError(t, envelope.GetOccurredAt().CheckValid())
	require.Equal(t, testCreatedAt, envelope.GetOccurredAt().AsTime(), "stored created_at must be the envelope occurred_at")
	require.Equal(t, testSpaceID.String(), envelope.GetSpaceUpdated().GetSpaceId())
	require.IsType(t, &eventsv1.ChatStreamEvent_SpaceUpdated{}, envelope.GetPayload())

	deterministic, err := proto.MarshalOptions{Deterministic: true}.Marshal(&envelope)
	require.NoError(t, err)
	require.Equal(t, deterministic, msg.Data)
	require.Equal(t, []deliveryMark{{eventID: testEventID, leaseToken: testLease}}, store.markCalls)
}

func TestOwnershipOutboxDelivery_RequiresChatEventsPubAckBeforeMarkingDelivered(t *testing.T) {
	tests := []struct {
		name          string
		ack           *nats.PubAck
		wantErr       bool
		wantDelivered bool
		wantMarks     int
	}{
		{name: "matching stream", ack: correctAck(), wantDelivered: true, wantMarks: 1},
		{name: "wrong stream", ack: &nats.PubAck{Stream: "other_events", Sequence: 1}, wantErr: true},
		{name: "missing acknowledgement", ack: nil, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newFakeDeliveryStore()
			transport := &scriptedTransport{outcomes: []publishOutcome{{ack: tt.ack, record: true}}}

			err := NewCoordinator(store, transport).DispatchOnce(context.Background())
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.wantDelivered, store.delivered)
			require.Len(t, store.markCalls, tt.wantMarks)
		})
	}
}

func TestOwnershipOutboxDelivery_RejectsStaleLeaseTokenAcknowledgement(t *testing.T) {
	store := newFakeDeliveryStore()
	store.currentLease = testReplayLease
	store.preserveLease = true
	transport := &scriptedTransport{outcomes: []publishOutcome{{ack: correctAck(), record: true}}}

	err := NewCoordinator(store, transport).DispatchOnce(context.Background())
	require.Error(t, err)
	require.False(t, store.delivered)
	require.Equal(t, []deliveryMark{{eventID: testEventID, leaseToken: testLease}}, store.markCalls)
}

func TestOwnershipOutboxDelivery_CrashBoundariesReplaySameEventID(t *testing.T) {
	errCrash := errors.New("simulated process crash")
	tests := []struct {
		name          string
		firstPublish  publishOutcome
		firstMarkErr  error
		firstMessages int
	}{
		{
			name:          "after claim before publish",
			firstPublish:  publishOutcome{err: errCrash, record: false},
			firstMessages: 0,
		},
		{
			name:          "after publish before acknowledgement",
			firstPublish:  publishOutcome{err: errCrash, record: true},
			firstMessages: 1,
		},
		{
			name:          "after PubAck before delivery mark",
			firstPublish:  publishOutcome{ack: correctAck(), record: true},
			firstMarkErr:  errCrash,
			firstMessages: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newFakeDeliveryStore()
			store.leaseTokens = []uuid.UUID{testLease, testReplayLease}
			if tt.firstMarkErr != nil {
				store.markErrs = []error{tt.firstMarkErr}
			}
			transport := &scriptedTransport{outcomes: []publishOutcome{
				tt.firstPublish,
				{ack: correctAck(), record: true},
			}}
			coordinator := NewCoordinator(store, transport)

			require.ErrorIs(t, coordinator.DispatchOnce(context.Background()), errCrash)
			require.False(t, store.delivered)
			require.Len(t, transport.msgs, tt.firstMessages)

			require.NoError(t, coordinator.DispatchOnce(context.Background()))
			require.True(t, store.delivered)
			require.Len(t, store.claims, 2)
			require.Equal(t, store.claims[0].EventID, store.claims[1].EventID)
			require.Equal(t, testEventID, store.claims[1].EventID)
			require.NotEqual(t, store.claims[0].LeaseToken, store.claims[1].LeaseToken, "post-expiry reclaim must use a fresh fencing token")

			for _, msg := range transport.msgs {
				require.Equal(t, testEventID.String(), msg.Header.Get(nats.MsgIdHdr))
				var envelope eventsv1.ChatStreamEvent
				require.NoError(t, proto.Unmarshal(msg.Data, &envelope))
				require.Equal(t, testEventID.String(), envelope.GetEventId())
			}
			if len(transport.msgs) == 2 {
				require.Equal(t, transport.msgs[0].Data, transport.msgs[1].Data, "replay must publish byte-identical deterministic payload")
			}
		})
	}
}
