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

	"github.com/google/uuid"
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
}

func (f *deletedAccountProfileListerForTest) ListProfileIDsForAccount(_ context.Context, accountID string) ([]uuid.UUID, error) {
	f.calls = append(f.calls, accountID)
	if f.err != nil {
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
	calls  []dmPeerDeletedPublishCallForTest
	failAt int
}

func (f *dmPeerDeletedPublisherForTest) PublishDMPeerDeleted(_ context.Context, eventID string, chatID, recipientProfileID uuid.UUID) error {
	call := dmPeerDeletedPublishCallForTest{
		EventID: eventID, ChatID: chatID, RecipientProfileID: recipientProfileID,
	}
	f.calls = append(f.calls, call)
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

func TestAccountDeletedConsumer_ResolvesAllProfilesAndPublishesOnlySurvivors(t *testing.T) {
	accountID := uuid.NewString()
	deletedA, deletedB := uuid.New(), uuid.New()
	chatA, chatB := uuid.New(), uuid.New()
	survivorA, survivorB := uuid.New(), uuid.New()
	profiles := &deletedAccountProfileListerForTest{profileIDs: []uuid.UUID{deletedA, deletedB}}
	targets := &deletedDMTargetStoreForTest{targets: []store.DMPeerDeletionTarget{
		{ChatID: chatA, SurvivingProfileID: survivorA},
		{ChatID: chatB, SurvivingProfileID: survivorB},
	}}
	publisher := &dmPeerDeletedPublisherForTest{}
	consumer := newAccountDeletedConsumerForTest(t, profiles, targets, publisher)

	err := consumer.handleUserAccountDeleted(context.Background(), accountDeletedMessageForTest(t, "source-delete-1", accountID))
	require.NoError(t, err)
	require.Equal(t, []string{accountID}, profiles.calls)
	require.Equal(t, [][]uuid.UUID{{deletedA, deletedB}}, targets.calls)
	require.Len(t, publisher.calls, 2)
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
	first := accountDeletedMessageForTest(t, "source-delete-redelivery", accountID)
	second := accountDeletedMessageForTest(t, "source-delete-redelivery", accountID)

	require.NoError(t, consumer.handleUserAccountDeleted(context.Background(), first))
	require.NoError(t, consumer.handleUserAccountDeleted(context.Background(), second))
	require.Len(t, publisher.calls, 2)
	require.Equal(t, publisher.calls[0].EventID, publisher.calls[1].EventID,
		"target event identity/Nats-Msg-Id must be stable across source redelivery")
}

func TestAccountDeletedConsumer_ZeroTargetsDoesNotPublish(t *testing.T) {
	profiles := &deletedAccountProfileListerForTest{profileIDs: []uuid.UUID{uuid.New()}}
	targets := &deletedDMTargetStoreForTest{}
	publisher := &dmPeerDeletedPublisherForTest{}
	consumer := newAccountDeletedConsumerForTest(t, profiles, targets, publisher)

	err := consumer.handleUserAccountDeleted(context.Background(), accountDeletedMessageForTest(t, "source-delete-empty", uuid.NewString()))
	require.NoError(t, err, "zero fanout targets is a successful Ack path")
	require.Empty(t, publisher.calls)
}

func TestAccountDeletedConsumer_ProfileLookupFailureRequestsRetry(t *testing.T) {
	profiles := &deletedAccountProfileListerForTest{err: errors.New("user unavailable")}
	targets := &deletedDMTargetStoreForTest{}
	publisher := &dmPeerDeletedPublisherForTest{}
	consumer := newAccountDeletedConsumerForTest(t, profiles, targets, publisher)

	err := consumer.handleUserAccountDeleted(context.Background(), accountDeletedMessageForTest(t, "source-delete-user-failure", uuid.NewString()))
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

	err := consumer.handleUserAccountDeleted(context.Background(), accountDeletedMessageForTest(t, "source-delete-publish-failure", uuid.NewString()))
	require.Error(t, err)
	require.Len(t, publisher.calls, 2)
	// The adapter must not Ack this message: its returned error is the NAK path.
	if err == nil {
		t.Fatal(fmt.Errorf("publish failure unexpectedly acknowledged"))
	}
}
