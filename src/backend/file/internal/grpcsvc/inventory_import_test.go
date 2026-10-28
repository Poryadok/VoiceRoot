package grpcsvc

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	commonv1 "voice.app/voice/common/v1"
	"voice/backend/file/internal/inventory"
	"voice/backend/file/internal/store"
	inv "voice/backend/pkg/referenceinventory"
)

func inventoryIdentity() *commonv1.ReferenceInventoryIdentity {
	return &commonv1.ReferenceInventoryIdentity{ProtocolVersion: 1, InstallationId: uuid.NewString(), RestoreCohortId: uuid.NewString(), MigrationId: uuid.NewString(), FileIncarnationId: uuid.NewString(), SourceService: "story", SourceComponent: "story.media.v1", OwnerIncarnationId: uuid.NewString(), WriterProtocolVersion: 1, SnapshotId: uuid.NewString()}
}
func inventoryPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	p := startR23FilePostgres(t, ctx)
	applyFileGateSQL(t, ctx, p, filepath.Join("src", "backend", "migrations", "file_db", "000016_reference_inventory.up.sql"))
	applyFileGateSQL(t, ctx, p, filepath.Join("src", "backend", "migrations", "file_db", "000017_reference_inventory_multiowner.up.sql"))
	return p
}
func inventoryEntry(file, owner, author uuid.UUID) *commonv1.ReferenceInventoryEntry {
	a := author.String()
	return &commonv1.ReferenceInventoryEntry{Key: &commonv1.ReferenceInventoryKey{FileId: file.String(), OwnerType: 2, OwnerId: owner.String()}, OwnerRevision: 1, Reasons: []commonv1.ReferenceInventoryProtectionReason{1}, AuthorProfileId: &a, MediaType: "photo"}
}

func TestInventoryCohortBindsDistinctComponentsOnce(t *testing.T) {
	ctx := r23TestContext(t)
	pool := inventoryPool(t, ctx)
	files := store.NewFilesStore(pool)
	story := inventoryIdentity()
	require.NoError(t, files.CreateInventoryOperation(ctx, uuid.New(), story))
	message := proto.Clone(story).(*commonv1.ReferenceInventoryIdentity)
	message.SourceService, message.SourceComponent = "messaging", "messaging.message.v1"
	message.OwnerIncarnationId, message.SnapshotId = uuid.NewString(), uuid.NewString()
	require.NoError(t, files.CreateInventoryOperation(ctx, uuid.New(), message))
	duplicate := proto.Clone(message).(*commonv1.ReferenceInventoryIdentity)
	duplicate.OwnerIncarnationId, duplicate.SnapshotId = uuid.NewString(), uuid.NewString()
	require.ErrorIs(t, files.CreateInventoryOperation(ctx, uuid.New(), duplicate), inv.ErrConflict)
}

func TestMessageInventoryProtectsMultipleFilesAndExactScope(t *testing.T) {
	ctx := r23TestContext(t)
	pool := inventoryPool(t, ctx)
	files := store.NewFilesStore(pool)
	id := inventoryIdentity()
	id.SourceService, id.SourceComponent = "messaging", "messaging.message.v1"
	op, owner, author, forwardedAuthor := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	first, second, forwarded := uuid.New(), uuid.New(), uuid.New()
	for _, file := range []uuid.UUID{first, second, forwarded} {
		seedStoryMediaFile(t, ctx, pool, file, author, "image", "ready", "clean", nil, false, nil, nil)
	}
	scope, otherScope, chat := uuid.NewString(), uuid.NewString(), uuid.NewString()
	entry := func(file uuid.UUID, space *string) *commonv1.ReferenceInventoryEntry {
		return &commonv1.ReferenceInventoryEntry{Key: &commonv1.ReferenceInventoryKey{FileId: file.String(), OwnerType: 1, OwnerId: owner.String(), ScopeSpaceId: space}, OwnerRevision: 1, Reasons: []commonv1.ReferenceInventoryProtectionReason{1}, AuthorProfileId: proto.String(forwardedAuthor.String()), OwnerChatId: &chat}
	}
	shared := entry(first, nil)
	shared.Key.OwnerId = uuid.NewString()
	entries := []*commonv1.ReferenceInventoryEntry{entry(first, nil), entry(second, &scope), entry(forwarded, &otherScope), shared}
	_, err := pool.Exec(ctx, `INSERT INTO file_references(file_id,owner_type,owner_id,scope_space_id) VALUES($1,1,$2,NULL),($3,1,$2,$4),($1,1,$5,NULL)`, first, owner, second, scope, shared.Key.OwnerId)
	require.NoError(t, err)
	sort.Slice(entries, func(i, j int) bool { return inv.Compare(entries[i], entries[j]) < 0 })
	page := &commonv1.ReferenceInventoryPage{Identity: id, BoundaryRevision: 1, Entries: entries}
	descriptor, err := inv.Describe([]*commonv1.ReferenceInventoryPage{page}, id, 1)
	require.NoError(t, err)
	require.NoError(t, files.CreateInventoryOperation(ctx, op, id))
	require.NoError(t, files.SaveInventoryDescriptor(ctx, op, descriptor))
	require.NoError(t, files.ApplyInventoryPage(ctx, op, page))
	var items, holds, unresolved int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*), count(*) FILTER (WHERE issue_reason IS NOT NULL) FROM file_reference_inventory_items WHERE operation_id=$1`, op).Scan(&items, &unresolved))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM file_reference_inventory_holds WHERE operation_id=$1`, op).Scan(&holds))
	require.Equal(t, 4, items)
	require.Equal(t, 4, holds)
	require.Equal(t, 1, unresolved, "forwarded evidence without an exact scope remains unresolved")
	retired := proto.Clone(entries[0]).(*commonv1.ReferenceInventoryEntry)
	retired.OwnerRevision, retired.Retired, retired.Reasons = 2, true, nil
	changes := &commonv1.ReadReferenceInventoryChangesResponse{Identity: id, HighWatermark: 2, Revisions: []*commonv1.ReferenceInventoryRevision{{Revision: 2, FinalCount: 1, Entries: []*commonv1.ReferenceInventoryEntry{retired}}}}
	require.NoError(t, files.ApplyInventoryChanges(ctx, op, 1, changes))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM file_reference_inventory_holds WHERE operation_id=$1`, op).Scan(&holds))
	require.Equal(t, 4, holds, "retirement does not drop protective holds")
	changed := proto.Clone(page).(*commonv1.ReferenceInventoryPage)
	changed.Entries[0].OwnerRevision++
	require.ErrorIs(t, files.ApplyInventoryPage(ctx, op, changed), inv.ErrConflict)
	var messageState, storyState string
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM file_reference_inventory_components WHERE migration_id=$1 AND component='messaging.message.v1'`, id.MigrationId).Scan(&messageState))
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM file_reference_inventory_components WHERE migration_id=$1 AND component='story.media.v1'`, id.MigrationId).Scan(&storyState))
	require.Equal(t, "BLOCKED", messageState)
	require.Equal(t, "MISSING", storyState, "a Messaging conflict must not alter Story coverage")
}

func TestInventoryMigrationRekeysRetainedStoryProof(t *testing.T) {
	ctx := r23TestContext(t)
	pool := startR23FilePostgres(t, ctx)
	applyFileGateSQL(t, ctx, pool, filepath.Join("src", "backend", "migrations", "file_db", "000016_reference_inventory.up.sql"))
	file, owner, author, migration, op := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	seedStoryMediaFile(t, ctx, pool, file, author, "image", "ready", "clean", nil, false, nil, nil)
	var blob uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT blob_id FROM files WHERE id=$1`, file).Scan(&blob))
	_, err := pool.Exec(ctx, `INSERT INTO file_reference_inventory_cohorts(migration_id,installation_id,restore_cohort_id,file_incarnation_id,state,registry_sha256) VALUES($1,$2,$3,$4,'MIGRATING',$5)`, migration, uuid.New(), uuid.New(), uuid.New(), make([]byte, 32))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO file_reference_inventory_components(migration_id,component) VALUES($1,'story.media.v1')`, migration)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO file_reference_inventory_imports(operation_id,migration_id,snapshot_id,identity_bytes) VALUES($1,$2,$3,$4)`, op, migration, uuid.New(), []byte{1})
	require.NoError(t, err)
	old := "key:" + file.String() + "/" + owner.String()
	_, err = pool.Exec(ctx, `INSERT INTO file_reference_inventory_items(operation_id,item_key,file_id,owner_id,owner_revision,entry_bytes) VALUES($1,$2,$3,$4,1,$5)`, op, old, file, owner, []byte{1})
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO file_reference_inventory_holds(operation_id,item_key,blob_id) VALUES($1,$2,$3)`, op, old, blob)
	require.NoError(t, err)
	applyFileGateSQL(t, ctx, pool, filepath.Join("src", "backend", "migrations", "file_db", "000017_reference_inventory_multiowner.up.sql"))
	want := "key:" + file.String() + "/2/" + owner.String() + "/-/-"
	var item, hold string
	require.NoError(t, pool.QueryRow(ctx, `SELECT item_key FROM file_reference_inventory_items WHERE operation_id=$1`, op).Scan(&item))
	require.NoError(t, pool.QueryRow(ctx, `SELECT item_key FROM file_reference_inventory_holds WHERE operation_id=$1`, op).Scan(&hold))
	require.Equal(t, want, item)
	require.Equal(t, want, hold)
	down, err := os.ReadFile(filepath.Join(fileGateRepoRoot(t), "src", "backend", "migrations", "file_db", "000017_reference_inventory_multiowner.down.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(down))
	require.Error(t, err)
}

func TestInventoryMigrationWaitsForConcurrentLegacyWriter(t *testing.T) {
	ctx := r23TestContext(t)
	pool := startR23FilePostgres(t, ctx)
	applyFileGateSQL(t, ctx, pool, filepath.Join("src", "backend", "migrations", "file_db", "000016_reference_inventory.up.sql"))
	file, owner, author, migration, op := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	seedStoryMediaFile(t, ctx, pool, file, author, "image", "ready", "clean", nil, false, nil, nil)
	var blob uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT blob_id FROM files WHERE id=$1`, file).Scan(&blob))
	_, err := pool.Exec(ctx, `INSERT INTO file_reference_inventory_cohorts(migration_id,installation_id,restore_cohort_id,file_incarnation_id,state,registry_sha256) VALUES($1,$2,$3,$4,'MIGRATING',$5)`, migration, uuid.New(), uuid.New(), uuid.New(), make([]byte, 32))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO file_reference_inventory_components(migration_id,component) VALUES($1,'story.media.v1')`, migration)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO file_reference_inventory_imports(operation_id,migration_id,snapshot_id,identity_bytes) VALUES($1,$2,$3,$4)`, op, migration, uuid.New(), []byte{1})
	require.NoError(t, err)
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(context.Background()) }()
	old := "key:" + file.String() + "/" + owner.String()
	_, err = tx.Exec(ctx, `INSERT INTO file_reference_inventory_items(operation_id,item_key,file_id,owner_id,owner_revision,entry_bytes) VALUES($1,$2,$3,$4,1,$5)`, op, old, file, owner, []byte{1})
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `INSERT INTO file_reference_inventory_holds(operation_id,item_key,blob_id) VALUES($1,$2,$3)`, op, old, blob)
	require.NoError(t, err)
	raw, err := os.ReadFile(filepath.Join(fileGateRepoRoot(t), "src", "backend", "migrations", "file_db", "000017_reference_inventory_multiowner.up.sql"))
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { _, e := pool.Exec(ctx, string(raw)); done <- e }()
	require.Eventually(t, func() bool {
		var n int
		e := pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%file_reference_inventory_imports%'`).Scan(&n)
		return e == nil && n > 0
	}, 3*time.Second, 10*time.Millisecond)
	require.NoError(t, tx.Commit(ctx))
	require.NoError(t, <-done)
	want := "key:" + file.String() + "/2/" + owner.String() + "/-/-"
	var item, hold string
	require.NoError(t, pool.QueryRow(ctx, `SELECT item_key FROM file_reference_inventory_items WHERE operation_id=$1`, op).Scan(&item))
	require.NoError(t, pool.QueryRow(ctx, `SELECT item_key FROM file_reference_inventory_holds WHERE operation_id=$1`, op).Scan(&hold))
	require.Equal(t, want, item)
	require.Equal(t, want, hold)
}

type inventorySourceFixture struct {
	descriptor *commonv1.ReferenceInventoryDescriptor
	page       *commonv1.ReferenceInventoryPage
	begin      func()
	changes    *commonv1.ReadReferenceInventoryChangesResponse
}

func (s *inventorySourceFixture) BeginReferenceInventory(context.Context, *commonv1.BeginReferenceInventoryRequest) (*commonv1.BeginReferenceInventoryResponse, error) {
	if s.begin != nil {
		s.begin()
	}
	return &commonv1.BeginReferenceInventoryResponse{Descriptor_: s.descriptor}, nil
}
func (s *inventorySourceFixture) ReadReferenceInventoryPage(context.Context, *commonv1.ReadReferenceInventoryPageRequest) (*commonv1.ReadReferenceInventoryPageResponse, error) {
	return &commonv1.ReadReferenceInventoryPageResponse{Page: s.page}, nil
}
func (s *inventorySourceFixture) ReadReferenceInventoryChanges(context.Context, *commonv1.ReadReferenceInventoryChangesRequest) (*commonv1.ReadReferenceInventoryChangesResponse, error) {
	return s.changes, nil
}

func TestInventoryPullPrivateHoldsReplayAndRetirement(t *testing.T) {
	ctx := r23TestContext(t)
	pool := inventoryPool(t, ctx)
	files := store.NewFilesStore(pool)
	file, author, owner := uuid.New(), uuid.New(), uuid.New()
	seedStoryMediaFile(t, ctx, pool, file, author, "image", "ready", "clean", nil, false, nil, nil)
	id := inventoryIdentity()
	op := uuid.New()
	entry := inventoryEntry(file, owner, author)
	page := &commonv1.ReferenceInventoryPage{Identity: id, BoundaryRevision: 1, Entries: []*commonv1.ReferenceInventoryEntry{entry}}
	d, err := inv.Describe([]*commonv1.ReferenceInventoryPage{page}, id, 1)
	require.NoError(t, err)
	source := &inventorySourceFixture{descriptor: d, page: page, changes: &commonv1.ReadReferenceInventoryChangesResponse{Identity: id, HighWatermark: 1}}
	gc := &r23ScriptedDeleter{}
	source.begin = func() {
		// The durable barrier exists before S0's materialized snapshot or any page.
		var blocked bool
		require.NoError(t, pool.QueryRow(ctx, `SELECT blocked FROM file_reference_inventory_gate`).Scan(&blocked))
		require.True(t, blocked)
		tx, e := pool.Begin(ctx)
		require.NoError(t, e)
		defer func() { _ = tx.Rollback(ctx) }()
		_, e = tx.Exec(ctx, `SELECT * FROM file_reference_inventory_gate FOR UPDATE NOWAIT`)
		require.NoError(t, e, "no File tx/guard held over owner RPC")
		require.NoError(t, tx.Rollback(ctx))
		n, e := files.RunReferenceGCOnce(ctx, gc, 1)
		require.NoError(t, e)
		require.Zero(t, n)
	}
	require.NoError(t, files.CreateInventoryOperation(ctx, op, id))
	worker := &inventory.Importer{Files: files, Source: source}
	n, err := worker.RunPendingOnce(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	var holds, refs, missing int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM file_reference_inventory_holds`).Scan(&holds))
	require.Equal(t, 1, holds)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM file_references`).Scan(&refs))
	require.Zero(t, refs, "historical evidence grants no readable reference/capability")
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM file_reference_inventory_components WHERE state='MISSING'`).Scan(&missing))
	require.Equal(t, 9, missing, "Story cannot activate global coverage")
	require.NoError(t, files.ApplyInventoryPage(ctx, op, page), "exact completed page retry is receipt only")
	retired := proto.Clone(entry).(*commonv1.ReferenceInventoryEntry)
	retired.Retired = true
	retired.Reasons = nil
	retired.OwnerRevision = 2
	changes := &commonv1.ReadReferenceInventoryChangesResponse{Identity: id, HighWatermark: 2, Revisions: []*commonv1.ReferenceInventoryRevision{{Revision: 2, FinalCount: 1, Entries: []*commonv1.ReferenceInventoryEntry{retired}}}}
	require.NoError(t, files.ApplyInventoryChanges(ctx, op, 1, changes))
	require.NoError(t, files.ApplyInventoryChanges(ctx, op, 1, changes), "complete revision replay")
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM file_reference_inventory_holds`).Scan(&holds))
	require.Equal(t, 1, holds, "negative source state cannot retire the last migration protection")
	_, err = files.DeleteLegacyFile(ctx, file, nil, gc)
	require.Error(t, err)
	require.Empty(t, gc.calls)
	apply := proto.Clone(page).(*commonv1.ReferenceInventoryPage)
	apply.Entries[0].MediaType = "video"
	require.ErrorIs(t, files.ApplyInventoryPage(ctx, op, apply), inv.ErrConflict)
	operation, err := files.LoadInventoryOperation(ctx, op)
	require.NoError(t, err)
	require.Equal(t, "BLOCKED", operation.State)
}

func TestInventoryIssuesCohortAndJournalGaps(t *testing.T) {
	ctx := r23TestContext(t)
	pool := inventoryPool(t, ctx)
	files := store.NewFilesStore(pool)
	id := inventoryIdentity()
	op := uuid.New()
	author := uuid.New()
	terminal := uuid.New()
	seedStoryMediaFile(t, ctx, pool, terminal, author, "image", "deleted", "clean", nil, false, nil, nil)
	processing := uuid.New()
	seedStoryMediaFile(t, ctx, pool, processing, author, "image", "processing", "skipped", nil, false, nil, nil)
	observed := terminal.String()
	issue := uuid.NewString()
	entries := []*commonv1.ReferenceInventoryEntry{inventoryEntry(uuid.New(), uuid.New(), author), inventoryEntry(terminal, uuid.New(), author), inventoryEntry(processing, uuid.New(), author), {IssueId: &issue, IssueKind: 2, ObservedFileId: &observed, OwnerRevision: 1}}
	sort.Slice(entries, func(i, j int) bool { return inv.Compare(entries[i], entries[j]) < 0 })
	page := &commonv1.ReferenceInventoryPage{Identity: id, BoundaryRevision: 1, Entries: entries}
	d, err := inv.Describe([]*commonv1.ReferenceInventoryPage{page}, id, 1)
	require.NoError(t, err)
	require.NoError(t, files.CreateInventoryOperation(ctx, op, id))
	require.NoError(t, files.SaveInventoryDescriptor(ctx, op, d))
	require.NoError(t, files.ApplyInventoryPage(ctx, op, page))
	var issues, holds int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM file_reference_inventory_items WHERE issue_reason IS NOT NULL`).Scan(&issues))
	require.Equal(t, 4, issues)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM file_reference_inventory_holds`).Scan(&holds))
	require.Equal(t, 3, holds, "known terminal and processing blobs remain held")
	row, err := files.GetFileByID(ctx, terminal)
	require.NoError(t, err)
	require.Equal(t, "deleted", row.Status)
	row, err = files.GetFileByID(ctx, processing)
	require.NoError(t, err)
	require.Equal(t, "processing", row.Status)
	other := proto.Clone(id).(*commonv1.ReferenceInventoryIdentity)
	other.RestoreCohortId = uuid.NewString()
	other.SnapshotId = uuid.NewString()
	require.ErrorIs(t, files.CreateInventoryOperation(ctx, uuid.New(), other), inv.ErrConflict)
	gap := &commonv1.ReadReferenceInventoryChangesResponse{Identity: id, HighWatermark: 3, Revisions: []*commonv1.ReferenceInventoryRevision{{Revision: 3, FinalCount: 0}}}
	require.ErrorIs(t, files.ApplyInventoryChanges(ctx, op, 1, gap), inv.ErrConflict)
	operation, err := files.LoadInventoryOperation(ctx, op)
	require.NoError(t, err)
	require.EqualValues(t, 1, operation.Cursor)
	// Every retained operation/cohort blocks DOWN under exclusive table locks.
	raw, err := os.ReadFile(filepath.Join(fileGateRepoRoot(t), "src", "backend", "migrations", "file_db", "000016_reference_inventory.down.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(raw))
	require.Error(t, err)
}

func TestInventoryOutOfOrderPageBlocksWithoutPartialAdvance(t *testing.T) {
	ctx := r23TestContext(t)
	pool := inventoryPool(t, ctx)
	files := store.NewFilesStore(pool)
	id, op := inventoryIdentity(), uuid.New()
	file, author := uuid.New(), uuid.New()
	entries := make([]*commonv1.ReferenceInventoryEntry, 1001)
	for i := range entries {
		entries[i] = inventoryEntry(file, uuid.New(), author)
	}
	sort.Slice(entries, func(i, j int) bool { return inv.Compare(entries[i], entries[j]) < 0 })
	pages := []*commonv1.ReferenceInventoryPage{{Identity: id, BoundaryRevision: 1, Entries: entries[:1000]}, {Identity: id, BoundaryRevision: 1, PageIndex: 1, Entries: entries[1000:]}}
	descriptor, err := inv.Describe(pages, id, 1)
	require.NoError(t, err)
	require.NoError(t, files.CreateInventoryOperation(ctx, op, id))
	require.NoError(t, files.SaveInventoryDescriptor(ctx, op, descriptor))
	require.ErrorIs(t, files.ApplyInventoryPage(ctx, op, pages[1]), inv.ErrConflict)
	operation, err := files.LoadInventoryOperation(ctx, op)
	require.NoError(t, err)
	require.Zero(t, operation.NextPage)
	require.Equal(t, "BLOCKED", operation.State)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM file_reference_inventory_receipts`).Scan(&count))
	require.Zero(t, count)
}
