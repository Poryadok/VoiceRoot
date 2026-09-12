package store

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestSpaceStore_ListAuditLogSignedPage documents newest-first, space-scoped stable cursor paging.
func TestSpaceStore_ListAuditLogSignedPage(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}

	owner := uuid.New()
	requested, err := st.CreateSpace(ctx, owner, "Audited", "", "private")
	require.NoError(t, err)
	other, err := st.CreateSpace(ctx, owner, "Other", "", "private")
	require.NoError(t, err)

	sharedAt := time.Date(2026, 8, 30, 12, 0, 0, 123000, time.UTC)
	olderAt := sharedAt.Add(-time.Minute)
	ids := []uuid.UUID{
		uuid.MustParse("dddddddd-dddd-4ddd-8ddd-dddddddddddd"),
		uuid.MustParse("cccccccc-cccc-4ccc-8ccc-cccccccccccc"),
		uuid.MustParse("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"),
		uuid.MustParse("11111111-1111-4111-8111-111111111111"),
	}
	for i, row := range []struct {
		id      uuid.UUID
		spaceID uuid.UUID
		action  string
		at      time.Time
	}{
		{ids[3], requested.ID, "oldest", olderAt},
		{ids[2], requested.ID, "same_time_low_id", sharedAt},
		{ids[1], requested.ID, "same_time_middle_id", sharedAt},
		{ids[0], requested.ID, "same_time_high_id", sharedAt},
		{uuid.New(), other.ID, "must_not_leak", sharedAt.Add(time.Hour)},
	} {
		_, err = pool.Exec(ctx, `
INSERT INTO audit_log (id, space_id, actor_profile_id, action, target_type, target_id, details, created_at)
VALUES ($1, $2, $3, $4, 'profile', $5, $6::jsonb, $7)
`, row.id, row.spaceID, owner, row.action, uuid.New(), fmt.Sprintf(`{"ordinal":%d}`, i), row.at)
		require.NoError(t, err)
	}

	key := bytes.Repeat([]byte{0x4a}, 32)
	page1, err := st.ListAuditLogSignedPage(ctx, requested.ID, "", 2, "reader", key)
	require.NoError(t, err)
	require.Len(t, page1.Rows, 2)
	require.Equal(t, ids[0], page1.Rows[0].ID)
	require.Equal(t, ids[1], page1.Rows[1].ID)
	require.NotEmpty(t, page1.NextCursor)
	require.NotContains(t, page1.NextCursor, ids[1].String(), "cursor must be opaque")

	// Insert a row ahead of the page boundary after page 1. Offset pagination would
	// now repeat ids[1], while a timestamp-only cursor would skip ids[2].
	insertedBetweenPages := uuid.MustParse("ffffffff-ffff-4fff-8fff-ffffffffffff")
	_, err = pool.Exec(ctx, `
INSERT INTO audit_log (id, space_id, actor_profile_id, action, target_type, target_id, details, created_at)
VALUES ($1, $2, $3, 'inserted_between_pages', 'profile', $4, '{}'::jsonb, $5)
`, insertedBetweenPages, requested.ID, owner, uuid.New(), sharedAt)
	require.NoError(t, err)

	page2, err := st.ListAuditLogSignedPage(ctx, requested.ID, page1.NextCursor, 2, "reader", key)
	require.NoError(t, err)
	require.Len(t, page2.Rows, 2)
	require.Equal(t, []uuid.UUID{ids[2], ids[3]}, []uuid.UUID{page2.Rows[0].ID, page2.Rows[1].ID})
	require.NotEqual(t, insertedBetweenPages, page2.Rows[0].ID, "rows newer than the cursor must not leak into later pages")
	require.Empty(t, page2.NextCursor, "final page must not advertise continuation")
}

func TestSpaceStore_ListAuditLogSignedPage_InvalidCursor(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}

	_, err := st.ListAuditLogSignedPage(ctx, uuid.New(), "not-an-audit-cursor", 50, "reader", bytes.Repeat([]byte{0x4a}, 32))
	require.ErrorIs(t, err, ErrInvalidAuditCursor)
}
