package grpcsvc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

type reportDowngradeCase struct {
	version                            int
	name, block, seed, write, retained string
}

func reportDowngradeCases() []reportDowngradeCase {
	return []reportDowngradeCase{
		{version: 7, name: "report_intake", block: "reports",
			write:    `INSERT INTO reports(id,reporter_profile_id,target_type,target_id,category) VALUES($1,$2,'story',$3,'spam')`,
			retained: `SELECT count(*) FROM reports WHERE id=$1 AND validation_state='PENDING' AND NOT validation_publication_legacy`},
		{version: 8, name: "report_subject_provenance", block: "reports",
			seed:     `INSERT INTO reports(id,reporter_profile_id,target_type,target_id,category,validation_state,validation_publication_legacy) VALUES($1,$2,'story',$3,'spam','VALID',true)`,
			write:    `UPDATE reports SET reporter_subject_id=$2,reporter_subject_revision=1,reporter_account_id=$3 WHERE id=$1`,
			retained: `SELECT count(*) FROM reports WHERE id=$1 AND reporter_subject_id IS NOT NULL AND reporter_subject_revision=1`},
		{version: 11, name: "report_privacy_recovery", block: "report_privacy_journal",
			write:    `INSERT INTO report_privacy_journal(report_id,slot,disposition,subject_id,evidence_kind,evidence_id,evidence_version) VALUES($1,'REPORTER','BOUND',$2,'USER_OWNER_FACT',$3,1)`,
			retained: `SELECT count(*) FROM report_privacy_journal WHERE report_id=$1 AND slot='REPORTER' AND disposition='BOUND'`},
	}
}

func reportDowngradeSQL(t *testing.T, c reportDowngradeCase, direction string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/moderation_db", fmt.Sprintf("%06d_%s.%s.sql", c.version, c.name, direction)))
	require.NoError(t, err)
	return string(b)
}

func reportDowngradeSeed(t *testing.T, pool *pgxpool.Pool, c reportDowngradeCase, ids []any) {
	t.Helper()
	if c.seed != "" {
		_, err := pool.Exec(context.Background(), c.seed, ids...)
		require.NoError(t, err)
	}
}

func TestReportDowngradesPreserveConcurrentEvidence(t *testing.T) {
	for _, c := range reportDowngradeCases() {
		t.Run(c.name, func(t *testing.T) {
			pool := reportIntakePoolThrough(t, c.version)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			ids := []any{uuid.New(), uuid.New(), uuid.New()}
			reportDowngradeSeed(t, pool, c, ids)
			blocker, err := pool.Begin(ctx)
			require.NoError(t, err)
			defer func() { _ = blocker.Rollback(context.Background()) }()
			_, err = blocker.Exec(ctx, "LOCK TABLE "+c.block+" IN ACCESS SHARE MODE")
			require.NoError(t, err)
			conn, err := pool.Acquire(ctx)
			require.NoError(t, err)
			defer conn.Release()
			pid := conn.Conn().PgConn().PID()
			down := reportDowngradeSQL(t, c, "down")
			done := make(chan error, 1)
			finished := make(chan struct{})
			go func() {
				defer close(finished)
				_, err := conn.Exec(ctx, down)
				done <- err
			}()
			defer func() {
				cancel()
				_ = blocker.Rollback(context.Background())
				<-finished // Never release a pgx connection still used by Exec.
			}()
			require.Eventually(t, func() bool {
				var waiting bool
				err := pool.QueryRow(ctx, `SELECT cardinality(pg_blocking_pids($1))>0`, pid).Scan(&waiting)
				return err == nil && waiting
			}, 5*time.Second, 10*time.Millisecond)
			// A new connection would queue behind DOWN's pending exclusive lock.
			// The existing holder can upgrade its own lock before releasing it.
			_, err = blocker.Exec(ctx, c.write, ids...)
			require.NoError(t, err)
			require.NoError(t, blocker.Commit(ctx))
			require.Error(t, <-done, "DOWN cannot discard evidence committed during its actual lock wait")
			var count int
			require.NoError(t, pool.QueryRow(ctx, c.retained, ids[0]).Scan(&count))
			require.Equal(t, 1, count)
		})
	}
}

func TestReportDowngradesRejectOldSnapshots(t *testing.T) {
	for _, c := range reportDowngradeCases() {
		t.Run(c.name, func(t *testing.T) {
			pool := reportIntakePoolThrough(t, c.version)
			ctx := context.Background()
			ids := []any{uuid.New(), uuid.New(), uuid.New()}
			reportDowngradeSeed(t, pool, c, ids)
			tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
			require.NoError(t, err)
			defer func() { _ = tx.Rollback(ctx) }()
			var count int
			require.NoError(t, tx.QueryRow(ctx, "SELECT count(*) FROM "+c.block).Scan(&count))
			_, err = pool.Exec(ctx, c.write, ids...)
			require.NoError(t, err)
			_, err = tx.Exec(ctx, reportDowngradeSQL(t, c, "down"))
			require.Error(t, err, "an old inventory snapshot is not current emptiness")
			require.NoError(t, tx.Rollback(ctx))
			require.NoError(t, pool.QueryRow(ctx, c.retained, ids[0]).Scan(&count))
			require.Equal(t, 1, count)
		})
	}
}

func TestReportDowngradesAllowEmptyExpand(t *testing.T) {
	for _, c := range reportDowngradeCases() {
		t.Run(c.name, func(t *testing.T) {
			pool := reportIntakePoolThrough(t, c.version)
			ctx := context.Background()
			_, err := pool.Exec(ctx, reportDowngradeSQL(t, c, "down"))
			require.NoError(t, err)
			_, err = pool.Exec(ctx, reportDowngradeSQL(t, c, "up"))
			require.NoError(t, err)
		})
	}
}
