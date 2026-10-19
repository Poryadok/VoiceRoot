package store

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/pkg/integrationtest"
)

func newBotPinRemovalFixture(t *testing.T) (*PinsStore, *messagingv1.ApplyBotChatPinRemovalRequest, uuid.UUID) {
	t.Helper()
	ctx := t.Context()
	pool := startPostgresForStoreTest(t, ctx)
	t.Cleanup(pool.Close)
	seedMessagingSchema(t, ctx, pool)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000035_bot_pin_removal.up.sql"))
	chat, space, actor, message := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	seedMessage(t, ctx, pool, chat, message, actor)
	s := &PinsStore{Pool: pool}
	require.NoError(t, s.UpsertPin(ctx, chat, message, uuid.New()))
	return s, botPinRemovalRequest(chat, space, actor), message
}

func TestBotPinCleanupFaultRollbackConcurrentReceiptAndConflictingAuthority(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL participant journal")
	}
	s, request, message := newBotPinRemovalFixture(t)
	ctx := t.Context()
	_, err := s.Pool.Exec(ctx, `ALTER TABLE bot_pin_removal_receipts ADD CONSTRAINT injected_receipt_failure CHECK(false)`)
	require.NoError(t, err)
	_, err = s.ApplyBotChatPinRemoval(ctx, request)
	require.Error(t, err)
	for _, table := range []string{"bot_pin_removal_decisions", "bot_pin_actor_bindings", "bot_pin_removal_revisions", "bot_pin_removal_receipts"} {
		var count int
		require.NoError(t, s.Pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&count))
		require.Zero(t, count, table)
	}
	pins, err := s.ListPins(ctx, uuid.MustParse(request.ChatId))
	require.NoError(t, err)
	require.Len(t, pins, 1)
	require.Equal(t, message, pins[0].MessageID)
	_, err = s.Pool.Exec(ctx, `ALTER TABLE bot_pin_removal_receipts DROP CONSTRAINT injected_receipt_failure`)
	require.NoError(t, err)
	type result struct {
		response *messagingv1.ApplyBotChatPinRemovalResponse
		err      error
	}
	results := make(chan result, 8)
	for range 8 {
		go func() {
			r, e := s.ApplyBotChatPinRemoval(ctx, proto.Clone(request).(*messagingv1.ApplyBotChatPinRemovalRequest))
			results <- result{r, e}
		}()
	}
	var first *messagingv1.ApplyBotChatPinRemovalResponse
	for range 8 {
		r := <-results
		require.NoError(t, r.err)
		if first == nil {
			first = r.response
		}
		require.True(t, proto.Equal(first, r.response))
	}
	var revision int64
	require.NoError(t, s.Pool.QueryRow(ctx, `SELECT revision FROM bot_pin_removal_revisions WHERE chat_id=$1 AND actor_profile_id=$2`, request.ChatId, request.ActorProfileId).Scan(&revision))
	require.EqualValues(t, 1, revision)
	require.EqualValues(t, 1, first.GetReceipt().GetRemovedPinCount())
	for _, mode := range []string{"decision hash", "foreign Bot", "orphan uninstall", "unknown evidence"} {
		t.Run(mode, func(t *testing.T) {
			bad := proto.Clone(request).(*messagingv1.ApplyBotChatPinRemovalRequest)
			bad.OperationId = uuid.NewString()
			switch mode {
			case "decision hash":
				bad.DecisionSha256[0] ^= 1
			case "foreign Bot":
				bad.DecisionId = uuid.NewString()
				bad.BotId = uuid.NewString()
			case "orphan uninstall":
				bad.Purpose = "space_uninstall"
				bad.Evidence = &messagingv1.ApplyBotChatPinRemovalRequest_OrphanWhitelist{OrphanWhitelist: &messagingv1.BotRemovalOrphanWhitelistEvidence{SnapshotId: uuid.NewString(), SnapshotSha256: make([]byte, 32)}}
			case "unknown evidence":
				bad.GetNative().ProtoReflect().SetUnknown([]byte{0x98, 0x06, 0x01})
			}
			_, err := s.ApplyBotChatPinRemoval(ctx, bad)
			require.Error(t, err)
		})
	}
	require.NoError(t, s.Pool.QueryRow(ctx, `SELECT revision FROM bot_pin_removal_revisions WHERE chat_id=$1 AND actor_profile_id=$2`, request.ChatId, request.ActorProfileId).Scan(&revision))
	require.EqualValues(t, 1, revision, "rejected authority cannot advance cleanup revision")
	corrupted := proto.Clone(first).(*messagingv1.ApplyBotChatPinRemovalResponse)
	corrupted.Receipt.Outcome = "PENDING"
	raw, err := (proto.MarshalOptions{Deterministic: true}).Marshal(corrupted)
	require.NoError(t, err)
	digest := sha256.Sum256(raw)
	_, err = s.Pool.Exec(ctx, `UPDATE bot_pin_removal_receipts SET response_bytes=$2,response_sha256=$3 WHERE operation_id=$1`, request.OperationId, raw, digest[:])
	require.NoError(t, err)
	_, err = s.ApplyBotChatPinRemoval(ctx, request)
	require.ErrorIs(t, err, ErrBotPinRemovalBinding, "digest-consistent incomplete receipt cannot become historical success")
}

func TestBotPinRemovalDownPreservesActualConcurrentFirstCleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL first-writer rollback interleaving")
	}
	s, request, _ := newBotPinRemovalFixture(t)
	ctx := t.Context()
	holder, err := s.Pool.Acquire(ctx)
	require.NoError(t, err)
	defer holder.Release()
	_, err = holder.Exec(ctx, `SELECT pg_advisory_lock(2473501)`)
	require.NoError(t, err)
	defer func() { _, _ = holder.Exec(context.Background(), `SELECT pg_advisory_unlock(2473501)`) }()
	_, err = s.Pool.Exec(ctx, `CREATE FUNCTION be170_pause_first_pin_decision() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_advisory_xact_lock(2473501); RETURN NEW; END $$; CREATE TRIGGER be170_pause_first_pin_decision BEFORE INSERT ON bot_pin_removal_decisions FOR EACH ROW EXECUTE FUNCTION be170_pause_first_pin_decision()`)
	require.NoError(t, err)
	cleanupDone := make(chan error, 1)
	go func() { _, err := s.ApplyBotChatPinRemoval(ctx, request); cleanupDone <- err }()
	require.Eventually(t, func() bool {
		var waiting bool
		err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE locktype='advisory' AND objid=2473501 AND NOT granted)`).Scan(&waiting)
		return err == nil && waiting
	}, 5*time.Second, 20*time.Millisecond, "actual cleanup holds its first authority table while paused")
	down, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "migrations", "messaging_db", "000035_bot_pin_removal.down.sql"))
	require.NoError(t, err)
	dropper, err := s.Pool.Acquire(ctx)
	require.NoError(t, err)
	defer dropper.Release()
	pid := dropper.Conn().PgConn().PID()
	downDone := make(chan error, 1)
	go func() { _, err := dropper.Exec(ctx, string(down)); downDone <- err }()
	require.Eventually(t, func() bool {
		var blocked bool
		err := s.Pool.QueryRow(ctx, `SELECT cardinality(pg_blocking_pids($1))>0`, pid).Scan(&blocked)
		return err == nil && blocked
	}, 5*time.Second, 20*time.Millisecond, "full official DOWN waits for actual first cleanup")
	_, err = holder.Exec(ctx, `SELECT pg_advisory_unlock(2473501)`)
	require.NoError(t, err)
	require.NoError(t, <-cleanupDone)
	require.ErrorContains(t, <-downDone, "authority is retained")
	var count int
	require.NoError(t, s.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_pin_removal_receipts`).Scan(&count))
	require.Equal(t, 1, count)
}

func TestBotPinRemovalDownRejectsOldSnapshotAndAllowsEmptyDefaultRC(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL retained authority rollback")
	}
	for _, oldSnapshot := range []bool{false, true} {
		t.Run(map[bool]string{false: "empty default RC", true: "old RR"}[oldSnapshot], func(t *testing.T) {
			s, request, _ := newBotPinRemovalFixture(t)
			ctx := t.Context()
			down, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "migrations", "messaging_db", "000035_bot_pin_removal.down.sql"))
			require.NoError(t, err)
			if !oldSnapshot {
				_, err = s.Pool.Exec(ctx, string(down))
				require.NoError(t, err)
				return
			}
			tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
			require.NoError(t, err)
			defer func() { _ = tx.Rollback(context.Background()) }()
			var count int
			require.NoError(t, tx.QueryRow(ctx, `SELECT count(*) FROM bot_pin_removal_decisions`).Scan(&count))
			require.Zero(t, count)
			_, err = s.ApplyBotChatPinRemoval(ctx, request)
			require.NoError(t, err)
			_, err = tx.Exec(ctx, string(down))
			require.ErrorContains(t, err, "READ COMMITTED")
			require.NoError(t, tx.Rollback(ctx))
			require.NoError(t, s.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_pin_removal_receipts`).Scan(&count))
			require.Equal(t, 1, count)
		})
	}
}

func TestBotPinRemovalOfficialMigration35(t *testing.T) {
	if testing.Short() {
		t.Skip("official complete Messaging migration source")
	}
	ctx := t.Context()
	database := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src", "backend", "migrations", "messaging_db"))
	database.Run(true, "up")
	chat, space, actor, message := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	seedMessage(t, ctx, database.Pool, chat, message, actor)
	s := &PinsStore{Pool: database.Pool}
	require.NoError(t, s.UpsertPin(ctx, chat, message, uuid.New()))
	response, err := s.ApplyBotChatPinRemoval(ctx, botPinRemovalRequest(chat, space, actor))
	require.NoError(t, err)
	require.EqualValues(t, 1, response.GetReceipt().GetRemovedPinCount())
	database.Run(false, "goto", "34")
	var count int
	require.NoError(t, database.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_pin_removal_receipts`).Scan(&count))
	require.Equal(t, 1, count)
}
