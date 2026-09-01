package store

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMergeListChatRows_dedupesAndSortsByActivity(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	older := now.Add(-time.Hour)
	newer := now.Add(-time.Minute)
	spaceActivity := now.Add(-30 * time.Second)

	idShared := uuid.New()
	dm := &ChatRow{ID: uuid.New(), Type: "dm", CreatedAt: older, LastMessageAt: &older}
	space := &ChatRow{ID: uuid.New(), Type: "channel", CreatedAt: spaceActivity, LastMessageAt: &spaceActivity}
	dup := &ChatRow{ID: idShared, Type: "group", CreatedAt: newer, LastMessageAt: &newer}

	merged := MergeListChatRows([]*ChatRow{dm, dup}, []*ChatRow{dup, space}, 0)
	require.Len(t, merged, 3)
	require.Equal(t, space.ID, merged[0].ID)
	require.Equal(t, idShared, merged[1].ID)
	require.Equal(t, dm.ID, merged[2].ID)
}

func TestListChatsPageCursorFromRows_emptyWhenWithinLimit(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	rows := []*ChatRow{
		{ID: uuid.New(), CreatedAt: now},
	}
	trimmed, next := ListChatsPageCursorFromRows(rows, 10)
	require.Equal(t, rows, trimmed)
	require.Empty(t, next)
}

func TestListChatsPageCursorFromRows_encodesWhenTruncated(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	lastAt := now.Add(-time.Minute)
	rows := []*ChatRow{
		{ID: uuid.New(), CreatedAt: now, LastMessageAt: &now},
		{ID: uuid.New(), CreatedAt: now, LastMessageAt: &lastAt},
	}
	trimmed, next := ListChatsPageCursorFromRows(rows, 1)
	require.Len(t, trimmed, 1)
	require.NotEmpty(t, next)

	ts, id, err := decodeListChatCursor(next)
	require.NoError(t, err)
	require.Equal(t, rows[0].ID, id)
	require.True(t, ts.Equal(now))
}
