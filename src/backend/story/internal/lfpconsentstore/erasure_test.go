package lfpconsentstore

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/storylfpadmission"
	"voice/backend/story/internal/lfperasure"
)

func TestMissingErasureAdmissionNeverTreatsAbsenceAsNew(t *testing.T) {
	s, id, author := fixture(t)
	o := s.authority.(fixtureAdmission).o
	shared, e := storylfpadmission.NewOrigin(o.environment, o.installation, o.incarnation, o.database)
	require.NoError(t, e)
	closed := NewBoundStore(s.Pool, storylfpadmission.NewBarrier(shared, fixtureSharedAdmission{shared}))
	_, e = closed.PublishOffer(context.Background(), AuthorAction{OperationID: uuid.New(), StoryID: id, ProfileID: author, RequestSHA256: [32]byte{2}})
	require.ErrorIs(t, e, ErrUnavailable, "a valid cohort is not a complete negative replay view")
}

type selectiveNegative struct {
	operation, account uuid.UUID
	released           atomic.Int32
}

func (n *selectiveNegative) Enter(context.Context) (lfperasure.View, error) {
	return &selectiveView{n}, nil
}

type selectiveView struct{ n *selectiveNegative }

func (v *selectiveView) Check(r lfperasure.References) lfperasure.Verdict {
	for _, id := range r.Operations {
		if id == v.n.operation {
			return lfperasure.ErasedOrRetired
		}
	}
	for _, id := range r.Accounts {
		if id == v.n.account {
			return lfperasure.ErasedOrRetired
		}
	}
	return lfperasure.LiveOrUnseen
}
func (v *selectiveView) Release() { v.n.released.Add(1) }

func TestErasedOperationCannotBeRecreatedAfterLocalHistoryRemoval(t *testing.T) {
	s, id, author := fixture(t)
	ctx := context.Background()
	op := uuid.New()
	a := AuthorAction{OperationID: op, StoryID: id, ProfileID: author, RequestSHA256: [32]byte{2}}
	_, e := s.PublishOffer(ctx, a)
	require.NoError(t, e)
	_, e = s.Pool.Exec(ctx, `DELETE FROM story_lfp_publications WHERE operation_id=$1`, op)
	require.NoError(t, e)
	// Synthetic negative state outside this fixture DB. This is not the missing
	// finite-retention policy or the complete eight-table production scrub.
	n := &selectiveNegative{operation: op}
	s.negative = n
	_, e = s.PublishOffer(ctx, a)
	require.ErrorIs(t, e, ErrErased)
	var count int
	require.NoError(t, s.Pool.QueryRow(ctx, `SELECT count(*) FROM story_lfp_publications WHERE operation_id=$1`, op).Scan(&count))
	require.Zero(t, count)
	require.EqualValues(t, 1, n.released.Load())
}

func TestNegativePermitAndMutationGateSpanNativeTransaction(t *testing.T) {
	s, _, _ := fixture(t)
	ctx := context.Background()
	n := &selectiveNegative{}
	s.negative = n
	tx, _, release, e := s.begin(ctx)
	require.NoError(t, e)
	defer func() { _ = tx.Rollback(ctx); release() }()
	lock, e := s.Pool.Begin(ctx)
	require.NoError(t, e)
	defer func() { _ = lock.Rollback(ctx) }()
	var acquired bool
	require.NoError(t, lock.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock(2910007)`).Scan(&acquired))
	require.False(t, acquired)
	require.Zero(t, n.released.Load())
	require.NoError(t, tx.Commit(ctx))
	release()
	release()
	require.EqualValues(t, 1, n.released.Load())
	require.NoError(t, lock.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock(2910007)`).Scan(&acquired))
	require.True(t, acquired)
}

func TestNegativePermitRetainedAfterUnknownCommit(t *testing.T) {
	s, id, author := fixture(t)
	ctx := context.Background()
	n := &selectiveNegative{}
	s.negative = n
	_, e := s.Pool.Exec(ctx, `CREATE FUNCTION erase_commit_wait() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_advisory_xact_lock(2910008); RETURN NEW; END $$; CREATE CONSTRAINT TRIGGER erase_commit_wait AFTER INSERT ON story_lfp_publications DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION erase_commit_wait()`)
	require.NoError(t, e)
	block, e := s.Pool.Begin(ctx)
	require.NoError(t, e)
	_, e = block.Exec(ctx, `SELECT pg_advisory_xact_lock(2910008)`)
	require.NoError(t, e)
	defer func() { _ = block.Rollback(ctx) }()
	caller, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := s.PublishOffer(caller, AuthorAction{OperationID: uuid.New(), StoryID: id, ProfileID: author, RequestSHA256: [32]byte{3}})
		done <- err
	}()
	require.Eventually(t, func() bool {
		var count int
		err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE application_name=$1 AND wait_event_type='Lock' AND lower(query)='commit'`, s.Pool.Config().ConnConfig.RuntimeParams["application_name"]).Scan(&count)
		return err == nil && count == 1
	}, 3*time.Second, 10*time.Millisecond, "caller must reach actual blocked native COMMIT before cancellation")
	cancel()
	select {
	case e = <-done:
		require.ErrorIs(t, e, ErrUnavailable)
	case <-time.After(time.Second):
		t.Fatal("canceled commit did not return")
	}
	require.Zero(t, n.released.Load(), "failed COMMIT never proves terminal backend completion")
	require.NoError(t, block.Rollback(ctx))
}
