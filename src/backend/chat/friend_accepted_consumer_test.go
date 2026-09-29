package main

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
)

type friendDMStoreStub struct {
	a, b  uuid.UUID
	calls int
	err   error
}

type acceptedFriendStub struct {
	accepted bool
	err      error
}

func (s acceptedFriendStub) AreFriends(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return s.accepted, s.err
}

func (s *friendDMStoreStub) PromoteFriendDMRequests(_ context.Context, a, b uuid.UUID) error {
	s.a, s.b = a, b
	s.calls++
	return s.err
}

func friendAcceptedMessage(t *testing.T, a, b uuid.UUID) *nats.Msg {
	t.Helper()
	data, err := proto.Marshal(&eventsv1.SocialStreamEvent{
		Payload: &eventsv1.SocialStreamEvent_FriendAdded{FriendAdded: &eventsv1.FriendAdded{
			RequesterProfileId: a.String(), TargetProfileId: b.String(),
		}},
	})
	require.NoError(t, err)
	return &nats.Msg{Subject: friendAcceptedSubject, Data: data}
}

func TestFriendAcceptedPromotesExistingDMRequests(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	store := &friendDMStoreStub{}
	require.NoError(t, handleFriendAccepted(context.Background(), store, acceptedFriendStub{accepted: true}, friendAcceptedMessage(t, a, b), nil))
	require.Equal(t, 1, store.calls)
	require.Equal(t, a, store.a)
	require.Equal(t, b, store.b)
}

func TestFriendAcceptedRetriesStoreFailureAndIgnoresInvalidPayload(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	store := &friendDMStoreStub{err: errors.New("database unavailable")}
	require.Error(t, handleFriendAccepted(context.Background(), store, acceptedFriendStub{accepted: true}, friendAcceptedMessage(t, a, b), nil))
	require.Equal(t, 1, store.calls)
	require.NoError(t, handleFriendAccepted(context.Background(), store, acceptedFriendStub{accepted: true}, &nats.Msg{Subject: friendAcceptedSubject, Data: []byte("invalid")}, nil))
	require.Equal(t, 1, store.calls)
}

func TestFriendAcceptedReplayAfterUnfriendKeepsRequest(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	store := &friendDMStoreStub{}
	require.NoError(t, handleFriendAccepted(context.Background(), store, acceptedFriendStub{}, friendAcceptedMessage(t, a, b), nil))
	require.Zero(t, store.calls)
	require.Error(t, handleFriendAccepted(context.Background(), store, acceptedFriendStub{err: errors.New("social unavailable")}, friendAcceptedMessage(t, a, b), nil))
	require.Zero(t, store.calls)
}
