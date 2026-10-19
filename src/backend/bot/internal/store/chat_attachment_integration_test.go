package store_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/integrationtest"
)

func TestChatAttachmentIntentHasDurableLifetimeBeforeRemoteAdd(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL durable attachment before uncertain remote Add")
	}
	ctx := t.Context()
	st := startBotStore(t)
	bot, _, err := st.CreateBot(ctx, uuid.New(), "Before remote Add", "", `[]`, uuid.New())
	require.NoError(t, err)
	space, chat, actor := uuid.New(), uuid.New(), uuid.New()
	observation, _, _ := chatScopeObservationFixture(t, space, chat)
	qualified := store.WithChatScopeObservations(ctx, []store.ChatScopeObservation{observation})
	intent, err := st.PrepareInstallation(qualified, bot.ID, space, actor, []uuid.UUID{chat})
	require.NoError(t, err)
	var raw []byte
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT to_jsonb(i)->'chat_attachment_ids' FROM bot_installation_intents i WHERE operation_id=$1`, intent.OperationID).Scan(&raw))
	require.NotEmpty(t, raw, "accepted pending intent needs a committed local lifetime, not a timestamp or Chat read ID")
	var ids []uuid.UUID
	require.NoError(t, json.Unmarshal(raw, &ids))
	require.Len(t, ids, 1)
	require.NotEqual(t, uuid.Nil, ids[0])
	restarted := &store.BotStore{Pool: st.Pool}
	again, err := restarted.PrepareInstallation(ctx, bot.ID, space, actor, []uuid.UUID{chat})
	require.NoError(t, err)
	require.Equal(t, intent.OperationID, again.OperationID)
	var retryRaw []byte
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT chat_attachment_ids FROM bot_installation_intents WHERE operation_id=$1`, intent.OperationID).Scan(&retryRaw))
	require.JSONEq(t, string(raw), string(retryRaw))
	require.NoError(t, st.CompleteInstallation(qualified, intent, []uuid.UUID{chat}))
	var current uuid.UUID
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT attachment_id FROM bot_chat_whitelist WHERE bot_id=$1 AND chat_id=$2`, bot.ID, chat).Scan(&current))
	require.Equal(t, ids[0], current)
	require.NoError(t, st.SetChatEnabled(ctx, bot.ID, chat, space, actor, false))
	fresh, _, _ := chatScopeObservationFixture(t, space, chat)
	require.NoError(t, st.SetChatEnabled(store.WithChatScopeObservations(ctx, []store.ChatScopeObservation{fresh}), bot.ID, chat, space, actor, true))
	_, err = st.InstallInSpace(store.WithChatScopeObservations(ctx, []store.ChatScopeObservation{fresh}), bot.ID, space, actor, []uuid.UUID{chat})
	// A source read cannot be relabelled across writer origins. Use a fresh read
	// for the setter rather than weakening this existing source-byte boundary.
	require.Error(t, err)
	setter, _, _ := chatScopeObservationFixture(t, space, chat)
	_, err = st.InstallInSpace(store.WithChatScopeObservations(ctx, []store.ChatScopeObservation{setter}), bot.ID, space, actor, []uuid.UUID{chat})
	require.NoError(t, err)
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT attachment_id FROM bot_chat_whitelist WHERE bot_id=$1 AND chat_id=$2`, bot.ID, chat).Scan(&current))
	require.Equal(t, ids[0], current, "overlap, enable and disable preserve one lifetime")
}

func TestDeletionRetainsOriginalAndReaddedAttachmentForSameChat(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL distinct same-chat cleanup obligations")
	}
	ctx := t.Context()
	st := startBotStore(t)
	bot, _, err := st.CreateBot(ctx, uuid.New(), "Same chat new lifetime", "", `[]`, uuid.New())
	require.NoError(t, err)
	space, chat, actor := uuid.New(), uuid.New(), uuid.New()
	original, originalReq, _ := chatScopeObservationFixture(t, space, chat)
	qualified := store.WithChatScopeObservations(ctx, []store.ChatScopeObservation{original})
	intent, err := st.PrepareInstallation(qualified, bot.ID, space, actor, []uuid.UUID{chat})
	require.NoError(t, err)
	require.NoError(t, st.CompleteInstallation(qualified, intent, []uuid.UUID{chat}))
	_, err = st.InstallInSpace(ctx, bot.ID, space, actor, nil)
	require.NoError(t, err)
	readded, readdedReq, _ := chatScopeObservationFixture(t, space, chat)
	_, err = st.InstallInSpace(store.WithChatScopeObservations(ctx, []store.ChatScopeObservation{readded}), bot.ID, space, actor, []uuid.UUID{chat})
	require.NoError(t, err)
	root, err := st.BeginBotDeletion(ctx, bot.ID, bot.OwnerAccountID)
	require.NoError(t, err)
	var raw []byte
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT scope_bytes FROM bot_deletion_installations WHERE operation_id=$1 AND space_id=$2`, root.OperationID, space).Scan(&raw))
	var snapshot struct {
		ChatSources []struct {
			Attachment uuid.UUID                          `json:"attachment_id"`
			Original   store.ChatScopeCleanupObservation  `json:"original"`
			Current    *store.ChatScopeCleanupObservation `json:"current"`
		} `json:"chat_sources"`
	}
	require.NoError(t, json.Unmarshal(raw, &snapshot))
	require.Len(t, snapshot.ChatSources, 2, "original admitted source and actual readded row are two obligations, not chat-dedup")
	found := map[uuid.UUID]bool{}
	origins := map[uuid.UUID]bool{}
	for _, s := range snapshot.ChatSources {
		require.NotEqual(t, uuid.Nil, s.Attachment)
		require.False(t, found[s.Attachment])
		found[s.Attachment] = true
		origins[s.Original.OperationID] = true
		if s.Original.OperationID == uuid.MustParse(originalReq.OperationId) {
			require.Nil(t, s.Current)
		}
	}
	require.True(t, origins[uuid.MustParse(originalReq.OperationId)])
	require.True(t, origins[uuid.MustParse(readdedReq.OperationId)])
	_, err = st.Pool.Exec(ctx, `UPDATE bot_chat_scope_observations SET response_bytes='later corruption'::bytea WHERE bot_id=$1`, bot.ID)
	require.NoError(t, err)
	var again []byte
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT scope_bytes FROM bot_deletion_installations WHERE operation_id=$1 AND space_id=$2`, root.OperationID, space).Scan(&again))
	require.Equal(t, raw, again, "both original/current lifetimes are frozen without later joins")
}

func TestLegacyChatAttachmentMigrationKeepsUnqualifiedOriginAfterEnable(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL one-time legacy row capture")
	}
	ctx := t.Context()
	pool := integrationtest.StartPostgres(t, ctx, "bot-attachment-legacy", "")
	up, err := os.ReadFile("../../../migrations/bot_db/000011_chat_attachment_lifetimes.up.sql")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, strings.Replace(migrationSQL(t), string(up), "", 1))
	require.NoError(t, err)
	st := &store.BotStore{Pool: pool}
	bot, _, err := st.CreateBot(ctx, uuid.New(), "Actual legacy row", "", `[]`, uuid.New())
	require.NoError(t, err)
	space, chat, actor, installation := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO bot_space_installations(id,bot_id,space_id,installed_by_profile_id) VALUES($1,$2,$3,$4);`, installation, bot.ID, space, actor)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO bot_chat_whitelist(bot_id,chat_id,space_id,enabled,added_by_profile_id) VALUES($1,$2,$3,false,$4)`, bot.ID, chat, space, actor)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(up))
	require.NoError(t, err)
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT attachment_id FROM bot_chat_whitelist WHERE bot_id=$1 AND chat_id=$2`, bot.ID, chat).Scan(&id))
	require.NotEqual(t, uuid.Nil, id)
	fresh, request, _ := chatScopeObservationFixture(t, space, chat)
	require.NoError(t, st.SetChatEnabled(store.WithChatScopeObservations(ctx, []store.ChatScopeObservation{fresh}), bot.ID, chat, space, actor, true))
	var current uuid.UUID
	var original *uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT w.attachment_id,a.origin_scope_observation_id FROM bot_chat_whitelist w JOIN bot_chat_attachments a USING(attachment_id) WHERE w.bot_id=$1 AND w.chat_id=$2`, bot.ID, chat).Scan(&current, &original))
	require.Equal(t, id, current)
	require.Nil(t, original, "a later current read cannot manufacture the legacy origin")
	root, err := st.BeginBotDeletion(ctx, bot.ID, bot.OwnerAccountID)
	require.NoError(t, err)
	var raw []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT scope_bytes FROM bot_deletion_installations WHERE operation_id=$1`, root.OperationID).Scan(&raw))
	var frozen store.BotDeletionTarget
	require.NoError(t, json.Unmarshal(raw, &frozen))
	require.Len(t, frozen.ChatSources, 1)
	source := frozen.ChatSources[0]
	require.Equal(t, id, source.AttachmentID)
	require.True(t, source.OriginalUnqualified)
	require.Equal(t, installation, source.InstallationID)
	require.Equal(t, uuid.Nil, source.Original.OperationID)
	require.NotNil(t, source.Current)
	require.Equal(t, uuid.MustParse(request.OperationId), source.Current.OperationID)
}

func TestConcurrentFirstCaptureOfActualLegacyWhitelistHasOneLifetime(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL guarded first capture")
	}
	ctx := t.Context()
	st := startBotStore(t)
	bot, _, err := st.CreateBot(ctx, uuid.New(), "First capture", "", `[]`, uuid.New())
	require.NoError(t, err)
	space, chat, actor, installation := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	_, err = st.Pool.Exec(ctx, `INSERT INTO bot_space_installations(id,bot_id,space_id,installed_by_profile_id) VALUES($1,$2,$3,$4)`, installation, bot.ID, space, actor)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `INSERT INTO bot_chat_whitelist(bot_id,chat_id,space_id,enabled,added_by_profile_id) VALUES($1,$2,$3,true,$4)`, bot.ID, chat, space, actor)
	require.NoError(t, err)
	start := make(chan struct{})
	results := make(chan error, 4)
	var workers sync.WaitGroup
	for range 4 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			results <- st.SetChatEnabled(context.Background(), bot.ID, chat, space, actor, false)
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	for err := range results {
		require.NoError(t, err)
	}
	var count int
	var id uuid.UUID
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_chat_attachments WHERE bot_id=$1 AND space_id=$2 AND chat_id=$3`, bot.ID, space, chat).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT attachment_id FROM bot_chat_whitelist WHERE bot_id=$1 AND chat_id=$2`, bot.ID, chat).Scan(&id))
	require.NotEqual(t, uuid.Nil, id)
}
