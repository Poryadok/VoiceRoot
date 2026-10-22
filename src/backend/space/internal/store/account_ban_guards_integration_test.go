//go:build spacebanintegration

package store

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// Explicit phase after the qualified core scenario. Every attempted mutation
// has the real same-backend fence AND native frame; a missing prerequisite is
// not accepted as evidence that the immutable ledger itself rejects a change.
func TestAccountBanLedgerGuards50(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("SPACE_BAN_TEST_DATABASE_URL")
	require.NotEmpty(t, dsn)
	owner, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(owner.Close)
	serving := projectionServingLogin(t, ctx, owner)
	st := &SpaceStore{Pool: serving}
	space := nativeBanSpace(1)
	cases := []struct{ name, sql string }{
		{"qualification rebind", `UPDATE space_account_qualifications SET account_id='20000000-0000-0000-0000-000000000099' WHERE space_id=$1 AND profile_id='10000000-0000-0000-0000-000000000002'`},
		{"suppression episode rebind", `UPDATE space_account_ban_suppressions SET episode_id=gen_random_uuid() WHERE space_id=$1 AND profile_id='10000000-0000-0000-0000-000000000002'`},
		{"suppression UUID reuse", `UPDATE space_account_ban_suppressions SET member_generation=member_generation+1 WHERE space_id=$1 AND profile_id='10000000-0000-0000-0000-000000000002'`},
		{"suppression source rewind", `UPDATE space_account_ban_suppressions SET source_revision=source_revision-1 WHERE space_id=$1 AND profile_id='10000000-0000-0000-0000-000000000002'`},
		{"terminal root body rewrite", `UPDATE space_account_ban_operations SET body_sha256=decode(repeat('ab',32),'hex') WHERE space_id=$1`},
		{"child manifest rewrite", `UPDATE space_account_ban_captures SET cohort_digest=decode(repeat('cd',32),'hex') WHERE space_id=$1`},
		{"early root deletion", `DELETE FROM space_account_ban_operations WHERE space_id=$1`},
		{"qualification deletion outside P3", `DELETE FROM space_account_qualifications WHERE space_id=$1`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var statementErr error
			var affected int64
			err := st.withOwnershipScope(ctx, []uuid.UUID{space}, func(scoped *SpaceStore) error {
				capture, err := scoped.captureMemberMutationIfPresent(ctx, space, []uuid.UUID{nativeBanProfile(2)})
				if err != nil {
					return err
				}
				require.NotNil(t, capture, "actual native frame required")
				tag, err := scoped.tx.Exec(ctx, tc.sql, space)
				statementErr, affected = err, tag.RowsAffected()
				// Even a broken guard cannot contaminate the source fixture.
				return errors.New("owned negative ledger transaction rollback")
			})
			require.Error(t, err)
			require.Error(t, statementErr, "guard accepted mutation of %d rows", affected)
			var pgErr *pgconn.PgError
			require.ErrorAs(t, statementErr, &pgErr)
			require.Equal(t, "55000", pgErr.Code)
		})
	}
}
