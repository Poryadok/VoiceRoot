package grpcsvc

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type stubFriendChecker struct {
	ok  bool
	err error
}

func (s stubFriendChecker) AreFriends(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return s.ok, s.err
}

func (s stubFriendChecker) AreFriendsOfFriends(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

type stubContactChecker struct {
	ok  bool
	err error
}

func (s stubContactChecker) HasContact(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return s.ok, s.err
}

func TestRecipientInboxBucket_strangerRequests(t *testing.T) {
	caller := uuid.New()
	recipient := uuid.New()
	got, err := recipientInboxBucket(context.Background(), caller, recipient, nil, nil)
	require.NoError(t, err)
	require.Equal(t, inboxRequests, got)
}

func TestRecipientInboxBucket_friendMain(t *testing.T) {
	caller := uuid.New()
	recipient := uuid.New()
	got, err := recipientInboxBucket(context.Background(), caller, recipient, stubFriendChecker{ok: true}, nil)
	require.NoError(t, err)
	require.Equal(t, inboxMain, got)
}

func TestRecipientInboxBucket_contactMain(t *testing.T) {
	caller := uuid.New()
	recipient := uuid.New()
	got, err := recipientInboxBucket(context.Background(), caller, recipient, stubFriendChecker{ok: false}, stubContactChecker{ok: true})
	require.NoError(t, err)
	require.Equal(t, inboxMain, got)
}

func TestRecipientInboxBucket_friendCheckerError(t *testing.T) {
	_, err := recipientInboxBucket(context.Background(), uuid.New(), uuid.New(), stubFriendChecker{err: errors.New("boom")}, nil)
	require.Error(t, err)
}
