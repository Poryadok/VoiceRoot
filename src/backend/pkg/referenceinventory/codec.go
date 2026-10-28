// Package referenceinventory validates private Story migration evidence.
// Its records are protective inventory, never readable references or activation.
package referenceinventory

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	commonv1 "voice.app/voice/common/v1"
)

const PageSize = 1000

var ErrInvalid = errors.New("invalid reference inventory evidence")

// Strict rejects unknown fields at every nesting level, including map values.
func Strict(m proto.Message) error {
	if m == nil || !m.ProtoReflect().IsValid() {
		return ErrInvalid
	}
	return strict(m.ProtoReflect())
}

func strict(m protoreflect.Message) error {
	if len(m.GetUnknown()) != 0 {
		return ErrInvalid
	}
	var err error
	m.Range(func(f protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		if f.IsMap() && f.MapValue().Kind() == protoreflect.MessageKind {
			v.Map().Range(func(_ protoreflect.MapKey, value protoreflect.Value) bool {
				err = strict(value.Message())
				return err == nil
			})
		} else if f.IsList() && f.Kind() == protoreflect.MessageKind {
			for i := 0; i < v.List().Len() && err == nil; i++ {
				err = strict(v.List().Get(i).Message())
			}
		} else if f.Kind() == protoreflect.MessageKind && !f.IsMap() && !f.IsList() {
			err = strict(v.Message())
		}
		return err == nil
	})
	return err
}

func UUID(s string) bool {
	id, err := uuid.Parse(s)
	return err == nil && id != uuid.Nil && id.String() == s
}

func ValidateIdentity(id *commonv1.ReferenceInventoryIdentity) error {
	if Strict(id) != nil || id.ProtocolVersion != 1 || id.WriterProtocolVersion != 1 || id.SourceService != "story" || id.SourceComponent != "story.media.v1" {
		return ErrInvalid
	}
	for _, s := range []string{id.InstallationId, id.RestoreCohortId, id.MigrationId, id.FileIncarnationId, id.OwnerIncarnationId, id.SnapshotId} {
		if !UUID(s) {
			return ErrInvalid
		}
	}
	return nil
}

func ValidateEntry(e *commonv1.ReferenceInventoryEntry, allowRetired bool) error {
	if Strict(e) != nil {
		return ErrInvalid
	}
	if e.Key != nil {
		k := e.Key
		if !UUID(k.FileId) || !UUID(k.OwnerId) || k.OwnerType != commonv1.ReferenceInventoryOwnerType_REFERENCE_INVENTORY_OWNER_TYPE_STORY || k.SubresourceId != nil || k.ScopeSpaceId != nil {
			return ErrInvalid
		}
	}
	if e.IssueKind < 0 || e.IssueKind > commonv1.ReferenceInventoryIssueKind_REFERENCE_INVENTORY_ISSUE_KIND_CONFLICTING_BINDING {
		return ErrInvalid
	}
	if (e.IssueId == nil) != (e.IssueKind == 0) || (e.IssueId != nil && !UUID(*e.IssueId)) || (e.Key == nil && e.IssueId == nil) {
		return ErrInvalid
	}
	if e.ObservedFileId != nil && (!UUID(*e.ObservedFileId) || e.Key != nil) {
		return ErrInvalid
	}
	if e.AuthorProfileId != nil && !UUID(*e.AuthorProfileId) {
		return ErrInvalid
	}
	if e.Retired {
		if !allowRetired || len(e.Reasons) != 0 {
			return ErrInvalid
		}
	} else if e.IssueKind == 0 && (e.MediaType != "photo" && e.MediaType != "video") {
		for _, reason := range e.Reasons {
			if reason != commonv1.ReferenceInventoryProtectionReason_REFERENCE_INVENTORY_PROTECTION_REASON_RELEASE_PENDING {
				return ErrInvalid
			}
		}
	}
	if !e.Retired && e.IssueKind == 0 && len(e.Reasons) == 0 {
		return ErrInvalid
	}
	for i, reason := range e.Reasons {
		if reason < 1 || reason > 3 || (i > 0 && e.Reasons[i-1] >= reason) {
			return ErrInvalid
		}
	}
	for i, operation := range e.SourceOperationIds {
		if !UUID(operation) || (i > 0 && e.SourceOperationIds[i-1] >= operation) {
			return ErrInvalid
		}
	}
	return nil
}

// Compare uses raw canonical UUID bytes. Unmappable issues sort after keys.
func Compare(a, b *commonv1.ReferenceInventoryEntry) int {
	if a.Key == nil && b.Key != nil {
		return 1
	}
	if a.Key != nil && b.Key == nil {
		return -1
	}
	if a.Key == nil {
		av, bv := uuid.MustParse(a.GetIssueId()), uuid.MustParse(b.GetIssueId())
		return bytes.Compare(av[:], bv[:])
	}
	for _, pair := range [][2]string{{a.Key.FileId, b.Key.FileId}, {a.Key.OwnerId, b.Key.OwnerId}} {
		av, bv := uuid.MustParse(pair[0]), uuid.MustParse(pair[1])
		if c := bytes.Compare(av[:], bv[:]); c != 0 {
			return c
		}
	}
	return 0
}

func ValidateStoryPage(p *commonv1.ReferenceInventoryPage) error {
	if Strict(p) != nil || ValidateIdentity(p.Identity) != nil || len(p.Entries) == 0 || len(p.Entries) > PageSize {
		return ErrInvalid
	}
	for i, entry := range p.Entries {
		if ValidateEntry(entry, false) != nil || entry.OwnerRevision > p.BoundaryRevision || (i > 0 && Compare(p.Entries[i-1], entry) >= 0) {
			return ErrInvalid
		}
	}
	return nil
}

// Encode stores exact deterministic protobuf bytes plus the P3 domain hash.
func Encode(m proto.Message) ([]byte, []byte, error) {
	if err := Strict(m); err != nil {
		return nil, nil, err
	}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(m)
	if err != nil {
		return nil, nil, err
	}
	h := sha256.New()
	_, _ = h.Write([]byte(m.ProtoReflect().Descriptor().FullName()))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write(raw)
	return raw, h.Sum(nil), nil
}

func Describe(pages []*commonv1.ReferenceInventoryPage, id *commonv1.ReferenceInventoryIdentity, boundary uint64) (*commonv1.ReferenceInventoryDescriptor, error) {
	if ValidateIdentity(id) != nil {
		return nil, ErrInvalid
	}
	m := &commonv1.InventoryManifest{Identity: proto.Clone(id).(*commonv1.ReferenceInventoryIdentity), BoundaryRevision: boundary}
	var last *commonv1.ReferenceInventoryEntry
	for i, p := range pages {
		if ValidateStoryPage(p) != nil || !proto.Equal(p.Identity, id) || p.BoundaryRevision != boundary || p.PageIndex != uint32(i) || (i < len(pages)-1 && len(p.Entries) != PageSize) || (last != nil && Compare(last, p.Entries[0]) >= 0) {
			return nil, ErrInvalid
		}
		_, hash, err := Encode(p)
		if err != nil {
			return nil, err
		}
		m.Commitments = append(m.Commitments, &commonv1.InventoryPageCommitment{Index: uint32(i), Count: uint32(len(p.Entries)), PageSha256: hash})
		m.TotalCount += uint64(len(p.Entries))
		for _, entry := range p.Entries {
			if entry.IssueKind != 0 {
				m.UnresolvedCount++
			}
		}
		last = p.Entries[len(p.Entries)-1]
	}
	_, hash, err := Encode(m)
	if err != nil {
		return nil, err
	}
	return &commonv1.ReferenceInventoryDescriptor{Manifest: m, ManifestSha256: hash}, nil
}

func ValidateDescriptor(d *commonv1.ReferenceInventoryDescriptor) error {
	if Strict(d) != nil || d.Manifest == nil || ValidateIdentity(d.Manifest.Identity) != nil || d.Manifest.UnresolvedCount > d.Manifest.TotalCount {
		return ErrInvalid
	}
	var count uint64
	for i, c := range d.Manifest.Commitments {
		if c.Index != uint32(i) || c.Count == 0 || c.Count > PageSize || len(c.PageSha256) != sha256.Size || (i < len(d.Manifest.Commitments)-1 && c.Count != PageSize) {
			return ErrInvalid
		}
		count += uint64(c.Count)
	}
	_, hash, err := Encode(d.Manifest)
	if err != nil || count != d.Manifest.TotalCount || !bytes.Equal(hash, d.ManifestSha256) {
		return ErrInvalid
	}
	return nil
}

func ValidateCommittedPage(d *commonv1.ReferenceInventoryDescriptor, p *commonv1.ReferenceInventoryPage) error {
	if ValidateDescriptor(d) != nil || ValidateStoryPage(p) != nil || !proto.Equal(p.Identity, d.Manifest.Identity) || p.BoundaryRevision != d.Manifest.BoundaryRevision || int(p.PageIndex) >= len(d.Manifest.Commitments) {
		return ErrInvalid
	}
	c := d.Manifest.Commitments[p.PageIndex]
	_, hash, err := Encode(p)
	if err != nil || c.Count != uint32(len(p.Entries)) || !bytes.Equal(hash, c.PageSha256) {
		return ErrInvalid
	}
	return nil
}

func ValidateChanges(r *commonv1.ReadReferenceInventoryChangesResponse, id *commonv1.ReferenceInventoryIdentity, after uint64) error {
	if Strict(r) != nil || ValidateIdentity(r.Identity) != nil || !proto.Equal(r.Identity, id) || after < r.RetainedFloor || after > r.HighWatermark || len(r.Revisions) > PageSize || (len(r.Revisions) == 0 && after != r.HighWatermark) {
		return ErrInvalid
	}
	for i, revision := range r.Revisions {
		if revision.Revision != after+uint64(i)+1 || revision.Revision > r.HighWatermark || int(revision.FinalCount) != len(revision.Entries) {
			return fmt.Errorf("%w: incomplete journal revision", ErrInvalid)
		}
		for j, entry := range revision.Entries {
			if ValidateEntry(entry, true) != nil || entry.OwnerRevision != revision.Revision || (j > 0 && Compare(revision.Entries[j-1], entry) >= 0) {
				return ErrInvalid
			}
		}
	}
	return nil
}
