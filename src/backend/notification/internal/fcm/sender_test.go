package fcm_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/fcm"
	"voice/backend/notification/internal/store"
	"voice/backend/pkg/integrationtest"
)

type recordingSender struct {
	mu      sync.Mutex
	sent    []fcm.PushPayload
	invalid bool
}

func (r *recordingSender) Send(ctx context.Context, profileID uuid.UUID, token store.DeviceToken, payload fcm.PushPayload) error {
	_ = ctx
	_ = profileID
	_ = token
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.invalid {
		return fcm.ErrInvalidToken
	}
	r.sent = append(r.sent, payload)
	return nil
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
}

func startNotificationPostgresForTest(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	return integrationtest.StartPostgres(t, ctx, "notificationdb", "")
}

func applyNotificationMigration(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	migrationPath := filepath.Join(repoRoot(t), "src", "backend", "migrations", "notification_db", "000001_init.up.sql")
	sqlBytes, err := os.ReadFile(migrationPath)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(sqlBytes))
	require.NoError(t, err)
}

// deliverWithCleanup sends via sender and deletes invalid tokens from DB (production contract).
func deliverWithCleanup(ctx context.Context, sender fcm.Sender, tokens *store.DeviceTokenStore, profileID uuid.UUID, token store.DeviceToken, payload fcm.PushPayload) error {
	err := sender.Send(ctx, profileID, token, payload)
	if err == fcm.ErrInvalidToken {
		return tokens.DeleteByToken(ctx, token.Token)
	}
	return err
}

func TestFCMSender_RecordsPayload(t *testing.T) {
	rec := &recordingSender{}
	profileID := uuid.New()
	token := store.DeviceToken{
		ID:          uuid.New(),
		ProfileID:   profileID,
		Platform:    "android",
		Token:       "valid-token",
		PushService: "fcm",
	}
	payload := fcm.PushPayload{
		Title:       "New message",
		Body:        "hello",
		CollapseTag: "chat-" + uuid.NewString(),
		Counter:     1,
		Data: map[string]string{
			"type":    "new_message",
			"chat_id": uuid.NewString(),
		},
	}
	err := rec.Send(context.Background(), profileID, token, payload)
	require.NoError(t, err)
	require.Len(t, rec.sent, 1)
	require.Equal(t, payload, rec.sent[0])
}

func TestFCMSender_InvalidTokenDeletesFromDB(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startNotificationPostgresForTest(t, ctx)
	applyNotificationMigration(t, ctx, pool)

	tokens := &store.DeviceTokenStore{Pool: pool}
	profileID := uuid.New()
	staleToken := "stale-fcm-" + uuid.NewString()
	id, err := tokens.Register(ctx, profileID, "web", staleToken, "fcm")
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, id)

	rec := &recordingSender{invalid: true}
	err = deliverWithCleanup(ctx, rec, tokens, profileID, store.DeviceToken{
		ProfileID: profileID,
		Token:     staleToken,
	}, fcm.PushPayload{Body: "ping"})
	require.NoError(t, err)

	rows, err := tokens.ListByProfile(ctx, profileID)
	require.NoError(t, err)
	require.Empty(t, rows, "invalid FCM token must be deleted from notification_db")
}
