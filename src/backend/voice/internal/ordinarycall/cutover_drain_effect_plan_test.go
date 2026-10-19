package ordinarycall

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestOrdinaryEffectPlanRetainsExactRevisionsWithoutCompletingEffects(t *testing.T) {
	pool := ordinaryExecutionPostgres(t)
	ctx := context.Background()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	dir := filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations", "voice_db")
	apply := func(name string) {
		raw, e := os.ReadFile(filepath.Join(dir, name))
		require.NoError(t, e)
		_, e = pool.Exec(ctx, string(raw))
		require.NoError(t, e)
	}
	apply("000025_ordinary_cutover_drain.up.sql")
	apply("000027_ordinary_cutover_effect_plans.up.sql")
	apply("000027_ordinary_cutover_effect_plans.down.sql")
	apply("000027_ordinary_cutover_effect_plans.up.sql")
	store := NewPostgresStore(pool, storageTestEncoder)
	media, projection := &mediaFake{}, &projectionFake{}
	coordinator := &Coordinator{Journal: store, Media: media, Projection: projection, Now: func() time.Time { return testNow }}
	pending, first, _ := pendingGroup(t)
	active, err := coordinator.Start(ctx, pending.Intent, nil)
	require.NoError(t, err)
	survivor := identity()
	join := uuid.New()
	active, err = coordinator.Admit(ctx, active.Intent.RoomID, join, func(r *Room) error { return r.BeginJoin(join, survivor, r.TransactionTime) }, nil)
	require.NoError(t, err)
	active, err = store.Mutate(ctx, active.Intent.RoomID, func(r *Room) error { return r.DecideLeave(first.ProfileID, r.TransactionTime) })
	require.NoError(t, err)
	pendingIdentity, pendingJoin := identity(), uuid.New()
	active, err = store.Mutate(ctx, active.Intent.RoomID, func(r *Room) error { return r.BeginJoin(pendingJoin, pendingIdentity, r.TransactionTime) })
	require.NoError(t, err)
	active, err = store.Mutate(ctx, active.Intent.RoomID, func(r *Room) error { return r.AbortAdmission(pendingJoin, r.TransactionTime) })
	require.NoError(t, err)
	owner := &CutoverDrainOwner{Store: store, Authority: cutoverAuthorityFixture{}}
	work, err := owner.PrepareCutoverDrain(ctx, ordinaryCutFixture(), CutoverExpectedRoom{WorkID: uuid.New(), RoomID: active.Intent.RoomID, Incarnation: active.Intent.OperationID, Version: active.Version})
	require.NoError(t, err)
	var original []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT document FROM voice_ordinary_rooms WHERE room_id=$1`, active.Intent.RoomID).Scan(&original))
	closed, err := store.Load(ctx, active.Intent.RoomID)
	require.NoError(t, err)
	ref, err := owner.PrepareCutoverEffectPlan(ctx, work)
	require.NoError(t, err, "exact effect/revision plan must survive retry without completing the native effects")
	require.Equal(t, work, ref.Work)
	require.Equal(t, uint64(len(closed.Effects)), ref.Count)
	require.NotZero(t, ref.Hash)
	plan, err := owner.ReadCutoverEffectPlan(ctx, ref)
	require.NoError(t, err)
	require.Equal(t, ref, plan.Ref)
	byID := map[uuid.UUID]Effect{}
	for _, effect := range closed.Effects {
		byID[effect.ID] = effect
	}
	zeroRevision := 0
	localFence := false
	for _, item := range plan.Effects {
		expected, exists := byID[item.Effect.ID]
		require.True(t, exists)
		require.Equal(t, expected, item.Effect)
		raw, e := json.Marshal(expected)
		require.NoError(t, e)
		require.Equal(t, raw, item.OriginalBytes)
		require.NotZero(t, item.OriginalHash)
		require.NotZero(t, item.TargetHash)
		switch expected.Kind {
		case EffectEnsure:
			require.Equal(t, CutoverPreserveOrCancelEnsure, item.StepKind)
		case EffectClose, EffectRemoveMember:
			require.Equal(t, CutoverQualifyMediaRemoval, item.StepKind)
		case EffectReleaseFence:
			require.Equal(t, CutoverVerifyNativeFence, item.StepKind)
			localFence = true
		}
		if item.Effect.Revision == 0 {
			zeroRevision++
		}
	}
	require.Positive(t, zeroRevision, "zero is an exact retained revision, never a wildcard")
	require.True(t, localFence, "aborted pending admission contributes a retained local fence obligation, never media retirement")
	require.Len(t, plan.Effects, len(closed.Effects), "Applied effects and pending local fence obligations also remain in the plan")
	restart := &CutoverDrainOwner{Store: NewPostgresStore(pool, storageTestEncoder), Authority: cutoverAuthorityFixture{}}
	replay, err := restart.PrepareCutoverEffectPlan(ctx, work)
	require.NoError(t, err)
	require.Equal(t, ref, replay)
	replayed, err := restart.ReadCutoverEffectPlan(ctx, ref)
	require.NoError(t, err)
	require.Equal(t, plan, replayed)
	wrong := ref
	wrong.Hash[0] ^= 1
	_, err = owner.ReadCutoverEffectPlan(ctx, wrong)
	require.ErrorIs(t, err, ErrConflict)
	wrong = ref
	wrong.Work.RequestHash[0] ^= 1
	_, err = owner.ReadCutoverEffectPlan(ctx, wrong)
	require.ErrorIs(t, err, ErrConflict)
	_, err = (&CutoverDrainOwner{Store: store}).PrepareCutoverEffectPlan(ctx, work)
	require.ErrorIs(t, err, ErrUnavailable)
	var after []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT document FROM voice_ordinary_rooms WHERE room_id=$1`, active.Intent.RoomID).Scan(&after))
	require.Equal(t, original, after)
	var state string
	var receipt []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT state,receipt_bytes FROM voice_ordinary_cutover_works WHERE work_id=$1`, work.WorkID).Scan(&state, &receipt))
	require.Equal(t, "PREPARED", state)
	require.Nil(t, receipt)
	_, err = pool.Exec(ctx, `UPDATE voice_ordinary_cutover_effect_plan_items SET effect_revision=effect_revision+1 WHERE work_id=$1`, work.WorkID)
	require.Error(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM voice_ordinary_cutover_effect_plans WHERE work_id=$1`, work.WorkID)
	require.Error(t, err)
	down, e := os.ReadFile(filepath.Join(dir, "000027_ordinary_cutover_effect_plans.down.sql"))
	require.NoError(t, e)
	conn, e := pool.Acquire(ctx)
	require.NoError(t, e)
	_, downErr := conn.Exec(ctx, string(down))
	_, rollbackErr := conn.Exec(ctx, "ROLLBACK")
	conn.Release()
	require.ErrorContains(t, downErr, "require reconciliation before rollback")
	require.NoError(t, rollbackErr)
	retained, err := owner.ReadCutoverEffectPlan(ctx, ref)
	require.NoError(t, err)
	require.Equal(t, plan, retained)

	// A later compensation revision must not reuse or upgrade the stored plan.
	changed, err := decodeRoom(original, active.Intent.RoomID)
	require.NoError(t, err)
	changed.requireClose(testNow)
	changed.Version++
	raw, e := json.Marshal(changed)
	require.NoError(t, e)
	_, err = pool.Exec(ctx, `UPDATE voice_ordinary_rooms SET document=$2,version=$3 WHERE room_id=$1`, active.Intent.RoomID, raw, changed.Version)
	require.NoError(t, err)
	_, err = owner.PrepareCutoverEffectPlan(ctx, work)
	require.ErrorIs(t, err, ErrConflict)
	_, err = owner.ReadCutoverEffectPlan(ctx, ref)
	require.ErrorIs(t, err, ErrConflict)
	var persisted []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT plan_bytes FROM voice_ordinary_cutover_effect_plans WHERE work_id=$1`, work.WorkID).Scan(&persisted))
	require.Equal(t, plan.CanonicalBytes, persisted, "stale-plan rejection preserves the old evidence")

	// An item-write failure must roll back the head and every earlier item.
	other, _, _ := pendingGroup(t)
	require.NoError(t, store.Create(ctx, other), "second room has never reached the SDK")
	secondWork, err := owner.PrepareCutoverDrain(ctx, ordinaryCutFixture(), CutoverExpectedRoom{WorkID: uuid.New(), RoomID: other.Intent.RoomID, Incarnation: other.Intent.OperationID, Version: other.Version})
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `CREATE FUNCTION fixture_effect_plan_failure() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF EXISTS(SELECT 1 FROM voice_ordinary_cutover_effect_plan_items WHERE work_id=NEW.work_id) THEN RAISE EXCEPTION 'fixture effect plan insert failure'; END IF; RETURN NEW; END $$;
 CREATE TRIGGER fixture_effect_plan_failure BEFORE INSERT ON voice_ordinary_cutover_effect_plan_items FOR EACH ROW EXECUTE FUNCTION fixture_effect_plan_failure()`)
	require.NoError(t, err)
	_, err = owner.PrepareCutoverEffectPlan(ctx, secondWork)
	require.ErrorContains(t, err, "fixture effect plan insert failure")
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_ordinary_cutover_effect_plans WHERE work_id=$1`, secondWork.WorkID).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_ordinary_cutover_effect_plan_items WHERE work_id=$1`, secondWork.WorkID).Scan(&count))
	require.Zero(t, count)
	_, err = pool.Exec(ctx, `DROP TRIGGER fixture_effect_plan_failure ON voice_ordinary_cutover_effect_plan_items; DROP FUNCTION fixture_effect_plan_failure()`)
	require.NoError(t, err)
	secondRef, err := owner.PrepareCutoverEffectPlan(ctx, secondWork)
	require.NoError(t, err)
	secondPlan, err := owner.ReadCutoverEffectPlan(ctx, secondRef)
	require.NoError(t, err)
	ensureFound := false
	for _, item := range secondPlan.Effects {
		if item.Effect.Kind == EffectEnsure {
			ensureFound = true
			require.False(t, item.Effect.Applied)
			require.Equal(t, CutoverPreserveOrCancelEnsure, item.StepKind)
		}
	}
	require.True(t, ensureFound, "planning cancellation must never manufacture successful creation")
	stillAborted, err := store.Load(ctx, other.Intent.RoomID)
	require.NoError(t, err)
	require.Equal(t, AdmissionAborted, stillAborted.Admissions[other.Intent.OperationID.String()].State)
}
