package lfpconsentstore

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func TestDownEmptyAndIsolationBoundaries(t *testing.T) {
	for _, staleSnapshot := range []bool{false, true} {
		name := "empty read committed"
		if staleSnapshot {
			name = "repeatable read rejected"
		}
		t.Run(name, func(t *testing.T) {
			s, _, _ := fixture(t)
			ctx := context.Background()
			for _, migration := range []string{"000006_lfp_actor_provenance", "000005_lfp_consent_journal"} {
				b, e := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "story_db", migration+".down.sql"))
				require.NoError(t, e)
				if staleSnapshot {
					tx, e := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
					require.NoError(t, e)
					_, e = tx.Exec(ctx, "SELECT count(*) FROM story_lfp_admission_head")
					require.NoError(t, e)
					_, e = tx.Exec(ctx, string(b))
					require.ErrorContains(t, e, "requires a fresh READ COMMITTED transaction")
					require.NoError(t, tx.Rollback(ctx))
				} else {
					if migration == "000005_lfp_consent_journal" {
						_, e = s.Pool.Exec(ctx, "TRUNCATE story_lfp_admission_head,story_lfp_offers CASCADE")
						require.NoError(t, e)
					}
					_, e = s.Pool.Exec(ctx, string(b))
					require.NoError(t, e, migration)
				}
			}
		})
	}
}

// Exercise the actual migration SQL with an uncommitted first evidence row.
// An empty MVCC observation before DROP waits is insufficient: the writer can
// commit in that gap, and a later DROP must not destroy the retained evidence.
func TestDownWaitsBeforeCheckingConcurrentFirstEvidence(t *testing.T) {
	for _, migration := range []string{"000006_lfp_actor_provenance", "000005_lfp_consent_journal"} {
		t.Run(migration, func(t *testing.T) {
			s, _, _ := fixture(t)
			ctx := context.Background()
			readDown := func(name string) string {
				b, e := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "story_db", name+".down.sql"))
				require.NoError(t, e)
				return string(b)
			}
			actor := migration == "000006_lfp_actor_provenance"
			relation := "story_lfp_publication_actors"
			if !actor {
				_, e := s.Pool.Exec(ctx, readDown("000006_lfp_actor_provenance"))
				require.NoError(t, e)
				_, e = s.Pool.Exec(ctx, "TRUNCATE story_lfp_admission_head,story_lfp_offers CASCADE")
				require.NoError(t, e)
				relation = "story_lfp_admission_head"
			}
			writer, e := s.Pool.Begin(ctx)
			require.NoError(t, e)
			defer func() { _ = writer.Rollback(ctx) }()
			if actor {
				_, e = writer.Exec(ctx, `INSERT INTO story_lfp_publication_actors(publication_operation_id,canonical_actor,canonical_offer_state,actor_sha256) SELECT operation_id,$1,$2,$3 FROM story_lfp_publications LIMIT 1`, []byte{1}, []byte{1}, make([]byte, 32))
			} else {
				_, e = writer.Exec(ctx, `INSERT INTO story_lfp_admission_head VALUES(true,$1,$2,$3,$4,'closed')`, uuid.New(), uuid.New(), uuid.New(), make([]byte, 32))
			}
			require.NoError(t, e)
			result := make(chan error, 1)
			down := readDown(migration)
			go func() { _, e := s.Pool.Exec(ctx, down); result <- e }()
			deadline := time.Now().Add(3 * time.Second)
			for {
				var waiting int
				require.NoError(t, s.Pool.QueryRow(ctx, `SELECT count(*) FROM pg_locks WHERE relation=$1::regclass AND mode='AccessExclusiveLock' AND NOT granted`, relation).Scan(&waiting))
				if waiting > 0 {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("DOWN did not wait for the actual uncommitted writer")
				}
				time.Sleep(10 * time.Millisecond)
			}
			require.NoError(t, writer.Commit(ctx))
			select {
			case e := <-result:
				require.ErrorContains(t, e, "retained Story LFP")
			case <-time.After(3 * time.Second):
				t.Fatal("DOWN did not finish after first writer committed")
			}
			var count int
			require.NoError(t, s.Pool.QueryRow(ctx, "SELECT count(*) FROM "+relation).Scan(&count))
			require.Equal(t, 1, count, "first retained evidence must survive DOWN")
		})
	}
}
