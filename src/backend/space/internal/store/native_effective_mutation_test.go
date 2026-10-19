package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

func TestNativeEffectiveMutationCapturesActualPublishedSources(t *testing.T) {
	if testing.Short() {
		t.Skip("requires real PostgreSQL native sources")
	}
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "34")
	st := &SpaceStore{Pool: db.Pool}
	space, err := st.CreateSpace(ctx, uuid.New(), "Native effective mutation", "", "public")
	require.NoError(t, err)
	legacyMember := uuid.New()
	_, err = db.Pool.Exec(ctx, `INSERT INTO space_members(space_id,profile_id) VALUES($1,$2)`, space.ID, legacyMember)
	require.NoError(t, err)
	db.Run(true, "up")
	profile := uuid.New()
	// The profile has no source before publication. Capture may create no grant.
	require.NoError(t, st.withOwnershipScope(ctx, []uuid.UUID{space.ID}, func(scoped *SpaceStore) error {
		capture, err := scoped.captureEffectiveMemberMutations(ctx, space.ID, []uuid.UUID{profile})
		require.NoError(t, err, "an empty native source is a valid pre-mutation capture")
		require.NoError(t, scoped.finishEffectiveMemberMutations(ctx, capture))
		_, err = scoped.ReadScopedMemberSourceBindings(ctx, space.ID, profile)
		require.Error(t, err, "empty ledger bookkeeping must not publish membership")
		return nil
	}))
	var state string
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT state FROM space_effective_member_heads WHERE space_id=$1 AND profile_id=$2`, space.ID, profile).Scan(&state))
	require.Equal(t, "NONE", state)
	// An existing active legacy source cannot be silently adopted as a cutover.
	require.NoError(t, st.withOwnershipScope(ctx, []uuid.UUID{space.ID}, func(scoped *SpaceStore) error {
		_, err := scoped.captureEffectiveMemberMutations(ctx, space.ID, []uuid.UUID{space.OwnerProfileID})
		require.ErrorIs(t, err, ErrEffectiveEpisodeUnavailable)
		return nil
	}))
	// Explicit owner-controlled cutover fixture uses the actual qualified OLD
	// source, not a fabricated membership or a caller-provided eligible flag.
	require.NoError(t, st.withOwnershipScope(ctx, []uuid.UUID{space.ID}, func(scoped *SpaceStore) error {
		var sampled time.Time
		require.NoError(t, scoped.tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&sampled))
		sources, err := scoped.readEffectiveMemberSources(ctx, space.ID, legacyMember, sampled)
		require.NoError(t, err)
		head, err := cutoverEffectiveEpisode(sources, sampled, uuid.New())
		require.NoError(t, err)
		require.False(t, head.Known)
		require.NoError(t, saveEffectiveHead(ctx, scoped.tx, space.ID, legacyMember, uuid.New(), 0, head))
		capture, err := scoped.captureEffectiveMemberMutations(ctx, space.ID, []uuid.UUID{legacyMember})
		require.NoError(t, err)
		require.NoError(t, scoped.RemoveMember(ctx, space.ID, legacyMember))
		require.NoError(t, scoped.finishEffectiveMemberMutations(ctx, capture))
		require.ErrorIs(t, scoped.finishEffectiveMemberMutations(ctx, capture), ErrEffectiveEpisodeUnavailable, "one logical mutation has one finalization")
		_, err = scoped.ReadScopedMemberSourceBindings(ctx, space.ID, legacyMember)
		require.ErrorIs(t, err, pgx.ErrNoRows)
		return nil
	}))
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT state FROM space_effective_member_heads WHERE space_id=$1 AND profile_id=$2`, space.ID, legacyMember).Scan(&state))
	require.Equal(t, "RETIRED", state)
	var oldRevision, newRevision int64
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT ledger_revision FROM space_effective_member_heads WHERE space_id=$1 AND profile_id=$2`, space.ID, profile).Scan(&oldRevision))
	err = st.withOwnershipScope(ctx, []uuid.UUID{space.ID}, func(scoped *SpaceStore) error {
		capture, err := scoped.captureEffectiveMemberMutations(ctx, space.ID, []uuid.UUID{profile, legacyMember})
		if err != nil {
			return err
		}
		_, err = scoped.tx.Exec(ctx, `SELECT pg_sleep(5.05)`)
		if err != nil {
			return err
		}
		return scoped.finishEffectiveMemberMutations(ctx, capture)
	})
	require.ErrorIs(t, err, ErrEffectiveEpisodeUnavailable, "final deadline covers every SQL write and every captured profile")
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT ledger_revision FROM space_effective_member_heads WHERE space_id=$1 AND profile_id=$2`, space.ID, profile).Scan(&newRevision))
	require.Equal(t, oldRevision, newRevision, "failure rolls back all ledger updates")
}
