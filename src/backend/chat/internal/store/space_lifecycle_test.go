package store

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
)

func TestPrepareSpaceDeletionManifestPersistsAndReplaysOrderedPages(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	store := &SpaceLifecycleStore{Pool: pool}
	spaceID, operationID := uuid.New(), uuid.New()
	profileID := uuid.New()
	ids := make([]uuid.UUID, 1001)
	for i := range ids {
		ids[i] = uuid.New()
		_, err := pool.Exec(ctx, `INSERT INTO chats(id,type,space_id,name,creator_profile_id) VALUES($1,'channel',$2,$3,$4)`,
			ids[i], spaceID, fmt.Sprintf("channel-%04d", i), profileID)
		require.NoError(t, err)
	}
	request := &chatv1.PrepareSpaceDeletionManifestRequest{ProtocolVersion: 1, SpaceId: spaceID.String(),
		DeletionOperationId: operationID.String(), ScheduleGeneration: 1}
	first, err := store.PrepareSpaceDeletionManifest(ctx, request)
	require.NoError(t, err)
	second, err := store.PrepareSpaceDeletionManifest(ctx, proto.Clone(request).(*chatv1.PrepareSpaceDeletionManifestRequest))
	require.NoError(t, err)
	require.True(t, proto.Equal(first, second), "lost response replay must return the exact saved receipt")
	receipt := first.GetReceipt()
	require.NotNil(t, receipt)
	require.Equal(t, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, receipt.GetAppliedState())
	require.Equal(t, uint64(1001), receipt.GetChatManifest().GetItemCount())
	require.Len(t, receipt.GetChatManifest().GetManifestSha256(), 32)

	page0, err := store.GetSpacePurgeManifestPage(ctx, &chatv1.GetSpacePurgeManifestPageRequest{
		ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: operationID.String(),
		Generation: 1, ManifestId: receipt.GetChatManifest().GetManifestId(),
	})
	require.NoError(t, err)
	require.Equal(t, uint64(0), page0.GetPage().GetPageIndex())
	require.Len(t, page0.GetPage().GetItemIds(), 1000)
	require.NotEmpty(t, page0.GetPage().GetNextPageToken())

	page1, err := store.GetSpacePurgeManifestPage(ctx, &chatv1.GetSpacePurgeManifestPageRequest{
		ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: operationID.String(),
		Generation: 1, ManifestId: receipt.GetChatManifest().GetManifestId(), PageToken: page0.GetPage().GetNextPageToken(),
	})
	require.NoError(t, err)
	require.Equal(t, uint64(1), page1.GetPage().GetPageIndex())
	require.Len(t, page1.GetPage().GetItemIds(), 1)
	require.Empty(t, page1.GetPage().GetNextPageToken())
	require.Len(t, page0.GetPage().GetPageSha256(), 32)
	require.Len(t, page1.GetPage().GetPageSha256(), 32)
	gotIDs := append(append([]string(nil), page0.GetPage().GetItemIds()...), page1.GetPage().GetItemIds()...)
	want := append([]uuid.UUID(nil), ids...)
	sort.Slice(want, func(i, j int) bool { return bytes.Compare(want[i][:], want[j][:]) < 0 })
	for i, expected := range want {
		require.Equal(t, expected.String(), gotIDs[i], "manifest uses raw UUID byte ordering")
	}

	changed := proto.Clone(request).(*chatv1.PrepareSpaceDeletionManifestRequest)
	changed.ScheduleGeneration = 2
	_, err = store.PrepareSpaceDeletionManifest(ctx, changed)
	require.ErrorIs(t, err, ErrSpaceLifecycleConflict)
}
