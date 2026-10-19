package partysearch

import (
	"context"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/matchmaking/internal/mmevents"
	"voice/backend/matchmaking/internal/rosterconsume"
	"voice/backend/matchmaking/internal/rosterruntime"
	"voice/backend/matchmaking/internal/store"
)

// Explicit delivery fixture; it is not a qualified installed NATS publisher.
type runtimeDeliveryFixture struct {
	mmevents.NoopPublisher
	calls atomic.Int32
}

func (p *runtimeDeliveryFixture) PublishRosterCancellation(context.Context, uuid.UUID, []byte) error {
	p.calls.Add(1)
	return nil
}

func TestRosterRuntimePrepareDoesNotFetchAckOrMutateBeforeStart(t *testing.T) {
	url := os.Getenv("BE011_TEST_NATS_URL")
	if url == "" {
		t.Skip("owned native NATS fixture unavailable: set BE011_TEST_NATS_URL")
	}
	ctx := context.Background()
	pool := partyDB(t)
	p := partyParams(t, pool)
	party, err := (&store.PartyStore{Pool: pool}).CreateSourceParty(ctx, p)
	require.NoError(t, err)
	admin, err := nats.Connect(url)
	require.NoError(t, err)
	defer admin.Close()
	t.Logf("owned native NATS version %s", admin.ConnectedServerVersion())
	js, err := admin.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: rosterconsume.Stream, Subjects: []string{rosterconsume.Subject}, Storage: nats.FileStorage})
	require.NoError(t, err)
	defer func() { _ = js.DeleteStream(rosterconsume.Stream) }()
	_, err = js.AddConsumer(rosterconsume.Stream, &nats.ConsumerConfig{Durable: rosterconsume.Durable, FilterSubject: rosterconsume.Subject, DeliverPolicy: nats.DeliverAllPolicy, AckPolicy: nats.AckNonePolicy})
	require.NoError(t, err)
	eventID := uuid.New()
	raw, err := proto.Marshal(&eventsv1.VoiceStreamEvent{EventId: eventID.String(), OccurredAt: timestamppb.Now(), Payload: &eventsv1.VoiceStreamEvent_VoiceRosterMembershipChanged{VoiceRosterMembershipChanged: &eventsv1.VoiceRosterMembershipChanged{ProtocolVersion: 1, RoomId: p.SourceRoomID.String(), RosterVersion: 3, ProfileId: p.MemberProfileIDs[0].String(), Change: eventsv1.VoiceRosterMembershipChanged_CHANGE_LEFT}}})
	require.NoError(t, err)
	_, err = js.Publish(rosterconsume.Subject, raw)
	require.NoError(t, err)
	publisher := &runtimeDeliveryFixture{}
	metrics := prometheus.NewRegistry()
	rejected, err := rosterruntime.Prepare(ctx, pool, url, publisher, metrics, nil)
	require.ErrorIs(t, err, rosterruntime.ErrUnavailable)
	require.Nil(t, rejected)
	info, err := js.ConsumerInfo(rosterconsume.Stream, rosterconsume.Durable)
	require.NoError(t, err)
	require.Zero(t, info.Delivered.Consumer)
	require.NoError(t, js.DeleteConsumer(rosterconsume.Stream, rosterconsume.Durable))
	_, err = js.AddConsumer(rosterconsume.Stream, &nats.ConsumerConfig{Durable: rosterconsume.Durable, FilterSubject: rosterconsume.Subject, DeliverPolicy: nats.DeliverAllPolicy, AckPolicy: nats.AckExplicitPolicy})
	require.NoError(t, err)
	prepared, err := rosterruntime.Prepare(ctx, pool, url, publisher, metrics, nil)
	require.NoError(t, err)
	defer prepared.Close()
	var closures atomic.Int32
	require.NoError(t, prepared.SetContractMismatchHandler(func() { closures.Add(1) }))
	require.Never(t, func() bool {
		info, e := js.ConsumerInfo(rosterconsume.Stream, rosterconsume.Durable)
		require.NoError(t, e)
		return info.Delivered.Consumer != 0 || info.AckFloor.Stream != 0 || publisher.calls.Load() != 0
	}, 500*time.Millisecond, 10*time.Millisecond, "preparation must not fetch, ACK or dispatch")
	var inboxRows, searching int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_voice_roster_inbox`).Scan(&inboxRows))
	require.Zero(t, inboxRows)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM search_sessions WHERE party_id=$1 AND status='searching'`, party.Party.ID).Scan(&searching))
	require.Equal(t, 2, searching)
	require.NoError(t, prepared.Start())
	require.NoError(t, prepared.Start(), "same prepared lifetime cannot start duplicate workers")
	require.Eventually(t, func() bool {
		info, e := js.ConsumerInfo(rosterconsume.Stream, rosterconsume.Durable)
		if e != nil {
			return false
		}
		return info.AckFloor.Stream == 1 && publisher.calls.Load() == 2
	}, 10*time.Second, 20*time.Millisecond)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_voice_roster_inbox`).Scan(&inboxRows))
	require.Equal(t, 1, inboxRows)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM search_sessions WHERE party_id=$1 AND status='searching'`, party.Party.ID).Scan(&searching))
	require.Zero(t, searching)
	conflict, err := proto.Marshal(&eventsv1.VoiceStreamEvent{EventId: eventID.String(), OccurredAt: timestamppb.Now(), Payload: &eventsv1.VoiceStreamEvent_VoiceRosterMembershipChanged{VoiceRosterMembershipChanged: &eventsv1.VoiceRosterMembershipChanged{ProtocolVersion: 1, RoomId: p.SourceRoomID.String(), RosterVersion: 4, ProfileId: p.MemberProfileIDs[0].String(), Change: eventsv1.VoiceRosterMembershipChanged_CHANGE_LEFT}}})
	require.NoError(t, err)
	_, err = js.Publish(rosterconsume.Subject, conflict)
	require.NoError(t, err)
	require.Eventually(t, func() bool { return closures.Load() > 0 }, 10*time.Second, 20*time.Millisecond, "persisted mismatch must invoke native closure")
	require.Error(t, prepared.SetContractMismatchHandler(func() {}), "closure cannot be rebound after delivery starts")
	prepared.Close()
	require.Error(t, prepared.Start(), "closed runtime cannot restart or reopen processing")
}
