package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	commonv1 "voice.app/voice/common/v1"
	inv "voice/backend/pkg/referenceinventory"
)

type inventoryCoverageComponent struct {
	Service, Component string
	Owners             []commonv1.ReferenceInventoryOwnerType
	WriterVersion      uint32
	KeyShapeKnown      bool
}

// This compiled registry is not a seal or a caller-selected declaration set.
// Enum coverage does not settle catalog retention or outstanding storage keys.
// In particular the catalog source role remains unknown, rather than being
// inferred from a component label or from Messaging's usage tuple ownership.
var requiredInventoryRegistry = []inventoryCoverageComponent{
	{"story", "story.media.v1", []commonv1.ReferenceInventoryOwnerType{2}, 1, true},
	{"user", "user.profile-avatar.v1", []commonv1.ReferenceInventoryOwnerType{3}, 1, true},
	{"chat", "chat.chat-avatar.v1", []commonv1.ReferenceInventoryOwnerType{4}, 1, true},
	{"space", "space.space-avatar.v1", []commonv1.ReferenceInventoryOwnerType{5}, 1, true},
	{"messaging", "messaging.message.v1", []commonv1.ReferenceInventoryOwnerType{1}, 1, true},
	{"messaging", "messaging.sticker-usage.v1", []commonv1.ReferenceInventoryOwnerType{6}, 1, true},
	{"messaging", "messaging.gif-usage.v1", []commonv1.ReferenceInventoryOwnerType{7}, 1, true},
	{"file", "file.upload-session.v1", []commonv1.ReferenceInventoryOwnerType{8}, 1, true},
	{"", "file.catalog-retention.v1", nil, 1, false},
	{"file", "file.pending-writes.v1", nil, 1, false},
}

func inventoryRegistryEncoding(registry []inventoryCoverageComponent) []byte {
	var b bytes.Buffer
	b.WriteString("voice.file.reference-inventory.coverage.v1\x00")
	var n [4]byte
	for _, c := range registry {
		b.WriteString(c.Service)
		b.WriteByte(0)
		b.WriteString(c.Component)
		b.WriteByte(0)
		binary.BigEndian.PutUint32(n[:], c.WriterVersion)
		b.Write(n[:])
		binary.BigEndian.PutUint32(n[:], uint32(len(c.Owners)))
		b.Write(n[:])
		for _, owner := range c.Owners {
			binary.BigEndian.PutUint32(n[:], uint32(owner))
			b.Write(n[:])
		}
		if c.KeyShapeKnown {
			b.WriteByte(1)
		} else {
			b.WriteByte(0)
		}
	}
	return b.Bytes()
}

type InventoryOperation struct {
	OperationID uuid.UUID
	Identity    *commonv1.ReferenceInventoryIdentity
	Descriptor  *commonv1.ReferenceInventoryDescriptor
	NextPage    uint32
	Cursor      uint64
	State       string
}

// CreateInventoryOperation is private controller composition, not an RPC or a
// restore/drain attestation. Supplied identities MUST come from trusted recovery
// control; this slice neither creates them at startup nor activates a cohort.
// The barrier commits BEFORE any owner snapshot is requested.
func (s *FilesStore) CreateInventoryOperation(ctx context.Context, op uuid.UUID, id *commonv1.ReferenceInventoryIdentity) error {
	if s == nil || s.Pool == nil {
		return inv.ErrUnavailable
	}
	if op == uuid.Nil || inv.ValidateIdentity(id) != nil {
		return inv.ErrInvalid
	}
	raw, _, err := inv.Encode(id)
	if err != nil {
		return err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var blocked bool
	if err = tx.QueryRow(ctx, `SELECT blocked FROM file_reference_inventory_gate WHERE singleton FOR UPDATE`).Scan(&blocked); err != nil {
		return err
	}
	// The exclusive local guard alone cannot fence a request after connection
	// loss. Refuse a new proof window while a prior storage attempt is unknown.
	var unsettled bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM file_reference_inventory_deletes WHERE state IN ('STARTED','UNSETTLED'))`).Scan(&unsettled); err != nil {
		return err
	}
	if unsettled {
		return ErrInventoryBarrier
	}
	var existing []byte
	err = tx.QueryRow(ctx, `SELECT identity_bytes FROM file_reference_inventory_imports WHERE operation_id=$1`, op).Scan(&existing)
	if err == nil {
		if !bytes.Equal(raw, existing) {
			return blockInventoryTx(ctx, tx, op, "operation identity changed")
		}
		return tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	var alreadyBound bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM file_reference_inventory_imports WHERE migration_id=$1 AND component=$2)`, id.MigrationId, id.SourceComponent).Scan(&alreadyBound); err != nil {
		return err
	}
	if alreadyBound {
		return inv.ErrConflict
	}
	// Only one current migration; invalidation does not reopen destructive work.
	var conflicting bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM file_reference_inventory_cohorts WHERE (state='MIGRATING' AND (migration_id<>$1 OR installation_id<>$2 OR restore_cohort_id<>$3 OR file_incarnation_id<>$4)) OR (migration_id=$1 AND state<>'MIGRATING'))`, id.MigrationId, id.InstallationId, id.RestoreCohortId, id.FileIncarnationId).Scan(&conflicting); err != nil {
		return err
	}
	if conflicting {
		return inv.ErrConflict
	}
	digest := sha256.Sum256(inventoryRegistryEncoding(requiredInventoryRegistry))
	if _, err = tx.Exec(ctx, `INSERT INTO file_reference_inventory_cohorts(migration_id,installation_id,restore_cohort_id,file_incarnation_id,state,registry_sha256) VALUES($1,$2,$3,$4,'MIGRATING',$5) ON CONFLICT DO NOTHING`, id.MigrationId, id.InstallationId, id.RestoreCohortId, id.FileIncarnationId, digest[:]); err != nil {
		return err
	}
	for _, component := range requiredInventoryRegistry {
		if _, err = tx.Exec(ctx, `INSERT INTO file_reference_inventory_components(migration_id,component) VALUES($1,$2) ON CONFLICT DO NOTHING`, id.MigrationId, component.Component); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO file_reference_inventory_imports(operation_id,migration_id,snapshot_id,identity_bytes,component) VALUES($1,$2,$3,$4,$5)`, op, id.MigrationId, id.SnapshotId, raw, id.SourceComponent); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE file_reference_inventory_gate SET blocked=true,generation=generation+1 WHERE singleton`); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *FilesStore) LoadInventoryOperation(ctx context.Context, op uuid.UUID) (InventoryOperation, error) {
	var result InventoryOperation
	var id, descriptor []byte
	var next, cursor int64
	result.OperationID = op
	err := s.Pool.QueryRow(ctx, `SELECT identity_bytes,descriptor_bytes,next_page,cursor_revision,state FROM file_reference_inventory_imports WHERE operation_id=$1`, op).Scan(&id, &descriptor, &next, &cursor, &result.State)
	if err != nil {
		return result, err
	}
	result.Identity = &commonv1.ReferenceInventoryIdentity{}
	if err = proto.Unmarshal(id, result.Identity); err != nil {
		return result, err
	}
	if descriptor != nil {
		result.Descriptor = &commonv1.ReferenceInventoryDescriptor{}
		if err = proto.Unmarshal(descriptor, result.Descriptor); err != nil {
			return result, err
		}
	}
	result.NextPage, result.Cursor = uint32(next), uint64(cursor)
	return result, nil
}

func (s *FilesStore) inventoryTx(ctx context.Context, op uuid.UUID) (pgx.Tx, InventoryOperation, error) {
	var empty InventoryOperation
	tx, err := BeginInventoryMutation(ctx, s.Pool, false)
	if err != nil {
		return nil, empty, err
	}
	fail := func(e error) (pgx.Tx, InventoryOperation, error) {
		_ = tx.Rollback(context.Background())
		return nil, empty, e
	}
	var state string
	if err = tx.QueryRow(ctx, `SELECT c.state FROM file_reference_inventory_imports i JOIN file_reference_inventory_cohorts c USING(migration_id) WHERE i.operation_id=$1 FOR UPDATE OF i`, op).Scan(&state); err != nil {
		return fail(err)
	}
	if state != "MIGRATING" {
		return fail(inv.ErrConflict)
	}
	// Load through this same transaction, never a second pooled connection.
	var id, d []byte
	var next, cursor int64
	result := InventoryOperation{OperationID: op}
	if err = tx.QueryRow(ctx, `SELECT identity_bytes,descriptor_bytes,next_page,cursor_revision,state FROM file_reference_inventory_imports WHERE operation_id=$1`, op).Scan(&id, &d, &next, &cursor, &result.State); err != nil {
		return fail(err)
	}
	result.Identity = &commonv1.ReferenceInventoryIdentity{}
	if err = proto.Unmarshal(id, result.Identity); err != nil {
		return fail(err)
	}
	if d != nil {
		result.Descriptor = &commonv1.ReferenceInventoryDescriptor{}
		if err = proto.Unmarshal(d, result.Descriptor); err != nil {
			return fail(err)
		}
	}
	result.NextPage, result.Cursor = uint32(next), uint64(cursor)
	if result.State == "BLOCKED" {
		return fail(inv.ErrConflict)
	}
	return tx, result, nil
}

func blockInventoryTx(ctx context.Context, tx pgx.Tx, op uuid.UUID, reason string) error {
	if _, err := tx.Exec(ctx, `UPDATE file_reference_inventory_imports SET state='BLOCKED',blocked_reason=$2 WHERE operation_id=$1`, op, reason); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE file_reference_inventory_components SET state='BLOCKED' WHERE (migration_id,component)=(SELECT migration_id,component FROM file_reference_inventory_imports WHERE operation_id=$1)`, op); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return inv.ErrConflict
}
func (s *FilesStore) BlockInventoryOperation(ctx context.Context, op uuid.UUID, reason string) error {
	tx, _, err := s.inventoryTx(ctx, op)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	return blockInventoryTx(ctx, tx, op, reason)
}

func (s *FilesStore) SaveInventoryDescriptor(ctx context.Context, op uuid.UUID, d *commonv1.ReferenceInventoryDescriptor) error {
	tx, o, err := s.inventoryTx(ctx, op)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if inv.ValidateDescriptor(d) != nil || !proto.Equal(o.Identity, d.GetManifest().GetIdentity()) || d.GetManifest().GetBoundaryRevision() > math.MaxInt64 {
		return blockInventoryTx(ctx, tx, op, "descriptor mismatch")
	}
	raw, _, err := inv.Encode(d)
	if err != nil {
		return err
	}
	if o.Descriptor != nil {
		old, _, e := inv.Encode(o.Descriptor)
		if e != nil {
			return e
		}
		if !bytes.Equal(old, raw) {
			return blockInventoryTx(ctx, tx, op, "descriptor changed")
		}
		return tx.Commit(ctx)
	}
	state := "SNAPSHOT"
	if len(d.Manifest.Commitments) == 0 {
		state = "CATCHING_UP"
	}
	if _, err = tx.Exec(ctx, `UPDATE file_reference_inventory_imports SET descriptor_bytes=$2,cursor_revision=$3,state=$4 WHERE operation_id=$1`, op, raw, int64(d.Manifest.BoundaryRevision), state); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE file_reference_inventory_components SET state=$2 WHERE migration_id=$1 AND component=$3`, o.Identity.MigrationId, state, o.Identity.SourceComponent); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func inventoryReceipt(ctx context.Context, tx pgx.Tx, op uuid.UUID, kind string, position uint64, raw []byte) (bool, error) {
	var old []byte
	err := tx.QueryRow(ctx, `SELECT payload_bytes FROM file_reference_inventory_receipts WHERE operation_id=$1 AND kind=$2 AND position=$3`, op, kind, int64(position)).Scan(&old)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !bytes.Equal(old, raw) {
		return true, inv.ErrConflict
	}
	return true, nil
}

func (s *FilesStore) ApplyInventoryPage(ctx context.Context, op uuid.UUID, p *commonv1.ReferenceInventoryPage) error {
	tx, o, err := s.inventoryTx(ctx, op)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if inv.ValidateCommittedPage(o.Descriptor, p) != nil {
		return blockInventoryTx(ctx, tx, op, "page commitment mismatch")
	}
	raw, _, err := inv.Encode(p)
	if err != nil {
		return err
	}
	found, err := inventoryReceipt(ctx, tx, op, "PAGE", uint64(p.PageIndex), raw)
	if errors.Is(err, inv.ErrConflict) {
		return blockInventoryTx(ctx, tx, op, "page changed")
	}
	if err != nil {
		return err
	}
	if found {
		return tx.Commit(ctx)
	}
	if o.State != "SNAPSHOT" || p.PageIndex != o.NextPage {
		return blockInventoryTx(ctx, tx, op, "page gap")
	}
	var last []byte
	if err = tx.QueryRow(ctx, `SELECT last_entry_bytes FROM file_reference_inventory_imports WHERE operation_id=$1`, op).Scan(&last); err != nil {
		return err
	}
	if last != nil {
		e := &commonv1.ReferenceInventoryEntry{}
		if err = proto.Unmarshal(last, e); err != nil {
			return err
		}
		if inv.Compare(e, p.Entries[0]) >= 0 {
			return blockInventoryTx(ctx, tx, op, "page overlap")
		}
	}
	if err = applyInventoryEntriesAtomic(ctx, tx, op, p.Entries); err != nil {
		if errors.Is(err, inv.ErrConflict) || errors.Is(err, inv.ErrInvalid) {
			return blockInventoryTx(ctx, tx, op, "entry revision conflict")
		}
		return err
	}
	var unresolved int64
	for _, e := range p.Entries {
		if e.IssueKind != 0 {
			unresolved++
		}
	}
	last, _, err = inv.Encode(p.Entries[len(p.Entries)-1])
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO file_reference_inventory_receipts VALUES($1,'PAGE',$2,$3)`, op, int64(p.PageIndex), raw); err != nil {
		return err
	}
	var total, issues int64
	if err = tx.QueryRow(ctx, `UPDATE file_reference_inventory_imports SET next_page=next_page+1,item_count=item_count+$2,source_unresolved_count=source_unresolved_count+$3,last_entry_bytes=$4 WHERE operation_id=$1 RETURNING item_count,source_unresolved_count`, op, len(p.Entries), unresolved, last).Scan(&total, &issues); err != nil {
		return err
	}
	if int(o.NextPage)+1 == len(o.Descriptor.Manifest.Commitments) {
		if uint64(total) != o.Descriptor.Manifest.TotalCount || uint64(issues) != o.Descriptor.Manifest.UnresolvedCount {
			return blockInventoryTx(ctx, tx, op, "snapshot count mismatch")
		}
		if _, err = tx.Exec(ctx, `UPDATE file_reference_inventory_imports SET state='CATCHING_UP' WHERE operation_id=$1`, op); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE file_reference_inventory_components SET state='CATCHING_UP' WHERE migration_id=$1 AND component=$2`, o.Identity.MigrationId, o.Identity.SourceComponent); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *FilesStore) ApplyInventoryChanges(ctx context.Context, op uuid.UUID, after uint64, r *commonv1.ReadReferenceInventoryChangesResponse) error {
	tx, o, err := s.inventoryTx(ctx, op)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if o.State != "CATCHING_UP" || inv.ValidateChanges(r, o.Identity, after) != nil || r.GetHighWatermark() > math.MaxInt64 {
		return blockInventoryTx(ctx, tx, op, "journal gap or binding mismatch")
	}
	for _, revision := range r.Revisions {
		raw, _, e := inv.Encode(revision)
		if e != nil {
			return e
		}
		found, e := inventoryReceipt(ctx, tx, op, "REVISION", revision.Revision, raw)
		if errors.Is(e, inv.ErrConflict) {
			return blockInventoryTx(ctx, tx, op, "revision changed")
		}
		if e != nil {
			return e
		}
		if found {
			continue
		}
		if revision.Revision != o.Cursor+1 {
			return blockInventoryTx(ctx, tx, op, "revision gap")
		}
		if e = applyInventoryEntriesAtomic(ctx, tx, op, revision.Entries); e != nil {
			if errors.Is(e, inv.ErrConflict) || errors.Is(e, inv.ErrInvalid) {
				return blockInventoryTx(ctx, tx, op, "entry revision conflict")
			}
			return e
		}
		if _, e = tx.Exec(ctx, `INSERT INTO file_reference_inventory_receipts VALUES($1,'REVISION',$2,$3)`, op, int64(revision.Revision), raw); e != nil {
			return e
		}
		o.Cursor = revision.Revision
	}
	if after > o.Cursor {
		return blockInventoryTx(ctx, tx, op, "cursor advanced without receipt")
	}
	if _, err = tx.Exec(ctx, `UPDATE file_reference_inventory_imports SET cursor_revision=$2 WHERE operation_id=$1`, op, int64(o.Cursor)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func inventoryItemKey(e *commonv1.ReferenceInventoryEntry) string {
	if e.Key == nil {
		return "issue:" + e.GetIssueId()
	}
	optional := func(s *string) string {
		if s == nil {
			return "-"
		}
		return *s
	}
	return "key:" + e.Key.FileId + "/" + strconv.FormatInt(int64(e.Key.OwnerType), 10) + "/" + e.Key.OwnerId + "/" + optional(e.Key.SubresourceId) + "/" + optional(e.Key.ScopeSpaceId)
}

// All File locks precede all sorted blob locks. Neither hold creation nor a
// higher source revision changes File state, ACL, uploader or retired refs.
func applyInventoryEntriesAtomic(ctx context.Context, parent pgx.Tx, op uuid.UUID, entries []*commonv1.ReferenceInventoryEntry) error {
	tx, err := parent.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err = applyInventoryEntries(ctx, tx, op, entries); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func applyInventoryEntries(ctx context.Context, tx pgx.Tx, op uuid.UUID, entries []*commonv1.ReferenceInventoryEntry) error {
	files := map[string]bool{}
	for _, e := range entries {
		if e.OwnerRevision > math.MaxInt64 {
			return inv.ErrInvalid
		}
		f := e.GetObservedFileId()
		if e.Key != nil {
			f = e.Key.FileId
		}
		if f != "" {
			files[f] = true
		}
	}
	ordered := make([]string, 0, len(files))
	for f := range files {
		ordered = append(ordered, f)
	}
	sort.Strings(ordered)
	blobs := map[uuid.UUID]bool{}
	for _, f := range ordered {
		var blob *uuid.UUID
		err := tx.QueryRow(ctx, `SELECT blob_id FROM files WHERE id=$1 FOR NO KEY UPDATE`, f).Scan(&blob)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		if blob != nil {
			blobs[*blob] = true
		}
	}
	orderedBlobs := make([]uuid.UUID, 0, len(blobs))
	for b := range blobs {
		orderedBlobs = append(orderedBlobs, b)
	}
	sort.Slice(orderedBlobs, func(i, j int) bool { return bytes.Compare(orderedBlobs[i][:], orderedBlobs[j][:]) < 0 })
	for _, b := range orderedBlobs {
		var id uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT blob_id FROM file_blobs WHERE blob_id=$1 FOR UPDATE`, b).Scan(&id); err != nil {
			return err
		}
	}
	for _, e := range entries {
		if err := applyInventoryEntry(ctx, tx, op, e); err != nil {
			return err
		}
	}
	return nil
}

func applyInventoryEntry(ctx context.Context, tx pgx.Tx, op uuid.UUID, e *commonv1.ReferenceInventoryEntry) error {
	item := inventoryItemKey(e)
	raw, _, err := inv.Encode(e)
	if err != nil {
		return err
	}
	var oldRevision, floor int64
	var old []byte
	var oldIssue *string
	err = tx.QueryRow(ctx, `SELECT owner_revision,retired_floor,entry_bytes,issue_reason FROM file_reference_inventory_items WHERE operation_id=$1 AND item_key=$2`, op, item).Scan(&oldRevision, &floor, &old, &oldIssue)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	issue := ""
	if oldIssue != nil {
		issue = *oldIssue
	}
	if err == nil {
		if uint64(oldRevision) > e.OwnerRevision {
			return inv.ErrConflict
		}
		if uint64(oldRevision) == e.OwnerRevision {
			if !bytes.Equal(raw, old) {
				return inv.ErrConflict
			}
			return nil
		}
		if floor > 0 && !e.Retired {
			issue = "retired owner key"
		}
	}
	if e.Retired {
		floor = int64(e.OwnerRevision)
	}
	f := e.GetObservedFileId()
	if e.Key != nil {
		f = e.Key.FileId
	}
	if e.IssueKind != 0 {
		issue = "source issue"
	}
	var owner any
	isStory := e.Key != nil && e.Key.OwnerType == 2
	if e.Key != nil {
		owner = e.Key.OwnerId
		var conflicting bool
		if isStory {
			if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM file_reference_inventory_items WHERE operation_id=$1 AND owner_id=$2 AND file_id<>$3)`, op, owner, f).Scan(&conflicting); err != nil {
				return err
			}
			if conflicting {
				issue = "inventory binding conflict"
			}
		}
	}
	var blob *uuid.UUID
	var uploader uuid.UUID
	var state, kind, scan string
	var chat, story *uuid.UUID
	var e2e bool
	err = tx.QueryRow(ctx, `SELECT blob_id,uploader_profile_id,status,file_type,scan_result,chat_id,story_id,is_e2e FROM files WHERE id=$1`, nullableInventoryUUID(f)).Scan(&blob, &uploader, &state, &kind, &scan, &chat, &story, &e2e)
	if errors.Is(err, pgx.ErrNoRows) {
		issue = "missing file"
	} else if err != nil {
		return err
	} else {
		if state == "deleted" || state == "expired" || state == "failed" {
			issue = "terminal file"
		} else if state != "ready" {
			issue = "unusable file"
		}
		if isStory && e.AuthorProfileId != nil && *e.AuthorProfileId != uploader.String() {
			issue = "author conflict"
		}
		if isStory && (chat != nil || e2e || (story != nil && story.String() != e.Key.OwnerId)) {
			issue = "context conflict"
		}
		if isStory && ((e.MediaType == "photo" && kind != "image") || (e.MediaType == "video" && kind != "video")) {
			issue = "type conflict"
		}
		if scan != "clean" && scan != "skipped" {
			issue = "scan conflict"
		}
		if blob == nil {
			issue = "missing blob"
		} else {
			var bs string
			var deleted *time.Time
			if err = tx.QueryRow(ctx, `SELECT state,deleted_at FROM file_blobs WHERE blob_id=$1`, blob).Scan(&bs, &deleted); err != nil {
				return err
			}
			if deleted != nil || bs == "GC_COMPLETE" {
				issue = "irreversible blob deletion"
			}
			// Retirements never drop an existing hold in this incomplete migration.
			if !e.Retired {
				if _, err = tx.Exec(ctx, `INSERT INTO file_reference_inventory_holds VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, op, item, blob); err != nil {
					return err
				}
			}
		}
		if isStory {
			var conflict bool
			if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM file_references WHERE owner_type=2 AND owner_id=$1 AND (file_id<>$2 OR released_at IS NOT NULL OR subresource_id IS NOT NULL OR scope_space_id IS NOT NULL))`, e.Key.OwnerId, f).Scan(&conflict); err != nil {
				return err
			}
			if conflict && !e.Retired {
				issue = "reference binding conflict"
			}
		} else if e.Key != nil && !e.Retired {
			// An upload's chat or uploader is not authority for a Message tuple.
			// Require an existing exact reference; missing historical proof remains
			// unresolved while the blob hold protects it.
			var exact bool
			if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM file_references WHERE owner_type=$1 AND owner_id=$2 AND file_id=$3 AND subresource_id IS NOT DISTINCT FROM $4::uuid AND scope_space_id IS NOT DISTINCT FROM $5::uuid AND released_at IS NULL)`, int32(e.Key.OwnerType), e.Key.OwnerId, f, e.Key.SubresourceId, e.Key.ScopeSpaceId).Scan(&exact); err != nil {
				return err
			}
			if !exact {
				issue = "unresolved historical binding"
			}
		}
	}
	var reason *string
	if issue != "" {
		reason = &issue
	}
	_, err = tx.Exec(ctx, `INSERT INTO file_reference_inventory_items(operation_id,item_key,owner_revision,retired_floor,entry_bytes,issue_reason,file_id,owner_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(operation_id,item_key) DO UPDATE SET owner_revision=EXCLUDED.owner_revision,retired_floor=GREATEST(file_reference_inventory_items.retired_floor,EXCLUDED.retired_floor),entry_bytes=EXCLUDED.entry_bytes,issue_reason=EXCLUDED.issue_reason`, op, item, int64(e.OwnerRevision), floor, raw, reason, nullableInventoryUUID(f), owner)
	return err
}
func nullableInventoryUUID(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (s *FilesStore) PendingInventoryOperations(ctx context.Context, limit int) ([]uuid.UUID, error) {
	if limit < 1 || limit > 100 {
		return nil, inv.ErrInvalid
	}
	rows, err := s.Pool.Query(ctx, `SELECT i.operation_id FROM file_reference_inventory_imports i JOIN file_reference_inventory_cohorts c USING(migration_id) WHERE i.state<>'BLOCKED' AND c.state='MIGRATING' ORDER BY i.created_at,i.operation_id LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		result = append(result, id)
	}
	return result, rows.Err()
}

func (s *FilesStore) InvalidateInventoryCohort(ctx context.Context, migration uuid.UUID) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var generation int64
	if err = tx.QueryRow(ctx, `SELECT generation FROM file_reference_inventory_gate WHERE singleton FOR UPDATE`).Scan(&generation); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE file_reference_inventory_cohorts SET state='INVALIDATED' WHERE migration_id=$1`, migration)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return inv.ErrConflict
	}
	if _, err = tx.Exec(ctx, `UPDATE file_reference_inventory_gate SET blocked=true,generation=generation+1 WHERE singleton`); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
