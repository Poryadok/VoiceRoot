package partysearch

import (
	"context"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"os"
	"testing"
	"time"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/matchmaking/internal/mmevents"
	"voice/backend/matchmaking/internal/store"
	"voice/backend/matchmaking/internal/timeout"
)

func TestNativeTimeoutDurableFactsRecoverLostSqlAckWithSameBrokerIdentity(t *testing.T) {
	url := os.Getenv("BE011_TEST_NATS_URL")
	if url == "" {
		t.Skip("owned native NATS fixture unavailable")
	}
	ctx := context.Background()
	pool, st, party, _ := nativeTimeoutParty(t)
	dueNativeTimeoutParty(t, pool, party.Party.ID)
	_, err := pool.Exec(ctx, `UPDATE mm_native_admission_head SET state='CLOSED'`)
	require.NoError(t, err)
	result, err := st.ExpireNativeSearchingParty(ctx, party.Sessions[0].ID)
	require.NoError(t, err)
	require.True(t, result.Expired, "no broker/Redis client exists during terminal commit")
	admin, err := nats.Connect(url)
	require.NoError(t, err)
	defer admin.Close()
	t.Logf("owned NATS %s; explicit plaintext fixture, not installed broker authority", admin.ConnectedServerVersion())
	js, err := admin.JetStream()
	require.NoError(t, err)
	_, err = js.StreamInfo("matchmaking_events")
	require.ErrorIs(t, err, nats.ErrStreamNotFound, "never replace a preexisting object")
	_, err = js.AddStream(&nats.StreamConfig{Name: "matchmaking_events", Subjects: []string{"mm.search_started", "mm.search_cancelled", "mm.match_found", "mm.match_completed", "mm.rating_submitted", "mm.player_banned", "mm.search_nudge", "mm.search_timeout"}, Storage: nats.FileStorage, Retention: nats.LimitsPolicy, MaxAge: 168 * time.Hour, Duplicates: 2 * time.Minute})
	require.NoError(t, err)
	defer func() { _ = js.DeleteStream("matchmaking_events") }()
	publisher, err := mmevents.NewJetStreamPublisher(url)
	require.NoError(t, err)
	defer publisher.Close()
	facts, err := st.ClaimNativeTimeouts(ctx, 100, 30*time.Second)
	require.NoError(t, err)
	require.Len(t, facts, len(party.Sessions))
	original := make(map[string][]byte)
	for _, fact := range facts {
		original[fact.EventID.String()] = append([]byte(nil), fact.Payload...)
		require.NoError(t, publisher.PublishCapturedTimeout(ctx, fact.EventID, fact.Payload))
		// Simulate process loss AFTER actual PubAck, BEFORE the SQL ACK.
	}
	var delivered int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_native_timeout_outbox WHERE delivered_at IS NOT NULL`).Scan(&delivered))
	require.Zero(t, delivered)
	_, err = pool.Exec(ctx, `UPDATE matchmaking_native_timeout_outbox SET lease_until=statement_timestamp()-interval '1 second'`)
	require.NoError(t, err)
	require.NoError(t, timeout.RelayNativeTimeouts(ctx, st, publisher))
	for _, stale := range facts {
		require.ErrorIs(t, st.FinishNativeTimeout(ctx, stale, true), store.ErrNativeTimeoutLeaseLost)
	}
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_native_timeout_outbox WHERE delivered_at IS NOT NULL`).Scan(&delivered))
	require.Equal(t, len(original), delivered)
	info, err := js.StreamInfo("matchmaking_events")
	require.NoError(t, err)
	require.EqualValues(t, len(original), info.State.Msgs, "broker duplicate window retains the same ID; this is not universal physical exactly-once")
	for sequence := uint64(1); sequence <= info.State.LastSeq; sequence++ {
		message, err := js.GetMsg("matchmaking_events", sequence)
		require.NoError(t, err)
		envelope := new(eventsv1.MatchmakingStreamEvent)
		require.NoError(t, proto.Unmarshal(message.Data, envelope))
		require.Equal(t, original[envelope.EventId], message.Data)
		require.Equal(t, envelope.EventId, message.Header.Get(nats.MsgIdHdr))
		require.NotNil(t, envelope.GetMatchTimeout())
		require.Nil(t, envelope.GetSearchCancelled())
	}
	more, err := st.ClaimNativeTimeouts(ctx, 100, time.Second)
	require.NoError(t, err)
	require.Empty(t, more)
}
