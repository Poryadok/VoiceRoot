package grpcsvc

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/user/internal/store"
)

// The benefit matrix requires concurrent creates to respect the account cap.
func TestCreateProfile_ConcurrentAccountCap(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool := startUserPostgresForSubscriptionTests(t, ctx)
	profiles := store.NewProfileStore(pool)
	cli, _ := startUserGRPCForSubscriptionTests(t, profiles)
	for _, tc := range []struct {
		tier string
		cap  int
	}{{"free", 2}, {"premium", 5}} {
		t.Run(tc.tier, func(t *testing.T) {
			account := uuid.New()
			_, err := profiles.EnsurePrimaryProfile(ctx, account, nil, "Primary", false)
			require.NoError(t, err)
			authed := withAccountTier(ctx, account, tc.tier)
			for i := 1; i < tc.cap-1; i++ {
				_, err := cli.CreateProfile(authed, &userv1.CreateProfileRequest{DisplayName: fmt.Sprintf("Seed %d", i)})
				require.NoError(t, err)
			}
			// Reads continue while inserts wait, forcing both RPCs past the old
			// non-transactional count check before either can commit.
			gate, err := pool.Begin(ctx)
			require.NoError(t, err)
			defer func() { _ = gate.Rollback(context.Background()) }()
			_, err = gate.Exec(ctx, "LOCK TABLE profiles IN SHARE MODE")
			require.NoError(t, err)
			results := make(chan error, 2)
			for i := 0; i < 2; i++ {
				go func(i int) {
					_, err := cli.CreateProfile(authed, &userv1.CreateProfileRequest{DisplayName: fmt.Sprintf("Contender %d", i)})
					results <- err
				}(i)
			}
			require.Eventually(t, func() bool {
				var waiting int
				err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity
					WHERE datname = current_database() AND wait_event_type = 'Lock'`).Scan(&waiting)
				return err == nil && waiting == 2
			}, 10*time.Second, 10*time.Millisecond, "both creates must reach the database gate")
			require.NoError(t, gate.Commit(ctx))
			got := map[codes.Code]int{}
			for i := 0; i < 2; i++ {
				got[status.Code(<-results)]++
			}
			require.Equal(t, map[codes.Code]int{codes.OK: 1, codes.ResourceExhausted: 1}, got)
			count, err := profiles.CountByAccountID(ctx, account)
			require.NoError(t, err)
			require.Equal(t, tc.cap, count)
			_, err = cli.CreateProfile(authed, &userv1.CreateProfileRequest{DisplayName: "Over cap"})
			require.Equal(t, codes.ResourceExhausted, status.Code(err))
		})
	}
}
