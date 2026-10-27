package main

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type catalogPurgerFixture struct {
	account, event uuid.UUID
	calls          int
	err            error
}

func (f *catalogPurgerFixture) PurgeAccount(_ context.Context, account, event uuid.UUID) error {
	f.calls++
	f.account = account
	f.event = event
	return f.err
}

func TestAccountDeletionClosesCatalogBeforeUnavailableUser(t *testing.T) {
	profiles := &deletedAccountProfileListerForTest{err: errors.New("User unavailable"), failRemain: -1}
	purger := &catalogPurgerFixture{}
	c := newAccountDeletedDMConsumer(profiles, &deletedDMTargetStoreForTest{}, &dmPeerDeletedPublisherForTest{}, slog.Default())
	c.catalog = purger
	account, event := uuid.New(), uuid.New()
	err := c.handleUserAccountDeleted(context.Background(), accountDeletedMessageForTest(t, event.String(), account.String()))
	require.Error(t, err, "User outage still prevents ACK of remaining fanout")
	require.Equal(t, 1, purger.calls)
	require.Equal(t, account, purger.account)
	require.Equal(t, event, purger.event)
	previousLookups := len(profiles.calls)
	purger.err = errors.New("catalog transaction unavailable")
	profiles.err = nil
	err = c.handleUserAccountDeleted(context.Background(), accountDeletedMessageForTest(t, event.String(), account.String()))
	require.ErrorContains(t, err, "catalog")
	require.Len(t, profiles.calls, previousLookups, "failed local barrier must stop downstream fanout")
}
