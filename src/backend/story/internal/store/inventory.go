package store

import (
	"bytes"
	"context"
	"errors"
	"sort"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	commonv1 "voice.app/voice/common/v1"
	"voice/backend/pkg/referenceinventory"
)

var ErrInventoryConflict = referenceinventory.ErrConflict
var ErrInventoryUnavailable = referenceinventory.ErrUnavailable

func inventoryInstalled(ctx context.Context, tx pgx.Tx) (bool, error) {
	var installed bool
	err := tx.QueryRow(ctx, `SELECT to_regclass('story_reference_inventory_gate') IS NOT NULL`).Scan(&installed)
	return installed, err
}

// The owner gate must precede every ownership row lock. Old schemas remain
// usable before migration installation, but cannot export or seal inventory.
func (s *StoryStore) beginInventoryTx(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	installed, err := inventoryInstalled(ctx, tx)
	if err == nil && installed {
		var present bool
		err = tx.QueryRow(ctx, `SELECT singleton FROM story_reference_inventory_gate WHERE singleton FOR UPDATE`).Scan(&present)
	}
	if err != nil {
		_ = tx.Rollback(ctx)
		return nil, err
	}
	return tx, nil
}

func inventoryItemKey(e *commonv1.ReferenceInventoryEntry) string {
	if e.Key != nil {
		return e.Key.FileId + "/" + e.Key.OwnerId
	}
	return "issue/" + e.GetIssueId()
}

func inventoryIssue(e *commonv1.ReferenceInventoryEntry, kind commonv1.ReferenceInventoryIssueKind) {
	if e.IssueKind > kind {
		return
	}
	e.IssueKind = kind
	id := uuid.NewSHA1(uuid.NameSpaceURL, []byte("voice.story.inventory.issue.v1:"+inventoryItemKey(e)))
	e.IssueId = proto.String(id.String())
}

func appendInventoryReason(e *commonv1.ReferenceInventoryEntry, reason commonv1.ReferenceInventoryProtectionReason) {
	for _, r := range e.Reasons {
		if r == reason {
			return
		}
	}
	e.Reasons = append(e.Reasons, reason)
	sort.Slice(e.Reasons, func(i, j int) bool { return e.Reasons[i] < e.Reasons[j] })
}

// loadInventory reads only identifiers and immutable media binding. It includes
// overdue/highlighted rows and old deleted rows without a proven retirement.
func loadInventory(ctx context.Context, tx pgx.Tx, owners, covers []uuid.UUID, all bool) (map[string]*commonv1.ReferenceInventoryEntry, error) {
	out := map[string]*commonv1.ReferenceInventoryEntry{}
	deletedWithoutReceipt := map[string]bool{}
	get := func(story, file uuid.UUID) *commonv1.ReferenceInventoryEntry {
		key := file.String() + "/" + story.String()
		if e := out[key]; e != nil {
			return e
		}
		e := &commonv1.ReferenceInventoryEntry{Key: &commonv1.ReferenceInventoryKey{FileId: file.String(), OwnerId: story.String(), OwnerType: commonv1.ReferenceInventoryOwnerType_REFERENCE_INVENTORY_OWNER_TYPE_STORY}}
		out[key] = e
		return e
	}
	rows, err := tx.Query(ctx, `SELECT s.id,s.media_file_id,s.author_profile_id,s.type,s.is_looking_for_party,s.deleted_at IS NOT NULL,
EXISTS(SELECT 1 FROM story_reference_inventory_retirements r WHERE r.story_id=s.id AND r.media_file_id=s.media_file_id)
FROM stories s WHERE s.media_file_id IS NOT NULL AND ($1 OR s.id=ANY($2::uuid[]))
AND (s.deleted_at IS NULL OR NOT EXISTS(SELECT 1 FROM story_reference_inventory_retirements r WHERE r.story_id=s.id AND r.media_file_id=s.media_file_id))`, all, owners)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var story, file, author uuid.UUID
		var media string
		var lfp, deleted, retired bool
		if err = rows.Scan(&story, &file, &author, &media, &lfp, &deleted, &retired); err != nil {
			rows.Close()
			return nil, err
		}
		e := get(story, file)
		e.AuthorProfileId = proto.String(author.String())
		e.MediaType = media
		if deleted {
			appendInventoryReason(e, commonv1.ReferenceInventoryProtectionReason_REFERENCE_INVENTORY_PROTECTION_REASON_RELEASE_PENDING)
			deletedWithoutReceipt[inventoryItemKey(e)] = true
		} else {
			appendInventoryReason(e, commonv1.ReferenceInventoryProtectionReason_REFERENCE_INVENTORY_PROTECTION_REASON_RETAINED)
		}
		if lfp || (media != "photo" && media != "video") {
			inventoryIssue(e, commonv1.ReferenceInventoryIssueKind_REFERENCE_INVENTORY_ISSUE_KIND_HISTORICAL_MEDIA_ANOMALY)
		}
		if retired {
			inventoryIssue(e, commonv1.ReferenceInventoryIssueKind_REFERENCE_INVENTORY_ISSUE_KIND_CONFLICTING_BINDING)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = tx.Query(ctx, `SELECT i.story_id,i.media_file_id,i.author_profile_id,i.media_type,
EXISTS(SELECT 1 FROM story_reference_inventory_retirements r WHERE r.story_id=i.story_id AND r.media_file_id=i.media_file_id)
FROM story_media_reference_intents i WHERE $1 OR i.story_id=ANY($2::uuid[])`, all, owners)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var story, file uuid.UUID
		var author *uuid.UUID
		var media *string
		var retired bool
		if err = rows.Scan(&story, &file, &author, &media, &retired); err != nil {
			rows.Close()
			return nil, err
		}
		e := get(story, file)
		appendInventoryReason(e, commonv1.ReferenceInventoryProtectionReason_REFERENCE_INVENTORY_PROTECTION_REASON_CREATE_PENDING)
		if author != nil {
			if e.AuthorProfileId != nil && e.GetAuthorProfileId() != author.String() {
				inventoryIssue(e, commonv1.ReferenceInventoryIssueKind_REFERENCE_INVENTORY_ISSUE_KIND_CONFLICTING_BINDING)
			}
			e.AuthorProfileId = proto.String(author.String())
		}
		if media != nil {
			if e.MediaType != "" && e.MediaType != *media {
				inventoryIssue(e, commonv1.ReferenceInventoryIssueKind_REFERENCE_INVENTORY_ISSUE_KIND_CONFLICTING_BINDING)
			}
			e.MediaType = *media
		}
		if author == nil || media == nil || (*media != "photo" && *media != "video") {
			inventoryIssue(e, commonv1.ReferenceInventoryIssueKind_REFERENCE_INVENTORY_ISSUE_KIND_HISTORICAL_MEDIA_ANOMALY)
		}
		if retired {
			inventoryIssue(e, commonv1.ReferenceInventoryIssueKind_REFERENCE_INVENTORY_ISSUE_KIND_CONFLICTING_BINDING)
		}
		e.SourceOperationIds = append(e.SourceOperationIds, uuid.NewSHA1(uuid.NameSpaceURL, []byte("voice.story.media.acquire.v1:"+story.String())).String())
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = tx.Query(ctx, `SELECT o.story_id,o.media_file_id,o.operation_id,
EXISTS(SELECT 1 FROM story_reference_inventory_retirements r WHERE r.story_id=o.story_id AND r.media_file_id=o.media_file_id)
FROM story_media_deletion_outbox o WHERE $1 OR o.story_id=ANY($2::uuid[])`, all, owners)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var story, file, op uuid.UUID
		var retired bool
		if err = rows.Scan(&story, &file, &op, &retired); err != nil {
			rows.Close()
			return nil, err
		}
		e := get(story, file)
		appendInventoryReason(e, commonv1.ReferenceInventoryProtectionReason_REFERENCE_INVENTORY_PROTECTION_REASON_RELEASE_PENDING)
		e.SourceOperationIds = append(e.SourceOperationIds, op.String())
		if retired {
			inventoryIssue(e, commonv1.ReferenceInventoryIssueKind_REFERENCE_INVENTORY_ISSUE_KIND_CONFLICTING_BINDING)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	// Cover ownership has no accepted independent tuple; count it rather than
	// synthesizing STORY owner_id=highlight_id. A linked retained Story can pin it.
	rows, err = tx.Query(ctx, `SELECT h.id,h.cover_file_id FROM highlights h WHERE h.cover_file_id IS NOT NULL AND ($1 OR h.id=ANY($2::uuid[])) AND NOT EXISTS(
SELECT 1 FROM highlight_stories hs JOIN stories s ON s.id=hs.story_id
WHERE hs.highlight_id=h.id AND s.media_file_id=h.cover_file_id AND s.deleted_at IS NULL AND s.type IN('photo','video') AND NOT s.is_looking_for_party
AND NOT EXISTS(SELECT 1 FROM story_reference_inventory_retirements r WHERE r.story_id=s.id AND r.media_file_id=s.media_file_id))`, all, covers)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var issue, file uuid.UUID
		if err = rows.Scan(&issue, &file); err != nil {
			rows.Close()
			return nil, err
		}
		e := &commonv1.ReferenceInventoryEntry{IssueId: proto.String(issue.String()), IssueKind: commonv1.ReferenceInventoryIssueKind_REFERENCE_INVENTORY_ISSUE_KIND_HIGHLIGHT_COVER_UNRESOLVED, ObservedFileId: proto.String(file.String())}
		out[inventoryItemKey(e)] = e
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	bindings := map[string]string{}
	for _, e := range out {
		if e.Key == nil {
			continue
		}
		if deletedWithoutReceipt[inventoryItemKey(e)] && len(e.SourceOperationIds) == 0 {
			inventoryIssue(e, commonv1.ReferenceInventoryIssueKind_REFERENCE_INVENTORY_ISSUE_KIND_HISTORICAL_MEDIA_ANOMALY)
		}
		if prev, ok := bindings[e.Key.OwnerId]; ok && prev != e.Key.FileId {
			for _, other := range out {
				if other.Key != nil && other.Key.OwnerId == e.Key.OwnerId {
					inventoryIssue(other, commonv1.ReferenceInventoryIssueKind_REFERENCE_INVENTORY_ISSUE_KIND_CONFLICTING_BINDING)
				}
			}
		}
		bindings[e.Key.OwnerId] = e.Key.FileId
		sort.Strings(e.SourceOperationIds)
		if len(e.SourceOperationIds) > 1 {
			dedup := e.SourceOperationIds[:0]
			for _, op := range e.SourceOperationIds {
				if len(dedup) == 0 || dedup[len(dedup)-1] != op {
					dedup = append(dedup, op)
				}
			}
			e.SourceOperationIds = dedup
		}
	}
	return out, nil
}

func saveInventoryState(ctx context.Context, tx pgx.Tx, e *commonv1.ReferenceInventoryEntry) error {
	raw, _, err := referenceinventory.Encode(e)
	if err != nil {
		return err
	}
	var owner *string
	if e.Key != nil {
		owner = proto.String(e.Key.OwnerId)
	}
	_, err = tx.Exec(ctx, `INSERT INTO story_reference_inventory_state(item_key,owner_id,is_cover,entry_bytes) VALUES($1,$2,$3,$4) ON CONFLICT(item_key) DO UPDATE SET entry_bytes=EXCLUDED.entry_bytes`, inventoryItemKey(e), owner, e.Key == nil, raw)
	return err
}

// recordInventory commits complete absolute assignments with the owner's
// transactional counter, including an explicit final count for empty revisions.
func recordInventory(ctx context.Context, tx pgx.Tx, owners []uuid.UUID, changedHighlights ...uuid.UUID) error {
	installed, err := inventoryInstalled(ctx, tx)
	if err != nil || !installed {
		return err
	}
	var revision uint64
	var capture bool
	if err = tx.QueryRow(ctx, `UPDATE story_reference_inventory_gate SET revision=revision+1 WHERE singleton RETURNING revision,capture_started`).Scan(&revision, &capture); err != nil {
		return err
	}
	// The gate/counter precede the first snapshot, but raw identifier history
	// starts only at its committed cut. Capture cannot be disabled by config.
	if !capture {
		return nil
	}
	rows, err := tx.Query(ctx, `SELECT DISTINCT highlight_id FROM highlight_stories WHERE story_id=ANY($1::uuid[]) UNION SELECT unnest($2::uuid[])`, owners, changedHighlights)
	if err != nil {
		return err
	}
	var covers []uuid.UUID
	var coverKeys []string
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		covers = append(covers, id)
		coverKeys = append(coverKeys, "issue/"+id.String())
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	current, err := loadInventory(ctx, tx, owners, covers, false)
	if err != nil {
		return err
	}
	rows, err = tx.Query(ctx, `SELECT item_key,entry_bytes FROM story_reference_inventory_state WHERE owner_id=ANY($1::uuid[]) OR item_key=ANY($2::text[])`, owners, coverKeys)
	if err != nil {
		return err
	}
	for rows.Next() {
		var key string
		var raw []byte
		if err = rows.Scan(&key, &raw); err != nil {
			rows.Close()
			return err
		}
		if current[key] != nil {
			continue
		}
		e := &commonv1.ReferenceInventoryEntry{}
		if err = proto.Unmarshal(raw, e); err != nil {
			rows.Close()
			return err
		}
		if !e.Retired {
			e.Retired = true
			e.Reasons = nil
			current[key] = e
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	change := &commonv1.ReferenceInventoryRevision{Revision: revision}
	for _, e := range current {
		e.OwnerRevision = revision
		if referenceinventory.ValidateEntry(e, true) != nil {
			return referenceinventory.ErrInvalid
		}
		change.Entries = append(change.Entries, e)
	}
	sort.Slice(change.Entries, func(i, j int) bool { return referenceinventory.Compare(change.Entries[i], change.Entries[j]) < 0 })
	change.FinalCount = uint32(len(change.Entries))
	for _, e := range change.Entries {
		if err = saveInventoryState(ctx, tx, e); err != nil {
			return err
		}
	}
	raw, _, err := referenceinventory.Encode(change)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO story_reference_inventory_journal(revision,revision_bytes) VALUES($1,$2)`, revision, raw)
	return err
}

func (s *StoryStore) BeginReferenceInventory(ctx context.Context, req *commonv1.BeginReferenceInventoryRequest) (*commonv1.ReferenceInventoryDescriptor, error) {
	if referenceinventory.Strict(req) != nil || referenceinventory.ValidateIdentity(req.GetIdentity()) != nil || !referenceinventory.UUID(req.GetOperationId()) {
		return nil, referenceinventory.ErrInvalid
	}
	if s == nil || s.Pool == nil {
		return nil, ErrInventoryUnavailable
	}
	tx, err := s.beginInventoryTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	installed, err := inventoryInstalled(ctx, tx)
	if err != nil {
		return nil, err
	}
	if !installed {
		return nil, ErrInventoryUnavailable
	}
	var incarnation string
	var boundary uint64
	if err = tx.QueryRow(ctx, `SELECT owner_incarnation_id,revision FROM story_reference_inventory_gate WHERE singleton`).Scan(&incarnation, &boundary); err != nil {
		return nil, err
	}
	if incarnation != req.Identity.OwnerIncarnationId {
		return nil, ErrInventoryConflict
	}
	requestBytes, _, err := referenceinventory.Encode(req)
	if err != nil {
		return nil, err
	}
	var saved, descriptor []byte
	err = tx.QueryRow(ctx, `SELECT request_bytes,descriptor_bytes FROM story_reference_inventory_snapshots WHERE operation_id=$1`, req.OperationId).Scan(&saved, &descriptor)
	if err == nil {
		if !bytes.Equal(saved, requestBytes) {
			return nil, ErrInventoryConflict
		}
		d := &commonv1.ReferenceInventoryDescriptor{}
		if err = proto.Unmarshal(descriptor, d); err != nil {
			return nil, err
		}
		if referenceinventory.ValidateDescriptor(d) != nil {
			return nil, ErrInventoryUnavailable
		}
		return d, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM story_reference_inventory_snapshots WHERE snapshot_id=$1)`, req.Identity.SnapshotId).Scan(&exists); err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrInventoryConflict
	}
	entries, err := loadInventory(ctx, tx, nil, nil, true)
	if err != nil {
		return nil, err
	}
	ordered := make([]*commonv1.ReferenceInventoryEntry, 0, len(entries))
	for _, e := range entries {
		e.OwnerRevision = boundary
		ordered = append(ordered, e)
	}
	sort.Slice(ordered, func(i, j int) bool { return referenceinventory.Compare(ordered[i], ordered[j]) < 0 })
	var pages []*commonv1.ReferenceInventoryPage
	for i := 0; i < len(ordered); i += referenceinventory.PageSize {
		end := i + referenceinventory.PageSize
		if end > len(ordered) {
			end = len(ordered)
		}
		pages = append(pages, &commonv1.ReferenceInventoryPage{Identity: proto.Clone(req.Identity).(*commonv1.ReferenceInventoryIdentity), BoundaryRevision: boundary, PageIndex: uint32(len(pages)), Entries: ordered[i:end]})
	}
	d, err := referenceinventory.Describe(pages, req.Identity, boundary)
	if err != nil {
		return nil, err
	}
	identityBytes, _, err := referenceinventory.Encode(req.Identity)
	if err != nil {
		return nil, err
	}
	descriptor, _, err = referenceinventory.Encode(d)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO story_reference_inventory_snapshots(snapshot_id,operation_id,identity_bytes,request_bytes,descriptor_bytes,boundary_revision) VALUES($1,$2,$3,$4,$5,$6)`, req.Identity.SnapshotId, req.OperationId, identityBytes, requestBytes, descriptor, boundary)
	if err != nil {
		return nil, err
	}
	for _, p := range pages {
		raw, _, e := referenceinventory.Encode(p)
		if e != nil {
			return nil, e
		}
		if _, err = tx.Exec(ctx, `INSERT INTO story_reference_inventory_pages(snapshot_id,page_index,page_bytes) VALUES($1,$2,$3)`, req.Identity.SnapshotId, p.PageIndex, raw); err != nil {
			return nil, err
		}
	}
	for _, e := range ordered {
		if err = saveInventoryState(ctx, tx, e); err != nil {
			return nil, err
		}
	}
	// Baseline state derives from authoritative rows at B, never a pre-cut
	// cache. The same writer gate makes capture of every B+1 revision atomic.
	if _, err = tx.Exec(ctx, `UPDATE story_reference_inventory_gate SET capture_started=true WHERE singleton`); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return d, nil
}

func inventorySnapshotIdentity(ctx context.Context, tx pgx.Tx, id *commonv1.ReferenceInventoryIdentity) error {
	if referenceinventory.ValidateIdentity(id) != nil {
		return referenceinventory.ErrInvalid
	}
	var raw []byte
	if err := tx.QueryRow(ctx, `SELECT identity_bytes FROM story_reference_inventory_snapshots WHERE snapshot_id=$1`, id.SnapshotId).Scan(&raw); err != nil {
		return err
	}
	want, _, err := referenceinventory.Encode(id)
	if err != nil {
		return err
	}
	if !bytes.Equal(raw, want) {
		return ErrInventoryConflict
	}
	return nil
}

func (s *StoryStore) ReadReferenceInventoryPage(ctx context.Context, id *commonv1.ReferenceInventoryIdentity, index uint32) (*commonv1.ReferenceInventoryPage, error) {
	if referenceinventory.ValidateIdentity(id) != nil {
		return nil, referenceinventory.ErrInvalid
	}
	if s == nil || s.Pool == nil {
		return nil, ErrInventoryUnavailable
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var incarnation string
	if err = tx.QueryRow(ctx, `SELECT owner_incarnation_id FROM story_reference_inventory_gate WHERE singleton FOR SHARE`).Scan(&incarnation); err != nil {
		return nil, err
	}
	if incarnation != id.OwnerIncarnationId {
		return nil, ErrInventoryConflict
	}
	if err = inventorySnapshotIdentity(ctx, tx, id); err != nil {
		return nil, err
	}
	var raw []byte
	if err = tx.QueryRow(ctx, `SELECT page_bytes FROM story_reference_inventory_pages WHERE snapshot_id=$1 AND page_index=$2`, id.SnapshotId, index).Scan(&raw); err != nil {
		return nil, err
	}
	p := &commonv1.ReferenceInventoryPage{}
	if err = proto.Unmarshal(raw, p); err != nil {
		return nil, err
	}
	if referenceinventory.ValidateStoryPage(p) != nil || !proto.Equal(p.Identity, id) || p.PageIndex != index {
		return nil, ErrInventoryUnavailable
	}
	return p, nil
}

func (s *StoryStore) ReadReferenceInventoryChanges(ctx context.Context, id *commonv1.ReferenceInventoryIdentity, after uint64, limit uint32) (*commonv1.ReadReferenceInventoryChangesResponse, error) {
	if referenceinventory.ValidateIdentity(id) != nil || limit == 0 || limit > referenceinventory.PageSize {
		return nil, referenceinventory.ErrInvalid
	}
	if s == nil || s.Pool == nil {
		return nil, ErrInventoryUnavailable
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = inventorySnapshotIdentity(ctx, tx, id); err != nil {
		return nil, err
	}
	var high, boundary uint64
	var incarnation string
	if err = tx.QueryRow(ctx, `SELECT revision,owner_incarnation_id FROM story_reference_inventory_gate WHERE singleton FOR SHARE`).Scan(&high, &incarnation); err != nil {
		return nil, err
	}
	if incarnation != id.OwnerIncarnationId {
		return nil, ErrInventoryConflict
	}
	if err = tx.QueryRow(ctx, `SELECT boundary_revision FROM story_reference_inventory_snapshots WHERE snapshot_id=$1`, id.SnapshotId).Scan(&boundary); err != nil {
		return nil, err
	}
	if after < boundary || after > high {
		return nil, referenceinventory.ErrInvalid
	}
	r := &commonv1.ReadReferenceInventoryChangesResponse{Identity: proto.Clone(id).(*commonv1.ReferenceInventoryIdentity), HighWatermark: high}
	rows, err := tx.Query(ctx, `SELECT revision_bytes FROM story_reference_inventory_journal WHERE revision>$1 ORDER BY revision LIMIT $2`, after, limit)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			rows.Close()
			return nil, err
		}
		revision := &commonv1.ReferenceInventoryRevision{}
		if err = proto.Unmarshal(raw, revision); err != nil {
			rows.Close()
			return nil, err
		}
		r.Revisions = append(r.Revisions, revision)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if err = referenceinventory.ValidateChanges(r, id, after); err != nil {
		return nil, err
	}
	return r, nil
}
