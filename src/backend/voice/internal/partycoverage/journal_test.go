package partycoverage

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCutoverJournalMonotoneExactRetryAndLostAck(t *testing.T) {
	ctx, c, _ := coverageDB(t)
	j := journal{c}
	plan := cutPlan{OperationID: uuid.New(), Binding: fixtureBinding(), Scope: []byte(`{"fixture":"storage-only-not-retirement"}`)}
	s, err := j.prepare(ctx, plan)
	require.NoError(t, err)
	require.Equal(t, "PREPARING", s.Phase)
	require.Equal(t, uint64(1), s.Revision)
	again, err := j.prepare(ctx, plan)
	require.NoError(t, err)
	require.Equal(t, s, again)
	changed := plan
	changed.Scope = []byte(`{"different":"scope"}`)
	_, err = j.prepare(ctx, changed)
	require.ErrorIs(t, err, ErrConflict)
	_, err = j.advance(ctx, s, "DRAINED", []byte("skip-fence"))
	require.Error(t, err)
	frozen, err := j.advance(ctx, s, "FROZEN", []byte("fixture-fence-proof"))
	require.NoError(t, err)
	retried, err := j.advance(ctx, s, "FROZEN", []byte("fixture-fence-proof"))
	require.NoError(t, err)
	require.Equal(t, frozen, retried)
	_, err = j.advance(ctx, s, "FROZEN", []byte("different-proof"))
	require.ErrorIs(t, err, ErrConflict)
	drained, err := j.advance(ctx, frozen, "DRAINED", []byte("fixture-drain-proof"))
	require.NoError(t, err)
	_, err = j.activate(ctx, drained)
	require.Error(t, err, "native validation cannot be omitted")
	validated, err := j.advance(ctx, drained, "VALIDATED", []byte("fixture-native-proof"))
	require.NoError(t, err)
	active, err := j.activate(ctx, validated)
	require.NoError(t, err)
	require.Equal(t, "ACTIVE", active.Phase)
	require.NotEqual(t, uuid.Nil, active.CoverageID)
	replay, err := j.activate(ctx, validated)
	require.NoError(t, err)
	require.Equal(t, active, replay, "lost commit acknowledgement must reuse receipt identity")
	var receipts int
	require.NoError(t, c.QueryRow(ctx, `SELECT count(*) FROM voice_party_coverage_receipts`).Scan(&receipts))
	require.Equal(t, 1, receipts)
	_, err = j.advance(ctx, active, "FROZEN", []byte("regression"))
	require.Error(t, err)
	_, err = j.prepare(ctx, cutPlan{OperationID: uuid.New(), Binding: plan.Binding, Scope: plan.Scope})
	require.ErrorIs(t, err, ErrConflict)
	suspended, err := j.advance(ctx, active, "SUSPENDED", []byte("fixture-settled-cohort-proof"))
	require.NoError(t, err)
	require.Greater(t, suspended.Revision, active.Revision)
	next := plan
	next.OperationID = uuid.New()
	next.Binding.CoverageEpoch++
	started, err := j.prepare(ctx, next)
	require.NoError(t, err)
	require.Greater(t, started.Revision, suspended.Revision)
	_, err = j.activate(ctx, validated)
	require.ErrorIs(t, err, ErrConflict, "previous operation may not revive old certificate")
}
