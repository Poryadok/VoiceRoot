package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func readLastMessageAt(t *testing.T, ctx context.Context, s *DMStore, chatID uuid.UUID) *time.Time {
	t.Helper()
	var at *time.Time
	err := s.Pool.QueryRow(ctx, `SELECT last_message_at FROM chats WHERE id = $1`, chatID).Scan(&at)
	require.NoError(t, err)
	return at
}

// TestTouchLastMessageAt_UpdatesDM documents chat-service.md timestamp ownership for DMs
// (message.sent → TouchLastMessageAt already shipped for type=dm).
func TestTouchLastMessageAt_UpdatesDM(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	s := &DMStore{Pool: pool}

	a, b := uuid.New(), uuid.New()
	row, _, err := s.EnsureDM(ctx, a, b)
	require.NoError(t, err)
	require.Nil(t, readLastMessageAt(t, ctx, s, row.ID))

	at := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	require.NoError(t, s.TouchLastMessageAt(ctx, row.ID, at))

	got := readLastMessageAt(t, ctx, s, row.ID)
	require.NotNil(t, got)
	require.True(t, got.UTC().Equal(at), "got %v want %v", got, at)
}

// TestTouchLastMessageAt_UpdatesGroup documents chat-service.md / messaging-service.md:
// Chat owns chats.last_message_at for groups on message.sent (not DM-only).
func TestTouchLastMessageAt_UpdatesGroup(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	s := &DMStore{Pool: pool}

	owner := uuid.New()
	row, err := s.CreateGroupChat(ctx, owner, "Activity group")
	require.NoError(t, err)
	require.Equal(t, "group", row.Type)
	require.Nil(t, readLastMessageAt(t, ctx, s, row.ID))

	at := time.Date(2026, 8, 29, 13, 0, 0, 0, time.UTC)
	require.NoError(t, s.TouchLastMessageAt(ctx, row.ID, at))

	got := readLastMessageAt(t, ctx, s, row.ID)
	require.NotNil(t, got, "group last_message_at must update from message activity")
	require.True(t, got.UTC().Equal(at), "got %v want %v", got, at)
}

// TestTouchLastMessageAt_UpdatesChannel documents chat-service.md target:
// channel rows also get last_message_at from message.sent activity.
func TestTouchLastMessageAt_UpdatesChannel(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	s := &DMStore{Pool: pool}

	creator := uuid.New()
	spaceID := uuid.New()
	row, err := s.CreateSpaceChannelChat(ctx, creator, spaceID, "general")
	require.NoError(t, err)
	require.Equal(t, "channel", row.Type)
	require.Nil(t, readLastMessageAt(t, ctx, s, row.ID))

	at := time.Date(2026, 8, 29, 14, 0, 0, 0, time.UTC)
	require.NoError(t, s.TouchLastMessageAt(ctx, row.ID, at))

	got := readLastMessageAt(t, ctx, s, row.ID)
	require.NotNil(t, got, "channel last_message_at must update from message activity")
	require.True(t, got.UTC().Equal(at), "got %v want %v", got, at)
}

// TestTouchLastMessageAt_Monotonic documents activity timestamp never moves backward.
func TestTouchLastMessageAt_Monotonic(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	s := &DMStore{Pool: pool}

	owner := uuid.New()
	row, err := s.CreateGroupChat(ctx, owner, "Monotonic")
	require.NoError(t, err)

	newer := time.Date(2026, 8, 29, 16, 0, 0, 0, time.UTC)
	older := time.Date(2026, 8, 29, 15, 0, 0, 0, time.UTC)
	require.NoError(t, s.TouchLastMessageAt(ctx, row.ID, newer))
	require.NoError(t, s.TouchLastMessageAt(ctx, row.ID, older))

	got := readLastMessageAt(t, ctx, s, row.ID)
	require.NotNil(t, got)
	require.True(t, got.UTC().Equal(newer), "older touch must not downgrade last_message_at")
}
