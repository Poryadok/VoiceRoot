package store

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

type creatorReplayAccess interface {
	ClaimCreatorPrincipalReplay(context.Context, string, string, time.Time) error
	CleanupCreatorPrincipalReplays(context.Context, int) (int64, error)
}

func creatorReplayJTI(t *testing.T) string {
	t.Helper()
	raw := make([]byte, 20)
	_, err := rand.Read(raw)
	require.NoError(t, err)
	return base64.RawURLEncoding.EncodeToString(raw)
}

func TestCreatorReplayReservationSurvivesConcurrentInstancesAndRestart(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	applyMemberAuthorityIfPresent(t, st)
	access, ok := any(st).(creatorReplayAccess)
	require.True(t, ok, "Create needs durable JTI reservation, not Redis Ping or reset replay history")
	otherPool, err := pgxpool.New(ctx, pool.Config().ConnString())
	require.NoError(t, err)
	defer otherPool.Close()
	other := any(&SpaceStore{Pool: otherPool}).(creatorReplayAccess)
	var dbNow time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&dbNow))
	expiry := dbNow.Add(30 * time.Second)
	jti := creatorReplayJTI(t)
	errors := make(chan error, 2)
	var wg sync.WaitGroup
	for _, instance := range []creatorReplayAccess{access, other} {
		wg.Add(1)
		go func(s creatorReplayAccess) {
			defer wg.Done()
			errors <- s.ClaimCreatorPrincipalReplay(ctx, "gateway", jti, expiry)
		}(instance)
	}
	wg.Wait()
	close(errors)
	succeeded, replayed := 0, 0
	for err := range errors {
		if err == nil {
			succeeded++
		} else {
			require.ErrorIs(t, err, ErrMemberAdmissionConflict)
			replayed++
		}
	}
	require.Equal(t, 1, succeeded)
	require.Equal(t, 1, replayed)
	otherPool.Close()
	restartedPool, err := pgxpool.New(ctx, pool.Config().ConnString())
	require.NoError(t, err)
	defer restartedPool.Close()
	restarted := any(&SpaceStore{Pool: restartedPool}).(creatorReplayAccess)
	require.ErrorIs(t, restarted.ClaimCreatorPrincipalReplay(ctx, "gateway", jti, expiry), ErrMemberAdmissionConflict)
	_, err = pool.Exec(ctx, `UPDATE space_creator_principal_replays SET expires_at=expires_at+interval '1 hour' WHERE jti=$1`, jti)
	require.Error(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM space_creator_principal_replays WHERE jti=$1`, jti)
	require.Error(t, err)
	_, err = pool.Exec(ctx, `TRUNCATE space_creator_principal_replays`)
	require.Error(t, err)
	count, err := access.CleanupCreatorPrincipalReplays(ctx, 100)
	require.NoError(t, err)
	require.Zero(t, count)
	// Historical expired markers are a retention fixture, not new admissions.
	_, err = pool.Exec(ctx, `INSERT INTO space_creator_principal_replays(issuer,jti,recorded_at,expires_at) VALUES('gateway',$1,clock_timestamp()-interval '1 minute',clock_timestamp()-interval '10 seconds')`, creatorReplayJTI(t))
	require.NoError(t, err)
	count, err = access.CleanupCreatorPrincipalReplays(ctx, 1)
	require.NoError(t, err)
	require.EqualValues(t, 1, count)
	require.ErrorIs(t, access.ClaimCreatorPrincipalReplay(ctx, "gateway", jti, expiry), ErrMemberAdmissionConflict, "cleanup cannot erase an unexpired reservation")
	for _, issuer := range []string{"role", "space", ""} {
		require.ErrorIs(t, access.ClaimCreatorPrincipalReplay(ctx, issuer, creatorReplayJTI(t), expiry), ErrMemberAdmissionConflict)
	}
	require.ErrorIs(t, access.ClaimCreatorPrincipalReplay(ctx, "gateway", "not-a-jti", expiry), ErrMemberAdmissionConflict)
	require.ErrorIs(t, access.ClaimCreatorPrincipalReplay(ctx, "gateway", uuid.NewString(), expiry), ErrMemberAdmissionConflict, "issuer JTI is canonical base64url20, not a UUID")
	require.ErrorIs(t, access.ClaimCreatorPrincipalReplay(ctx, "gateway", creatorReplayJTI(t), dbNow.Add(-time.Second)), ErrMemberAdmissionConflict)
	require.ErrorIs(t, access.ClaimCreatorPrincipalReplay(ctx, "gateway", creatorReplayJTI(t), dbNow.Add(time.Hour)), ErrMemberAdmissionConflict)
	restartedPool.Close()
	require.ErrorIs(t, restarted.ClaimCreatorPrincipalReplay(ctx, "gateway", creatorReplayJTI(t), expiry), ErrMemberAuthorityUnavailable, "unknown durability cannot authorize creation")
	var spaces int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM spaces`).Scan(&spaces))
	require.Zero(t, spaces, "replay reservation does not create a member or Space")
}
