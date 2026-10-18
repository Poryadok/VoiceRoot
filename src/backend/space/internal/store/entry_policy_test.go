package store

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/space/internal/entrypolicy"
)

func applyEntryPolicyMigration(t *testing.T, st *SpaceStore) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db/000030_entry_policy.up.sql"))
	require.NoError(t, err)
	_, err = st.Pool.Exec(context.Background(), string(raw))
	require.NoError(t, err)
}

func TestEntryPolicyMigrationAndLegacyJoinCannotBypass(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Entry", "", "public")
	require.NoError(t, err)
	requirement := "phone"
	_, err = st.UpdateSpace(ctx, space.ID, UpdateSpaceInput{EntryRequirement: &requirement})
	require.NoError(t, err)
	invite, err := st.CreateInvite(ctx, CreateInviteInput{SpaceID: space.ID, CreatorProfileID: owner})
	require.NoError(t, err)
	applyEntryPolicyMigration(t, st)
	policy, err := st.GetEntryPolicy(ctx, space.ID)
	require.NoError(t, err)
	require.True(t, policy.Policy.RequirePhone)
	require.EqualValues(t, 1, policy.Version)
	_, err = st.JoinByInvite(ctx, invite.Code, uuid.New(), uuid.New())
	require.ErrorIs(t, err, ErrAdmissionRequired)
	_, err = st.JoinSpace(ctx, space.ID, uuid.New(), uuid.New())
	require.ErrorIs(t, err, ErrAdmissionRequired)
	got, err := st.GetInviteByCode(ctx, invite.Code)
	require.NoError(t, err)
	require.Zero(t, got.UseCount)
	none := "none"
	_, err = st.UpdateSpace(ctx, space.ID, UpdateSpaceInput{EntryRequirement: &none})
	require.ErrorIs(t, err, ErrLegacyEntryPolicyWrite)
	desc := "unrelated update"
	_, err = st.UpdateSpace(ctx, space.ID, UpdateSpaceInput{Description: &desc})
	require.NoError(t, err)
}

func TestEntryPolicyCASReplayAndABA(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	actor := uuid.New()
	space, err := st.CreateSpace(ctx, actor, "Policy", "", "private")
	require.NoError(t, err)
	applyEntryPolicyMigration(t, st)
	policy := entrypolicy.Policy{SchemaVersion: 1, Questions: []entrypolicy.Question{}}
	set := SetEntryPolicyInput{SpaceID: space.ID, ActorProfileID: actor, OperationID: uuid.New(), ExpectedVersion: 1, Policy: policy}
	first, err := st.SetEntryPolicy(ctx, set)
	require.NoError(t, err)
	require.EqualValues(t, 1, first.Version, "semantic no-op must preserve revision")
	set.OperationID = uuid.New()
	set.Policy.RequireManualApproval = true
	second, err := st.SetEntryPolicy(ctx, set)
	require.NoError(t, err)
	require.EqualValues(t, 2, second.Version)
	replayed, err := st.SetEntryPolicy(ctx, set)
	require.NoError(t, err)
	require.Equal(t, second, replayed)
	set.Policy.RequireManualApproval = false
	_, err = st.SetEntryPolicy(ctx, set)
	require.ErrorIs(t, err, ErrEntryPolicyConflict)
	set.OperationID = uuid.New()
	_, err = st.SetEntryPolicy(ctx, set)
	require.ErrorIs(t, err, ErrEntryPolicyVersion)
	set.ExpectedVersion = 2
	third, err := st.SetEntryPolicy(ctx, set)
	require.NoError(t, err)
	require.EqualValues(t, 3, third.Version, "A-B-A cannot resurrect version1")
}

func TestEntryPolicyConcurrentWritersUseDatabaseFence(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	first := &SpaceStore{Pool: pool}
	actor := uuid.New()
	space, err := first.CreateSpace(ctx, actor, "Concurrent policy", "", "public")
	require.NoError(t, err)
	applyEntryPolicyMigration(t, first)
	secondPool, err := pgxpool.New(ctx, pool.Config().ConnString())
	require.NoError(t, err)
	t.Cleanup(secondPool.Close)
	second := &SpaceStore{Pool: secondPool}
	start := make(chan struct{})
	results := make(chan error, 2)
	var ready sync.WaitGroup
	ready.Add(2)
	for _, st := range []*SpaceStore{first, second} {
		go func(st *SpaceStore) {
			ready.Done()
			<-start
			_, err := st.SetEntryPolicy(ctx, SetEntryPolicyInput{SpaceID: space.ID, ActorProfileID: actor, OperationID: uuid.New(), ExpectedVersion: 1, Policy: entrypolicy.Policy{SchemaVersion: 1, RequireManualApproval: true, Questions: []entrypolicy.Question{}}})
			results <- err
		}(st)
	}
	ready.Wait()
	close(start)
	var success, stale int
	for range 2 {
		err := <-results
		if err == nil {
			success++
		} else {
			require.ErrorIs(t, err, ErrEntryPolicyVersion)
			stale++
		}
	}
	require.Equal(t, 1, success)
	require.Equal(t, 1, stale)
	row, err := first.GetEntryPolicy(ctx, space.ID)
	require.NoError(t, err)
	require.EqualValues(t, 2, row.Version)
}
