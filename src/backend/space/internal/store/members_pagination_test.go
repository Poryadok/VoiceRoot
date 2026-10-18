package store

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestSpaceStore_ListSpaceMembersPage_Pagination(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	pool := memberPaginationPostgres(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	joined := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
	ids := []uuid.UUID{
		uuid.MustParse("00000000-0000-0000-0000-000000000300"),
		uuid.MustParse("00000000-0000-0000-0000-000000000100"),
		uuid.MustParse("00000000-0000-0000-0000-000000000200"),
		uuid.MustParse("00000000-0000-0000-0000-000000000050"),
	}
	space, err := st.CreateSpace(ctx, ids[0], "Member pagination", "", "private")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE space_members SET joined_at=$3 WHERE space_id=$1 AND profile_id=$2`, space.ID, ids[0], joined)
	require.NoError(t, err)
	for i, id := range ids[1:] {
		// The middle pair ties on joined_at; the last member has the lowest UUID.
		offset := time.Hour
		if i == 2 {
			offset = 2 * time.Hour
		}
		_, err = pool.Exec(ctx, `INSERT INTO space_members(space_id,profile_id,joined_at) VALUES($1,$2,$3)`, space.ID, id, joined.Add(offset))
		require.NoError(t, err)
	}
	otherSpace, err := st.CreateSpace(ctx, uuid.New(), "Unrelated roster", "", "private")
	require.NoError(t, err)

	t.Run("complete ordered traversal", func(t *testing.T) {
		for _, size := range []int32{1, 2, 3, 4} {
			t.Run(strconv.Itoa(int(size)), func(t *testing.T) {
				var got []uuid.UUID
				cursor := ""
				seenCursors := map[string]bool{}
				for page := 0; ; page++ {
					require.Less(t, page, len(ids)+1, "pagination must terminate")
					rows, next, err := st.ListSpaceMembersPage(ctx, space.ID, size, cursor)
					require.NoError(t, err)
					require.NotEmpty(t, rows, "a continuation must not skip lower-UUID later members")
					for _, row := range rows {
						require.Equal(t, space.ID, row.SpaceID)
						got = append(got, row.ProfileID)
					}
					if next == "" {
						break
					}
					require.Len(t, rows, int(size))
					require.False(t, seenCursors[next], "cursor must advance")
					seenCursors[next] = true
					cursor = next
				}
				require.Equal(t, ids, got, "joined_at ASC, profile_id ASC with no omissions or duplicates")
			})
		}
	})

	t.Run("legacy UUID boundary", func(t *testing.T) {
		rows, next, err := st.ListSpaceMembersPage(ctx, space.ID, 100, ids[0].String())
		require.NoError(t, err)
		require.Equal(t, ids[1:], memberPaginationIDs(rows))
		require.Empty(t, next)
		rows, next, err = st.ListSpaceMembersPage(ctx, space.ID, 100, ids[1].String())
		require.NoError(t, err)
		require.Equal(t, ids[2:], memberPaginationIDs(rows), "equal-time boundary uses the UUID tie-break")
		require.Empty(t, next)
	})

	t.Run("invalid or foreign legacy boundary", func(t *testing.T) {
		for _, cursor := range []string{"not-a-cursor", "e30", otherSpace.OwnerProfileID.String()} {
			rows, next, err := st.ListSpaceMembersPage(ctx, space.ID, 1, cursor)
			require.ErrorIs(t, err, ErrInvalidListCursor)
			require.Empty(t, rows)
			require.Empty(t, next)
		}
	})

	t.Run("terminal legacy boundary", func(t *testing.T) {
		rows, next, err := st.ListSpaceMembersPage(ctx, space.ID, 1, ids[len(ids)-1].String())
		require.NoError(t, err)
		require.Empty(t, rows)
		require.Empty(t, next)
	})

	t.Run("tuple survives removed boundary", func(t *testing.T) {
		rows, cursor, err := st.ListSpaceMembersPage(ctx, space.ID, 1, "")
		require.NoError(t, err)
		require.Equal(t, ids[:1], memberPaginationIDs(rows))
		require.NotEmpty(t, cursor)
		_, err = pool.Exec(ctx, `DELETE FROM space_members WHERE space_id=$1 AND profile_id=$2`, space.ID, ids[0])
		require.NoError(t, err)
		rows, next, err := st.ListSpaceMembersPage(ctx, space.ID, 100, cursor)
		require.NoError(t, err)
		require.Equal(t, ids[1:], memberPaginationIDs(rows))
		require.Empty(t, next)
		_, _, err = st.ListSpaceMembersPage(ctx, space.ID, 1, ids[0].String())
		require.ErrorIs(t, err, ErrInvalidListCursor, "an unresolvable old boundary must not claim a complete empty page")
	})

	t.Run("empty roster", func(t *testing.T) {
		_, err := pool.Exec(ctx, `DELETE FROM space_members WHERE space_id=$1`, otherSpace.ID)
		require.NoError(t, err)
		rows, next, err := st.ListSpaceMembersPage(ctx, otherSpace.ID, 1, "")
		require.NoError(t, err)
		require.Empty(t, rows)
		require.Empty(t, next)
	})

	t.Run("default and maximum page sizes", func(t *testing.T) {
		capped, err := st.CreateSpace(ctx, uuid.New(), "Page caps", "", "private")
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `INSERT INTO space_members(space_id,profile_id)
SELECT $1,gen_random_uuid() FROM generate_series(1,101)`, capped.ID)
		require.NoError(t, err)
		for _, tc := range []struct {
			requested int32
			want      int
		}{{0, 50}, {-1, 50}, {1000, 100}} {
			rows, next, err := st.ListSpaceMembersPage(ctx, capped.ID, tc.requested, "")
			require.NoError(t, err)
			require.Len(t, rows, tc.want)
			require.NotEmpty(t, next)
		}
	})

	t.Run("effective roster preserves eligibility and manual membership", func(t *testing.T) {
		applyMemberAuthorityIfPresent(t, st)
		owner := uuid.MustParse("00000000-0000-0000-0000-000000000900")
		community, _, err := st.CreateCommunityBootstrapSpace(ctx, CommunityBootstrapSpaceInput{
			OperationID: uuid.New(), ApplicationID: uuid.New(), EnvironmentID: uuid.New(),
			OwnerAccountID: uuid.New(), OwnerProfileID: owner,
			CorporationKey: "pagination-corp", TemplateID: "guild-default",
		})
		require.NoError(t, err)
		var applicationID, environmentID uuid.UUID
		require.NoError(t, pool.QueryRow(ctx, `SELECT application_id,environment_id FROM community_owner_authority WHERE space_id=$1`, community.ID).Scan(&applicationID, &environmentID))
		manual, managedA, managedB, expired := ids[2], ids[1], ids[0], ids[3]
		revoked := uuid.MustParse("00000000-0000-0000-0000-000000000400")
		_, err = pool.Exec(ctx, `UPDATE space_members SET joined_at=$2 WHERE space_id=$1`, community.ID, joined)
		require.NoError(t, err)
		publishAdmissionFixture(t, st, community.ID, owner, manual)
		_, err = pool.Exec(ctx, `UPDATE space_members SET joined_at=$3 WHERE space_id=$1 AND profile_id=$2`, community.ID, manual, joined.Add(time.Hour))
		require.NoError(t, err)
		_, err = st.ApplyCommunityRoster(ctx, CommunityRosterInput{
			OperationID: uuid.New(), ApplicationID: applicationID, EnvironmentID: environmentID,
			CorporationKey: "pagination-corp", SpaceID: community.ID, OwnerGeneration: 1,
			SourceRevision: 1, SnapshotSHA256: []byte("11111111111111111111111111111111"),
			LeaseExpiresAt: time.Now().Add(time.Minute), ProfileIDs: []uuid.UUID{expired, managedA, manual, managedB, revoked},
		})
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `UPDATE community_roster_operations SET created_at=$2 WHERE space_id=$1`, community.ID, joined.Add(2*time.Hour))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `UPDATE community_roster_members SET lease_expires_at=clock_timestamp()-interval '1 second' WHERE space_id=$1 AND profile_id=$2`, community.ID, expired)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `UPDATE community_roster_members SET revoked_at=clock_timestamp() WHERE space_id=$1 AND profile_id=$2`, community.ID, revoked)
		require.NoError(t, err)
		var got []uuid.UUID
		cursor := ""
		for page := 0; ; page++ {
			require.Less(t, page, 5)
			rows, next, err := st.ListSpaceMembersPage(ctx, community.ID, 1, cursor)
			require.NoError(t, err)
			got = append(got, memberPaginationIDs(rows)...)
			if next == "" {
				break
			}
			cursor = next
		}
		require.Equal(t, []uuid.UUID{owner, manual, managedA, managedB}, got)
		rows, next, err := st.ListSpaceMembersPage(ctx, community.ID, 100, managedA.String())
		require.NoError(t, err)
		require.Equal(t, []uuid.UUID{managedB}, memberPaginationIDs(rows), "legacy boundary can come from the managed roster")
		require.Empty(t, next)
		for _, update := range []string{
			`UPDATE community_owner_authority SET owner_generation=2 WHERE space_id=$1`,
			`UPDATE community_owner_authority SET owner_generation=1,roster_lease_expires_at=clock_timestamp()-interval '1 second' WHERE space_id=$1`,
		} {
			_, err = pool.Exec(ctx, update, community.ID)
			require.NoError(t, err)
			rows, _, err := st.ListSpaceMembersPage(ctx, community.ID, 100, "")
			require.NoError(t, err)
			require.Equal(t, []uuid.UUID{owner, manual}, memberPaginationIDs(rows), "source expiry/generation never removes manual membership")
		}
	})
}

func memberPaginationIDs(rows []*MembershipRow) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ProfileID)
	}
	return ids
}

// The optional DSN is for a dedicated disposable fixture on socket-free runners.
// Ordinary CI continues to use the canonical Testcontainers helper.
func memberPaginationPostgres(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("VOICE_SPACE_PAGINATION_TEST_DSN")
	if dsn == "" {
		return startSpacePostgresForStoreTest(t, ctx)
	}
	admin, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(admin.Close)
	name := "space_pagination_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = admin.Exec(ctx, `CREATE DATABASE `+pgx.Identifier{name}.Sanitize())
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, err := admin.Exec(cleanupCtx, `DROP DATABASE `+pgx.Identifier{name}.Sanitize()+` WITH (FORCE)`)
		require.NoError(t, err)
	})
	config, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	config.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}
