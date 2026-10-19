package store

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSpaceReportAttachmentNativeProducer(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	applyScopeAuthorityMigration(t, ctx, pool)
	row, err := (&DMStore{Pool: pool}).CreateSpaceChannelChat(ctx, uuid.New(), uuid.New(), "Before report migration", nil)
	require.NoError(t, err)
	var incarnation string
	var revision int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT scope_incarnation::text,revision FROM chat_scope_authority_heads WHERE chat_id=$1`, row.ID).Scan(&incarnation, &revision))
	body, err := os.ReadFile(filepath.Join(chatRepoRoot(t), "src/backend/migrations/chat_db/000026_space_report_attachments.up.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(body))
	require.NoError(t, err)
	var generation, afterRevision int64
	var afterIncarnation string
	require.NoError(t, pool.QueryRow(ctx, `SELECT c.report_attachment_generation,h.scope_incarnation::text,h.revision
        FROM chats c JOIN chat_scope_authority_heads h ON h.chat_id=c.id WHERE c.id=$1`, row.ID).Scan(&generation, &afterIncarnation, &afterRevision))
	require.Equal(t, int64(1), generation)
	require.Equal(t, incarnation, afterIncarnation)
	require.Equal(t, revision, afterRevision, "migration is not a native mutation")
	var history int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM chat_report_attachment_retirements`).Scan(&history))
	require.Zero(t, history, "migration cannot invent historical exits")
	regression, err := os.ReadFile(filepath.Join(chatRepoRoot(t), "scripts/ci/chat-report-attachments_test.sql"))
	require.NoError(t, err)
	var statements []string
	for _, line := range strings.Split(string(regression), "\n") {
		if !strings.HasPrefix(line, "\\") {
			statements = append(statements, line)
		}
	}
	_, err = pool.Exec(ctx, strings.Join(statements, "\n"))
	require.NoError(t, err)
	var countBefore, countAfter int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM chat_report_attachment_retirements`).Scan(&countBefore))
	down, err := os.ReadFile(filepath.Join(chatRepoRoot(t), "src/backend/migrations/chat_db/000026_space_report_attachments.down.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(down))
	require.ErrorContains(t, err, "roll-forward recovery")
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM chat_report_attachment_retirements`).Scan(&countAfter))
	require.Equal(t, countBefore, countAfter)

	// A real blocked second writer observes the committed move's generation;
	// a later type change cannot retire that new binding or erase the old one.
	spaceA, spaceB := uuid.New(), uuid.New()
	concurrent, err := (&DMStore{Pool: pool}).CreateSpaceChannelChat(ctx, uuid.New(), spaceA, "Two writers", nil)
	require.NoError(t, err)
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `UPDATE chats SET space_id=$2 WHERE id=$1`, concurrent.ID, spaceB)
	require.NoError(t, err)
	finished := make(chan error, 1)
	go func() {
		_, e := pool.Exec(ctx, `UPDATE chats SET type='group' WHERE id=$1 /*report-attachment-writer-barrier*/`, concurrent.ID)
		finished <- e
	}()
	require.Eventually(t, func() bool {
		var waiting bool
		e := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE wait_event_type='Lock'
            AND query LIKE '%report-attachment-writer-barrier%' AND pid<>pg_backend_pid())`).Scan(&waiting)
		return e == nil && waiting
	}, 5*time.Second, 10*time.Millisecond)
	require.NoError(t, tx.Commit(ctx))
	select {
	case e := <-finished:
		require.NoError(t, e)
	case <-time.After(5 * time.Second):
		t.Fatal("native writer did not finish")
	}
	require.NoError(t, pool.QueryRow(ctx, `SELECT report_attachment_generation FROM chats WHERE id=$1`, concurrent.ID).Scan(&generation))
	require.Equal(t, int64(2), generation)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM chat_report_attachment_retirements
        WHERE chat_id=$1 AND attachment_generation=1 AND space_id=$2 AND reason='TARGET_MOVED'`, concurrent.ID, spaceA).Scan(&history))
	require.Equal(t, 1, history)
}
