//go:build listthreads_successor_red

package threadlist

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// This RED-only scaffold specifies the private seam required by the accepted
// ListThreads successor. It deliberately cannot be satisfied by the current
// MessagesStore aggregate: production types do not exist yet. The implementation
// may choose the event subject and persistence encoding, but not these semantics.

func TestThreadListSchemaHasOnlyMessagingOwnedProjectionData(t *testing.T) {
	schema := ThreadListSchema()
	require.Contains(t, schema.Tables(), "thread_list_viewers")
	require.Contains(t, schema.Tables(), "thread_list_changes")
	require.Contains(t, schema.Tables(), "thread_list_cursor_states")
	require.True(t, schema.ImmutableNodesRejectInPlaceUpdate())
	require.False(t, schema.HasCrossDatabaseForeignKeys())
}

func TestMembershipInboxDeduplicatesRevisionedChatEvents(t *testing.T) {
	inbox := NewMembershipInbox()
	event := MembershipEvent{EventID: uuid.New(), ChatID: uuid.New(), ProfileID: uuid.New(), MembershipRevision: 7, Kind: MembershipJoined}
	require.NoError(t, inbox.Apply(event))
	require.NoError(t, inbox.Apply(event), "duplicate durable delivery is idempotent")
	require.ErrorIs(t, inbox.Apply(MembershipEvent{EventID: uuid.New(), ChatID: event.ChatID, ProfileID: event.ProfileID, MembershipRevision: 6, Kind: MembershipRemoved}), ErrStaleMembershipRevision)
}

func TestImmutableAVLPathCopyAndBoundedTraversal(t *testing.T) {
	tree := NewActivityAVL()
	first, err := tree.Apply(ThreadChange{ParentID: uuid.New(), LastReplyAt: time.Unix(100, 0), ReplyCount: 1})
	require.NoError(t, err)
	second, err := first.Apply(ThreadChange{ParentID: uuid.New(), LastReplyAt: time.Unix(101, 0), ReplyCount: 1})
	require.NoError(t, err)
	require.Equal(t, 1, first.Len(), "path-copy update must not mutate an old root")
	page, visited := second.Page(ActivityKey{}, 100)
	require.LessOrEqual(t, visited, second.Height()+101)
	require.Len(t, page, 2)
}

func TestCursorBindsViewerAndKeepsOriginalExpiry(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	state := CursorState{ID: uuid.New(), ChatID: uuid.New(), ProfileID: uuid.New(), PageSize: 50, ExpiresAt: now.Add(15 * time.Minute)}
	codec := NewCursorCodec([]byte("01234567890123456789012345678901"))
	token, err := codec.Encode(state)
	require.NoError(t, err)
	next, err := codec.Decode(token, state.ChatID, state.ProfileID, 50, now.Add(time.Minute))
	require.NoError(t, err)
	require.Equal(t, state.ExpiresAt, next.ExpiresAt, "a later page must not renew expiry")
	_, err = codec.Decode(token, state.ChatID, uuid.New(), 50, now.Add(time.Minute))
	require.ErrorIs(t, err, ErrInvalidCursor)
}

func TestLiveRevocationMarksCursorUpdatingBeforeServingAStalePage(t *testing.T) {
	viewer := ViewerKey{ChatID: uuid.New(), ProfileID: uuid.New()}
	projector := NewTransactionalProjector()
	state := ReadyCursorState(viewer, uuid.New(), time.Now().Add(15*time.Minute))
	require.NoError(t, projector.ApplyVisibilityRevocation(viewer, uuid.New(), VisibilityForMe))
	current, err := projector.CursorState(state.ID)
	require.NoError(t, err)
	require.Equal(t, CursorStateUpdating, current.Status)
}

func TestProjectorFailureRollsBackMessageAndRoots(t *testing.T) {
	projector := NewTransactionalProjector()
	before := projector.Snapshot()
	err := projector.ApplyMessageMutation(ThreadChange{ParentID: uuid.New(), LastReplyAt: time.Now()}, FailAfterJournalAppend)
	require.Error(t, err)
	require.Equal(t, before, projector.Snapshot(), "message mutation, journal and heads must roll back together")
}

func TestListThreadsFailsClosedWhileProjectionIsBuildingOrUpdating(t *testing.T) {
	reader := NewListThreadsReader()
	for _, viewerStatus := range []ViewerStatus{ViewerBuilding, ViewerUpdating} {
		viewer := ViewerKey{ChatID: uuid.New(), ProfileID: uuid.New()}
		reader.SetViewerStatus(viewer, viewerStatus)
		_, err := reader.Page(viewer, "", 50)
		require.ErrorIs(t, err, ErrUnavailable)
		require.Equal(t, "thread list unavailable", PublicMessage(err))
	}
}

func TestListThreadsAuthorizesBeforeParsingCursor(t *testing.T) {
	reader := NewListThreadsReader()
	request := ListThreadsRequest{Viewer: ViewerKey{ChatID: uuid.New(), ProfileID: uuid.New()}, Cursor: "tampered", PageSize: 50}
	_, err := reader.PageForMember(request, DenyMembership)
	require.ErrorIs(t, err, ErrPermissionDenied)
}

func TestSnapshotRevocationNeverServesRootOrReplyData(t *testing.T) {
	for _, visibility := range []Visibility{VisibilityForMe, VisibilityForEveryone, VisibilityGhost, VisibilityHide} {
		for _, target := range []ThreadVisibilityTarget{ThreadRoot, ThreadReply} {
			t.Run(string(visibility)+"/"+string(target), func(t *testing.T) {
				fixture := NewSnapshotFixture()
				pageOne, err := fixture.FirstPage(1)
				require.NoError(t, err)
				require.NotEmpty(t, pageOne.NextCursor)
				require.NoError(t, fixture.Revoke(target, visibility))
				pageTwo, err := fixture.NextPage(pageOne.NextCursor)
				require.ErrorIs(t, err, ErrUnavailable)
				require.Empty(t, pageTwo.Threads, "a stale private root/reply must never be served")
			})
		}
	}
}

func TestCursorGCDoesNotCollectSharedNodeReachableFromReadyHead(t *testing.T) {
	gc := NewThreadListGC()
	shared := uuid.New()
	gc.TrackReachability(shared, ReadyHeadReachable|UnexpiredCursorReachable)
	require.NoError(t, gc.Collect(time.Now().Add(16*time.Minute)))
	require.True(t, gc.Exists(shared))
}
