package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestManagedChatRetentionIsImmutableAndCutsOffHistoryAtDBDeadline(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	chatStore := &DMStore{Pool: pool}
	applicationID, environmentID, profileID := uuid.New(), uuid.New(), uuid.New()
	created, err := chatStore.ProvisionManagedChat(ctx, ManagedChatCreate{
		ApplicationID: applicationID, EnvironmentID: environmentID,
		OperationID: uuid.New(), ExternalKey: "match:retention-boundary",
		RequestHash: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Name:        "Retention boundary",
	})
	require.NoError(t, err)
	_, err = chatStore.SyncManagedChatMembers(ctx, ManagedChatMemberSync{
		ApplicationID: applicationID, EnvironmentID: environmentID,
		OperationID: uuid.New(), ChatID: created.ChatID,
		RequestHash: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		ProfileIDs:  []uuid.UUID{profileID},
	})
	require.NoError(t, err)
	var joinedAt time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT joined_at FROM managed_chat_member_intervals WHERE chat_id=$1 AND profile_id=$2 AND revoked_at IS NULL`, created.ChatID, profileID).Scan(&joinedAt))

	request := ManagedChatRetention{
		ApplicationID: applicationID, EnvironmentID: environmentID,
		OperationID: uuid.New(), ExternalKey: "match:retention-boundary",
		RequestHash: "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		PurgeAfter:  time.Now().UTC().Add(300 * time.Millisecond),
	}
	first, err := chatStore.SetManagedChatRetention(ctx, request)
	require.NoError(t, err)
	require.Equal(t, created.ChatID, first.ChatID)
	require.Equal(t, request.OperationID, first.ReceiptID)
	require.False(t, first.Replayed)

	allowed, err := chatStore.ManagedChatMessageEntitled(ctx, created.ChatID, profileID, joinedAt)
	require.NoError(t, err)
	require.True(t, allowed, "history remains readable before the absolute cutoff")

	replayed, err := chatStore.SetManagedChatRetention(ctx, request)
	require.NoError(t, err)
	require.True(t, replayed.Replayed)
	require.Equal(t, first.PurgeAfter, replayed.PurgeAfter)
	require.Equal(t, first.ReceiptID, replayed.ReceiptID)

	changed := request
	changed.OperationID = uuid.New()
	changed.RequestHash = "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	changed.PurgeAfter = request.PurgeAfter.Add(time.Hour)
	_, err = chatStore.SetManagedChatRetention(ctx, changed)
	require.ErrorIs(t, err, ErrManagedResourceConflict, "a second operation cannot move a managed chat's purge cutoff")

	time.Sleep(time.Until(request.PurgeAfter) + 20*time.Millisecond)
	allowed, err = chatStore.ManagedChatMessageEntitled(ctx, created.ChatID, profileID, joinedAt)
	require.NoError(t, err)
	require.False(t, allowed, "the DB-clock cutoff denies history regardless of the message's creation time")
}
