package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestJoinAttempt_PublicOperationDowngradePreservesHistory(t *testing.T) {
	for _, populated := range []bool{false, true} {
		name := "empty"
		if populated {
			name = "populated"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			pool := startJoinAttemptPostgres(t, ctx)
			applySpaceMigrationForStoreTest(t, ctx, pool)
			st := &SpaceStore{Pool: pool}
			space, err := st.CreateSpace(ctx, uuid.New(), "Operation downgrade", "", "public")
			require.NoError(t, err)
			applyEntryPolicyMigration(t, st)
			applyJoinAttemptMigration(t, st)
			applyJoinOperationMigration(t, st)
			var before string
			if populated {
				_, err = st.ReserveJoinOperation(ctx, JoinOperationInput{SpaceID: space.ID, ActorAccountID: uuid.New(), ActorProfileID: uuid.New(), OperationID: uuid.New(), Method: JoinOperationStart, RequestSHA256: joinDigest("downgrade fixture", "start")}, nil)
				require.NoError(t, err)
				require.NoError(t, pool.QueryRow(ctx, `SELECT md5(row_to_json(o)::text) FROM space_join_operations o`).Scan(&before))
			}
			raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db/000036_join_operation_namespace.down.sql"))
			require.NoError(t, err)
			conn, err := pool.Acquire(ctx)
			require.NoError(t, err)
			defer conn.Release()
			_, err = conn.Exec(ctx, string(raw))
			if populated {
				require.Error(t, err)
				_, rollbackErr := conn.Exec(ctx, `ROLLBACK`)
				require.NoError(t, rollbackErr)
				var after string
				require.NoError(t, conn.QueryRow(ctx, `SELECT md5(row_to_json(o)::text) FROM space_join_operations o`).Scan(&after))
				require.Equal(t, before, after)
			} else {
				require.NoError(t, err)
				var table *string
				require.NoError(t, conn.QueryRow(ctx, `SELECT to_regclass('space_join_operations')::text`).Scan(&table))
				require.Nil(t, table)
			}
		})
	}
}
