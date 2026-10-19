package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	commonv1 "voice.app/voice/common/v1"
	"voice/backend/pkg/integrationtest"
)

// Product storage composition: canonical34/eleven participant domain interfaces,
// real native39 predicate, then41. Existing domain receipt fixtures represent
// accepted participant evidence; this does not claim eleven hosted transports.
func TestEffectiveEpisodeP3UsesNativeTerminalProvider(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	paths, err := filepath.Glob(filepath.Join(repoRoot(t), "src/backend/migrations/space_db/*.up.sql"))
	require.NoError(t, err)
	for _, path := range paths {
		name := filepath.Base(path)
		if name >= "000039_" {
			continue
		}
		raw, e := os.ReadFile(path)
		require.NoError(t, e)
		_, e = pool.Exec(ctx, string(raw))
		require.NoError(t, e, name)
	}
	st := &SpaceStore{Pool: pool}
	durable, _, _, actor, space, request, proof, hash := reserveLifecycleOperationFixture(t, st)
	operation := uuid.MustParse(request.GetOperationId())
	// The Space and ordinary membership existed before native cutover. Native39
	// itself backfills the exact legacy publication; no capability is minted.
	for _, name := range []string{"000039_native_membership_authority.up.sql", "000041_effective_member_episodes.up.sql"} {
		raw, e := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db", name))
		require.NoError(t, e)
		_, e = pool.Exec(ctx, string(raw))
		require.NoError(t, e, name)
	}
	var publication uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT h.publication_id FROM space_ordinary_member_heads h JOIN space_members m USING(space_id,profile_id)
 WHERE h.space_id=$1 AND h.profile_id=$2 AND h.state='ACTIVE' AND h.legacy AND h.generation=0 AND h.publication_kind=2
 AND m.membership_generation=h.generation AND m.admission_operation_id=h.publication_id`, space, actor).Scan(&publication))
	var sampled time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&sampled))
	head, err := cutoverEffectiveEpisode(effectiveSources{Ordinary: fmt.Sprintf("ordinary:0:%s:legacy", publication), Legacy: true}, sampled, uuid.New())
	require.NoError(t, err)
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, saveEffectiveHead(ctx, tx, space, actor, uuid.New(), 0, head))
	require.NoError(t, tx.Commit(ctx))
	_, err = pool.Exec(ctx, `DELETE FROM space_effective_member_heads WHERE space_id=$1`, space)
	require.Error(t, err, "live profile head is not disposable")
	_, err = durable.RecordLifecycleDeletionProofReceipt(ctx, actor, operation, proof, hash)
	require.NoError(t, err)
	aggregate, err := st.BeginLifecycleFreeze(ctx, space, lifecycleManifestFixture())
	require.NoError(t, err)
	recordLifecycleFenceBarrier(t, aggregate, space, operation, 1, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN)
	require.NoError(t, st.PersistLifecycle(ctx, aggregate))
	_, _, err = st.CompleteLifecycleSchedule(ctx, space)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE space_lifecycle_aggregates SET scheduled_at=scheduled_at-interval '8 days',purge_after=purge_after-interval '8 days' WHERE space_id=$1`, space)
	require.NoError(t, err)
	aggregate, err = st.DecideLifecycleRecovery(ctx, space, 2)
	require.NoError(t, err)
	recordLifecycleFenceBarrier(t, aggregate, space, operation, 2, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED)
	require.NoError(t, aggregate.BeginPurging())
	require.NoError(t, st.PersistLifecycle(ctx, aggregate))
	snapshot := aggregate.Snapshot()
	require.NoError(t, aggregate.RecordRoleRetirementReceipt(lifecycleRoleRetirementReceipt(t, space, operation, 2, snapshot.PurgeDecidedAt)))
	for _, id := range lifecycleTestParticipants[1 : len(lifecycleTestParticipants)-1] {
		require.NoError(t, aggregate.RecordPurgeReceipt(lifecyclePurgeReceipt(t, space, operation, id, 2, snapshot.PurgeDecidedAt)))
	}
	require.NoError(t, st.PersistLifecycle(ctx, aggregate))
	_, err = st.FinalizeLifecyclePurge(ctx, space, testLifecycleHasher{})
	require.Error(t, err, "ten participants cannot delete the ledger")
	var retained bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_effective_member_heads WHERE space_id=$1)`, space).Scan(&retained))
	require.True(t, retained)
	last := lifecycleTestParticipants[len(lifecycleTestParticipants)-1]
	require.Equal(t, commonv1.ParticipantId_PARTICIPANT_ID_REALTIME, last)
	require.NoError(t, aggregate.RecordPurgeReceipt(lifecyclePurgeReceipt(t, space, operation, last, 2, snapshot.PurgeDecidedAt)))
	require.NoError(t, st.PersistLifecycle(ctx, aggregate))
	_, err = st.FinalizeLifecyclePurge(ctx, space, testLifecycleHasher{})
	require.NoError(t, err)
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_permanent_purge_fences WHERE space_id=$1)
 AND NOT EXISTS(SELECT 1 FROM spaces WHERE id=$1)
 AND NOT EXISTS(SELECT 1 FROM space_effective_member_heads WHERE space_id=$1)`, space).Scan(&retained))
	require.True(t, retained, "native P3 must minimize effective heads behind permanent Space retirement")
	// The irreversible Space fence must deny a delayed insert even if its tuple
	// resembles an old or newly generated effective membership identity.
	tx, err = pool.Begin(ctx)
	require.NoError(t, err)
	err = saveEffectiveHead(ctx, tx, space, actor, uuid.New(), 0, head)
	_ = tx.Rollback(ctx)
	require.Error(t, err, "late head replay must not reidentify a purged Space")
	var definition string
	require.NoError(t, pool.QueryRow(ctx, `SELECT pg_get_functiondef('space_terminal_purge_ready(uuid)'::regprocedure)`).Scan(&definition))
	require.True(t, strings.Contains(definition, "= 11") || strings.Contains(definition, "=11"), "canonical participant eleven predicate must remain installed")
}

func TestEffectiveEpisode41PinnedDriverStorageCohort(t *testing.T) {
	ctx := context.Background()
	directory := filepath.Join(repoRoot(t), "src/backend/migrations/space_db")
	fixture := integrationtest.NewSourceMigrationFixture(t, ctx, directory)
	fixture.Run(true, "up")
	var version uint
	var dirty bool
	require.NoError(t, fixture.Pool.QueryRow(ctx, `SELECT version,dirty FROM schema_migrations`).Scan(&version, &dirty))
	require.Equal(t, uint(41), version)
	require.False(t, dirty)
	files, err := filepath.Glob(filepath.Join(directory, "*.up.sql"))
	require.NoError(t, err)
	previous := uint(0)
	for _, path := range files {
		n, e := strconv.Atoi(filepath.Base(path)[:6])
		require.NoError(t, e)
		if n < 41 && uint(n) > previous {
			previous = uint(n)
		}
	}
	fixture.Run(true, "down", "1")
	require.NoError(t, fixture.Pool.QueryRow(ctx, `SELECT version,dirty FROM schema_migrations`).Scan(&version, &dirty))
	require.Equal(t, previous, version)
	require.False(t, dirty)
	fixture.Run(true, "up")
	fixture.Run(true, "up") // actual pinned driver no-change replay, not a raw SQL loop
	space, profile, cutover := uuid.New(), uuid.New(), uuid.New()
	_, err = fixture.Pool.Exec(ctx, `INSERT INTO spaces(id,name,owner_profile_id) VALUES($1,'retained driver oracle',$2)`, space, profile)
	require.NoError(t, err)
	_, err = fixture.Pool.Exec(ctx, `INSERT INTO space_effective_member_heads(space_id,profile_id,generation,retired_through,ledger_revision,state,boundary_state,observed_at,cutover_id)
 VALUES($1,$2,0,0,1,'NONE','UNKNOWN',clock_timestamp(),$3)`, space, profile, cutover)
	require.NoError(t, err)
	fixture.Run(false, "down", "1")
	var retained bool
	require.NoError(t, fixture.Pool.QueryRow(ctx, `SELECT version,dirty,EXISTS(SELECT 1 FROM space_effective_member_heads WHERE space_id=$1 AND profile_id=$2) FROM schema_migrations`, space, profile).Scan(&version, &dirty, &retained))
	// v4.18.1 records the attempted DOWN target dirty before running its body.
	// The previous version is therefore dirty even though41 data was preserved.
	require.Equal(t, previous, version)
	require.True(t, dirty, "driver must retain its failed migration fence")
	require.True(t, retained)
	// No force clears this expected private dirty state. The owned fixture is
	// destroyed; production recovery is not inferred from the negative oracle.
}

func TestEffectiveEpisode41RejectsMissingNativeAndHistoricalTenProvider(t *testing.T) {
	for _, tc := range []struct {
		name           string
		native, eleven bool
	}{
		{"missing native39", false, true},
		{"real historical ten participant provider", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			pool := startSpacePostgresForStoreTest(t, ctx)
			paths, err := filepath.Glob(filepath.Join(repoRoot(t), "src/backend/migrations/space_db/*.up.sql"))
			require.NoError(t, err)
			for _, path := range paths {
				name := filepath.Base(path)
				if name >= "000041_" || (!tc.native && strings.HasPrefix(name, "000039_")) || (!tc.eleven && strings.HasPrefix(name, "000034_")) {
					continue
				}
				raw, e := os.ReadFile(path)
				require.NoError(t, e)
				_, e = pool.Exec(ctx, string(raw))
				require.NoError(t, e, name)
			}
			space, profile := uuid.New(), uuid.New()
			_, err = pool.Exec(ctx, `INSERT INTO spaces(id,name,owner_profile_id) VALUES($1,'qualifier preservation',$2)`, space, profile)
			require.NoError(t, err)
			raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db/000041_effective_member_episodes.up.sql"))
			require.NoError(t, err)
			conn, err := pool.Acquire(ctx)
			require.NoError(t, err)
			defer conn.Release()
			_, err = conn.Exec(ctx, string(raw))
			require.Error(t, err)
			var refusal *pgconn.PgError
			require.ErrorAs(t, err, &refusal)
			require.Equal(t, "55000", refusal.Code)
			_, err = conn.Exec(ctx, `ROLLBACK`)
			require.NoError(t, err)
			var preserved bool
			require.NoError(t, conn.QueryRow(ctx, `SELECT to_regclass('public.space_effective_member_heads') IS NULL AND EXISTS(SELECT 1 FROM spaces WHERE id=$1)`, space).Scan(&preserved))
			require.True(t, preserved, "unqualified source rejection must leave current Space and schema intact")
		})
	}
}
