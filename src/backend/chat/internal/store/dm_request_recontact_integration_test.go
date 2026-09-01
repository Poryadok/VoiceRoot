package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestPromoteDeclinedDMRecipients_recontact documents text-chat.md §«Запросы сообщений» re-contact after decline.
func TestPromoteDeclinedDMRecipients_recontact(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	s := &DMStore{Pool: pool}

	sender := uuid.New()
	recipient := uuid.New()
	dm, _, err := s.EnsureDM(ctx, sender, recipient, InboxRequests)
	require.NoError(t, err)

	require.NoError(t, s.SetInboxBucket(ctx, dm.ID, recipient, "declined"))

	requestsPage, err := s.ListChatsPage(ctx, recipient, "", 10, InboxRequests, nil)
	require.NoError(t, err)
	require.Empty(t, requestsPage.Rows)

	require.NoError(t, s.PromoteDeclinedDMRecipients(ctx, dm.ID, sender))

	requestsPage, err = s.ListChatsPage(ctx, recipient, "", 10, InboxRequests, nil)
	require.NoError(t, err)
	require.Len(t, requestsPage.Rows, 1)
	require.Equal(t, dm.ID, requestsPage.Rows[0].ID)
}
