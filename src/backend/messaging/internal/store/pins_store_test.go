package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"voice/backend/pkg/integrationtest"
)

func TestPinsStore_upsertListDelete(t *testing.T) {
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "db", "")
	applyPinsMigrations(t, ctx, pool)

	chatID := uuid.New()
	msgA := uuid.New()
	msgB := uuid.New()
	pinner := uuid.New()
	s := &PinsStore{Pool: pool}
	seedMessage(t, ctx, pool, chatID, msgA, pinner)
	seedMessage(t, ctx, pool, chatID, msgB, pinner)

	require.NoError(t, s.UpsertPin(ctx, chatID, msgA, pinner))
	require.NoError(t, s.UpsertPin(ctx, chatID, msgB, pinner))

	pins, err := s.ListPins(ctx, chatID)
	require.NoError(t, err)
	require.Len(t, pins, 2)

	set, err := s.PinnedSetForMessageIDs(ctx, chatID, []uuid.UUID{msgA, msgB})
	require.NoError(t, err)
	require.True(t, set[msgA])
	require.True(t, set[msgB])

	require.NoError(t, s.DeletePin(ctx, chatID, msgA))
	pins, err = s.ListPins(ctx, chatID)
	require.NoError(t, err)
	require.Len(t, pins, 1)
}

func TestPinsStore_limit50(t *testing.T) {
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "db", "")
	applyPinsMigrations(t, ctx, pool)

	chatID := uuid.New()
	pinner := uuid.New()
	s := &PinsStore{Pool: pool}

	for range MaxPinsPerChat {
		msgID := uuid.New()
		seedMessage(t, ctx, pool, chatID, msgID, pinner)
		require.NoError(t, s.UpsertPin(ctx, chatID, msgID, pinner))
	}
	extra := uuid.New()
	seedMessage(t, ctx, pool, chatID, extra, pinner)
	err := s.UpsertPin(ctx, chatID, extra, pinner)
	require.ErrorIs(t, err, ErrPinLimitReached)
}

func seedMessage(t *testing.T, ctx context.Context, pool *pgxpool.Pool, chatID, msgID, sender uuid.UUID) {
	t.Helper()
	_, err := pool.Exec(ctx, `
INSERT INTO messages (id, chat_id, chat_type, sender_profile_id, content, attachments, mentions)
VALUES ($1, $2, 'dm', $3, 'pin store test', '[]'::jsonb, '[]'::jsonb)
`, msgID, chatID, sender)
	require.NoError(t, err)
}

func applyPinsMigrations(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	root := filepath.Clean(filepath.Join("..", "..", "..", "..", ".."))
	for _, rel := range []string{
		"src/backend/migrations/messaging_db/000001_init.up.sql",
		"src/backend/migrations/messaging_db/000002_client_message_id.up.sql",
		"src/backend/migrations/messaging_db/000003_attachment_only_messages.up.sql",
		"src/backend/migrations/messaging_db/000004_delete_for_me.up.sql",
		"src/backend/migrations/messaging_db/000005_reactions.up.sql",
		"src/backend/migrations/messaging_db/000006_pins.up.sql",
	} {
		b, err := os.ReadFile(filepath.Join(root, rel))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(b))
		require.NoError(t, err)
	}
}
