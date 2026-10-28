package grpcsvc

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

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
	return p
}
func inventoryEntry(file, owner, author uuid.UUID) *commonv1.ReferenceInventoryEntry {
	a := author.String()
	return &commonv1.ReferenceInventoryEntry{Key: &commonv1.ReferenceInventoryKey{FileId: file.String(), OwnerType: 2, OwnerId: owner.String()}, OwnerRevision: 1, Reasons: []commonv1.ReferenceInventoryProtectionReason{1}, AuthorProfileId: &a, MediaType: "photo"}
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
	observed := terminal.String()
	issue := uuid.NewString()
	entries := []*commonv1.ReferenceInventoryEntry{inventoryEntry(uuid.New(), uuid.New(), author), inventoryEntry(terminal, uuid.New(), author), {IssueId: &issue, IssueKind: 2, ObservedFileId: &observed, OwnerRevision: 1}}
	sort.Slice(entries, func(i, j int) bool { return inv.Compare(entries[i], entries[j]) < 0 })
	page := &commonv1.ReferenceInventoryPage{Identity: id, BoundaryRevision: 1, Entries: entries}
	d, err := inv.Describe([]*commonv1.ReferenceInventoryPage{page}, id, 1)
	require.NoError(t, err)
	require.NoError(t, files.CreateInventoryOperation(ctx, op, id))
	require.NoError(t, files.SaveInventoryDescriptor(ctx, op, d))
	require.NoError(t, files.ApplyInventoryPage(ctx, op, page))
	var issues, holds int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM file_reference_inventory_items WHERE issue_reason IS NOT NULL`).Scan(&issues))
	require.Equal(t, 3, issues)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM file_reference_inventory_holds`).Scan(&holds))
	require.Equal(t, 2, holds, "known terminal blobs remain held")
	row, err := files.GetFileByID(ctx, terminal)
	require.NoError(t, err)
	require.Equal(t, "deleted", row.Status)
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
