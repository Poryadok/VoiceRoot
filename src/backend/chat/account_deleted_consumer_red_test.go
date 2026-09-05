package main

// This file is intentionally RED.  It specifies the Chat-side seam for the
// typed user.account_deleted hop without selecting an Auth publisher or a
// Realtime implementation.  The P0 ChatStreamEvent tag-19 generated type is
// not present on this worktree yet; the consumer contract therefore uses the
// existing store result and a publisher seam rather than naming that missing
// generated type.

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/chat/internal/store"
)

type deletedAccountProfileListerForTest struct {
	calls      []string
	profileIDs []uuid.UUID
	err        error
	failRemain int // -1 means every lookup fails; positive values are transient failures.
	callSignal chan struct{}
}

func (f *deletedAccountProfileListerForTest) ListProfileIDsForAccount(_ context.Context, accountID string) ([]uuid.UUID, error) {
	f.calls = append(f.calls, accountID)
	if f.callSignal != nil {
		f.callSignal <- struct{}{}
	}
	if f.err != nil && f.failRemain != 0 {
		if f.failRemain > 0 {
			f.failRemain--
		}
		return nil, f.err
	}
	return append([]uuid.UUID(nil), f.profileIDs...), nil
}

type deletedDMTargetStoreForTest struct {
	calls   [][]uuid.UUID
	targets []store.DMPeerDeletionTarget
	err     error
}

func (f *deletedDMTargetStoreForTest) ListDMPeerDeletionTargets(_ context.Context, profileIDs []uuid.UUID) ([]store.DMPeerDeletionTarget, error) {
	f.calls = append(f.calls, append([]uuid.UUID(nil), profileIDs...))
	if f.err != nil {
		return nil, f.err
	}
	return append([]store.DMPeerDeletionTarget(nil), f.targets...), nil
}

type dmPeerDeletedPublishCallForTest struct {
	EventID            string
	ChatID             uuid.UUID
	RecipientProfileID uuid.UUID
}

type dmPeerDeletedPublisherForTest struct {
	calls      []dmPeerDeletedPublishCallForTest
	failAt     int
	callSignal chan struct{}
	block      chan struct{}
}

func (f *dmPeerDeletedPublisherForTest) PublishDMPeerDeleted(_ context.Context, eventID string, chatID, recipientProfileID uuid.UUID) error {
	call := dmPeerDeletedPublishCallForTest{
		EventID: eventID, ChatID: chatID, RecipientProfileID: recipientProfileID,
	}
	f.calls = append(f.calls, call)
	if f.callSignal != nil {
		f.callSignal <- struct{}{}
	}
	if f.block != nil {
		<-f.block
	}
	if f.failAt > 0 && len(f.calls) == f.failAt {
		return errors.New("publisher unavailable")
	}
	return nil
}

// accountDeletedDMConsumerForTest is the smallest handler boundary needed by
// the JetStream adapter.  A nil error means the adapter may Ack; an error means
// it must Nak/retry.  The handler itself must Ack only after every publish has
// completed, so tests can use an actual JetStream message without sleeps.
type accountDeletedDMConsumerForTest interface {
	handleUserAccountDeleted(context.Context, *nats.Msg) error
}

func newAccountDeletedConsumerForTest(
	t *testing.T,
	profiles *deletedAccountProfileListerForTest,
	targets *deletedDMTargetStoreForTest,
	publisher *dmPeerDeletedPublisherForTest,
) accountDeletedDMConsumerForTest {
	t.Helper()
	// The production constructor is deliberately referenced by name here: the
	// compile failure is the RED signal until the Chat consumer seam exists.
	return newAccountDeletedDMConsumer(profiles, targets, publisher, slog.Default())
}

func accountDeletedMessageForTest(t *testing.T, eventID, accountID string) *nats.Msg {
	t.Helper()
	b, err := proto.Marshal(&eventsv1.UserStreamEvent{
		EventId: eventID,
		Payload: &eventsv1.UserStreamEvent_UserAccountDeleted{
			UserAccountDeleted: &eventsv1.UserAccountDeleted{AccountId: accountID},
		},
	})
	require.NoError(t, err)
	return &nats.Msg{Data: b}
}

func accountDeletedEventIDForTest(label string) string {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte("voice-chat-account-deleted-test:"+label)).String()
}

func TestAccountDeletedConsumer_ResolvesAllProfilesAndPublishesOnlySurvivors(t *testing.T) {
	accountID := uuid.NewString()
	deletedA, deletedB := uuid.New(), uuid.New()
	chatA := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	chatB := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	survivorA := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	survivorB := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	profiles := &deletedAccountProfileListerForTest{profileIDs: []uuid.UUID{deletedA, deletedB}}
	targets := &deletedDMTargetStoreForTest{targets: []store.DMPeerDeletionTarget{
		{ChatID: chatA, SurvivingProfileID: survivorA},
		{ChatID: chatB, SurvivingProfileID: survivorB},
	}}
	publisher := &dmPeerDeletedPublisherForTest{}
	consumer := newAccountDeletedConsumerForTest(t, profiles, targets, publisher)

	err := consumer.handleUserAccountDeleted(context.Background(), accountDeletedMessageForTest(t, accountDeletedEventIDForTest("source-delete-1"), accountID))
	require.NoError(t, err)
	require.Equal(t, []string{accountID}, profiles.calls)
	require.Equal(t, [][]uuid.UUID{{deletedA, deletedB}}, targets.calls)
	require.Len(t, publisher.calls, 2)
	require.Equal(t, []dmPeerDeletedPublishCallForTest{
		{EventID: publisher.calls[0].EventID, ChatID: chatA, RecipientProfileID: survivorA},
		{EventID: publisher.calls[1].EventID, ChatID: chatB, RecipientProfileID: survivorB},
	}, publisher.calls, "fanout must preserve canonical target order")
	require.NotEqual(t, publisher.calls[0].EventID, publisher.calls[1].EventID,
		"each target needs a distinct durable event identity")
	for _, call := range publisher.calls {
		require.NotEmpty(t, call.EventID)
		require.NotContains(t, []uuid.UUID{deletedA, deletedB}, call.RecipientProfileID)
	}
}

func TestAccountDeletedConsumer_RedeliveryReusesStableTargetIdentity(t *testing.T) {
	accountID := uuid.NewString()
	deleted, survivor, chat := uuid.New(), uuid.New(), uuid.New()
	profiles := &deletedAccountProfileListerForTest{profileIDs: []uuid.UUID{deleted}}
	targets := &deletedDMTargetStoreForTest{targets: []store.DMPeerDeletionTarget{{ChatID: chat, SurvivingProfileID: survivor}}}
	publisher := &dmPeerDeletedPublisherForTest{}
	consumer := newAccountDeletedConsumerForTest(t, profiles, targets, publisher)
	first := accountDeletedMessageForTest(t, accountDeletedEventIDForTest("source-delete-redelivery"), accountID)
	second := accountDeletedMessageForTest(t, accountDeletedEventIDForTest("source-delete-redelivery"), accountID)

	require.NoError(t, consumer.handleUserAccountDeleted(context.Background(), first))
	require.NoError(t, consumer.handleUserAccountDeleted(context.Background(), second))
	require.Len(t, publisher.calls, 2)
	require.Equal(t, publisher.calls[0].EventID, publisher.calls[1].EventID,
		"target event identity/Nats-Msg-Id must be stable across source redelivery")
	require.Equal(t, publisher.calls[0].ChatID, publisher.calls[1].ChatID)
	require.Equal(t, publisher.calls[0].RecipientProfileID, publisher.calls[1].RecipientProfileID)
}

func TestAccountDeletedConsumer_ChildEventIDIsVersionedDeterministicAndDistinct(t *testing.T) {
	sourceID := uuid.MustParse("10000000-0000-0000-0000-000000000001")
	chatID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	survivorID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")

	got := dmPeerDeletedChildEventID(sourceID, chatID, survivorID)
	require.Equal(t, "f3c502d2-b56f-5eea-b132-27af463b00a6", got)
	require.Equal(t, got, dmPeerDeletedChildEventID(sourceID, chatID, survivorID))
	require.NotEqual(t, got, dmPeerDeletedChildEventID(sourceID, uuid.New(), survivorID))
	require.NotEqual(t, got, dmPeerDeletedChildEventID(uuid.New(), chatID, survivorID))
	require.NotEqual(t, got, dmPeerDeletedChildEventID(sourceID, chatID, uuid.New()))
}

func TestAccountDeletedConsumer_RejectsNonUUIDSourceIdentity(t *testing.T) {
	profiles := &deletedAccountProfileListerForTest{profileIDs: []uuid.UUID{uuid.New()}}
	targets := &deletedDMTargetStoreForTest{}
	publisher := &dmPeerDeletedPublisherForTest{}
	consumer := newAccountDeletedConsumerForTest(t, profiles, targets, publisher)

	for name, tc := range map[string]struct {
		eventID   string
		accountID string
	}{
		"event id":    {eventID: "not-a-uuid", accountID: uuid.NewString()},
		"account id":  {eventID: uuid.NewString(), accountID: "not-a-uuid"},
		"nil event":   {eventID: uuid.Nil.String(), accountID: uuid.NewString()},
		"nil account": {eventID: uuid.NewString(), accountID: uuid.Nil.String()},
	} {
		t.Run(name, func(t *testing.T) {
			err := consumer.handleUserAccountDeleted(context.Background(), accountDeletedMessageForTest(t, tc.eventID, tc.accountID))
			require.Error(t, err)
			require.Empty(t, profiles.calls)
			require.Empty(t, targets.calls)
			require.Empty(t, publisher.calls)
		})
	}
}

func startAccountDeletedJSTestServer(t *testing.T) *server.Server {
	t.Helper()
	ns, err := server.NewServer(&server.Options{
		Host: "127.0.0.1", Port: -1, NoLog: true, NoSigs: true,
		JetStream: true, StoreDir: t.TempDir(),
	})
	require.NoError(t, err)
	go ns.Start()
	require.True(t, ns.ReadyForConnections(5*time.Second))
	t.Cleanup(ns.Shutdown)
	return ns
}

func waitForAckFloorForTest(t *testing.T, sub *nats.Subscription) {
	t.Helper()
	deadline := time.NewTimer(3 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	t.Cleanup(func() { deadline.Stop() })
	t.Cleanup(ticker.Stop)
	for {
		info, err := sub.ConsumerInfo()
		if err == nil && info.NumAckPending == 0 {
			return
		}
		select {
		case <-deadline.C:
			require.FailNow(t, "JetStream message was not Acked after all publishes completed", err)
		case <-ticker.C:
		}
	}
}

func subscribeAccountDeletedManualAckForTest(
	t *testing.T,
	js nats.JetStreamContext,
	profiles *deletedAccountProfileListerForTest,
	targets *deletedDMTargetStoreForTest,
	publisher *dmPeerDeletedPublisherForTest,
	instanceID string,
) *nats.Subscription {
	t.Helper()
	// The RED seam requires an explicit ManualAck adapter.  Its callback must
	// Ack only after handleUserAccountDeleted returns nil and Nak every error.
	sub, err := subscribeAccountDeletedConsumer(js, profiles, targets, publisher, instanceID, slog.Default())
	require.NoError(t, err)
	info, err := sub.ConsumerInfo()
	require.NoError(t, err)
	require.Equal(t, nats.AckExplicitPolicy, info.Config.AckPolicy,
		"account deletion consumer must use explicit/manual acknowledgements")
	t.Cleanup(func() { _ = sub.Unsubscribe() })
	return sub
}

func TestAccountDeletedConsumer_JetStreamManualAckAfterAllPublishes(t *testing.T) {
	server := startAccountDeletedJSTestServer(t)
	nc, err := nats.Connect(server.ClientURL())
	require.NoError(t, err)
	t.Cleanup(func() { _ = nc.Drain() })
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: "user_events", Subjects: []string{"user.account_deleted"}, Storage: nats.MemoryStorage})
	require.NoError(t, err)

	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	profiles := &deletedAccountProfileListerForTest{profileIDs: []uuid.UUID{uuid.New()}}
	targets := &deletedDMTargetStoreForTest{targets: []store.DMPeerDeletionTarget{
		{ChatID: uuid.New(), SurvivingProfileID: uuid.New()},
		{ChatID: uuid.New(), SurvivingProfileID: uuid.New()},
	}}
	publisher := &dmPeerDeletedPublisherForTest{callSignal: entered, block: release}
	sub := subscribeAccountDeletedManualAckForTest(t, js, profiles, targets, publisher, "manual-ack-success")

	_, err = js.Publish("user.account_deleted", accountDeletedMessageForTest(t, accountDeletedEventIDForTest("source-manual-ack"), uuid.NewString()).Data)
	require.NoError(t, err)
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("consumer did not begin first publish")
	}
	info, err := sub.ConsumerInfo()
	require.NoError(t, err)
	require.Equal(t, 1, info.NumAckPending, "message must remain pending while a publish is blocked")
	close(release)
	for range targets.targets[1:] {
		select {
		case <-entered:
		case <-time.After(3 * time.Second):
			t.Fatal("consumer did not complete every target publish")
		}
	}
	waitForAckFloorForTest(t, sub)
}

func TestAccountDeletedConsumer_JetStreamManualAckZeroTargets(t *testing.T) {
	server := startAccountDeletedJSTestServer(t)
	nc, err := nats.Connect(server.ClientURL())
	require.NoError(t, err)
	t.Cleanup(func() { _ = nc.Drain() })
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: "user_events", Subjects: []string{"user.account_deleted"}, Storage: nats.MemoryStorage})
	require.NoError(t, err)
	profiles := &deletedAccountProfileListerForTest{profileIDs: []uuid.UUID{uuid.New()}, callSignal: make(chan struct{}, 1)}
	targets := &deletedDMTargetStoreForTest{}
	publisher := &dmPeerDeletedPublisherForTest{}
	sub := subscribeAccountDeletedManualAckForTest(t, js, profiles, targets, publisher, "manual-ack-empty")
	_, err = js.Publish("user.account_deleted", accountDeletedMessageForTest(t, accountDeletedEventIDForTest("source-manual-empty"), uuid.NewString()).Data)
	require.NoError(t, err)
	select {
	case <-profiles.callSignal:
	case <-time.After(3 * time.Second):
		t.Fatal("zero-target event was not handled")
	}
	waitForAckFloorForTest(t, sub)
	require.Empty(t, publisher.calls)
}

func TestAccountDeletedConsumer_JetStreamNakOnLookupOrPublishFailure(t *testing.T) {
	server := startAccountDeletedJSTestServer(t)
	nc, err := nats.Connect(server.ClientURL())
	require.NoError(t, err)
	t.Cleanup(func() { _ = nc.Drain() })
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: "user_events", Subjects: []string{"user.account_deleted"}, Storage: nats.MemoryStorage})
	require.NoError(t, err)

	t.Run("lookup failure", func(t *testing.T) {
		profiles := &deletedAccountProfileListerForTest{
			err: errors.New("user unavailable"), failRemain: 1, callSignal: make(chan struct{}, 1),
		}
		targets := &deletedDMTargetStoreForTest{}
		publisher := &dmPeerDeletedPublisherForTest{}
		sub := subscribeAccountDeletedManualAckForTest(t, js, profiles, targets, publisher, "manual-nak-lookup")
		_, err = js.Publish("user.account_deleted", accountDeletedMessageForTest(t, accountDeletedEventIDForTest("source-manual-lookup-failure"), uuid.NewString()).Data)
		require.NoError(t, err)
		select {
		case <-profiles.callSignal:
		case <-time.After(3 * time.Second):
			t.Fatal("lookup failure was not delivered")
		}
		select {
		case <-profiles.callSignal:
		case <-time.After(3 * time.Second):
			t.Fatal("lookup failure was not redelivered after Nak")
		}
		waitForAckFloorForTest(t, sub)
		require.NoError(t, sub.Drain())
		require.GreaterOrEqual(t, len(profiles.calls), 2,
			"lookup failure must trigger JetStream redelivery, not auto-ack")
	})

	t.Run("publish failure", func(t *testing.T) {
		profiles := &deletedAccountProfileListerForTest{profileIDs: []uuid.UUID{uuid.New()}}
		targets := &deletedDMTargetStoreForTest{targets: []store.DMPeerDeletionTarget{{ChatID: uuid.New(), SurvivingProfileID: uuid.New()}}}
		publisher := &dmPeerDeletedPublisherForTest{failAt: 1, callSignal: make(chan struct{}, 1)}
		sub := subscribeAccountDeletedManualAckForTest(t, js, profiles, targets, publisher, "manual-nak-publish")
		_, err = js.Publish("user.account_deleted", accountDeletedMessageForTest(t, accountDeletedEventIDForTest("source-manual-publish-failure"), uuid.NewString()).Data)
		require.NoError(t, err)
		select {
		case <-publisher.callSignal:
		case <-time.After(3 * time.Second):
			t.Fatal("publish failure was not delivered")
		}
		select {
		case <-publisher.callSignal:
		case <-time.After(3 * time.Second):
			t.Fatal("publish failure was not redelivered after Nak")
		}
		waitForAckFloorForTest(t, sub)
		require.NoError(t, sub.Drain())
		require.GreaterOrEqual(t, len(publisher.calls), 2,
			"publish failure must trigger JetStream redelivery, not auto-ack")
	})
}

func TestAccountDeletedConsumer_ZeroTargetsDoesNotPublish(t *testing.T) {
	profiles := &deletedAccountProfileListerForTest{profileIDs: []uuid.UUID{uuid.New()}}
	targets := &deletedDMTargetStoreForTest{}
	publisher := &dmPeerDeletedPublisherForTest{}
	consumer := newAccountDeletedConsumerForTest(t, profiles, targets, publisher)

	err := consumer.handleUserAccountDeleted(context.Background(), accountDeletedMessageForTest(t, accountDeletedEventIDForTest("source-delete-empty"), uuid.NewString()))
	require.NoError(t, err, "zero fanout targets is a successful Ack path")
	require.Empty(t, publisher.calls)
}

func TestAccountDeletedConsumer_ProfileLookupFailureRequestsRetry(t *testing.T) {
	profiles := &deletedAccountProfileListerForTest{err: errors.New("user unavailable"), failRemain: -1}
	targets := &deletedDMTargetStoreForTest{}
	publisher := &dmPeerDeletedPublisherForTest{}
	consumer := newAccountDeletedConsumerForTest(t, profiles, targets, publisher)

	err := consumer.handleUserAccountDeleted(context.Background(), accountDeletedMessageForTest(t, accountDeletedEventIDForTest("source-delete-user-failure"), uuid.NewString()))
	require.Error(t, err)
	require.Empty(t, targets.calls)
	require.Empty(t, publisher.calls)
}

func TestAccountDeletedConsumer_PublishFailureRequestsRetryAfterPartialFanout(t *testing.T) {
	profiles := &deletedAccountProfileListerForTest{profileIDs: []uuid.UUID{uuid.New()}}
	targets := &deletedDMTargetStoreForTest{targets: []store.DMPeerDeletionTarget{
		{ChatID: uuid.New(), SurvivingProfileID: uuid.New()},
		{ChatID: uuid.New(), SurvivingProfileID: uuid.New()},
	}}
	publisher := &dmPeerDeletedPublisherForTest{failAt: 2}
	consumer := newAccountDeletedConsumerForTest(t, profiles, targets, publisher)

	err := consumer.handleUserAccountDeleted(context.Background(), accountDeletedMessageForTest(t, accountDeletedEventIDForTest("source-delete-publish-failure"), uuid.NewString()))
	require.Error(t, err)
	require.Len(t, publisher.calls, 2)
	// The adapter must not Ack this message: its returned error is the NAK path.
	if err == nil {
		t.Fatal(fmt.Errorf("publish failure unexpectedly acknowledged"))
	}
}
