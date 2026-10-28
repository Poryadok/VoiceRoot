package store

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	commonv1 "voice.app/voice/common/v1"
	"voice/backend/pkg/integrationtest"
	inv "voice/backend/pkg/referenceinventory"
)

type inventoryDeleteFunc func(context.Context, string) error

func (f inventoryDeleteFunc) DeleteObject(ctx context.Context, key string) error { return f(ctx, key) }
func inventoryGCPool(t *testing.T) (context.Context, *pgxpool.Pool, *FilesStore, uuid.UUID) {
	t.Helper()
	if testing.Short() {
		t.Skip("isolated PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	t.Cleanup(cancel)
	pool := integrationtest.StartPostgres(t, ctx, "inventory_gc", "")
	_, source, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Join(filepath.Dir(source), "..", "..", "..", "migrations", "file_db")
	for _, migration := range []string{"000001_init", "000002_premium_upload_limit", "000003_story_context", "000004_file_reference_lifecycle", "000005_reference_gc_receipts", "000006_music_metadata", "000007_preview_metadata", "000008_expiry_event_outbox", "000009_gif_processing", "000016_reference_inventory"} {
		raw, err := os.ReadFile(filepath.Join(root, migration+".up.sql"))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(raw))
		require.NoError(t, err)
	}
	blob := uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO file_blobs(blob_id,sha256,original_r2_key,state) VALUES($1,decode(repeat('a',64),'hex'),'inventory/original','GC_PENDING')`, blob)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO files(id,blob_id,uploader_profile_id,original_name,mime_type,size_bytes,r2_key,status,file_type,is_e2e,scan_result) VALUES($1,$1,$2,'photo','image/png',1,'inventory/original','ready','image',false,'clean')`, blob, uuid.New())
	require.NoError(t, err)
	return ctx, pool, NewFilesStore(pool), blob
}
func inventoryGCIdentity() *commonv1.ReferenceInventoryIdentity {
	return &commonv1.ReferenceInventoryIdentity{ProtocolVersion: 1, InstallationId: uuid.NewString(), RestoreCohortId: uuid.NewString(), MigrationId: uuid.NewString(), FileIncarnationId: uuid.NewString(), SourceService: "story", SourceComponent: "story.media.v1", OwnerIncarnationId: uuid.NewString(), WriterProtocolVersion: 1, SnapshotId: uuid.NewString()}
}

func TestInventoryGCRechecksBarrierAfterDurableClaim(t *testing.T) {
	ctx, _, files, _ := inventoryGCPool(t)
	candidate, ok, err := files.claimReferenceGC(ctx)
	require.NoError(t, err)
	require.True(t, ok)
	id := inventoryGCIdentity()
	require.NoError(t, files.CreateInventoryOperation(ctx, uuid.New(), id))
	var calls atomic.Int32
	deleted, err := files.deleteClaimedReferenceGC(ctx, inventoryDeleteFunc(func(context.Context, string) error { calls.Add(1); return nil }), candidate)
	require.NoError(t, err)
	require.False(t, deleted)
	require.Zero(t, calls.Load())
}

func TestInventoryGCPhysicalDeleteSerializesCohortAndQuarantinesClaim(t *testing.T) {
	ctx, pool, files, file := inventoryGCPool(t)
	entered, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := files.RunReferenceGCOnce(ctx, inventoryDeleteFunc(func(context.Context, string) error { close(entered); <-release; return nil }), 1)
		done <- err
	}()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	id, op := inventoryGCIdentity(), uuid.New()
	created := make(chan error, 1)
	go func() { created <- files.CreateInventoryOperation(ctx, op, id) }()
	require.Eventually(t, func() bool {
		var n int
		err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%file_reference_inventory_gate%'`).Scan(&n)
		return err == nil && n > 0
	}, 3*time.Second, 10*time.Millisecond)
	close(release)
	require.NoError(t, <-done)
	require.NoError(t, <-created)
	entry := &commonv1.ReferenceInventoryEntry{Key: &commonv1.ReferenceInventoryKey{FileId: file.String(), OwnerId: uuid.NewString(), OwnerType: 2}, OwnerRevision: 1, Reasons: []commonv1.ReferenceInventoryProtectionReason{1}, MediaType: "photo"}
	page := &commonv1.ReferenceInventoryPage{Identity: id, BoundaryRevision: 1, Entries: []*commonv1.ReferenceInventoryEntry{entry}}
	descriptor, err := inv.Describe([]*commonv1.ReferenceInventoryPage{page}, id, 1)
	require.NoError(t, err)
	require.NoError(t, files.SaveInventoryDescriptor(ctx, op, descriptor))
	require.NoError(t, files.ApplyInventoryPage(ctx, op, page))
	var issue string
	require.NoError(t, pool.QueryRow(ctx, `SELECT issue_reason FROM file_reference_inventory_items`).Scan(&issue))
	require.Equal(t, "irreversible blob deletion", issue)
	var state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM file_blobs WHERE blob_id=$1`, file).Scan(&state))
	require.Equal(t, "GC_COMPLETE", state, "historical import never resurrects claimed/collected bytes")
}

func TestInventoryHoldsGuardGCAndAlternateObjectCleanup(t *testing.T) {
	ctx, pool, files, file := inventoryGCPool(t)
	id, op := inventoryGCIdentity(), uuid.New()
	require.NoError(t, files.CreateInventoryOperation(ctx, op, id))
	entry := &commonv1.ReferenceInventoryEntry{Key: &commonv1.ReferenceInventoryKey{FileId: file.String(), OwnerId: uuid.NewString(), OwnerType: 2}, OwnerRevision: 1, Reasons: []commonv1.ReferenceInventoryProtectionReason{1}, MediaType: "photo"}
	page := &commonv1.ReferenceInventoryPage{Identity: id, BoundaryRevision: 1, Entries: []*commonv1.ReferenceInventoryEntry{entry}}
	d, err := inv.Describe([]*commonv1.ReferenceInventoryPage{page}, id, 1)
	require.NoError(t, err)
	require.NoError(t, files.SaveInventoryDescriptor(ctx, op, d))
	require.NoError(t, files.ApplyInventoryPage(ctx, op, page))
	// Isolate the hold predicate in this private test; production has NO path to
	// open this incomplete barrier or activate the Story-only cohort.
	_, err = pool.Exec(ctx, `UPDATE file_reference_inventory_gate SET blocked=false`)
	require.NoError(t, err)
	var calls atomic.Int32
	deleter := inventoryDeleteFunc(func(context.Context, string) error { calls.Add(1); return nil })
	n, err := files.RunReferenceGCOnce(ctx, deleter, 1)
	require.NoError(t, err)
	require.Zero(t, n)
	require.ErrorIs(t, files.InventoryGuardedDeleter(deleter).DeleteObject(ctx, "inventory/original"), ErrInventoryBarrier)
	_, err = pool.Exec(ctx, `INSERT INTO file_gif_cleanup(object_key,file_id,not_before) VALUES('inventory/original',$1,clock_timestamp()-interval '1 minute')`, file)
	require.NoError(t, err)
	deleted, err := files.CleanupGIFOnce(ctx, deleter)
	require.NoError(t, err)
	require.False(t, deleted, "actual GIF cleanup rechecks blob holds under its File then blob locks")
	_, err = files.DeleteLegacyFile(ctx, file, nil, deleter)
	require.ErrorIs(t, err, ErrFileRetained)
	require.Zero(t, calls.Load())
}
