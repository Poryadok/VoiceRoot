package store

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	chatv1 "voice.app/voice/chat/v1"
)

func applyReportAttachmentMigration(t *testing.T, ctx context.Context, pool interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(chatRepoRoot(t), "src/backend/migrations/chat_db/000026_space_report_attachments.up.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(body))
	require.NoError(t, err)
}

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
	beforeMove, err := (&DMStore{Pool: pool}).GetSpaceChatScopeAuthority(ctx, concurrent.ID, spaceA)
	require.NoError(t, err)
	require.Equal(t, uint64(1), beforeMove.ReportAttachmentGeneration)
	observe := func(chat, space uuid.UUID, saved uint64, disposition chatv1.SpaceReportAttachmentDisposition) *chatv1.ReadSpaceReportAttachmentResponse {
		result, e := (&DMStore{Pool: pool}).ReadSpaceReportAttachment(ctx, chat, space, saved)
		require.NoError(t, e)
		require.Equal(t, disposition, result.Disposition)
		require.Equal(t, space.String(), result.ExpectedSpaceId)
		require.Equal(t, saved, result.SavedAttachmentGeneration)
		if disposition != chatv1.SpaceReportAttachmentDisposition_SPACE_REPORT_ATTACHMENT_DISPOSITION_RETIRED {
			require.Empty(t, result.OwnerEventId)
			require.Zero(t, result.OwnerRevision)
			require.Zero(t, result.Reason)
		}
		return result
	}
	unknown := chatv1.SpaceReportAttachmentDisposition_SPACE_REPORT_ATTACHMENT_DISPOSITION_UNKNOWN
	unchanged := chatv1.SpaceReportAttachmentDisposition_SPACE_REPORT_ATTACHMENT_DISPOSITION_UNCHANGED
	retired := chatv1.SpaceReportAttachmentDisposition_SPACE_REPORT_ATTACHMENT_DISPOSITION_RETIRED
	observe(uuid.New(), spaceA, 1, unknown)
	observe(concurrent.ID, spaceA, 2, unknown)
	observe(concurrent.ID, uuid.New(), 1, unknown)
	observe(concurrent.ID, spaceA, 1, unchanged)
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `UPDATE chats SET space_id=$2 WHERE id=$1`, concurrent.ID, spaceB)
	require.NoError(t, err)
	observe(concurrent.ID, spaceA, 1, unchanged) // Complete committed old snapshot, no read lock over the writer.
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
	afterMove, err := (&DMStore{Pool: pool}).GetSpaceChatScopeAuthority(ctx, concurrent.ID, spaceB)
	require.NoError(t, err)
	require.True(t, afterMove.Live)
	require.Equal(t, chatv1.ChatType_CHAT_TYPE_GROUP, afterMove.ChatType)
	require.Equal(t, beforeMove.ChatAuthorityRevision+2, afterMove.ChatAuthorityRevision)
	require.Equal(t, uint64(2), afterMove.ReportAttachmentGeneration)
	oldReceipt := observe(concurrent.ID, spaceA, 1, retired)
	require.Equal(t, chatv1.SpaceReportAttachmentRetirementReason_SPACE_REPORT_ATTACHMENT_RETIREMENT_REASON_TARGET_MOVED, oldReceipt.Reason)
	require.NotEmpty(t, oldReceipt.OwnerEventId)
	observe(concurrent.ID, spaceB, 1, unknown)
	observe(concurrent.ID, spaceB, 2, unchanged)
	_, err = (&SpaceLifecycleStore{Pool: pool}).PrepareSpaceDeletionManifest(ctx, &chatv1.PrepareSpaceDeletionManifestRequest{
		ProtocolVersion: 1, SpaceId: spaceB.String(), DeletionOperationId: uuid.NewString(), ScheduleGeneration: 1,
	})
	require.NoError(t, err)
	frozen, err := (&DMStore{Pool: pool}).GetSpaceChatScopeAuthority(ctx, concurrent.ID, spaceB)
	require.NoError(t, err)
	require.False(t, frozen.Live)
	require.Equal(t, chatv1.ChatScopeLifecycleState_CHAT_SCOPE_LIFECYCLE_STATE_FROZEN, frozen.LifecycleState)
	require.Equal(t, afterMove.ChatScopeIncarnation, frozen.ChatScopeIncarnation)
	require.Greater(t, frozen.ChatAuthorityRevision, afterMove.ChatAuthorityRevision)
	require.NoError(t, pool.QueryRow(ctx, `SELECT report_attachment_generation FROM chats WHERE id=$1`, concurrent.ID).Scan(&generation))
	require.Equal(t, int64(2), generation, "freeze cannot retire the report binding")
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM chat_report_attachment_retirements WHERE chat_id=$1`, concurrent.ID).Scan(&history))
	require.Equal(t, 1, history, "lifecycle observation is not positive target retirement")
	observe(concurrent.ID, spaceB, 2, unchanged) // Freeze is not a positive exit.
	// Return to A cannot resurrect A's saved first attachment.
	aba, err := (&DMStore{Pool: pool}).CreateSpaceChannelChat(ctx, uuid.New(), spaceA, "Observed ABA", nil)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE chats SET space_id=$2 WHERE id=$1`, aba.ID, uuid.New())
	require.NoError(t, err)
	first := observe(aba.ID, spaceA, 1, retired)
	_, err = pool.Exec(ctx, `UPDATE chats SET space_id=$2 WHERE id=$1`, aba.ID, spaceA)
	require.NoError(t, err)
	again := observe(aba.ID, spaceA, 1, retired)
	require.Equal(t, first.OwnerEventId, again.OwnerEventId)
	observe(aba.ID, spaceA, 3, unchanged)
	_, err = pool.Exec(ctx, `DELETE FROM chats WHERE id=$1`, aba.ID)
	require.NoError(t, err)
	deleted := observe(aba.ID, spaceA, 3, retired)
	require.Equal(t, chatv1.SpaceReportAttachmentRetirementReason_SPACE_REPORT_ATTACHMENT_RETIREMENT_REASON_TARGET_DELETED, deleted.Reason)
	observe(aba.ID, spaceA, 4, unknown)
}
