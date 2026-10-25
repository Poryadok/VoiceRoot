package grpcsvc

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/file/internal/fileevents"
	"voice/backend/file/internal/jobs"
	"voice/backend/file/internal/store"
)

type expiryRecoveryPublisher struct {
	fileevents.NoopPublisher
	attempts  [][]byte
	delivered []*eventsv1.FileStreamEvent
	fail      bool
}

func (p *expiryRecoveryPublisher) PublishExpiryEvent(_ context.Context, raw []byte) error {
	p.attempts = append(p.attempts, append([]byte(nil), raw...))
	if p.fail {
		p.fail = false
		return errors.New("ambiguous broker response")
	}
	e := new(eventsv1.FileStreamEvent)
	if err := proto.Unmarshal(raw, e); err != nil {
		return err
	}
	p.delivered = append(p.delivered, e)
	return nil
}

func TestBE264ExpiryEventRecoversAfterStorageAndPublicationFailure(t *testing.T) {
	ctx := r23TestContext(t)
	pool := expiryProofPostgres(t, ctx)
	id := insertR23ReadyFile(t, ctx, pool, "expiry-event-recovery")
	chat := uuid.New()
	now := time.Now().UTC()
	_, err := pool.Exec(ctx, `UPDATE files SET expires_at=$2,chat_id=$3,chat_type='group' WHERE id=$1`, id, now.Add(-time.Hour), chat)
	require.NoError(t, err)
	files := store.NewFilesStore(pool)
	deleter := &r23ScriptedDeleter{failures: map[string]int{"r23/expiry-event-recovery": 1}}
	publisher := &expiryRecoveryPublisher{fail: true}
	n, err := jobs.RunExpiryPurgeOnce(ctx, files, deleter, publisher, now)
	require.Error(t, err)
	require.Zero(t, n)
	row, err := files.GetFileByID(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "expired", row.Status)
	// A restarted storage-only worker finishes the bytes. The ready-only scanner
	// now finds nothing: event recovery must come from committed publication intent.
	n, err = store.NewFilesStore(pool).RunReferenceGCOnce(ctx, deleter, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, n)
	for i := 0; i < 3; i++ {
		_, _ = jobs.RunExpiryPurgeOnce(ctx, store.NewFilesStore(pool), deleter, publisher, now)
	}
	require.Len(t, publisher.delivered, 1, "expired status must not strand file.expired after a transient storage failure")
	require.Len(t, publisher.attempts, 2, "one failed publication then acknowledged retry; no further publication")
	require.Equal(t, publisher.attempts[0], publisher.attempts[1], "retry must preserve exact event ID/time/payload")
	event := publisher.delivered[0]
	require.NotEmpty(t, event.EventId)
	require.NotNil(t, event.OccurredAt)
	require.Equal(t, id.String(), event.GetFileExpired().GetFileId())
	require.Equal(t, chat.String(), event.GetFileExpired().GetChatId())
}

// A private-network PostgreSQL endpoint avoids Docker host-port publication in
// isolated Linux proofs. Ordinary CI retains the existing Testcontainers seam.
func expiryProofPostgres(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("BE264_TEST_POSTGRES_DSN")
	if dsn == "" {
		return startR23FilePostgres(t, ctx)
	}
	require.False(t, testing.Short(), "private PostgreSQL proof must not run under short")
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	schema := "expiry_" + uuid.New().String()
	schema = strings.ReplaceAll(schema, "-", "_")
	_, err = pool.Exec(ctx, `CREATE SCHEMA `+schema)
	require.NoError(t, err)
	pool.Close()
	cfg, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err = pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	for _, name := range []string{"000001_init", "000002_premium_upload_limit", "000003_story_context", "000004_file_reference_lifecycle", "000005_reference_gc_receipts", "000006_music_metadata", "000007_preview_metadata", "000008_expiry_event_outbox"} {
		applyFileGateSQL(t, ctx, pool, filepath.Join("src", "backend", "migrations", "file_db", name+".up.sql"))
	}
	return pool
}

func TestBE264ExpiryEventVetoAndAtomicRollback(t *testing.T) {
	ctx := r23TestContext(t)
	pool := expiryProofPostgres(t, ctx)
	files := store.NewFilesStore(pool)
	now := time.Now().UTC()
	protected := insertR23ReadyFile(t, ctx, pool, "protected-event")
	insertR23Reference(t, ctx, pool, r23MessageReference(protected, uuid.New(), uuid.New()))
	renewed := insertR23ReadyFile(t, ctx, pool, "renewed-event")
	_, err := pool.Exec(ctx, `UPDATE files SET expires_at=$2 WHERE id=$1`, protected, now.Add(-time.Hour))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE files SET expires_at=$2 WHERE id=$1`, renewed, now.Add(time.Hour))
	require.NoError(t, err)
	deleter := &r23ScriptedDeleter{}
	publisher := &expiryRecoveryPublisher{}
	n, err := jobs.RunExpiryPurgeOnce(ctx, files, deleter, publisher, now)
	require.NoError(t, err)
	require.Zero(t, n)
	require.Empty(t, deleter.calls)
	require.Empty(t, publisher.attempts)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM file_expiry_event_outbox`).Scan(&count))
	require.Zero(t, count)
	// A failed event insert must roll back status, preview/music facts and blob
	// deletion intent together; storage must never be reached.
	id := insertR23ReadyFile(t, ctx, pool, "atomic-event")
	_, err = pool.Exec(ctx, `UPDATE files SET expires_at=$2 WHERE id=$1`, id, now.Add(-time.Hour))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `CREATE FUNCTION fail_expiry_insert() RETURNS trigger LANGUAGE plpgsql AS $$BEGIN RAISE EXCEPTION 'injected expiry insert failure'; END$$; CREATE TRIGGER fail_expiry BEFORE INSERT ON file_expiry_event_outbox FOR EACH ROW EXECUTE FUNCTION fail_expiry_insert()`)
	require.NoError(t, err)
	_, err = jobs.RunExpiryPurgeOnce(ctx, files, deleter, publisher, now)
	require.ErrorContains(t, err, "injected expiry insert failure")
	row, err := files.GetFileByID(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "ready", row.Status)
	require.Equal(t, "LIVE", r23BlobGCState(t, ctx, pool, id))
	require.Empty(t, deleter.calls)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM file_preview_event_outbox WHERE file_id=$1`, id).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM file_expiry_event_outbox`).Scan(&count))
	require.Zero(t, count)
	down, err := os.ReadFile(filepath.Join(fileGateRepoRoot(t), "src/backend/migrations/file_db/000008_expiry_event_outbox.down.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(down))
	require.NoError(t, err, "empty outbox can roll back safely")
	applyFileGateSQL(t, ctx, pool, "src/backend/migrations/file_db/000008_expiry_event_outbox.up.sql")
}

func TestBE264ExpiryEventLeaseNoopAndRollbackGuard(t *testing.T) {
	ctx := r23TestContext(t)
	pool := expiryProofPostgres(t, ctx)
	files := store.NewFilesStore(pool)
	now := time.Now().UTC()
	id := insertR23ReadyFile(t, ctx, pool, "lease-event")
	_, err := pool.Exec(ctx, `UPDATE files SET expires_at=$2 WHERE id=$1`, id, now.Add(-time.Hour))
	require.NoError(t, err)
	_, err = jobs.RunExpiryPurgeOnce(ctx, files, &r23ScriptedDeleter{}, fileevents.NoopPublisher{}, now)
	require.NoError(t, err)
	var pending int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM file_expiry_event_outbox WHERE published_at IS NULL`).Scan(&pending))
	require.Equal(t, 1, pending, "NATS-disabled mode must retain intent")
	entered, release := make(chan struct{}), make(chan struct{})
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	done := make(chan error, 1)
	var first []byte
	go func() {
		_, e := files.DispatchExpiryEvent(ctx, func(_ context.Context, payload []byte) error {
			first = append([]byte(nil), payload...)
			close(entered)
			<-release
			return errors.New("lost publish response")
		})
		done <- e
	}()
	<-entered
	published, err := files.DispatchExpiryEvent(ctx, func(context.Context, []byte) error { t.Error("second worker stole locked event"); return nil })
	require.NoError(t, err)
	require.False(t, published)
	close(release)
	require.ErrorContains(t, <-done, "lost publish response")
	publisher := &expiryRecoveryPublisher{}
	published, err = store.NewFilesStore(pool).DispatchExpiryEvent(ctx, publisher.PublishExpiryEvent)
	require.NoError(t, err)
	require.True(t, published)
	require.Len(t, publisher.attempts, 1)
	require.Equal(t, first, publisher.attempts[0])
	down, err := os.ReadFile(filepath.Join(fileGateRepoRoot(t), "src/backend/migrations/file_db/000008_expiry_event_outbox.down.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(down))
	require.ErrorContains(t, err, "erase durable event evidence")
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM file_expiry_event_outbox WHERE published_at IS NOT NULL`).Scan(&pending))
	require.Equal(t, 1, pending)
}
