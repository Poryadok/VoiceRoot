package store

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/pkg/integrationtest"
)

func userModuleRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	// internal/store/*.go -> monorepo root
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
}

func TestProfileStore_SearchProfilesAfter_postgres(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "userdb", "")
	integrationtest.ApplyUserDBMigrations(t, ctx, pool, userModuleRepoRoot(t))

	viewerAcc := uuid.New()
	otherAcc := uuid.New()
	pidOther := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
		VALUES ($1, $2, 'likeuser', '0001', 'Literal 50%_off sale', true)`,
		pidOther, otherAcc)
	require.NoError(t, err)

	st := NewProfileStore(pool)
	backfillSearchFixtures(t, ctx, st)

	t.Run("literal percent and underscore in display_name", func(t *testing.T) {
		rows, err := st.SearchProfilesAfter(ctx, viewerAcc, "50%_off", nil, 20)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		require.Equal(t, pidOther, rows[0].ID)
	})

	t.Run("keyset cursor continues after first page", func(t *testing.T) {
		acc1, acc2 := uuid.New(), uuid.New()
		p1, p2 := uuid.New(), uuid.New()
		_, err := pool.Exec(ctx, `
			INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
			VALUES ($1, $2, 'csr_a', '0002', 'Cursor A', true)`,
			p1, acc1)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `
			INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
			VALUES ($1, $2, 'csr_b', '0003', 'Cursor B', true)`,
			p2, acc2)
		require.NoError(t, err)
		backfillSearchFixtures(t, ctx, st)

		first, err := st.SearchProfilesAfter(ctx, viewerAcc, "Cursor", nil, 1)
		require.NoError(t, err)
		require.Len(t, first, 1)
		c := ProfileSearchCursor{
			UsernameLower: strings.ToLower(first[0].Username),
			Discriminator: first[0].Discriminator,
			ID:            first[0].ID,
		}
		second, err := st.SearchProfilesAfter(ctx, viewerAcc, "Cursor", &c, 10)
		require.NoError(t, err)
		require.Len(t, second, 1)
		require.Equal(t, "csr_b", second[0].Username)
	})
}

func backfillSearchFixtures(t *testing.T, ctx context.Context, st *ProfileStore) {
	t.Helper()
	for {
		result, err := st.BackfillSearchKeys(ctx, 128)
		require.NoError(t, err)
		if result.Done {
			return
		}
	}
}

// This exercises the actual PostgreSQL predicate and ranked cursor, rather than
// only proving that the shared normalizer produces equivalent strings.
func TestProfileStore_SearchProfilesAfter_NormalizedLookup(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "userdb", "")
	integrationtest.ApplyUserDBMigrations(t, ctx, pool, userModuleRepoRoot(t))
	viewer := uuid.New()
	verifiedID := uuid.New()
	var want []uuid.UUID
	fixtures := []struct {
		id       uuid.UUID
		account  uuid.UUID
		username string
		display  string
		verified string
		deleted  bool
		match    bool
	}{
		{verifiedID, uuid.New(), "zzzofficial", "Fаker", "personal", false, true},
		{uuid.New(), uuid.New(), "Faker", "Latin username", "none", false, true},
		{uuid.New(), uuid.New(), "fаker", "Cyrillic a username", "none", false, true},
		{uuid.New(), uuid.New(), "Ｆａｋｅｒ", "NFKC username", "none", false, true},
		{uuid.New(), uuid.New(), "displayonly", "Ｆａｋｅｒ", "none", false, true},
		{uuid.New(), viewer, "ownfaker", "Own account", "none", false, false},
		{uuid.New(), uuid.New(), "deletedfaker", "Deleted profile", "none", true, false},
		{uuid.New(), uuid.New(), "unrelated", "No matching name", "none", false, false},
	}
	for _, f := range fixtures {
		_, err := pool.Exec(ctx, `INSERT INTO profiles
			(id, account_id, username, discriminator, display_name, is_primary, verification_type, deleted_at)
			VALUES ($1,$2,$3,'0001',$4,true,$5,CASE WHEN $6 THEN now() ELSE NULL END)`,
			f.id, f.account, f.username, f.display, f.verified, f.deleted)
		require.NoError(t, err)
		if f.match {
			want = append(want, f.id)
		}
	}
	st := NewProfileStore(pool)
	backfillSearchFixtures(t, ctx, st)
	for _, query := range []string{"Faker", "fаker", "ＦＡＫＥＲ"} {
		t.Run(query, func(t *testing.T) {
			first, err := st.SearchProfilesAfter(ctx, viewer, query, nil, 1)
			require.NoError(t, err)
			require.Len(t, first, 1)
			require.Equal(t, verifiedID, first[0].ID, "verification outranks alphabetical username")
			got := []uuid.UUID{first[0].ID}
			last := first[0]
			for page := 0; page <= len(want); page++ {
				rank := 1
				if last.VerificationType != "none" {
					rank = 0
				}
				encoded, err := EncodeSearchCursor(ProfileSearchCursor{
					VerificationRank: &rank, UsernameLower: strings.ToLower(last.Username),
					Discriminator: last.Discriminator, ID: last.ID,
				})
				require.NoError(t, err)
				cursor, err := DecodeSearchCursor(encoded)
				require.NoError(t, err)
				rows, err := st.SearchProfilesAfter(ctx, viewer, query, cursor, 2)
				require.NoError(t, err)
				if len(rows) == 0 {
					break
				}
				for _, row := range rows {
					got = append(got, row.ID)
				}
				last = rows[len(rows)-1]
			}
			require.ElementsMatch(t, want, got, "all normalized matches exactly once; own, deleted and unrelated rows excluded")
		})
	}
}
