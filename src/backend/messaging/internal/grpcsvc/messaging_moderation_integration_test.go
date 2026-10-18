package grpcsvc

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	messagingv1 "voice.app/voice/messaging/v1"
)

func messagingRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
}

func applyModerationSchemasForMessagingTest(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	root := messagingRepoRoot(t)
	for _, path := range []string{
		"src/backend/migrations/chat_db/000004_slow_mode.up.sql",
		"src/backend/migrations/chat_db/000005_thread_settings.up.sql",
		"src/backend/migrations/chat_db/000006_e2e_enabled.up.sql",
		"src/backend/migrations/space_db/000001_init.up.sql",
		"src/backend/migrations/space_db/000004_moderation.up.sql",
	} {
		sqlBytes, err := os.ReadFile(filepath.Join(root, path))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(sqlBytes))
		require.NoError(t, err)
	}
}

// TestSpaceModeration_SendMessage_FailsWhenTimedOut documents timeout blocks messaging in space chats.
func TestSpaceModeration_SendMessage_FailsWhenTimedOut(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, "src/backend/migrations/chat_db/000001_init.up.sql")
	applyBaseMessagingMigrations(t, ctx, pool)
	applyModerationSchemasForMessagingTest(t, ctx, pool)

	profA, acctA := uuid.New(), uuid.New()
	profB, acctB := uuid.New(), uuid.New()
	profiles := profileAcctMap{profA: acctA, profB: acctB}

	chatID := uuid.New()
	spaceID := uuid.New()
	seedGroupChat(t, ctx, pool, chatID, profA, profB)
	_, err := pool.Exec(ctx, `UPDATE chats SET space_id = $2 WHERE id = $1`, chatID, spaceID)
	require.NoError(t, err)

	authority := &communicationStateStub{state: 2}
	client, cleanup := startMessagingServerWired(t, pool, messagingWire{UserProfiles: profiles, CommunicationStates: authority})
	t.Cleanup(cleanup)

	_, err = client.SendMessage(withProfileCtx(ctx, acctB, profB), &messagingv1.SendMessageRequest{
		Chat:    chatGroupRef(chatID),
		Content: "hello after timeout",
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

// TestSpaceModeration_SendMessage_SucceedsAfterTimeoutRemoved documents remove timeout restores send.
func TestSpaceModeration_SendMessage_SucceedsAfterTimeoutRemoved(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, "src/backend/migrations/chat_db/000001_init.up.sql")
	applyBaseMessagingMigrations(t, ctx, pool)
	applyModerationSchemasForMessagingTest(t, ctx, pool)

	profA, acctA := uuid.New(), uuid.New()
	profB, acctB := uuid.New(), uuid.New()
	profiles := profileAcctMap{profA: acctA, profB: acctB}

	chatID := uuid.New()
	spaceID := uuid.New()
	seedGroupChat(t, ctx, pool, chatID, profA, profB)
	_, err := pool.Exec(ctx, `UPDATE chats SET space_id = $2 WHERE id = $1`, chatID, spaceID)
	require.NoError(t, err)

	authority := &communicationStateStub{state: 2}
	client, cleanup := startMessagingServerWired(t, pool, messagingWire{UserProfiles: profiles, CommunicationStates: authority})
	t.Cleanup(cleanup)

	_, err = client.SendMessage(withProfileCtx(ctx, acctB, profB), &messagingv1.SendMessageRequest{
		Chat:    chatGroupRef(chatID),
		Content: "blocked while timed out",
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	authority.state = 1

	_, err = client.SendMessage(withProfileCtx(ctx, acctB, profB), &messagingv1.SendMessageRequest{
		Chat:    chatGroupRef(chatID),
		Content: "restored after timeout removed",
	})
	require.NoError(t, err)
}

// TestSpaceSlowMode_SecondMessageWithinWindow_Fails documents slow mode rate limit on SendMessage.
func TestSpaceSlowMode_SecondMessageWithinWindow_Fails(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, "src/backend/migrations/chat_db/000001_init.up.sql")
	applyBaseMessagingMigrations(t, ctx, pool)
	applyModerationSchemasForMessagingTest(t, ctx, pool)

	profA, acctA := uuid.New(), uuid.New()
	profB := uuid.New()
	profiles := profileAcctMap{profA: acctA, profB: acctA}

	chatID := uuid.New()
	spaceID := uuid.New()
	seedGroupChat(t, ctx, pool, chatID, profA, profB)
	_, err := pool.Exec(ctx, `UPDATE chats SET space_id = $2 WHERE id = $1`, chatID, spaceID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE chats SET slow_mode_seconds = 10 WHERE id = $1`, chatID)
	require.NoError(t, err, "chat_db migration must allow slow_mode_seconds > 0")

	client, cleanup := startMessagingServerWired(t, pool, messagingWire{UserProfiles: profiles, CommunicationStates: &communicationStateStub{state: 1}})
	t.Cleanup(cleanup)

	sendCtx := withProfileCtx(ctx, acctA, profA)
	chatRef := chatGroupRef(chatID)

	_, err = client.SendMessage(sendCtx, &messagingv1.SendMessageRequest{
		Chat:    chatRef,
		Content: "first",
	})
	require.NoError(t, err)

	_, err = client.SendMessage(sendCtx, &messagingv1.SendMessageRequest{
		Chat:    chatRef,
		Content: "too fast",
	})
	require.Error(t, err)
	require.Equal(t, codes.ResourceExhausted, status.Code(err))
}
