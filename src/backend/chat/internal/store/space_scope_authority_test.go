package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	chatv1 "voice.app/voice/chat/v1"
)

// This assertion can execute against the original schema: missing persisted
// authority is an observable empty projection, not an undefined Go symbol.
func TestSpaceScopeAuthorityPersistsAttachmentIdentity(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	applyScopeAuthorityMigration(t, ctx, pool)
	row, err := (&DMStore{Pool: pool}).CreateSpaceChannelChat(ctx, uuid.New(), uuid.New(), "Scope", nil)
	require.NoError(t, err)
	var raw *string
	require.NoError(t, pool.QueryRow(ctx, `SELECT to_jsonb(c)->>'scope_incarnation' FROM chats c WHERE id=$1`, row.ID).Scan(&raw))
	require.NotNil(t, raw, "a stored Space attachment needs a durable native incarnation")
	id, err := uuid.Parse(*raw)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, id)
}

func TestSpaceScopeAuthorityConcurrentNativeWritesAndSnapshot(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	applyScopeAuthorityMigration(t, ctx, pool)
	st := &DMStore{Pool: pool}
	space, other := uuid.New(), uuid.New()
	row, err := st.CreateSpaceChannelChat(ctx, uuid.New(), space, "Concurrency", nil)
	require.NoError(t, err)
	before, err := st.GetSpaceChatScopeAuthority(ctx, row.ID, space)
	require.NoError(t, err)
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `UPDATE chats SET space_id=$2 WHERE id=$1`, row.ID, other)
	require.NoError(t, err)
	// One MVCC statement reads the complete old committed source/head while
	// another transaction holds the uncommitted replacement, with no RPC lock.
	observed, err := st.GetSpaceChatScopeAuthority(ctx, row.ID, space)
	require.NoError(t, err)
	require.Equal(t, before.ChatScopeIncarnation, observed.ChatScopeIncarnation)
	require.Equal(t, before.ChatAuthorityRevision, observed.ChatAuthorityRevision)
	done := make(chan error, 1)
	go func() {
		_, e := pool.Exec(ctx, `UPDATE chats SET type='group' WHERE id=$1 /*scope-authority-barrier*/`, row.ID)
		done <- e
	}()
	require.Eventually(t, func() bool {
		var waiting bool
		e := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE wait_event_type='Lock' AND query LIKE '%scope-authority-barrier%')`).Scan(&waiting)
		return e == nil && waiting
	}, 5*time.Second, 10*time.Millisecond)
	require.NoError(t, tx.Commit(ctx))
	select {
	case e := <-done:
		require.NoError(t, e)
	case <-time.After(5 * time.Second):
		t.Fatal("native writer did not finish")
	}
	after, err := st.GetSpaceChatScopeAuthority(ctx, row.ID, other)
	require.NoError(t, err)
	require.Equal(t, before.ChatAuthorityRevision+2, after.ChatAuthorityRevision)
	require.NotEqual(t, before.ChatScopeIncarnation, after.ChatScopeIncarnation)
	require.Equal(t, chatv1.ChatType_CHAT_TYPE_GROUP, after.ChatType)
	_, err = st.GetSpaceChatScopeAuthority(ctx, row.ID, space)
	require.ErrorIs(t, err, pgx.ErrNoRows)
	_, err = pool.Exec(ctx, `DELETE FROM chat_scope_authority_heads WHERE chat_id=$1`, row.ID)
	require.Error(t, err)
	_, err = pool.Exec(ctx, `UPDATE chat_scope_authority_heads SET revision=1 WHERE chat_id=$1`, row.ID)
	require.Error(t, err)
}

func applyScopeAuthorityMigration(t *testing.T, ctx context.Context, pool interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(chatRepoRoot(t), "src/backend/migrations/chat_db/000025_space_scope_authority.up.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(body))
	require.NoError(t, err)
}

func TestSpaceScopeAuthorityRejectsABAAndTracksIndependentLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	st := &DMStore{Pool: pool}
	space, other, creator := uuid.New(), uuid.New(), uuid.New()
	row, err := st.CreateSpaceChannelChat(ctx, creator, space, "Before migration", nil)
	require.NoError(t, err)
	applyScopeAuthorityMigration(t, ctx, pool)
	read := func() *chatv1.GetSpaceChatScopeAuthorityResponse {
		v, e := st.GetSpaceChatScopeAuthority(ctx, row.ID, space)
		require.NoError(t, e)
		return v
	}
	initial := read()
	require.True(t, initial.Live)
	require.Positive(t, initial.ChatAuthorityRevision)
	_, err = pool.Exec(ctx, `UPDATE chats SET name='cosmetic',topic='changed',updated_at=clock_timestamp() WHERE id=$1`, row.ID)
	require.NoError(t, err)
	cosmetic := read()
	require.Equal(t, initial.ChatScopeIncarnation, cosmetic.ChatScopeIncarnation)
	require.Equal(t, initial.ChatAuthorityRevision, cosmetic.ChatAuthorityRevision)
	_, err = pool.Exec(ctx, `UPDATE chats SET space_id=$2 WHERE id=$1`, row.ID, other)
	require.NoError(t, err)
	_, err = st.GetSpaceChatScopeAuthority(ctx, row.ID, space)
	require.ErrorIs(t, err, pgx.ErrNoRows)
	_, err = pool.Exec(ctx, `UPDATE chats SET space_id=$2 WHERE id=$1`, row.ID, space)
	require.NoError(t, err)
	moved := read()
	require.NotEqual(t, initial.ChatScopeIncarnation, moved.ChatScopeIncarnation)
	require.Greater(t, moved.ChatAuthorityRevision, initial.ChatAuthorityRevision)
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `UPDATE chats SET type='group' WHERE id=$1`, row.ID)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback(ctx))
	require.Equal(t, moved.ChatAuthorityRevision, read().ChatAuthorityRevision, "aborted mutation cannot advance the head")
	prepared, err := (&SpaceLifecycleStore{Pool: pool}).PrepareSpaceDeletionManifest(ctx, &chatv1.PrepareSpaceDeletionManifestRequest{ProtocolVersion: 1, SpaceId: space.String(), DeletionOperationId: uuid.NewString(), ScheduleGeneration: 1})
	require.NoError(t, err)
	frozen := read()
	require.False(t, frozen.Live)
	require.Equal(t, uint64(1), frozen.LifecycleGeneration)
	require.Greater(t, frozen.ChatAuthorityRevision, moved.ChatAuthorityRevision)
	require.Equal(t, moved.ChatScopeIncarnation, frozen.ChatScopeIncarnation)
	// Exercise the actual native lifecycle row transition; source validation is
	// separately covered by existing lifecycle RPC tests.
	_, err = pool.Exec(ctx, `UPDATE chat_space_lifecycle_fences SET generation=2,state='LIVE' WHERE space_id=$1`, space)
	require.NoError(t, err)
	restored := read()
	require.True(t, restored.Live)
	require.Equal(t, uint64(2), restored.LifecycleGeneration)
	require.Greater(t, restored.ChatAuthorityRevision, frozen.ChatAuthorityRevision)
	require.NotEmpty(t, prepared.Receipt.ReceiptId)
	_, err = pool.Exec(ctx, `DELETE FROM chats WHERE id=$1`, row.ID)
	require.NoError(t, err)
	_, err = st.GetSpaceChatScopeAuthority(ctx, row.ID, space)
	require.ErrorIs(t, err, pgx.ErrNoRows)
	_, err = pool.Exec(ctx, `INSERT INTO chats(id,type,space_id,creator_profile_id) VALUES($1,'channel',$2,$3)`, row.ID, space, creator)
	require.NoError(t, err)
	recreated := read()
	require.NotEqual(t, restored.ChatScopeIncarnation, recreated.ChatScopeIncarnation)
	require.Greater(t, recreated.ChatAuthorityRevision, restored.ChatAuthorityRevision)
	_, err = pool.Exec(ctx, `UPDATE chat_scope_authority_heads SET revision=9223372036854775807 WHERE chat_id=$1`, row.ID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE chats SET type='group' WHERE id=$1`, row.ID)
	require.Error(t, err, "overflow must abort rather than reset authority")
	require.Equal(t, recreated.ChatScopeIncarnation, read().ChatScopeIncarnation)
	_, err = pool.Exec(ctx, `TRUNCATE chats`)
	require.Error(t, err)
}
