package store_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	commonv1 "voice.app/voice/common/v1"
	"voice/backend/pkg/referenceinventory"
	"voice/backend/story/internal/store"
)

func inventoryStore(t *testing.T) (*store.StoryStore, *commonv1.ReferenceInventoryIdentity) {
	t.Helper()
	st := startStoryStore(t)
	id := &commonv1.ReferenceInventoryIdentity{ProtocolVersion: 1, InstallationId: uuid.NewString(), RestoreCohortId: uuid.NewString(), MigrationId: uuid.NewString(), FileIncarnationId: uuid.NewString(), SourceService: "story", SourceComponent: "story.media.v1", WriterProtocolVersion: 1, SnapshotId: uuid.NewString()}
	require.NoError(t, st.Pool.QueryRow(context.Background(), `SELECT owner_incarnation_id FROM story_reference_inventory_gate WHERE singleton`).Scan(&id.OwnerIncarnationId))
	return st, id
}

func TestStoryInventoryMaterializesRetainedPendingAndUnresolved(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st, id := inventoryStore(t)
	author, media := uuid.New(), uuid.New()
	s, err := st.CreateStory(ctx, store.CreateStoryInput{AuthorProfileID: author, Type: "photo", MediaFileID: &media, Visibility: "everyone"})
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `UPDATE stories SET expired_at=now(),archived_until=now()-interval '1 day' WHERE id=$1`, s.ID)
	require.NoError(t, err)
	h, err := st.CreateHighlight(ctx, author, "retained", "everyone")
	require.NoError(t, err)
	require.NoError(t, st.AddToHighlight(ctx, h.ID, author, s.ID))
	_, err = st.Pool.Exec(ctx, `UPDATE highlights SET cover_file_id=$2 WHERE id=$1`, h.ID, uuid.New())
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `INSERT INTO story_media_reference_intents(story_id,media_file_id,author_profile_id,media_type,available_at) VALUES($1,$2,$3,'video',now())`, uuid.New(), uuid.New(), author)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `INSERT INTO story_media_deletion_outbox(operation_id,story_id,media_file_id) VALUES($1,$1,$2)`, uuid.New(), uuid.New())
	require.NoError(t, err)
	req := &commonv1.BeginReferenceInventoryRequest{OperationId: uuid.NewString(), Identity: id}
	d, err := st.BeginReferenceInventory(ctx, req)
	require.NoError(t, err)
	require.NoError(t, referenceinventory.ValidateDescriptor(d))
	require.EqualValues(t, 4, d.Manifest.TotalCount)
	require.EqualValues(t, 1, d.Manifest.UnresolvedCount)
	p, err := st.ReadReferenceInventoryPage(ctx, id, 0)
	require.NoError(t, err)
	require.NoError(t, referenceinventory.ValidateCommittedPage(d, p))
	before, _, err := referenceinventory.Encode(p)
	require.NoError(t, err)
	require.NoError(t, st.DeleteStory(ctx, s.ID, author))
	changes, err := st.ReadReferenceInventoryChanges(ctx, id, d.Manifest.BoundaryRevision, 100)
	require.NoError(t, err)
	require.NoError(t, referenceinventory.ValidateChanges(changes, id, d.Manifest.BoundaryRevision))
	require.NotEmpty(t, changes.Revisions)
	var releasePending bool
	for _, revision := range changes.Revisions {
		for _, e := range revision.Entries {
			if e.GetKey().GetOwnerId() == s.ID.String() {
				releasePending = !e.Retired && len(e.Reasons) == 1 && e.Reasons[0] == commonv1.ReferenceInventoryProtectionReason_REFERENCE_INVENTORY_PROTECTION_REASON_RELEASE_PENDING
			}
		}
	}
	require.True(t, releasePending, "logical deletion retains the exact pending release pin")
	p, err = st.ReadReferenceInventoryPage(ctx, id, 0)
	require.NoError(t, err)
	after, _, err := referenceinventory.Encode(p)
	require.NoError(t, err)
	require.Equal(t, before, after, "live source changes cannot rewrite a materialized page")
	replayed, err := st.BeginReferenceInventory(ctx, req)
	require.NoError(t, err)
	require.True(t, proto.Equal(d, replayed))
	changed := proto.Clone(req).(*commonv1.BeginReferenceInventoryRequest)
	changed.Identity.RestoreCohortId = uuid.NewString()
	_, err = st.BeginReferenceInventory(ctx, changed)
	require.Error(t, err, "operation identity cannot be rebound to a new restore cohort")
}

func TestStoryInventoryRollbackAndFailedRemoteAcquireRetainOwnership(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st, id := inventoryStore(t)
	tx, err := st.Pool.Begin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `UPDATE story_reference_inventory_gate SET revision=revision+1 WHERE singleton`)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback(ctx))
	media := uuid.New()
	_, err = st.CreateStoryWithMedia(ctx, store.CreateStoryInput{AuthorProfileID: uuid.New(), Type: "photo", MediaFileID: &media, Visibility: "everyone"}, func(context.Context, uuid.UUID) error { return errors.New("ambiguous File result") })
	require.Error(t, err)
	d, err := st.BeginReferenceInventory(ctx, &commonv1.BeginReferenceInventoryRequest{OperationId: uuid.NewString(), Identity: id})
	require.NoError(t, err)
	require.EqualValues(t, 1, d.Manifest.BoundaryRevision, "only the committed creation intent consumes a revision")
	require.EqualValues(t, 1, d.Manifest.TotalCount)
	p, err := st.ReadReferenceInventoryPage(ctx, id, 0)
	require.NoError(t, err)
	require.Equal(t, media.String(), p.Entries[0].Key.FileId)
	require.Equal(t, []commonv1.ReferenceInventoryProtectionReason{commonv1.ReferenceInventoryProtectionReason_REFERENCE_INVENTORY_PROTECTION_REASON_CREATE_PENDING}, p.Entries[0].Reasons)
	require.EqualValues(t, 0, d.Manifest.UnresolvedCount, "new intents retain immutable author/media binding")
}

func TestStoryInventoryChangedOwnerIncarnationCannotServeOldPages(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st, id := inventoryStore(t)
	media := uuid.New()
	_, err := st.CreateStory(ctx, store.CreateStoryInput{AuthorProfileID: uuid.New(), Type: "photo", MediaFileID: &media, Visibility: "everyone"})
	require.NoError(t, err)
	_, err = st.BeginReferenceInventory(ctx, &commonv1.BeginReferenceInventoryRequest{OperationId: uuid.NewString(), Identity: id})
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `UPDATE story_reference_inventory_gate SET owner_incarnation_id=$1 WHERE singleton`, uuid.New())
	require.NoError(t, err)
	_, err = st.ReadReferenceInventoryPage(ctx, id, 0)
	require.ErrorIs(t, err, store.ErrInventoryConflict, "stored snapshot bytes cannot prove current owner continuity")
}

func TestStoryInventoryHistoricalContradictionsAreCounted(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st, id := inventoryStore(t)
	for _, kind := range []string{"text", "photo", "photo"} {
		media := uuid.New()
		s, err := st.CreateStory(ctx, store.CreateStoryInput{AuthorProfileID: uuid.New(), Type: kind, MediaFileID: &media, Visibility: "everyone"})
		require.NoError(t, err)
		if kind == "text" {
			continue
		}
		var deleted int
		require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM stories WHERE deleted_at IS NOT NULL`).Scan(&deleted))
		if deleted == 0 {
			_, err = st.Pool.Exec(ctx, `UPDATE stories SET deleted_at=now() WHERE id=$1`, s.ID)
		} else {
			_, err = st.Pool.Exec(ctx, `INSERT INTO story_reference_inventory_retirements(story_id,media_file_id,operation_id) VALUES($1,$2,$1)`, s.ID, media)
		}
		require.NoError(t, err)
	}
	_, err := st.Pool.Exec(ctx, `INSERT INTO story_media_reference_intents(story_id,media_file_id,available_at) VALUES($1,$2,now())`, uuid.New(), uuid.New())
	require.NoError(t, err)
	d, err := st.BeginReferenceInventory(ctx, &commonv1.BeginReferenceInventoryRequest{OperationId: uuid.NewString(), Identity: id})
	require.NoError(t, err)
	require.EqualValues(t, 4, d.Manifest.TotalCount, "an active owner conflicting with retirement must not disappear")
	require.EqualValues(t, 4, d.Manifest.UnresolvedCount, "text media, missing release obligation, retired active tuple and old unbound intent all block completeness")
}

func TestStoryInventorySnapshotWaitsForRollbackAndUsesCommittedCounter(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	st, id := inventoryStore(t)
	tx, err := st.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `UPDATE story_reference_inventory_gate SET revision=revision+1 WHERE singleton`)
	require.NoError(t, err)
	type result struct {
		descriptor *commonv1.ReferenceInventoryDescriptor
		err        error
	}
	done := make(chan result, 1)
	go func() {
		d, e := st.BeginReferenceInventory(ctx, &commonv1.BeginReferenceInventoryRequest{OperationId: uuid.NewString(), Identity: id})
		done <- result{d, e}
	}()
	require.Eventually(t, func() bool {
		var n int
		e := st.Pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%story_reference_inventory_gate%'`).Scan(&n)
		return e == nil && n > 0
	}, 5*time.Second, 20*time.Millisecond)
	require.NoError(t, tx.Rollback(ctx))
	r := <-done
	require.NoError(t, r.err)
	require.Zero(t, r.descriptor.Manifest.BoundaryRevision, "an allocated but aborted revision is not a snapshot cut")
	media := uuid.New()
	_, err = st.CreateStory(ctx, store.CreateStoryInput{AuthorProfileID: uuid.New(), Type: "photo", MediaFileID: &media, Visibility: "everyone"})
	require.NoError(t, err)
	changes, err := st.ReadReferenceInventoryChanges(ctx, id, 0, 10)
	require.NoError(t, err)
	require.Len(t, changes.Revisions, 1)
	require.EqualValues(t, 1, changes.Revisions[0].Revision)
	require.EqualValues(t, 1, changes.Revisions[0].FinalCount)
}

func TestStoryInventoryPagesAreContiguousAndCrossPageOrdered(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st, id := inventoryStore(t)
	_, err := st.Pool.Exec(ctx, `INSERT INTO stories(id,author_profile_id,type,media_file_id,visibility,expires_at,archived_until)
SELECT gen_random_uuid(),$1,'photo',gen_random_uuid(),'everyone',now()+interval '1 day',now()+interval '31 days' FROM generate_series(1,1001)`, uuid.New())
	require.NoError(t, err)
	d, err := st.BeginReferenceInventory(ctx, &commonv1.BeginReferenceInventoryRequest{OperationId: uuid.NewString(), Identity: id})
	require.NoError(t, err)
	require.EqualValues(t, 1001, d.Manifest.TotalCount)
	require.Len(t, d.Manifest.Commitments, 2)
	a, err := st.ReadReferenceInventoryPage(ctx, id, 0)
	require.NoError(t, err)
	b, err := st.ReadReferenceInventoryPage(ctx, id, 1)
	require.NoError(t, err)
	require.Len(t, a.Entries, 1000)
	require.Len(t, b.Entries, 1)
	require.NoError(t, referenceinventory.ValidateCommittedPage(d, a))
	require.NoError(t, referenceinventory.ValidateCommittedPage(d, b))
	require.Negative(t, referenceinventory.Compare(a.Entries[999], b.Entries[0]))
	_, err = st.ReadReferenceInventoryPage(ctx, id, 2)
	require.Error(t, err, "a missing page is not an empty declaration")
}

func TestStoryInventoryDownWaitsForWriterAndPreservesPendingOwnership(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	st, _ := inventoryStore(t)
	_, file, _, _ := runtime.Caller(0)
	down, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations", "story_db", "000008_reference_inventory.down.sql"))
	require.NoError(t, err)
	acquiring, settle := make(chan struct{}), make(chan struct{})
	created := make(chan error, 1)
	media := uuid.New()
	go func() {
		_, e := st.CreateStoryWithMedia(ctx, store.CreateStoryInput{AuthorProfileID: uuid.New(), Type: "photo", MediaFileID: &media, Visibility: "everyone"}, func(context.Context, uuid.UUID) error { close(acquiring); <-settle; return nil })
		created <- e
	}()
	<-acquiring
	rolledBack := make(chan error, 1)
	go func() { _, e := st.Pool.Exec(ctx, string(down)); rolledBack <- e }()
	require.Eventually(t, func() bool {
		var n int
		e := st.Pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%LOCK TABLE story_reference_inventory_gate%'`).Scan(&n)
		return e == nil && n > 0
	}, 5*time.Second, 20*time.Millisecond)
	close(settle)
	require.NoError(t, <-created)
	require.Error(t, <-rolledBack, "concurrent writer evidence cannot be dropped after an emptiness check")
	var count int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM story_reference_inventory_journal`).Scan(&count))
	require.Equal(t, 2, count)
	empty, _ := inventoryStore(t)
	_, err = empty.Pool.Exec(ctx, string(down))
	require.NoError(t, err, "unused quiescent migration can roll back")
}
