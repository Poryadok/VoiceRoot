package referenceinventory

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"testing"
	commonv1 "voice.app/voice/common/v1"
)

func TestMessageInventoryUsesClosedComponentAndCompleteTuple(t *testing.T) {
	id := identityFixture()
	id.SourceService, id.SourceComponent = "messaging", "messaging.message.v1"
	require.NoError(t, ValidateIdentity(id))
	e := entryFixture()
	e.Key.OwnerType = 1
	e.Key.ScopeSpaceId = proto.String(uuid.NewString())
	e.MediaType = "image"
	p := &commonv1.ReferenceInventoryPage{Identity: id, Entries: []*commonv1.ReferenceInventoryEntry{e}}
	d, err := Describe([]*commonv1.ReferenceInventoryPage{p}, id, 0)
	require.NoError(t, err)
	require.NoError(t, ValidateCommittedPage(d, p))
	require.Error(t, ValidateStoryPage(p), "Story exporter remains Story-specific")
	e.Key.OwnerType = 2
	_, err = Describe([]*commonv1.ReferenceInventoryPage{p}, id, 0)
	require.Error(t, err, "forged owner kind cannot use MESSAGE identity")
	id.SourceService = "story"
	require.Error(t, ValidateIdentity(id), "service/component pair is closed")
}

func TestCompareIncludesKindSubresourceAndScope(t *testing.T) {
	a := entryFixture()
	b := proto.Clone(a).(*commonv1.ReferenceInventoryEntry)
	b.Key.OwnerType++
	require.Negative(t, Compare(a, b))
	b.Key.OwnerType = a.Key.OwnerType
	b.Key.SubresourceId = proto.String(uuid.NewString())
	require.Negative(t, Compare(a, b))
	a.Key.SubresourceId = b.Key.SubresourceId
	b.Key.ScopeSpaceId = proto.String(uuid.NewString())
	require.Negative(t, Compare(a, b))
	a.Key.ScopeSpaceId = b.Key.ScopeSpaceId
	require.Zero(t, Compare(a, b))
}

func TestOwnerChatBindingIsCanonicalAndMessageOnly(t *testing.T) {
	id := identityFixture()
	e := entryFixture()
	e.OwnerChatId = proto.String(uuid.NewString())
	require.Error(t, ValidateEntryForIdentity(e, id, false), "Story cannot carry Messaging owner context")
	id.SourceService, id.SourceComponent = "messaging", "messaging.message.v1"
	e.Key.OwnerType = 1
	require.NoError(t, ValidateEntryForIdentity(e, id, false))
	e.OwnerChatId = proto.String("not-a-uuid")
	require.Error(t, ValidateEntryForIdentity(e, id, false))
}
