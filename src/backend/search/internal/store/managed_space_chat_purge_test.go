package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	searchv1 "voice.app/voice/search/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
)

func TestMessageSearchStore_SpaceChildPurgeRequiresBoundDecisionAndReplaysAfterTerminal_postgres(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx := context.Background()
	root := searchModuleRepoRoot(t)
	pool := integrationtest.StartPostgres(t, ctx, "searchdb", filepath.Join(root, "src", "backend", "migrations", "search_db", "000001_init.up.sql"))
	for _, name := range []string{"000003_space_lifecycle.up.sql", "000009_managed_chat_message_purge.up.sql", "000010_chat_manifest_root_binding.up.sql"} {
		integrationtest.ApplySQLFile(t, ctx, pool, root, filepath.Join("src", "backend", "migrations", "search_db", name))
	}
	st := NewMessageSearchStore(pool)
	space, deletion, chat, message, controlChat, controlMessage := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	for _, pair := range [][2]uuid.UUID{{chat, message}, {controlChat, controlMessage}} {
		require.NoError(t, st.Upsert(ctx, MessageDocument{MessageID: pair[1], ChatID: pair[0], SenderProfileID: uuid.New(), Body: "purge scope", CreatedAt: time.Now()}))
	}
	digest := sha256.Sum256([]byte("sealed fixture manifest"))
	_, err := pool.Exec(ctx, `INSERT INTO search_space_lifecycle_fences VALUES($1,1,'FROZEN',$2,clock_timestamp())`, space, deletion)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO search_space_chat_manifests(space_id,deletion_operation_id,generation,manifest_id,manifest_sha256,item_count,page_count,sealed,created_at,root_manifest_id,root_manifest_sha256,root_manifest_item_count) VALUES($1,$2,1,$3,$4,1,1,true,clock_timestamp(),$3,$4,1)`, space, deletion, uuid.NewString(), digest[:])
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO search_space_chat_manifest_items VALUES($1,$2,1,0,0,$3)`, space, deletion, chat)
	require.NoError(t, err)
	operation := uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("voice.messaging.v1.SpaceChatPurge\x00%s\x00%s\x00%s", space, deletion, chat)))
	request := &searchv1.PurgeManagedChatMessagesRequest{OperationId: operation.String(), ChatId: chat.String(), MessageIds: []string{message.String()}}
	verified, hash := managedSearchPurgeContext(t, request)
	_, err = st.PurgeManagedChatMessages(verified, operation, chat, []uuid.UUID{message}, hash)
	require.Error(t, err, "Messaging cannot purge a reversible FROZEN chat")
	_, err = pool.Exec(ctx, `UPDATE search_space_lifecycle_fences SET state='PURGE_DECIDED',generation=2 WHERE space_id=$1`, space)
	require.NoError(t, err)
	_, err = st.PurgeManagedChatMessages(ctx, operation, chat, []uuid.UUID{message}, hash)
	require.Error(t, err, "public child UUID is not authorization")
	wrong := proto.Clone(request).(*searchv1.PurgeManagedChatMessagesRequest)
	wrong.OperationId = uuid.NewString()
	wrongCtx, wrongHash := managedSearchPurgeContext(t, wrong)
	_, err = st.PurgeManagedChatMessages(wrongCtx, uuid.MustParse(wrong.OperationId), chat, []uuid.UUID{message}, wrongHash)
	require.Error(t, err, "child must match the saved Space deletion")

	foreign := []uuid.UUID{message, controlMessage}
	sort.Slice(foreign, func(i, j int) bool { return foreign[i].String() < foreign[j].String() })
	mixed := &searchv1.PurgeManagedChatMessagesRequest{OperationId: operation.String(), ChatId: chat.String(), MessageIds: []string{foreign[0].String(), foreign[1].String()}}
	mixedCtx, mixedHash := managedSearchPurgeContext(t, mixed)
	_, err = st.PurgeManagedChatMessages(mixedCtx, operation, chat, foreign, mixedHash)
	require.Error(t, err, "another chat's indexed message cannot be tombstoned")
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM search_managed_chat_message_purge_fences`).Scan(&count))
	require.Zero(t, count, "rejected mixed work set must roll back atomically")

	receipt, err := st.PurgeManagedChatMessages(verified, operation, chat, []uuid.UUID{message}, hash)
	require.NoError(t, err, "bound Messaging child must progress through irreversible fence")
	require.Equal(t, uint64(1), receipt.DeletedCount)
	_, _, err = st.SearchInChat(ctx, chat, "purge", nil, 20)
	require.Error(t, err, "ordinary frozen reads stay closed")
	_, err = pool.Exec(ctx, `UPDATE search_space_lifecycle_fences SET state='PURGED' WHERE space_id=$1`, space)
	require.NoError(t, err)
	replayed, err := NewMessageSearchStore(pool).PurgeManagedChatMessages(verified, operation, chat, []uuid.UUID{message}, hash)
	require.NoError(t, err)
	require.Equal(t, receipt, replayed, "terminal restart replay returns the first owner evidence")
	_, err = st.PurgeManagedChatMessages(mixedCtx, operation, chat, foreign, mixedHash)
	require.ErrorIs(t, err, ErrManagedChatSearchPurgeConflict)
	hits, _, err := st.SearchInChat(ctx, controlChat, "purge", nil, 20)
	require.NoError(t, err)
	require.Len(t, hits, 1)
	require.Equal(t, controlMessage, hits[0].MessageID)
}

func managedSearchPurgeContext(t *testing.T, request *searchv1.PurgeManagedChatMessagesRequest) (context.Context, []byte) {
	t.Helper()
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	wireHash, err := hex.DecodeString(strings.TrimPrefix(hash, "sha256:"))
	require.NoError(t, err)
	p := principal.Principal{Kind: "service", Issuer: "messaging", Subject: "service:messaging", Audience: "search", RPC: searchv1.SearchService_PurgeManagedChatMessages_FullMethodName, RequestID: request.OperationId, RequestHash: hash}
	return principal.WithVerified(context.Background(), p), wireHash
}

func TestMessageSearchStore_ConcurrentManagedPurgeReturnsFirstReceipt_postgres(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	root := searchModuleRepoRoot(t)
	pool := integrationtest.StartPostgres(t, ctx, "searchdb", filepath.Join(root, "src", "backend", "migrations", "search_db", "000001_init.up.sql"))
	for _, name := range []string{"000003_space_lifecycle.up.sql", "000009_managed_chat_message_purge.up.sql"} {
		integrationtest.ApplySQLFile(t, ctx, pool, root, filepath.Join("src", "backend", "migrations", "search_db", name))
	}
	st := NewMessageSearchStore(pool)
	for _, nonempty := range []bool{true, false} {
		t.Run(fmt.Sprintf("nonempty=%t", nonempty), func(t *testing.T) {
			op, chat := uuid.New(), uuid.New()
			var ids []uuid.UUID
			if nonempty {
				ids = []uuid.UUID{uuid.New()}
			}
			hash := sha256.Sum256([]byte("same concurrent request"))
			type result struct {
				receipt *ManagedChatSearchPurgeReceipt
				err     error
			}
			results := make(chan result, 8)
			start := make(chan struct{})
			var release func()
			if nonempty {
				locker, err := pool.Begin(ctx)
				require.NoError(t, err)
				require.NoError(t, lockMessageProjection(ctx, locker, ids[0]))
				release = func() { require.NoError(t, locker.Rollback(ctx)) }
				defer locker.Rollback(context.Background())
			}
			for i := 0; i < 8; i++ {
				go func() {
					<-start
					receipt, err := st.PurgeManagedChatMessages(ctx, op, chat, ids, hash[:])
					results <- result{receipt, err}
				}()
			}
			close(start)
			if release != nil {
				require.Eventually(t, func() bool {
					var count int
					err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event='advisory'`).Scan(&count)
					return err == nil && count >= 2
				}, 5*time.Second, 20*time.Millisecond, "concurrent attempts must reach the database barrier")
				release()
			}
			var first *ManagedChatSearchPurgeReceipt
			for i := 0; i < 8; i++ {
				result := <-results
				require.NoError(t, result.err, "an exact concurrent retry must return saved evidence")
				if first == nil {
					first = result.receipt
				} else {
					require.Equal(t, first, result.receipt)
				}
			}
		})
	}
}
