package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestPromoteFriendDMRequests_PreservesOtherRequestsAndDeclines(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	s := &DMStore{Pool: pool}
	a, b, stranger := uuid.New(), uuid.New(), uuid.New()
	dm, _, err := s.EnsureDM(ctx, a, b, InboxRequests)
	require.NoError(t, err)
	other, _, err := s.EnsureDM(ctx, stranger, b, InboxRequests)
	require.NoError(t, err)

	require.NoError(t, s.PromoteFriendDMRequests(ctx, a, b))
	require.NoError(t, s.PromoteFriendDMRequests(ctx, a, b), "redelivery must be idempotent")
	require.Equal(t, InboxMain, memberInboxBucket(t, ctx, pool, dm.ID, b))
	require.Equal(t, InboxRequests, memberInboxBucket(t, ctx, pool, other.ID, b))

	require.NoError(t, s.SetInboxBucket(ctx, dm.ID, b, "declined"))
	require.NoError(t, s.PromoteFriendDMRequests(ctx, a, b))
	require.Equal(t, "declined", memberInboxBucket(t, ctx, pool, dm.ID, b), "friendship must not undo an explicit DM decline")
}

func memberInboxBucket(t *testing.T, ctx context.Context, pool *pgxpool.Pool, chatID, profileID uuid.UUID) string {
	t.Helper()
	var bucket string
	require.NoError(t, pool.QueryRow(ctx, `SELECT inbox_bucket FROM chat_members WHERE chat_id = $1 AND profile_id = $2`, chatID, profileID).Scan(&bucket))
	return bucket
}
