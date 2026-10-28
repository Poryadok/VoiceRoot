package referenceinventory

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	commonv1 "voice.app/voice/common/v1"
)

func identityFixture() *commonv1.ReferenceInventoryIdentity {
	return &commonv1.ReferenceInventoryIdentity{ProtocolVersion: 1, InstallationId: uuid.NewString(), RestoreCohortId: uuid.NewString(), MigrationId: uuid.NewString(), FileIncarnationId: uuid.NewString(), SourceService: "story", SourceComponent: "story.media.v1", OwnerIncarnationId: uuid.NewString(), WriterProtocolVersion: 1, SnapshotId: uuid.NewString()}
}

func entryFixture() *commonv1.ReferenceInventoryEntry {
	return &commonv1.ReferenceInventoryEntry{Key: &commonv1.ReferenceInventoryKey{FileId: uuid.NewString(), OwnerType: commonv1.ReferenceInventoryOwnerType_REFERENCE_INVENTORY_OWNER_TYPE_STORY, OwnerId: uuid.NewString()}, Reasons: []commonv1.ReferenceInventoryProtectionReason{commonv1.ReferenceInventoryProtectionReason_REFERENCE_INVENTORY_PROTECTION_REASON_RETAINED}, MediaType: "photo", AuthorProfileId: proto.String(uuid.NewString())}
}

func TestStoryPageRejectsUnknownNestedFieldsAndDuplicateReasons(t *testing.T) {
	p := &commonv1.ReferenceInventoryPage{Identity: identityFixture(), Entries: []*commonv1.ReferenceInventoryEntry{entryFixture()}}
	require.NoError(t, ValidateStoryPage(p))
	p.Entries[0].Key.ProtoReflect().SetUnknown([]byte{0x78, 1})
	require.Error(t, ValidateStoryPage(p))
	p.Entries[0].Key.ProtoReflect().SetUnknown(nil)
	p.Entries[0].Reasons = append(p.Entries[0].Reasons, p.Entries[0].Reasons[0])
	require.Error(t, ValidateStoryPage(p))
}

func TestManifestBindsTypedIdentityAndImmutablePageBytes(t *testing.T) {
	p := &commonv1.ReferenceInventoryPage{Identity: identityFixture(), Entries: []*commonv1.ReferenceInventoryEntry{entryFixture()}}
	d, err := Describe([]*commonv1.ReferenceInventoryPage{p}, p.Identity, 0)
	require.NoError(t, err)
	require.NoError(t, ValidateDescriptor(d))
	require.NoError(t, ValidateCommittedPage(d, p))
	p.Entries[0].Reasons = []commonv1.ReferenceInventoryProtectionReason{commonv1.ReferenceInventoryProtectionReason_REFERENCE_INVENTORY_PROTECTION_REASON_RELEASE_PENDING}
	require.Error(t, ValidateCommittedPage(d, p), "same index with different protection bytes must conflict")
	empty, err := Describe(nil, p.Identity, 0)
	require.NoError(t, err)
	require.NoError(t, ValidateDescriptor(empty))
	require.Empty(t, empty.Manifest.Commitments)
	require.NotEqual(t, d.ManifestSha256, empty.ManifestSha256)
}

func TestStoryPageCanonicalTupleOrderAndIssueAccounting(t *testing.T) {
	a, b := entryFixture(), entryFixture()
	a.Key.FileId = "00000000-0000-0000-0000-000000000001"
	b.Key.FileId = "00000000-0000-0000-0000-000000000002"
	p := &commonv1.ReferenceInventoryPage{Identity: identityFixture(), Entries: []*commonv1.ReferenceInventoryEntry{b, a}}
	require.Error(t, ValidateStoryPage(p))
	p.Entries = []*commonv1.ReferenceInventoryEntry{a, b}
	require.NoError(t, ValidateStoryPage(p))
	b.IssueId = proto.String(uuid.NewString())
	b.IssueKind = commonv1.ReferenceInventoryIssueKind_REFERENCE_INVENTORY_ISSUE_KIND_HISTORICAL_MEDIA_ANOMALY
	d, err := Describe([]*commonv1.ReferenceInventoryPage{p}, p.Identity, 0)
	require.NoError(t, err)
	require.EqualValues(t, 1, d.Manifest.UnresolvedCount)
	p.Entries = append(p.Entries, proto.Clone(b).(*commonv1.ReferenceInventoryEntry))
	require.Error(t, ValidateStoryPage(p), "duplicate tuple must not disappear under dedupe")
}

func TestChangesRequireCompleteContiguousRevisions(t *testing.T) {
	id := identityFixture()
	r := &commonv1.ReadReferenceInventoryChangesResponse{Identity: id, HighWatermark: 2, Revisions: []*commonv1.ReferenceInventoryRevision{{Revision: 2, FinalCount: 1, Entries: []*commonv1.ReferenceInventoryEntry{entryFixture()}}}}
	require.Error(t, ValidateChanges(r, id, 0), "revision1 cannot be skipped")
	r.Revisions[0].Revision = 1
	r.Revisions[0].FinalCount = 2
	require.Error(t, ValidateChanges(r, id, 0), "partial transaction cannot be acknowledged")
	r.Revisions[0].FinalCount = 1
	r.Revisions[0].Entries[0].OwnerRevision = 1
	require.NoError(t, ValidateChanges(r, id, 0))
	r.RetainedFloor = 1
	require.Error(t, ValidateChanges(r, id, 0), "pruned cursor requires a new snapshot")
}
