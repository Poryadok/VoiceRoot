package store

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	chatv1 "voice.app/voice/chat/v1"
)

func TestBotScopeAuthorityReadsCurrentAndRetiredHeadWithoutChangingOldRead(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL Chat source")
	}
	ctx := t.Context()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	applyScopeAuthorityMigration(t, ctx, pool)
	s := &DMStore{Pool: pool}
	space, other := uuid.New(), uuid.New()
	row, err := s.CreateSpaceChannelChat(ctx, uuid.New(), space, "Bot scope", nil)
	require.NoError(t, err)
	live, err := s.GetBotChatScopeAuthority(ctx, row.ID, space)
	require.NoError(t, err)
	require.True(t, live.Exists)
	require.True(t, live.Live)
	require.Equal(t, chatv1.ChatType_CHAT_TYPE_CHANNEL, live.ChatType)
	_, err = s.GetBotChatScopeAuthority(ctx, row.ID, other)
	require.ErrorIs(t, err, pgx.ErrNoRows)
	_, err = pool.Exec(ctx, `DELETE FROM chats WHERE id=$1`, row.ID)
	require.NoError(t, err)
	retired, err := s.GetBotChatScopeAuthority(ctx, row.ID, space)
	require.NoError(t, err, "a real retained head qualifies only the last retired attachment")
	require.False(t, retired.Exists)
	require.False(t, retired.Live)
	require.Equal(t, live.ChatScopeIncarnation, retired.ChatScopeIncarnation)
	require.Greater(t, retired.ChatAuthorityRevision, live.ChatAuthorityRevision)
	require.Equal(t, chatv1.ChatType_CHAT_TYPE_UNSPECIFIED, retired.ChatType)
	require.Equal(t, chatv1.ChatScopeManagedKind_CHAT_SCOPE_MANAGED_KIND_UNSPECIFIED, retired.ManagedKind)
	_, err = s.GetSpaceChatScopeAuthority(ctx, row.ID, space)
	require.ErrorIs(t, err, pgx.ErrNoRows, "old Role/Space read does not gain retired semantics")
	_, err = s.GetBotChatScopeAuthority(ctx, row.ID, other)
	require.ErrorIs(t, err, pgx.ErrNoRows)
	_, err = s.GetBotChatScopeAuthority(ctx, uuid.New(), space)
	require.ErrorIs(t, err, pgx.ErrNoRows, "absence without a head is not completed cleanup")
	_, err = pool.Exec(ctx, `UPDATE chat_scope_authority_heads SET present=true,revision=revision+1 WHERE chat_id=$1`, row.ID)
	require.NoError(t, err)
	_, err = s.GetBotChatScopeAuthority(ctx, row.ID, space)
	require.Error(t, err, "contradictory head cannot qualify")
	require.NotErrorIs(t, err, pgx.ErrNoRows)
}
