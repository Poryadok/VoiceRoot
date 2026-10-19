package grpcsvc

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	chatv1 "voice.app/voice/chat/v1"
	moderationv1 "voice.app/voice/moderation/v1"
)

// An explicit DSN lets an isolated Linux runner use its private Docker network
// without publishing PostgreSQL ports on the Windows host. CI retains its usual
// disposable testcontainer when no explicit runner is configured.
func automodAdmissionDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	raw := os.Getenv("MODERATION_AUTOMOD_TEST_DSN")
	if raw == "" {
		return startModerationPostgresPlatform(t, ctx)
	}
	parsed, err := url.Parse(raw)
	require.NoError(t, err)
	admin, err := pgxpool.New(ctx, raw)
	require.NoError(t, err)
	name := "automod_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = admin.Exec(ctx, fmt.Sprintf(`CREATE DATABASE "%s"`, name))
	require.NoError(t, err)
	parsed.Path = "/" + name
	pool, err := pgxpool.New(ctx, parsed.String())
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Close()
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, cleanupErr := admin.Exec(cleanup, fmt.Sprintf(`DROP DATABASE "%s" WITH (FORCE)`, name))
		require.NoError(t, cleanupErr)
		admin.Close()
	})
	for _, file := range []string{"000001_reports.up.sql", "000002_sanctions.up.sql", "000004_account_status_sync.up.sql"} {
		integrationtestApplySQL(t, ctx, pool, filepath.Join(repoRoot(t), "src", "backend", "migrations", "moderation_db", file))
	}
	return pool
}

func TestAutoModAdmission_LegacyMetadataCannotCountUncommittedText(t *testing.T) {
	if testing.Short() {
		t.Skip("requires disposable PostgreSQL")
	}
	pool := automodAdmissionDatabase(t)
	client, cleanup := startModerationGRPCTestServer(t, pool)
	t.Cleanup(cleanup)
	ctx := withInternalModCtx(context.Background(), uuid.New())
	_, err := client.CheckMessage(ctx, &moderationv1.CheckMessageRequest{
		SenderProfileId: uuid.NewString(),
		Chat:            &chatv1.ChatRef{Id: uuid.NewString()},
		Content:         "https://one.example https://two.example https://three.example",
	})
	var count int
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM auto_mod_log`).Scan(&count))
	require.Zero(t, count, "unverified pre-commit text must not create an automod strike")
	require.Equal(t, codes.Unauthenticated, status.Code(err), "raw internal/profile headers are not committed-message authority")
}
