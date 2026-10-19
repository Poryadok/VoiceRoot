package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
)

func TestNative41ExpiryOnlyMutatesCapturedProfiles(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "41")
	st := &SpaceStore{Pool: db.Pool}
	create, ownerAccount, owner := signedNativeCreatorCurrent(t, st, &spacev1.CreateSpaceRequest{Name: "Expiry snapshot41", Visibility: "public"})
	space, err := create()
	require.NoError(t, err)
	app, environment := uuid.New(), uuid.New()
	_, err = db.Pool.Exec(ctx, `INSERT INTO public.community_owner_authority(space_id,application_id,environment_id,corporation_key,owner_account_id,owner_profile_id,owner_generation,status)
	 VALUES($1,$2,$3,'snapshot41',$4,$5,1,'active')`, space.ID, app, environment, ownerAccount, owner)
	require.NoError(t, err)
	a := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	b := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	_, err = st.ApplyCommunityRoster(ctx, CommunityRosterInput{OperationID: uuid.New(), ApplicationID: app, EnvironmentID: environment, CorporationKey: "snapshot41", SpaceID: space.ID, OwnerGeneration: 1, SourceRevision: 1, SnapshotSHA256: []byte("11111111111111111111111111111111"), LeaseExpiresAt: time.Now().Add(time.Minute), ProfileIDs: []uuid.UUID{a, b}})
	require.NoError(t, err)
	var first, second time.Time
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT clock_timestamp()+interval '2 seconds',clock_timestamp()+interval '4 seconds'`).Scan(&first, &second))
	// Real source mutation with the actual native capture/finalizer gives each
	// row its own finite support. No fake ledger/provider or unhooked renewal.
	require.NoError(t, st.withOwnershipScope(ctx, []uuid.UUID{space.ID}, func(scoped *SpaceStore) error {
		capture, err := scoped.captureNativeEffectiveMemberMutation(ctx, space.ID, []uuid.UUID{a, b})
		if err != nil {
			return err
		}
		_, err = scoped.db().Exec(ctx, `UPDATE public.community_roster_members SET lease_expires_at=CASE WHEN profile_id=$2 THEN $4::timestamptz ELSE $5::timestamptz END WHERE space_id=$1 AND profile_id=ANY($3::uuid[])`, space.ID, a, []uuid.UUID{a, b}, first, second)
		if err != nil {
			return err
		}
		return scoped.finishEffectiveMemberMutations(ctx, capture)
	}))
	blocker, err := db.Pool.Begin(ctx)
	require.NoError(t, err)
	defer blocker.Rollback(ctx)
	_, err = blocker.Exec(ctx, `SELECT 1 FROM public.space_effective_member_heads WHERE space_id=$1 AND profile_id=$2 FOR UPDATE`, space.ID, a)
	require.NoError(t, err)
	_, err = db.Pool.Exec(ctx, `SELECT pg_sleep(GREATEST(0,EXTRACT(epoch FROM ($1::timestamptz-clock_timestamp())))+0.05)`, first)
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { _, err := st.RevokeExpiredCommunityRosterMemberships(ctx); done <- err }()
	require.Eventually(t, func() bool {
		var waiting bool
		err := db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE wait_event_type='Lock' AND query LIKE '%FROM space_effective_member_heads%' AND query LIKE '%FOR UPDATE%')`).Scan(&waiting)
		return err == nil && waiting
	}, time.Second, 10*time.Millisecond)
	_, err = db.Pool.Exec(ctx, `SELECT pg_sleep(GREATEST(0,EXTRACT(epoch FROM ($1::timestamptz-clock_timestamp())))+0.05)`, second)
	require.NoError(t, err)
	require.NoError(t, blocker.Commit(ctx))
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("expiry writer did not terminate")
	}
	var untouched bool
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT revoked_at IS NULL FROM public.community_roster_members WHERE space_id=$1 AND profile_id=$2`, space.ID, b).Scan(&untouched))
	require.True(t, untouched, "B expired during A capture wait: only a later snapshot may mutate B")
	_, err = st.RevokeExpiredCommunityRosterMemberships(ctx)
	require.NoError(t, err)
	var state string
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT state FROM public.space_effective_member_heads WHERE space_id=$1 AND profile_id=$2`, space.ID, b).Scan(&state))
	require.Equal(t, "RETIRED", state, "later snapshot finalizes B's actual history")
}
