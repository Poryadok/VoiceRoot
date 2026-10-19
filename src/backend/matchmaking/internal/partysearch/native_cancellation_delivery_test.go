package partysearch

import (
	"context"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"os"
	"testing"
	"time"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/matchmaking/internal/mmevents"
	"voice/backend/matchmaking/internal/rosterconsume"
	"voice/backend/matchmaking/internal/rosterruntime"
	"voice/backend/matchmaking/internal/store"
)

func TestNativeCancellationOutboxDeliversOriginalIDsThroughPreparedRuntime(t *testing.T) {
	url := os.Getenv("BE011_TEST_NATS_URL")
	if url == "" {
		t.Skip("owned native NATS fixture unavailable")
	}
	ctx := context.Background()
	pool := partyDB(t)
	binding, _, _ := historyReceiptFixture(t, pool)
	p := partyParams(t, pool)
	party, err := (&store.PartyStore{Pool: pool, NativeAdmission: &binding}).CreateSourceParty(ctx, p)
	require.NoError(t, err)
	st := &store.MatchStore{Pool: pool, NativeAdmission: &binding}
	var session uuid.UUID
	for _, row := range party.Sessions {
		if row.ProfileID == p.InitiatorProfileID {
			session = row.ID
		}
	}
	_, err = pool.Exec(ctx, `UPDATE mm_native_admission_head SET state='CLOSED'`)
	require.NoError(t, err)
	require.NoError(t, st.CancelNativeSearch(ctx, p.InitiatorAccountID, p.InitiatorProfileID, session))
	original := map[string][]byte{}
	rows, err := pool.Query(ctx, `SELECT event_id,payload FROM matchmaking_voice_roster_cancellation_outbox`)
	require.NoError(t, err)
	for rows.Next() {
		var id uuid.UUID
		var raw []byte
		require.NoError(t, rows.Scan(&id, &raw))
		original[id.String()] = raw
	}
	require.NoError(t, rows.Err())
	rows.Close()
	require.Len(t, original, len(p.MemberProfileIDs))
	admin, err := nats.Connect(url)
	require.NoError(t, err)
	defer admin.Close()
	t.Logf("owned native NATS %s; explicit noninstalled plaintext fixture", admin.ConnectedServerVersion())
	js, err := admin.JetStream()
	require.NoError(t, err)
	_, err = js.StreamInfo("matchmaking_events")
	require.ErrorIs(t, err, nats.ErrStreamNotFound, "do not replace a preexisting broker object")
	_, err = js.AddStream(&nats.StreamConfig{Name: "matchmaking_events", Subjects: []string{"mm.search_started", "mm.search_cancelled", "mm.match_found", "mm.match_completed", "mm.rating_submitted", "mm.player_banned", "mm.search_nudge", "mm.search_timeout"}, Storage: nats.FileStorage, Retention: nats.LimitsPolicy, MaxAge: 168 * time.Hour, Duplicates: 2 * time.Minute})
	require.NoError(t, err)
	defer func() { _ = js.DeleteStream("matchmaking_events") }()
	_, err = js.StreamInfo(rosterconsume.Stream)
	require.ErrorIs(t, err, nats.ErrStreamNotFound)
	_, err = js.AddStream(&nats.StreamConfig{Name: rosterconsume.Stream, Subjects: []string{rosterconsume.Subject}, Storage: nats.FileStorage})
	require.NoError(t, err)
	defer func() { _ = js.DeleteStream(rosterconsume.Stream) }()
	_, err = js.AddConsumer(rosterconsume.Stream, &nats.ConsumerConfig{Durable: rosterconsume.Durable, FilterSubject: rosterconsume.Subject, DeliverPolicy: nats.DeliverAllPolicy, AckPolicy: nats.AckExplicitPolicy})
	require.NoError(t, err)
	publisher, err := mmevents.NewJetStreamPublisher(url)
	require.NoError(t, err)
	defer publisher.Close()
	require.NoError(t, publisher.CheckNativeConnection(ctx))
	runtime, err := rosterruntime.PrepareNative(ctx, pool, url, publisher, prometheus.NewRegistry(), nil)
	require.NoError(t, err)
	defer runtime.Close()
	var delivered int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_voice_roster_cancellation_outbox WHERE delivered_at IS NOT NULL`).Scan(&delivered))
	require.Zero(t, delivered)
	info, err := js.StreamInfo("matchmaking_events")
	require.NoError(t, err)
	require.Zero(t, info.State.Msgs)
	require.NoError(t, runtime.Start())
	require.Eventually(t, func() bool {
		return pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_voice_roster_cancellation_outbox WHERE delivered_at IS NOT NULL`).Scan(&delivered) == nil && delivered == len(original)
	}, 6*time.Second, 10*time.Millisecond)
	runtime.Close()
	info, err = js.StreamInfo("matchmaking_events")
	require.NoError(t, err)
	require.EqualValues(t, len(original), info.State.Msgs)
	for sequence := uint64(1); sequence <= info.State.LastSeq; sequence++ {
		msg, e := js.GetMsg("matchmaking_events", sequence)
		require.NoError(t, e)
		envelope := new(eventsv1.MatchmakingStreamEvent)
		require.NoError(t, proto.Unmarshal(msg.Data, envelope))
		require.Equal(t, original[envelope.EventId], msg.Data)
		require.Equal(t, envelope.EventId, msg.Header.Get(nats.MsgIdHdr))
		require.Empty(t, envelope.GetSearchCancelled().Reason)
		require.NoError(t, publisher.PublishCapturedCancellation(ctx, uuid.MustParse(envelope.EventId), msg.Data))
	}
	info, err = js.StreamInfo("matchmaking_events")
	require.NoError(t, err)
	require.EqualValues(t, len(original), info.State.Msgs, "lost-ack retry keeps original dedup identity")
}
