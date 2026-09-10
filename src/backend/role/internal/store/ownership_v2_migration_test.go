package store

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOwnershipTransferV2_Cycle2DownCannotDiscardConcurrentDurableWrite(t *testing.T) {
	for _, durable := range []string{"receipt", "retirement"} {
		t.Run(durable, func(t *testing.T) {
			s, in := v2Fixture(t)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			down, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "migrations", "role_db", "000010_ownership_v2.down.sql"))
			require.NoError(t, err)
			writer, err := s.Pool.Begin(ctx)
			require.NoError(t, err)
			defer func() { _ = writer.Rollback(context.Background()) }()
			if durable == "retirement" {
				_, err = writer.Exec(ctx, `INSERT INTO role_space_lifecycle(space_id,retired_at) VALUES($1,now())`, in.SpaceID)
			} else {
				hash := sha256.Sum256(in.IntentBytes)
				_, err = writer.Exec(ctx, `INSERT INTO ownership_transfer_v2(operation_id,space_id,protocol_version,old_owner_profile_id,new_owner_profile_id,intent_bytes,intent_hash,state,abort_request_hash) VALUES($1,$2,2,$3,$4,$5,$6,'aborted',$7)`, in.OperationID, in.SpaceID, in.OldOwnerProfileID, in.NewOwnerProfileID, in.IntentBytes, hash[:], in.RequestHash)
			}
			require.NoError(t, err)
			// The new durable fact is uncommitted while the rollback starts. The
			// migration must lock first, then evaluate its guard after this commit.
			migration, err := s.Pool.Acquire(ctx)
			require.NoError(t, err)
			defer migration.Release()
			pid := migration.Conn().PgConn().PID()
			done := make(chan error, 1)
			go func() { _, e := migration.Exec(ctx, string(down)); done <- e }()
			defer func() {
				cancel()
				select {
				case <-done:
				case <-time.After(3 * time.Second):
					t.Error("migration query did not stop")
				}
			}()
			require.Eventually(t, func() bool {
				var waiting bool
				err := s.Pool.QueryRow(ctx, `SELECT wait_event_type='Lock' FROM pg_stat_activity WHERE pid=$1`, pid).Scan(&waiting)
				return err == nil && waiting
			}, 5*time.Second, 20*time.Millisecond, "migration must reach conflicting table lock before writer commits")
			require.NoError(t, writer.Commit(ctx))
			select {
			case err = <-done:
				// Refill for deferred join without retaining a live goroutine.
				done <- err
				require.Error(t, err, "down migration must refuse a durable fact committed while it waits")
			case <-ctx.Done():
				t.Fatal("migration did not finish after writer commit")
			}
			var count int
			if durable == "retirement" {
				err = s.Pool.QueryRow(context.Background(), `SELECT count(*) FROM role_space_lifecycle WHERE space_id=$1 AND retired_at IS NOT NULL`, in.SpaceID).Scan(&count)
			} else {
				err = s.Pool.QueryRow(context.Background(), `SELECT count(*) FROM ownership_transfer_v2 WHERE operation_id=$1`, in.OperationID).Scan(&count)
			}
			require.NoError(t, err)
			require.Equal(t, 1, count, "accepted durable fact survives failed down migration")
		})
	}
}
