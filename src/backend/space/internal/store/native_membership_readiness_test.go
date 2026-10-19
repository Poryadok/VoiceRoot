package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
)

// Provision a real login, not SET ROLE on an elevated session: the production
// guard authenticates session_user and rejects a serving owner/superuser.
func nativeReadinessLogin(t *testing.T, ctx context.Context, admin *pgxpool.Pool) *pgxpool.Pool {
	t.Helper()
	role := "readiness_" + uuid.New().String()[:8]
	_, err := admin.Exec(ctx, "CREATE ROLE "+pgx.Identifier{role}.Sanitize()+" LOGIN PASSWORD 'isolated-fixture-only'")
	require.NoError(t, err)
	_, err = admin.Exec(ctx, "GRANT USAGE ON SCHEMA public TO "+pgx.Identifier{role}.Sanitize()+"; GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA public TO "+pgx.Identifier{role}.Sanitize())
	require.NoError(t, err)
	cfg := admin.Config().Copy()
	cfg.ConnConfig.User = role
	cfg.ConnConfig.Password = "isolated-fixture-only"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), "DROP OWNED BY "+pgx.Identifier{role}.Sanitize())
		_, _ = admin.Exec(context.Background(), "DROP ROLE "+pgx.Identifier{role}.Sanitize())
	})
	var actual string
	require.NoError(t, pool.QueryRow(ctx, "SELECT session_user").Scan(&actual))
	require.Equal(t, role, actual)
	return pool
}

func nativeReadinessTx(ctx context.Context, pool *pgxpool.Pool, space uuid.UUID, fn func(pgx.Tx) error) error {
	tx, e := pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, spaceMutationAdvisoryKey(space)); e != nil {
		return e
	}
	if e = fn(tx); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

// This uses a genuine signed Creator publication before44, rather than a
// fixture-minted readiness capability or synthetic native proof.
func TestNativeReadinessAdversarialTransactions(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "41")
	st := &SpaceStore{Pool: db.Pool}
	create, account, profile := signedNativeCreatorCurrent(t, st, &spacev1.CreateSpaceRequest{Name: "pre-barrier Creator", Visibility: "private"})
	space, e := create()
	require.NoError(t, e)
	var oldEpisode, oldCutover uuid.UUID
	var oldGeneration int64
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT episode_id,cutover_id,generation FROM space_effective_member_heads WHERE space_id=$1 AND profile_id=$2 AND boundary_state='KNOWN'`, space.ID, profile).Scan(&oldEpisode, &oldCutover, &oldGeneration))
	db.Run(true, "up")
	serving := nativeReadinessLogin(t, ctx, db.Pool)
	require.NoError(t, nativeReadinessTx(ctx, db.Pool, space.ID, func(tx pgx.Tx) error {
		_, err := readOrdinaryMemberAuthority(ctx, tx, space.ID, profile)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `SELECT public.space_native_membership_initialize($1)`, space.ID)
		return err
	}))
	var episode, cutover uuid.UUID
	var generation int64
	var boundary string
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT episode_id,cutover_id,generation,boundary_state FROM space_effective_member_heads WHERE space_id=$1 AND profile_id=$2`, space.ID, profile).Scan(&episode, &cutover, &generation, &boundary))
	require.NotEqual(t, oldEpisode, episode)
	require.Equal(t, oldCutover, cutover)
	require.Equal(t, oldGeneration+1, generation)
	require.Equal(t, "UNKNOWN", boundary)
	// Finalize the actual existing transition engine, including every DB profile.
	finish := func(tx pgx.Tx, mutation func() error) (*nativeMembershipDBMutation, error) {
		scoped := &SpaceStore{Pool: serving, tx: tx}
		token, err := scoped.beginNativeMembershipDBMutation(ctx, space.ID, []uuid.UUID{profile})
		if err != nil {
			return nil, err
		}
		var started time.Time
		if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&started); err != nil {
			return nil, err
		}
		old, localCutover, err := loadEffectiveHead(ctx, tx, space.ID, profile)
		if err != nil {
			return nil, err
		}
		before, err := scoped.readEffectiveMemberSources(ctx, space.ID, profile, started)
		if err != nil {
			return nil, err
		}
		if mutation != nil {
			if err = mutation(); err != nil {
				return nil, err
			}
		}
		var at time.Time
		if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&at); err != nil {
			return nil, err
		}
		after, err := scoped.readEffectiveMemberSources(ctx, space.ID, profile, at)
		if err != nil {
			return nil, err
		}
		next, err := advanceEffectiveEpisode(old, before, after, started, at, uuid.New())
		if err != nil {
			return nil, err
		}
		if err = saveEffectiveHead(ctx, tx, space.ID, profile, localCutover, old.Revision, next); err != nil {
			return nil, err
		}
		return token, scoped.completeNativeMembershipDBMutation(ctx, token)
	}
	t.Run("capture alone cannot commit", func(t *testing.T) {
		err := nativeReadinessTx(ctx, serving, space.ID, func(tx pgx.Tx) error {
			var token uuid.UUID
			return tx.QueryRow(ctx, `SELECT public.space_native_membership_begin($1,$2)`, space.ID, []uuid.UUID{profile}).Scan(&token)
		})
		require.Error(t, err)
	})
	t.Run("old endpoint equality after ban ABA cannot preserve history", func(t *testing.T) {
		err := nativeReadinessTx(ctx, serving, space.ID, func(tx pgx.Tx) error {
			_, err := finish(tx, func() error {
				if _, e := tx.Exec(ctx, `INSERT INTO space_bans(space_id,account_id,banned_by_profile_id) VALUES($1,$2,$3)`, space.ID, account, profile); e != nil {
					return e
				}
				_, e := tx.Exec(ctx, `DELETE FROM space_bans WHERE space_id=$1 AND account_id=$2`, space.ID, account)
				return e
			})
			return err
		})
		require.Error(t, err, "DB observes intermediate real ordinary loss even though endpoints match")
	})
	t.Run("mutation after finalizer dirties completion", func(t *testing.T) {
		err := nativeReadinessTx(ctx, serving, space.ID, func(tx pgx.Tx) error {
			if _, e := finish(tx, nil); e != nil {
				return e
			}
			_, e := tx.Exec(ctx, `UPDATE space_members SET joined_at=joined_at WHERE space_id=$1 AND profile_id=$2`, space.ID, profile)
			return e
		})
		require.Error(t, err)
	})
	t.Run("rolled back later mutation preserves only original seal", func(t *testing.T) {
		require.NoError(t, nativeReadinessTx(ctx, serving, space.ID, func(tx pgx.Tx) error {
			if _, e := finish(tx, nil); e != nil {
				return e
			}
			if _, e := tx.Exec(ctx, `SAVEPOINT later_source`); e != nil {
				return e
			}
			if _, e := tx.Exec(ctx, `UPDATE space_members SET joined_at=joined_at WHERE space_id=$1 AND profile_id=$2`, space.ID, profile); e != nil {
				return e
			}
			_, e := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT later_source`)
			return e
		}))
	})
	t.Run("immediate constraints cannot bless later SQL", func(t *testing.T) {
		immediatePassed := false
		err := nativeReadinessTx(ctx, serving, space.ID, func(tx pgx.Tx) error {
			if _, e := finish(tx, nil); e != nil {
				return e
			}
			if _, e := tx.Exec(ctx, `SET CONSTRAINTS ALL IMMEDIATE`); e != nil {
				return e
			}
			immediatePassed = true
			_, e := tx.Exec(ctx, `UPDATE space_members SET joined_at=joined_at WHERE space_id=$1 AND profile_id=$2`, space.ID, profile)
			return e
		})
		require.True(t, immediatePassed, "completed frame must pass IMMEDIATE without recursive trigger updates")
		require.ErrorContains(t, err, "requires DB capture")
	})
	t.Run("runtime cannot forge private storage or disable trigger", func(t *testing.T) {
		for _, sql := range []string{`UPDATE native_membership_guard.spaces SET state='COMPLETE'`, `ALTER TABLE space_members DISABLE TRIGGER a000_native_membership_capture`, `SET session_replication_role=replica`} {
			_, err := serving.Exec(ctx, sql)
			require.Error(t, err, sql)
		}
	})
	t.Run("actual source trigger tamper closes epoch", func(t *testing.T) {
		tx, err := db.Pool.Begin(ctx)
		require.NoError(t, err)
		defer tx.Rollback(ctx)
		_, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, spaceMutationAdvisoryKey(space.ID))
		require.NoError(t, err)
		_, err = tx.Exec(ctx, `ALTER TABLE space_members DISABLE TRIGGER a000_native_membership_capture`)
		require.NoError(t, err)
		_, err = tx.Exec(ctx, `SELECT public.space_native_membership_begin($1,$2)`, space.ID, []uuid.UUID{profile})
		require.ErrorContains(t, err, "catalog is not exact")
	})
	t.Run("finite completion rechecked at commit", func(t *testing.T) {
		err := nativeReadinessTx(ctx, serving, space.ID, func(tx pgx.Tx) error {
			if _, e := finish(tx, nil); e != nil {
				return e
			}
			_, e := tx.Exec(ctx, `SELECT pg_sleep(5.05)`)
			return e
		})
		require.Error(t, err, "earlier finalization cannot cover waiting SQL after it")
	})
	t.Run("born empty baseline is Space specific", func(t *testing.T) {
		born, owner := uuid.New(), uuid.New()
		require.NoError(t, nativeReadinessTx(ctx, serving, born, func(tx pgx.Tx) error {
			scoped := &SpaceStore{Pool: serving, tx: tx}
			token, err := scoped.beginNativeMembershipDBMutation(ctx, born, []uuid.UUID{owner})
			if err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `INSERT INTO spaces(id,name,owner_profile_id) VALUES($1,'born guarded empty',$2)`, born, owner); err != nil {
				return err
			}
			var at time.Time
			if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&at); err != nil {
				return err
			}
			head, err := cutoverEffectiveEpisode(effectiveSources{}, at, uuid.Nil)
			if err != nil {
				return err
			}
			if err = saveEffectiveHead(ctx, tx, born, owner, uuid.New(), 0, head); err != nil {
				return err
			}
			return scoped.completeNativeMembershipDBMutation(ctx, token)
		}))
		require.NoError(t, nativeReadinessTx(ctx, serving, born, func(tx pgx.Tx) error {
			_, e := tx.Exec(ctx, `SELECT public.space_native_membership_require($1)`, born)
			return e
		}))
		require.Error(t, nativeReadinessTx(ctx, serving, uuid.New(), func(tx pgx.Tx) error {
			_, e := tx.Exec(ctx, `SELECT public.space_native_membership_require($1)`, uuid.New())
			return e
		}))
	})
	// The protected Go reader must stay unavailable until186 qualifies the real44
	// native catalog and integrates all writers; no fake catalog overlay is used.
	require.Error(t, nativeReadinessTx(ctx, serving, space.ID, func(tx pgx.Tx) error {
		return (&SpaceStore{Pool: serving, tx: tx}).RequireScopedNativeMembershipReadiness(ctx, space.ID)
	}))
}

func TestNativeReadinessDownRequiresFreshRC(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "up")
	raw, e := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db/000044_native_membership_readiness.down.sql"))
	require.NoError(t, e)
	rr, e := db.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	require.NoError(t, e)
	defer rr.Rollback(ctx)
	var empty bool
	require.NoError(t, rr.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM native_membership_guard.spaces)`).Scan(&empty))
	require.True(t, empty)
	_, e = rr.Exec(ctx, string(raw))
	require.ErrorContains(t, e, "DOWN requires READ COMMITTED")
	require.NoError(t, rr.Rollback(ctx))
	db.Run(true, "down", "1")
	db.Run(true, "up")
}

func TestNativeReadinessCutoverAndGuardedRemoval(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "34")
	space, profile := uuid.New(), uuid.New()
	_, err := db.Pool.Exec(ctx, `INSERT INTO spaces(id,name,owner_profile_id) VALUES($1,'guarded legacy',$2)`, space, profile)
	require.NoError(t, err)
	_, err = db.Pool.Exec(ctx, `INSERT INTO space_members(space_id,profile_id) VALUES($1,$2)`, space, profile)
	require.NoError(t, err)
	db.Run(true, "up")
	serving := nativeReadinessLogin(t, ctx, db.Pool)
	scoped := func(pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
		tx, e := pool.Begin(ctx)
		if e != nil {
			return e
		}
		defer tx.Rollback(ctx)
		if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, spaceMutationAdvisoryKey(space)); e != nil {
			return e
		}
		if e = fn(tx); e != nil {
			return e
		}
		return tx.Commit(ctx)
	}
	require.Error(t, scoped(serving, func(tx pgx.Tx) error {
		_, e := tx.Exec(ctx, `SELECT public.space_native_membership_require($1)`, space)
		return e
	}), "head/table presence cannot ready an uncutover Space")
	require.Error(t, scoped(serving, func(tx pgx.Tx) error {
		_, e := tx.Exec(ctx, `SELECT public.space_native_membership_initialize($1)`, space)
		return e
	}), "ordinary application cannot initialize")
	require.NoError(t, scoped(db.Pool, func(tx pgx.Tx) error {
		authority, e := readOrdinaryMemberAuthority(ctx, tx, space, profile)
		if e != nil {
			return e
		}
		require.True(t, authority.GetExplicitLegacy())
		_, e = tx.Exec(ctx, `SELECT public.space_native_membership_initialize($1)`, space)
		return e
	}))
	require.NoError(t, scoped(serving, func(tx pgx.Tx) error {
		_, e := tx.Exec(ctx, `SELECT public.space_native_membership_require($1)`, space)
		return e
	}), "actual nonowner SQL guard positive, not whole native catalog qualification")
	require.Error(t, scoped(db.Pool, func(tx pgx.Tx) error {
		_, e := tx.Exec(ctx, `SELECT public.space_native_membership_require($1)`, space)
		return e
	}), "owner session must never be a serving positive")
	require.Error(t, scoped(db.Pool, func(tx pgx.Tx) error {
		_, e := tx.Exec(ctx, `SELECT public.space_native_membership_initialize($1)`, space)
		return e
	}), "completed cutover cannot be repeated")
	var boundary string
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT boundary_state FROM space_effective_member_heads WHERE space_id=$1 AND profile_id=$2`, space, profile).Scan(&boundary))
	require.Equal(t, "UNKNOWN", boundary)
	// Actual DB token + actual native retirement + actual existing transition/CAS.
	require.NoError(t, scoped(serving, func(tx pgx.Tx) error {
		st := &SpaceStore{Pool: serving, tx: tx}
		token, e := st.beginNativeMembershipDBMutation(ctx, space, []uuid.UUID{profile})
		if e != nil {
			return e
		}
		require.Equal(t, []uuid.UUID{profile}, token.profiles)
		old, cutover, e := loadEffectiveHead(ctx, tx, space, profile)
		if e != nil {
			return e
		}
		var started time.Time
		if e = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&started); e != nil {
			return e
		}
		before, e := st.readEffectiveMemberSources(ctx, space, profile, started)
		if e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, `DELETE FROM space_members WHERE space_id=$1 AND profile_id=$2`, space, profile); e != nil {
			return e
		}
		var at time.Time
		if e = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&at); e != nil {
			return e
		}
		after, e := st.readEffectiveMemberSources(ctx, space, profile, at)
		if e != nil {
			return e
		}
		next, e := advanceEffectiveEpisode(old, before, after, started, at, uuid.New())
		if e != nil {
			return e
		}
		if e = saveEffectiveHead(ctx, tx, space, profile, cutover, old.Revision, next); e != nil {
			return e
		}
		return st.completeNativeMembershipDBMutation(ctx, token)
	}))
	var state string
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT state FROM space_effective_member_heads WHERE space_id=$1 AND profile_id=$2`, space, profile).Scan(&state))
	require.Equal(t, "RETIRED", state)
	var frames int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM native_membership_guard.frames`).Scan(&frames))
	require.Zero(t, frames, "commit removes profile-bearing capture records")
	rr, e := serving.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	require.NoError(t, e)
	defer rr.Rollback(ctx)
	_, e = rr.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, spaceMutationAdvisoryKey(space))
	require.NoError(t, e)
	_, e = rr.Exec(ctx, `SELECT public.space_native_membership_begin($1,$2)`, space, []uuid.UUID{profile})
	require.Error(t, e, "RR cannot capture under inherited snapshot")
}

func TestNativeReadinessRejectsUnadaptedServingWriter(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "34")
	space, profile := uuid.New(), uuid.New()
	_, err := db.Pool.Exec(ctx, `INSERT INTO spaces(id,name,owner_profile_id) VALUES($1,'old writer oracle',$2);`, space, profile)
	require.NoError(t, err)
	_, err = db.Pool.Exec(ctx, `INSERT INTO space_members(space_id,profile_id) VALUES($1,$2)`, space, profile)
	require.NoError(t, err)
	db.Run(true, "up")
	serving := nativeReadinessLogin(t, ctx, db.Pool)
	tx, err := serving.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, spaceMutationAdvisoryKey(space))
	require.NoError(t, err)
	_, mutationErr := tx.Exec(ctx, `DELETE FROM space_members WHERE space_id=$1 AND profile_id=$2`, space, profile)
	commitErr := tx.Commit(ctx)
	require.True(t, mutationErr != nil || commitErr != nil, "unadapted serving writer committed without DB capture/completion")
	var present bool
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_members WHERE space_id=$1 AND profile_id=$2)`, space, profile).Scan(&present))
	require.True(t, present, "refused source mutation must roll back")
}
