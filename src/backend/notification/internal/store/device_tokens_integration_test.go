package store_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/store"
	"voice/backend/pkg/integrationtest"
)

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

func TestDeviceTokenStore_RegisterUpsert_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startNotificationPostgresForTest(t, ctx)
	applyNotificationMigration(t, ctx, pool)

	s := &store.DeviceTokenStore{Pool: pool}
	profileID := uuid.New()
	token := "fcm-token-upsert-" + uuid.NewString()

	id1, err := s.Register(ctx, profileID, "android", token, "fcm")
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, id1)

	id2, err := s.Register(ctx, profileID, "android", token, "fcm")
	require.NoError(t, err)
	require.Equal(t, id1, id2, "same profile+token should upsert, not create duplicate row")

	rows, err := s.ListByProfile(ctx, profileID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, token, rows[0].Token)
}

func TestDeviceTokenStore_MultiDevicePerProfile_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startNotificationPostgresForTest(t, ctx)
	applyNotificationMigration(t, ctx, pool)

	s := &store.DeviceTokenStore{Pool: pool}
	profileID := uuid.New()

	_, err := s.Register(ctx, profileID, "android", "android-token", "fcm")
	require.NoError(t, err)
	_, err = s.Register(ctx, profileID, "web", "web-token", "fcm")
	require.NoError(t, err)

	rows, err := s.ListByProfile(ctx, profileID)
	require.NoError(t, err)
	require.Len(t, rows, 2)
}

func TestDeviceTokenStore_DeleteByToken_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startNotificationPostgresForTest(t, ctx)
	applyNotificationMigration(t, ctx, pool)

	s := &store.DeviceTokenStore{Pool: pool}
	profileID := uuid.New()
	token := "delete-by-token-" + uuid.NewString()
	_, err := s.Register(ctx, profileID, "android", token, "fcm")
	require.NoError(t, err)

	err = s.DeleteByToken(ctx, token)
	require.NoError(t, err)

	rows, err := s.ListByProfile(ctx, profileID)
	require.NoError(t, err)
	require.Empty(t, rows)
}

func TestDeviceTokenStore_Unregister_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startNotificationPostgresForTest(t, ctx)
	applyNotificationMigration(t, ctx, pool)

	s := &store.DeviceTokenStore{Pool: pool}
	profileID := uuid.New()
	id, err := s.Register(ctx, profileID, "web", "web-unregister", "fcm")
	require.NoError(t, err)

	err = s.Unregister(ctx, profileID, id)
	require.NoError(t, err)

	rows, err := s.ListByProfile(ctx, profileID)
	require.NoError(t, err)
	require.Empty(t, rows)
}
