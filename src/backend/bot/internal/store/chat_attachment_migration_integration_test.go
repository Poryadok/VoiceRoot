package store_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/integrationtest"
)

func attachmentMigrationSQL(t *testing.T, direction string) string {
	t.Helper()
	raw, err := os.ReadFile("../../../migrations/bot_db/000011_chat_attachment_lifetimes." + direction + ".sql")
	require.NoError(t, err)
	return string(raw)
}

func TestQualifiedPreLifetimeIntentAndWhitelistMigrateAsOneAttachment(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL exact retained local source continuity")
	}
	ctx := t.Context()
	pool := integrationtest.StartPostgres(t, ctx, "bot-attachment-qualified-up", "")
	up := attachmentMigrationSQL(t, "up")
	before := migrationSQL(t)
	require.Equal(t, 1, strings.Count(before, up))
	_, err := pool.Exec(ctx, strings.Replace(before, up, "", 1))
	require.NoError(t, err)
	st := &store.BotStore{Pool: pool}
	bot, _, err := st.CreateBot(ctx, uuid.New(), "Qualified old local attachment", "", `[]`, uuid.New())
	require.NoError(t, err)
	space, chat, actor, installation, operation := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	_, request, response := chatScopeObservationFixture(t, space, chat)
	requestBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(request)
	require.NoError(t, err)
	responseBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(response)
	require.NoError(t, err)
	requestSHA, responseSHA := sha256.Sum256(requestBytes), sha256.Sum256(responseBytes)
	chats, err := json.Marshal([]uuid.UUID{chat})
	require.NoError(t, err)
	scopes, err := json.Marshal([]string{request.OperationId})
	require.NoError(t, err)
	// Exact schema10 facts model an already accepted local source. This fixture
	// does not assert that its native request was ever published in Space.
	_, err = pool.Exec(ctx, `INSERT INTO bot_installation_intents(operation_id,bot_id,installation_id,space_id,actor_profile_id,installed_by_profile_id,generation,state,chat_ids,legacy_evidence,chat_scope_observation_ids) VALUES($1,$2,$3,$4,$5,$6,1,'INSTALLED',$7::jsonb,false,$8::jsonb)`, operation, bot.ID, installation, space, bot.ActorProfileID, actor, chats, scopes)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO bot_chat_scope_observations(observation_id,bot_id,chat_id,space_id,intent_operation_id,origin_kind,request_bytes,request_sha256,response_bytes,response_sha256,scope_incarnation,authority_revision,lifecycle_generation,source_exists,source_live) VALUES($1,$2,$3,$4,$5,'install_admission',$6,$7,$8,$9,$10,1,0,true,true)`, request.OperationId, bot.ID, chat, space, operation, requestBytes, requestSHA[:], responseBytes, responseSHA[:], response.Scope.ChatScopeIncarnation)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO bot_space_installations(id,bot_id,space_id,installed_by_profile_id) VALUES($1,$2,$3,$4)`, installation, bot.ID, space, actor)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO bot_chat_whitelist(bot_id,chat_id,space_id,enabled,added_by_profile_id,scope_observation_id,origin_scope_observation_id,source_installation_id) VALUES($1,$2,$3,true,$4,$5,$5,$6)`, bot.ID, chat, space, actor, request.OperationId, installation)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, up)
	require.NoError(t, err)
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT attachment_id FROM bot_chat_whitelist WHERE bot_id=$1 AND chat_id=$2`, bot.ID, chat).Scan(&id))
	intent, err := st.PrepareInstallation(ctx, bot.ID, space, actor, []uuid.UUID{chat})
	require.NoError(t, err)
	require.Equal(t, operation, intent.OperationID)
	var raw []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT chat_attachment_ids FROM bot_installation_intents WHERE operation_id=$1`, operation).Scan(&raw))
	var ids []uuid.UUID
	require.NoError(t, json.Unmarshal(raw, &ids))
	require.Equal(t, []uuid.UUID{id}, ids)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM bot_chat_attachments WHERE bot_id=$1`, bot.ID).Scan(&count))
	require.Equal(t, 1, count, "exact original read/install binding must not become two cleanup obligations")
	root, err := st.BeginBotDeletion(ctx, bot.ID, bot.OwnerAccountID)
	require.NoError(t, err)
	require.NoError(t, pool.QueryRow(ctx, `SELECT scope_bytes FROM bot_deletion_installations WHERE operation_id=$1`, root.OperationID).Scan(&raw))
	var target store.BotDeletionTarget
	require.NoError(t, json.Unmarshal(raw, &target))
	require.Len(t, target.ChatSources, 1)
	require.Equal(t, id, target.ChatSources[0].AttachmentID)
}

func TestAttachmentDownWaitsForActualFirstWriterAndRetainsItsCommit(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL actual first-writer DOWN race")
	}
	st := startBotStore(t)
	down := attachmentMigrationSQL(t, "down")
	_, err := st.Pool.Exec(t.Context(), down)
	require.NoError(t, err, "empty default RC rollback is supported")
	_, err = st.Pool.Exec(t.Context(), attachmentMigrationSQL(t, "up"))
	require.NoError(t, err)
	bot, _, err := st.CreateBot(t.Context(), uuid.New(), "First attachment writer", "", `[]`, uuid.New())
	require.NoError(t, err)
	_, err = st.Pool.Exec(t.Context(), `CREATE FUNCTION hold_attachment_first_writer() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_advisory_xact_lock(247011); RETURN NEW; END $$; CREATE TRIGGER hold_attachment_first_writer BEFORE INSERT ON bot_chat_attachments FOR EACH ROW EXECUTE FUNCTION hold_attachment_first_writer()`)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	blocker, err := st.Pool.Begin(ctx)
	require.NoError(t, err)
	defer blocker.Rollback(context.Background())
	_, err = blocker.Exec(ctx, `SELECT pg_advisory_xact_lock(247011)`)
	require.NoError(t, err)
	written := make(chan error, 1)
	go func() {
		_, err := st.PrepareInstallation(ctx, bot.ID, uuid.New(), uuid.New(), []uuid.UUID{uuid.New()})
		written <- err
	}()
	require.Eventually(t, func() bool {
		var n int
		err := st.Pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%INSERT INTO bot_chat_attachments%'`).Scan(&n)
		return err == nil && n > 0
	}, 5*time.Second, 20*time.Millisecond)
	connection, err := st.Pool.Acquire(ctx)
	require.NoError(t, err)
	defer connection.Release()
	pid := connection.Conn().PgConn().PID()
	result := make(chan error, 1)
	go func() { _, err := connection.Exec(ctx, down); result <- err }()
	require.Eventually(t, func() bool {
		var waiting bool
		err := st.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE pid=$1 AND wait_event_type='Lock')`, pid).Scan(&waiting)
		return err == nil && waiting
	}, 5*time.Second, 20*time.Millisecond)
	require.NoError(t, blocker.Commit(ctx))
	require.NoError(t, <-written)
	err = <-result
	require.ErrorContains(t, err, "lifetimes are retained")
	_, err = connection.Exec(ctx, "ROLLBACK")
	require.NoError(t, err)
	var count int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_chat_attachments`).Scan(&count))
	require.Equal(t, 1, count)
}

func TestAttachmentDownRejectsOldRepeatableReadBeforeDroppingFirstCommit(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL stale snapshot DOWN")
	}
	st := startBotStore(t)
	ctx := t.Context()
	rr, err := st.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	require.NoError(t, err)
	defer rr.Rollback(context.Background())
	var old int
	require.NoError(t, rr.QueryRow(ctx, `SELECT count(*) FROM bot_chat_attachments`).Scan(&old))
	require.Zero(t, old)
	bot, _, err := st.CreateBot(ctx, uuid.New(), "New commit after RR snapshot", "", `[]`, uuid.New())
	require.NoError(t, err)
	_, err = st.PrepareInstallation(ctx, bot.ID, uuid.New(), uuid.New(), []uuid.UUID{uuid.New()})
	require.NoError(t, err)
	_, err = rr.Exec(ctx, attachmentMigrationSQL(t, "down"))
	require.ErrorContains(t, err, "READ COMMITTED")
	require.NoError(t, rr.Rollback(context.Background()))
	var count int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_chat_attachments`).Scan(&count))
	require.Equal(t, 1, count)
}
