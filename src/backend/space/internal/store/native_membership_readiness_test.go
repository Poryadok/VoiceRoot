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
	t.Run("normal frame cannot invent terminal disposition", func(t *testing.T) {
		require.ErrorIs(t, nativeReadinessTx(ctx, serving, space.ID, func(tx pgx.Tx) error {
			scoped := &SpaceStore{Pool: serving, tx: tx}
			token, e := scoped.beginNativeMembershipDBMutation(ctx, space.ID, []uuid.UUID{profile})
			if e != nil {
				return e
			}
			return scoped.completeNativeMembershipDBPurge(ctx, token)
		}), ErrEffectiveEpisodeUnavailable)
	})
	t.Run("real temporary relation cannot shadow source authority", func(t *testing.T) {
		err := nativeReadinessTx(ctx, serving, space.ID, func(tx pgx.Tx) error {
			if _, e := tx.Exec(ctx, `CREATE TEMP TABLE space_members(space_id uuid,profile_id uuid) ON COMMIT DROP`); e != nil {
				return e
			}
			_, e := tx.Exec(ctx, `SELECT public.space_native_membership_require($1)`, space.ID)
			return e
		})
		require.ErrorContains(t, err, "temporary relation shadowing")
	})
	t.Run("runtime replication bypass privilege closes serving", func(t *testing.T) {
		role := pgx.Identifier{serving.Config().ConnConfig.User}.Sanitize()
		_, err := db.Pool.Exec(ctx, "GRANT SET ON PARAMETER session_replication_role TO "+role)
		require.NoError(t, err)
		defer db.Pool.Exec(context.Background(), "REVOKE SET ON PARAMETER session_replication_role FROM "+role)
		require.ErrorContains(t, nativeReadinessTx(ctx, serving, space.ID, func(tx pgx.Tx) error {
			_, e := tx.Exec(ctx, `SELECT public.space_native_membership_require($1)`, space.ID)
			return e
		}), "can bypass enforcement")
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
	// The composed native qualifier now authenticates exact44 for this real
	// nonowner session. This proves the registry gate, not all product writers.
	require.NoError(t, nativeReadinessTx(ctx, serving, space.ID, func(tx pgx.Tx) error {
		return (&SpaceStore{Pool: serving, tx: tx}).RequireScopedNativeMembershipReadiness(ctx, space.ID)
	}))
}

func TestNativeReadinessDownRequiresFreshRC(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "up")
	db.Run(true, "down", "1") // genuine empty fresh-RC driver control
	db.Run(true, "up")
	raw, e := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db/000044_native_membership_readiness.down.sql"))
	require.NoError(t, e)
	rr, e := db.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	require.NoError(t, e)
	defer rr.Rollback(ctx)
	var empty bool
	require.NoError(t, rr.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM native_membership_guard.spaces)`).Scan(&empty))
	require.True(t, empty)
	// Commit the actual first guarded writer AFTER the RR empty snapshot.
	born, owner := uuid.New(), uuid.New()
	require.NoError(t, nativeReadinessTx(ctx, db.Pool, born, func(tx pgx.Tx) error {
		scoped := &SpaceStore{Pool: db.Pool, tx: tx}
		token, err := scoped.beginNativeMembershipDBMutation(ctx, born, []uuid.UUID{owner})
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO spaces(id,name,owner_profile_id) VALUES($1,'RR concurrent first writer',$2)`, born, owner); err != nil {
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
	require.NoError(t, rr.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM native_membership_guard.spaces)`).Scan(&empty))
	require.True(t, empty, "RR still sees its genuinely old empty snapshot")
	_, e = rr.Exec(ctx, string(raw))
	require.ErrorContains(t, e, "DOWN requires READ COMMITTED")
	require.NoError(t, rr.Rollback(ctx))
	var retained bool
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM native_membership_guard.spaces WHERE space_id=$1)`, born).Scan(&retained))
	require.True(t, retained)
	db.Run(false, "down", "1") // retained positive guard epoch is roll-forward only
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM native_membership_guard.spaces WHERE space_id=$1)`, born).Scan(&retained))
	require.True(t, retained)
}

// A DB-protocol integration helper, using the real native source reader,
// publication method, transition and CAS. It does not bypass or replace the
// production native catalog: that separate composition stays unavailable.
func nativeReadinessLogicalMutation(ctx context.Context, pool *pgxpool.Pool, space uuid.UUID, prospective []uuid.UUID, action func(*SpaceStore) error, afterSeal func(pgx.Tx) error) error {
	return nativeReadinessTx(ctx, pool, space, func(tx pgx.Tx) error {
		scoped := &SpaceStore{Pool: pool, tx: tx}
		token, err := scoped.beginNativeMembershipDBMutation(ctx, space, prospective)
		if err != nil {
			return err
		}
		var started time.Time
		if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&started); err != nil {
			return err
		}
		heads := make([]effectiveHead, len(token.profiles))
		cutovers := make([]uuid.UUID, len(heads))
		sources := make([]effectiveSources, len(heads))
		for i, p := range token.profiles {
			heads[i], cutovers[i], err = loadEffectiveHead(ctx, tx, space, p)
			if err != nil {
				return err
			}
			sources[i], err = scoped.readEffectiveMemberSources(ctx, space, p, started)
			if err != nil {
				return err
			}
		}
		if action != nil {
			if err = action(scoped); err != nil {
				return err
			}
		}
		var at time.Time
		if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&at); err != nil {
			return err
		}
		for i, p := range token.profiles {
			after, err := scoped.readEffectiveMemberSources(ctx, space, p, at)
			if err != nil {
				return err
			}
			next, err := advanceEffectiveEpisode(heads[i], sources[i], after, started, at, uuid.New())
			if err != nil {
				return err
			}
			if err = saveEffectiveHead(ctx, tx, space, p, cutovers[i], heads[i].Revision, next); err != nil {
				return err
			}
		}
		if err = scoped.completeNativeMembershipDBMutation(ctx, token); err != nil {
			return err
		}
		if afterSeal != nil {
			return afterSeal(tx)
		}
		return nil
	})
}

func TestNativeReadinessOldFiniteBridgeCannotUseRenewedLease(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "41")
	st := &SpaceStore{Pool: db.Pool}
	app, environment, viewer := uuid.New(), uuid.New(), uuid.New()
	space, _, err := st.CreateCommunityBootstrapSpace(ctx, CommunityBootstrapSpaceInput{OperationID: uuid.New(), ApplicationID: app, EnvironmentID: environment, OwnerAccountID: uuid.New(), OwnerProfileID: uuid.New(), CorporationKey: "guard-lease", TemplateID: "guild-default"})
	require.NoError(t, err)
	var lease time.Time
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT clock_timestamp()+interval '120 seconds'`).Scan(&lease))
	roster := func(scoped *SpaceStore, revision int64, deadline time.Time) error {
		_, e := scoped.ApplyCommunityRoster(ctx, CommunityRosterInput{OperationID: uuid.New(), ApplicationID: app, EnvironmentID: environment, SpaceID: space.ID, CorporationKey: "guard-lease", OwnerGeneration: 1, SourceRevision: revision, SnapshotSHA256: []byte("11111111111111111111111111111111"), LeaseExpiresAt: deadline, ProfileIDs: []uuid.UUID{viewer}})
		return e
	}
	require.NoError(t, roster(st, 1, lease))
	db.Run(true, "up")
	serving := nativeReadinessLogin(t, ctx, db.Pool)
	require.NoError(t, nativeReadinessTx(ctx, db.Pool, space.ID, func(tx pgx.Tx) error {
		scoped := &SpaceStore{Pool: db.Pool, tx: tx}
		var at time.Time
		if e := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&at); e != nil {
			return e
		}
		rows, e := tx.Query(ctx, `SELECT native_membership_guard.cohort($1)`, space.ID)
		if e != nil {
			return e
		}
		var profiles []uuid.UUID
		for rows.Next() {
			var p uuid.UUID
			if e = rows.Scan(&p); e != nil {
				rows.Close()
				return e
			}
			profiles = append(profiles, p)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}
		for _, p := range profiles {
			if _, e = scoped.readEffectiveMemberSources(ctx, space.ID, p, at); e != nil {
				return e
			}
		}
		_, e = tx.Exec(ctx, `SELECT public.space_native_membership_initialize($1)`, space.ID)
		return e
	}))
	var short time.Time
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT clock_timestamp()+interval '2 seconds'`).Scan(&short))
	require.NoError(t, nativeReadinessLogicalMutation(ctx, serving, space.ID, []uuid.UUID{viewer}, func(scoped *SpaceStore) error { return roster(scoped, 2, short) }, nil))
	// Both owner and member overwrite are genuine production SQL. Complete while
	// OLD coverage is still valid, then wait less than the five-second budget but
	// longer than the OLD horizon; the new long lease must not mask the expiry.
	sealed := false
	err = nativeReadinessLogicalMutation(ctx, serving, space.ID, []uuid.UUID{viewer}, func(scoped *SpaceStore) error {
		var longer time.Time
		if e := scoped.tx.QueryRow(ctx, `SELECT clock_timestamp()+interval '120 seconds'`).Scan(&longer); e != nil {
			return e
		}
		return roster(scoped, 3, longer)
	}, func(tx pgx.Tx) error { sealed = true; _, e := tx.Exec(ctx, `SELECT pg_sleep(2.1)`); return e })
	require.True(t, sealed, "test must actually seal the renewed long lease before the late wait")
	require.ErrorContains(t, err, "OLD finite bridge expired")
	var revision int64
	var saved time.Time
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT source_revision,lease_expires_at FROM community_roster_members WHERE space_id=$1 AND profile_id=$2`, space.ID, viewer).Scan(&revision, &saved))
	require.Equal(t, int64(2), revision)
	require.True(t, short.Equal(saved), "failed renewal/source/ledger must roll back atomically")
}

func TestNativeReadinessInstallationDrainsOldWriters(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "34")
	space, profile := uuid.New(), uuid.New()
	_, err := db.Pool.Exec(ctx, `INSERT INTO spaces(id,name,owner_profile_id) VALUES($1,'installation barrier',$2)`, space, profile)
	require.NoError(t, err)
	_, err = db.Pool.Exec(ctx, `INSERT INTO space_members(space_id,profile_id) VALUES($1,$2)`, space, profile)
	require.NoError(t, err)
	db.Run(true, "goto", "43")
	serving := nativeReadinessLogin(t, ctx, db.Pool)
	late, err := serving.Begin(ctx)
	require.NoError(t, err)
	defer late.Rollback(ctx)
	var now time.Time
	require.NoError(t, late.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now)) // begun before installation, no source lock yet
	old, err := serving.Begin(ctx)
	require.NoError(t, err)
	defer old.Rollback(ctx)
	var oldPID int32
	require.NoError(t, old.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&oldPID))
	_, err = old.Exec(ctx, `DELETE FROM space_members WHERE space_id=$1 AND profile_id=$2`, space, profile)
	require.NoError(t, err)
	done := make(chan struct{})
	go func() { defer close(done); db.Run(true, "up") }()
	require.Eventually(t, func() bool {
		var waiting bool
		return db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND $1=ANY(pg_blocking_pids(pid)))`, oldPID).Scan(&waiting) == nil && waiting
	}, 5*time.Second, 10*time.Millisecond, "installer must really wait for the old writer")
	var published bool
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_namespace WHERE nspname='native_membership_guard')`).Scan(&published))
	require.False(t, published, "epoch cannot precede old writer drain")
	require.NoError(t, old.Commit(ctx))
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	_, err = late.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, spaceMutationAdvisoryKey(space))
	require.NoError(t, err)
	_, err = late.Exec(ctx, `UPDATE spaces SET owner_profile_id=$2 WHERE id=$1`, space, uuid.New())
	require.ErrorContains(t, err, "requires DB capture", "old RC session must encounter newly installed guard")
	require.NoError(t, late.Rollback(ctx))
}

func TestNativeReadinessAdministrativeNonServingCutover(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "34")
	st := &SpaceStore{Pool: db.Pool}
	_, _, _, frozenActor, frozen, _, _, _ := reserveLifecycleOperationFixture(t, st)
	binding := seedOwnershipJournalBinding(t, st)
	unknownBinding := seedOwnershipJournalBinding(t, st)
	_, err := st.ReserveOwnership(ctx, unknownBinding)
	require.NoError(t, err)
	empty := uuid.New()
	_, err = db.Pool.Exec(ctx, `INSERT INTO spaces(id,name,owner_profile_id) VALUES($1,'genuinely empty maintenance',$2)`, empty, uuid.New())
	require.NoError(t, err)
	db.Run(true, "goto", "41")
	// Native39 deliberately refuses to invent an ownership head for an already
	// pending legacy journal. The positive PREPARED case must reserve after its
	// genuine native ownership head exists; retain the old case as UNKNOWN deny.
	_, err = st.ReserveOwnership(ctx, binding)
	require.NoError(t, err)
	db.Run(true, "up")
	for _, tc := range []struct {
		name   string
		space  uuid.UUID
		active int
		denial error
	}{
		{"actual lifecycle FROZEN", frozen, 1, ErrLifecycleFrozen},
		{"actual ownership PREPARED", binding.SpaceID, 2, ErrOwnershipFrozen},
		{"genuine empty cohort", empty, 0, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Exercise the real owner-local source+UNKNOWN initializer stage. The public
			// wrapper's full native44 catalog remains separately producer-qualified; it
			// is neither mocked nor overridden in this scoped integration proof.
			require.NoError(t, nativeReadinessTx(ctx, db.Pool, tc.space, func(tx pgx.Tx) error {
				lifecycle, e := readMemberLifecycle(ctx, tx, tc.space)
				require.NoError(t, e, "actual lifecycle evidence before cutover")
				t.Logf("native lifecycle=%v", lifecycle.GetState())
				ownership, e := readMemberOwnership(ctx, tx, tc.space)
				require.NoError(t, e, "actual ownership evidence before cutover")
				t.Logf("native ownership=%v", ownership.GetState())
				return (&SpaceStore{Pool: db.Pool, tx: tx}).initializeNativeMembershipCutover(ctx, tc.space)
			}))
			var count int
			require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM space_effective_member_heads WHERE space_id=$1 AND state='ACTIVE' AND boundary_state='UNKNOWN' AND history_visible_since IS NULL`, tc.space).Scan(&count))
			require.Equal(t, tc.active, count, "serving unavailability must not collapse real active sources into NONE")
			if tc.denial != nil {
				require.ErrorIs(t, st.CheckOwnershipAvailable(ctx, tc.space), tc.denial, "successful UNKNOWN initialization cannot grant interactive eligibility")
			}
			if tc.space == empty {
				require.NoError(t, nativeReadinessTx(ctx, db.Pool, empty, func(tx pgx.Tx) error {
					scoped := &SpaceStore{Pool: db.Pool, tx: tx}
					token, e := scoped.beginNativeMembershipDBMutation(ctx, empty, nil)
					if e != nil {
						return e
					}
					require.Empty(t, token.profiles, "no dummy profile for maintenance")
					return scoped.completeNativeMembershipDBMutation(ctx, token)
				}))
			}
		})
	}
	t.Run("pre-native pending ownership remains unproved", func(t *testing.T) {
		require.ErrorIs(t, nativeReadinessTx(ctx, db.Pool, unknownBinding.SpaceID, func(tx pgx.Tx) error {
			return (&SpaceStore{Pool: db.Pool, tx: tx}).initializeNativeMembershipCutover(ctx, unknownBinding.SpaceID)
		}), ErrEffectiveEpisodeUnavailable)
		var registered bool
		require.NoError(t, db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM native_membership_guard.spaces WHERE space_id=$1)`, unknownBinding.SpaceID).Scan(&registered))
		require.False(t, registered)
	})
	var original uuid.UUID
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT profile_id FROM space_ordinary_member_heads WHERE space_id=$1`, frozen).Scan(&original))
	require.Equal(t, frozenActor, original)
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
