package store

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"voice/backend/pkg/integrationtest"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
}

func startSpacePostgresForStoreTest(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	return integrationtest.StartPostgres(t, ctx, "spacedb", "")
}

func applySpaceMigrationForStoreTest(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	applySpaceMigrationsThrough12ForStoreTest(t, ctx, pool)
	applyLifecycleMigration(t, ctx, pool, "up")
	_, err := pool.Exec(ctx, restoreMigrationSQL(t, "up"))
	require.NoError(t, err)
}

func applySpaceMigrationsThrough12ForStoreTest(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	applySpaceMigrationsThrough7ForStoreTest(t, ctx, pool)
	for _, name := range []string{
		"000008_ownership_journal.up.sql", "000009_ownership_journal_decision.up.sql",
		"000010_ownership_journal_commit.up.sql", "000011_voice_access_epoch.up.sql",
		"000012_ownership_journal_completion.up.sql", "000018_ownership_consume_started.up.sql",
	} {
		migrationPath := filepath.Join(repoRoot(t), "src", "backend", "migrations", "space_db", name)
		sqlBytes, err := os.ReadFile(migrationPath)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(sqlBytes))
		require.NoError(t, err)
	}
}

func applySpaceMigrationsThrough7ForStoreTest(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	for _, name := range []string{
		"000001_init.up.sql", "000002_tree.up.sql", "000003_invites.up.sql",
		"000004_moderation.up.sql", "000005_space_subscriptions.up.sql",
		"000006_allow_guests.up.sql", "000007_tree_pin.up.sql", "000016_allow_guests_fail_closed.up.sql",
	} {
		migrationPath := filepath.Join(repoRoot(t), "src", "backend", "migrations", "space_db", name)
		sqlBytes, err := os.ReadFile(migrationPath)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(sqlBytes))
		require.NoError(t, err)
	}
}

func TestDecodeListSpaceCursor_Invalid(t *testing.T) {
	t.Parallel()
	_, _, err := decodeListSpaceCursor("%%%")
	require.ErrorIs(t, err, ErrInvalidListCursor)

	raw := base64.RawURLEncoding.EncodeToString([]byte("not-json"))
	_, _, err = decodeListSpaceCursor(raw)
	require.ErrorIs(t, err, ErrInvalidListCursor)
}

func TestEncodeDecodeListSpaceCursor_RoundTrip(t *testing.T) {
	t.Parallel()
	joined := time.Date(2026, 6, 10, 12, 0, 0, 123, time.UTC)
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	raw := encodeListSpaceCursor(joined, id)
	gotJoined, gotID, err := decodeListSpaceCursor(raw)
	require.NoError(t, err)
	require.Equal(t, joined, gotJoined)
	require.Equal(t, id, gotID)
}

// TestSpaceStore_ListMySpacesPage_Pagination documents cursor paging for sidebar list.
func TestSpaceStore_ListMySpacesPage_Pagination(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	owner := uuid.New()

	names := []string{"Space A", "Space B", "Space C"}
	for _, name := range names {
		_, err := st.CreateSpace(ctx, owner, name, "desc", "private")
		require.NoError(t, err)
	}

	page1, err := st.ListMySpacesPage(ctx, owner, "", 2)
	require.NoError(t, err)
	require.Len(t, page1.Rows, 2)
	require.NotEmpty(t, page1.NextCursor)

	page2, err := st.ListMySpacesPage(ctx, owner, page1.NextCursor, 2)
	require.NoError(t, err)
	require.Len(t, page2.Rows, 1)
	require.Empty(t, page2.NextCursor)
}

// TestSpaceStore_UpdateSpace_DescriptionOnly documents partial update without touching name/icon.
func TestSpaceStore_UpdateSpace_DescriptionOnly(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	owner := uuid.New()

	row, err := st.CreateSpace(ctx, owner, "Store space", "Before", "private")
	require.NoError(t, err)

	desc := "After"
	updated, err := st.UpdateSpace(ctx, row.ID, UpdateSpaceInput{Description: &desc})
	require.NoError(t, err)
	require.Equal(t, "After", updated.Description)
	require.Equal(t, "Store space", updated.Name)
}

func TestSpaceStore_GetSpace_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}

	row, err := st.GetSpace(ctx, uuid.New())
	require.NoError(t, err)
	require.Nil(t, row)
}

func TestSpaceStore_IsSpaceMember(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	owner := uuid.New()

	created, err := st.CreateSpace(ctx, owner, "Member test", "desc", "private")
	require.NoError(t, err)

	member, err := st.IsSpaceMember(ctx, created.ID, owner)
	require.NoError(t, err)
	require.True(t, member)

	outsider, err := st.IsSpaceMember(ctx, created.ID, uuid.New())
	require.NoError(t, err)
	require.False(t, outsider)
}

func TestSpaceStore_CreateSpace_EmptyName(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}

	_, err := st.CreateSpace(ctx, uuid.New(), "   ", "desc", "private")
	require.Error(t, err)
}

func TestSpaceStore_ListMySpacesPage_InvalidCursor(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}

	_, err := st.ListMySpacesPage(ctx, uuid.New(), "not-a-valid-cursor", 10)
	require.ErrorIs(t, err, ErrInvalidListCursor)
}

func TestSpaceStore_UpdateSpace_NoFields(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	owner := uuid.New()

	row, err := st.CreateSpace(ctx, owner, "No-op", "Before", "private")
	require.NoError(t, err)

	updated, err := st.UpdateSpace(ctx, row.ID, UpdateSpaceInput{})
	require.NoError(t, err)
	require.Equal(t, "No-op", updated.Name)
	require.Equal(t, "Before", updated.Description)
}

func TestSpaceStore_UpdateSpace_IconAndBanner(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	owner := uuid.New()

	row, err := st.CreateSpace(ctx, owner, "Assets", "desc", "private")
	require.NoError(t, err)

	icon := "https://cdn.voice.gg/spaces/icon.webp"
	banner := "https://cdn.voice.gg/spaces/banner.webp"
	updated, err := st.UpdateSpace(ctx, row.ID, UpdateSpaceInput{IconURL: &icon, BannerURL: &banner})
	require.NoError(t, err)
	require.NotNil(t, updated.IconURL)
	require.NotNil(t, updated.BannerURL)
	require.Equal(t, icon, *updated.IconURL)
	require.Equal(t, banner, *updated.BannerURL)
}

func TestSpaceStore_AllowGuests_DefaultsFalseAndUpdates(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	row, err := st.CreateSpace(ctx, uuid.New(), "Guest admission", "", "private")
	require.NoError(t, err)
	require.False(t, row.AllowGuests)

	allow := true
	updated, err := st.UpdateSpace(ctx, row.ID, UpdateSpaceInput{AllowGuests: &allow})
	require.NoError(t, err)
	require.True(t, updated.AllowGuests)
}

func TestAllowGuestsFailClosedMigration_BackfillsLegacyRowsAndDefault(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	for _, name := range []string{
		"000001_init.up.sql", "000002_tree.up.sql", "000003_invites.up.sql",
		"000004_moderation.up.sql", "000005_space_subscriptions.up.sql", "000006_allow_guests.up.sql",
	} {
		migrationPath := filepath.Join(repoRoot(t), "src", "backend", "migrations", "space_db", name)
		sqlBytes, err := os.ReadFile(migrationPath)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(sqlBytes))
		require.NoError(t, err)
	}
	st := &SpaceStore{Pool: pool}
	legacy, err := st.CreateSpace(ctx, uuid.New(), "Legacy open admission", "", "private")
	require.NoError(t, err)
	require.True(t, legacy.AllowGuests, "000006 historically defaulted open")

	migrationPath := filepath.Join(repoRoot(t), "src", "backend", "migrations", "space_db", "000016_allow_guests_fail_closed.up.sql")
	sqlBytes, err := os.ReadFile(migrationPath)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(sqlBytes))
	require.NoError(t, err)

	var legacyAllowGuests bool
	err = pool.QueryRow(ctx, `SELECT allow_guests FROM spaces WHERE id = $1`, legacy.ID).Scan(&legacyAllowGuests)
	require.NoError(t, err)
	require.False(t, legacyAllowGuests)
	createdAfterMigration, err := st.CreateSpace(ctx, uuid.New(), "New closed admission", "", "private")
	require.NoError(t, err)
	var newAllowGuests bool
	err = pool.QueryRow(ctx, `SELECT allow_guests FROM spaces WHERE id = $1`, createdAfterMigration.ID).Scan(&newAllowGuests)
	require.NoError(t, err)
	require.False(t, newAllowGuests)
}

func TestAllowGuestsFailClosedMigration_DownRefusesAndPreservesHardenedAdmission(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	for _, name := range []string{
		"000001_init.up.sql", "000002_tree.up.sql", "000003_invites.up.sql",
		"000004_moderation.up.sql", "000005_space_subscriptions.up.sql", "000006_allow_guests.up.sql",
	} {
		raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "migrations", "space_db", name))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(raw))
		require.NoError(t, err)
	}
	st := &SpaceStore{Pool: pool}
	legacy, err := st.CreateSpace(ctx, uuid.New(), "Legacy admission state", "", "private")
	require.NoError(t, err)
	require.True(t, legacy.AllowGuests)

	upSQL, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "migrations", "space_db", "000016_allow_guests_fail_closed.up.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(upSQL))
	require.NoError(t, err)
	createdAfterHardening, err := st.CreateSpace(ctx, uuid.New(), "Hardened default", "", "private")
	require.NoError(t, err)

	downSQL, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "migrations", "space_db", "000016_allow_guests_fail_closed.down.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(downSQL))
	require.Error(t, err)
	var refusal *pgconn.PgError
	require.ErrorAs(t, err, &refusal)
	require.Equal(t, "P0001", refusal.Code)

	for _, spaceID := range []uuid.UUID{legacy.ID, createdAfterHardening.ID} {
		var allowGuests bool
		require.NoError(t, pool.QueryRow(ctx, `SELECT allow_guests FROM spaces WHERE id = $1`, spaceID).Scan(&allowGuests))
		require.False(t, allowGuests, "refusing DOWN must preserve the hardened backfill")
	}
	var columnDefault string
	require.NoError(t, pool.QueryRow(ctx, `
SELECT column_default
FROM information_schema.columns
WHERE table_schema = 'public' AND table_name = 'spaces' AND column_name = 'allow_guests'`).Scan(&columnDefault))
	require.Equal(t, "false", columnDefault, "refusing DOWN must preserve the fail-closed default")
}
