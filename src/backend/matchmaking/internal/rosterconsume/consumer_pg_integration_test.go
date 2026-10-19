package rosterconsume

import (
	"bytes"
	"context"
	"crypto/sha256"
	"net"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/matchmaking/internal/store"
	"voice/backend/pkg/partyauthority"
)

const pgAckBrokerImage = "nats@sha256:2ca98656a279b2d88cfdf2b8c3f0d5d7f3941ae9dc2ab12ebaa92d83e0f4ccdb"

type committedPGInbox struct {
	store                           *store.MatchStore
	entered, failed, committed      chan struct{}
	enterOnce, failOnce, commitOnce sync.Once
	afterCommit                     func()
	calls                           atomic.Int32
}

func (s *committedPGInbox) ApplyVoiceRosterEvent(ctx context.Context, raw []byte) (store.VoiceRosterResetResult, error) {
	s.calls.Add(1)
	s.enterOnce.Do(func() { close(s.entered) })
	result, err := s.store.ApplyVoiceRosterEvent(ctx, raw)
	if err != nil {
		s.failOnce.Do(func() { close(s.failed) })
		return result, err
	}
	s.commitOnce.Do(func() {
		// This hook runs only after the real product transaction returned from
		// COMMIT. Closing its TCP connection prevents the later product ACK.
		s.afterCommit()
		close(s.committed)
	})
	return result, nil
}

func pgAckFixture(t *testing.T, ctx context.Context) (*pgxpool.Pool, store.SourcePartySearch, store.CreateSourcePartyParams) {
	t.Helper()
	dsn := os.Getenv("BE011_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("set BE011_TEST_PG_DSN for real product PostgreSQL proof")
	}
	admin, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	database := "mm_roster_ack_" + uuid.New().String()[:8]
	_, err = admin.Exec(ctx, `CREATE DATABASE `+database)
	require.NoError(t, err)
	config, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	config.ConnConfig.Database = database
	config.MaxConns = 4
	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), `DROP DATABASE `+database)
		admin.Close()
	})
	paths, err := filepath.Glob("../../../migrations/matchmaking_db/*.up.sql")
	require.NoError(t, err)
	require.NotEmpty(t, paths)
	for _, path := range paths {
		body, err := os.ReadFile(path)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(body))
		require.NoError(t, err, filepath.Base(path))
	}
	var game uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT id FROM games ORDER BY id LIMIT 1`).Scan(&game))
	profiles := []uuid.UUID{uuid.New(), uuid.New()}
	sort.Slice(profiles, func(i, j int) bool { return bytes.Compare(profiles[i][:], profiles[j][:]) < 0 })
	room, account := uuid.New(), uuid.New()
	p := store.CreateSourcePartyParams{SearchOperationID: uuid.New(), SearchRequestHash: [32]byte{1}, InitiatorAccountID: account, InitiatorProfileID: profiles[0], InitiatorSessionEpoch: 7,
		ProtocolVersion: 1, SourceKind: "VOICE_ROSTER", SourceRoomID: &room, SourceRosterVersion: 2, MemberProfileIDs: profiles, MemberAuthorityVersion: 1,
		MemberAuthorities:   []partyauthority.Member{{ProfileID: profiles[0], AccountID: account, SessionEpoch: 3}, {ProfileID: profiles[1], AccountID: uuid.New(), SessionEpoch: 9}},
		SourceAuthorityHash: [32]byte{2}, GameID: game, Mode: "5v5 Ranked", Criteria: `{"region":"eu"}`, TimeoutAt: time.Now().Add(time.Hour)}
	party, err := (&store.PartyStore{Pool: pool}).CreateSourceParty(ctx, p)
	require.NoError(t, err)
	return pool, party, p
}

// C4/C5 product transaction evidence. The fixture grants no installed NATS ACL
// or rollout qualification; those authority gates remain independently owned.
func TestRosterConsumerRealPGCommitBeforeAckAndLostAckRedelivery(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL and pinned native NATS integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	runner := os.Getenv("BE012_TEST_NETWORK_CONTAINER")
	if runner == "" {
		t.Skip("set BE012_TEST_NETWORK_CONTAINER to the owned isolated Linux runner")
	}
	connection, probeErr := net.DialTimeout("tcp", "127.0.0.1:4222", 100*time.Millisecond)
	if probeErr == nil {
		_ = connection.Close()
		t.Fatal("owned runner NATS fixture port already in use")
	}
	pool, party, p := pgAckFixture(t, ctx)
	product := &store.MatchStore{Pool: pool}
	broker, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: testcontainers.ContainerRequest{
		Image: pgAckBrokerImage, Name: "voice-rudeus147-pg-ack-" + uuid.NewString()[:8], Cmd: []string{"-js"}, WaitingFor: wait.ForLog("Server is ready"),
		HostConfigModifier: func(config *dockercontainer.HostConfig) {
			config.NetworkMode = dockercontainer.NetworkMode("container:" + runner)
			config.PortBindings = nil
			config.PublishAllPorts = false
		},
	}, Started: true})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, broker.Terminate(context.Background())) })
	inspection, err := broker.Inspect(ctx)
	require.NoError(t, err)
	require.Empty(t, inspection.HostConfig.PortBindings, "broker stays portless on the owned Linux bridge")
	require.True(t, inspection.HostConfig.NetworkMode.IsContainer())
	url := "nats://127.0.0.1:4222"
	admin, err := nats.Connect(url)
	require.NoError(t, err)
	defer admin.Close()
	require.Equal(t, "2.12.12", admin.ConnectedServerVersion())
	js, err := admin.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: Stream, Subjects: []string{"voice.>"}, Storage: nats.FileStorage})
	require.NoError(t, err)
	eventID := uuid.New()
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(&eventsv1.VoiceStreamEvent{EventId: eventID.String(), OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.VoiceStreamEvent_VoiceRosterMembershipChanged{VoiceRosterMembershipChanged: &eventsv1.VoiceRosterMembershipChanged{
			ProtocolVersion: 1, RoomId: p.SourceRoomID.String(), RosterVersion: 3, ProfileId: p.MemberProfileIDs[0].String(), Change: eventsv1.VoiceRosterMembershipChanged_CHANGE_LEFT}}})
	require.NoError(t, err)
	_, err = js.Publish("voice.unrelated_fixture", []byte("nonmatching retained prefix"))
	require.NoError(t, err)
	ack, err := js.Publish(Subject, raw)
	require.NoError(t, err)
	require.EqualValues(t, 2, ack.Sequence)
	_, err = js.Publish("voice.unrelated_fixture", []byte("nonmatching retained suffix"))
	require.NoError(t, err)
	initial, err := js.AddConsumer(Stream, &nats.ConsumerConfig{Durable: Durable, FilterSubject: Subject, DeliverPolicy: nats.DeliverAllPolicy, AckPolicy: nats.AckExplicitPolicy, AckWait: 3 * time.Second})
	require.NoError(t, err)
	require.EqualValues(t, 1, initial.NumPending, "original cut-derived matching M=1, not stream length3")
	require.EqualValues(t, 0, initial.Delivered.Consumer)
	require.EqualValues(t, 0, initial.Delivered.Stream)
	require.EqualValues(t, 0, initial.AckFloor.Stream)
	_, err = pool.Exec(ctx, `ALTER TABLE matchmaking_voice_roster_cancellation_outbox ADD CONSTRAINT force_c4_rollback CHECK(false)`)
	require.NoError(t, err)
	lock, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = lock.Rollback(context.Background()) }()
	_, err = lock.Exec(ctx, `LOCK TABLE matchmaking_voice_roster_cancellation_outbox IN ACCESS EXCLUSIVE MODE`)
	require.NoError(t, err)
	inbox := &committedPGInbox{store: product, entered: make(chan struct{}), failed: make(chan struct{}), committed: make(chan struct{})}
	first, err := Start(url, inbox, nil)
	require.NoError(t, err)
	defer first.Close()
	runCtx, stopRun := context.WithCancel(ctx)
	defer stopRun()
	inbox.afterCommit = func() { first.nc.Close(); stopRun() }
	done := make(chan error, 1)
	go func() { done <- first.Run(runCtx) }()
	select {
	case <-inbox.entered:
	case <-ctx.Done():
		t.Fatal("consumer did not enter product inbox")
	}
	require.Eventually(t, func() bool {
		var blocked bool
		err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%INSERT INTO matchmaking_voice_roster_cancellation_outbox%')`).Scan(&blocked)
		return err == nil && blocked
	}, 10*time.Second, 20*time.Millisecond, "actual product transaction must reach the held SQL lock")
	info, err := js.ConsumerInfo(Stream, Durable)
	require.NoError(t, err)
	require.EqualValues(t, 0, info.AckFloor.Stream)
	require.EqualValues(t, 1, info.NumAckPending)
	var inboxRows int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_voice_roster_inbox`).Scan(&inboxRows))
	require.Zero(t, inboxRows, "uncommitted product INSERT is not visible to an independent PG connection")
	require.NoError(t, lock.Commit(ctx))
	select {
	case <-inbox.failed:
	case <-ctx.Done():
		t.Fatal("forced cancellation failure did not roll back")
	}
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_voice_roster_inbox`).Scan(&inboxRows))
	require.Zero(t, inboxRows)
	var searching int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM search_sessions WHERE party_id=$1 AND status='searching'`, party.Party.ID).Scan(&searching))
	require.Equal(t, 2, searching)
	info, err = js.ConsumerInfo(Stream, Durable)
	require.NoError(t, err)
	require.Zero(t, info.AckFloor.Stream, "failed real PG transaction must never ACK")
	_, err = pool.Exec(ctx, `ALTER TABLE matchmaking_voice_roster_cancellation_outbox DROP CONSTRAINT force_c4_rollback`)
	require.NoError(t, err)
	select {
	case <-inbox.committed:
	case <-ctx.Done():
		t.Fatal("retry did not commit the product inbox")
	}
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-ctx.Done():
		t.Fatal("closed-connection consumer did not stop")
	}
	info, err = js.ConsumerInfo(Stream, Durable)
	require.NoError(t, err)
	require.EqualValues(t, 0, info.AckFloor.Stream, "TCP closed after COMMIT, before product ACK")
	require.EqualValues(t, 1, info.NumAckPending)
	var cancellations int
	var generations []int64
	var hash []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_voice_roster_inbox`).Scan(&inboxRows))
	require.Equal(t, 1, inboxRows)
	require.NoError(t, pool.QueryRow(ctx, `SELECT payload_sha256 FROM matchmaking_voice_roster_inbox WHERE event_id=$1`, eventID).Scan(&hash))
	wantHash := sha256.Sum256(raw)
	require.Equal(t, wantHash[:], hash)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_voice_roster_cancellation_outbox`).Scan(&cancellations))
	require.Equal(t, 2, cancellations)
	require.NoError(t, pool.QueryRow(ctx, `SELECT array_agg(recovery_generation ORDER BY id) FROM search_sessions WHERE party_id=$1`, party.Party.ID).Scan(&generations))
	second, err := Start(url, product, nil)
	require.NoError(t, err)
	defer second.Close()
	restartCtx, stopRestart := context.WithCancel(ctx)
	defer stopRestart()
	restarted := make(chan error, 1)
	go func() { restarted <- second.Run(restartCtx) }()
	require.Eventually(t, func() bool {
		info, err := js.ConsumerInfo(Stream, Durable)
		return err == nil && info.AckFloor.Stream == ack.Sequence && info.NumAckPending == 0
	}, 10*time.Second, 20*time.Millisecond)
	final, err := js.ConsumerInfo(Stream, Durable)
	require.NoError(t, err)
	require.Equal(t, initial.Created, final.Created, "restart binds existing durable without CREATE/reset")
	require.Equal(t, initial.Config, final.Config)
	require.GreaterOrEqual(t, final.Delivered.Consumer, uint64(3), "failed transaction, committed/lostACK and committed inbox replay")
	var finalGenerations []int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT array_agg(recovery_generation ORDER BY id) FROM search_sessions WHERE party_id=$1`, party.Party.ID).Scan(&finalGenerations))
	require.Equal(t, generations, finalGenerations, "redelivery cannot reset the party again")
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_voice_roster_inbox`).Scan(&inboxRows))
	require.Equal(t, 1, inboxRows)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_voice_roster_cancellation_outbox`).Scan(&cancellations))
	require.Equal(t, 2, cancellations)
	stopRestart()
	select {
	case err := <-restarted:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(3 * time.Second):
		t.Fatal("restarted consumer did not stop")
	}
	t.Logf("C4/C5 pinned2.12.12 F1/L3/M1 targetS2: realPG rollback noACK; commit/TCP-loss retained; redelivery exact inbox/outbox/generation; no installed-rights claim")
}
