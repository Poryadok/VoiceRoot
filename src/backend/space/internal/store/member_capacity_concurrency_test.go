package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// Separate pools model separate service instances. Holding the Space row keeps
// the first mutation open while the second admission competes for the last slot.
func TestMemberCapacityConcurrentLastSlot(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	for _, path := range []string{"public", "invite", "guest", "public_and_invite"} {
		t.Run(path, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()
			owner := uuid.New()
			st := &SpaceStore{Pool: pool}
			space, err := st.CreateSpace(ctx, owner, "Last slot", "", "public")
			require.NoError(t, err)
			// Use actual membership rows, including the owner, rather than only
			// assigning the cached count used by admission.
			_, err = pool.Exec(ctx, `INSERT INTO space_members(space_id,profile_id)
			 SELECT $1,gen_random_uuid() FROM generate_series(1,48)`, space.ID)
			require.NoError(t, err)
			_, err = pool.Exec(ctx, `UPDATE spaces SET member_count=49,allow_guests=true WHERE id=$1`, space.ID)
			require.NoError(t, err)
			codes := make([]string, 2)
			for index := range codes {
				invite, createErr := st.CreateInvite(ctx, CreateInviteInput{SpaceID: space.ID, CreatorProfileID: owner})
				require.NoError(t, createErr)
				codes[index] = invite.Code
			}
			blocker, err := pool.Begin(ctx)
			require.NoError(t, err)
			defer func() { _ = blocker.Rollback(context.Background()) }()
			_, err = blocker.Exec(ctx, `SELECT id FROM spaces WHERE id=$1 FOR UPDATE`, space.ID)
			require.NoError(t, err)
			results := make(chan error, 2)
			names := []string{"capacity_first_" + space.ID.String(), "capacity_second_" + space.ID.String()}
			for index, name := range names {
				config := pool.Config().Copy()
				config.ConnConfig.RuntimeParams["application_name"] = name
				workerPool, poolErr := pgxpool.NewWithConfig(ctx, config)
				require.NoError(t, poolErr)
				defer func() {
					cancel()
					workerPool.Close()
				}()
				go func(index int, workerPool *pgxpool.Pool) {
					worker := &SpaceStore{Pool: workerPool}
					var joinErr error
					switch {
					case path == "guest":
						_, joinErr = worker.JoinGuestByInvite(ctx, codes[index], uuid.New(), uuid.New())
					case path == "invite" || path == "public_and_invite" && index == 1:
						_, joinErr = worker.JoinByInvite(ctx, codes[index], uuid.New(), uuid.New())
					default:
						_, joinErr = worker.JoinSpace(ctx, space.ID, uuid.New(), uuid.New())
					}
					results <- joinErr
				}(index, workerPool)
			}
			// Observe both competing transactions in PostgreSQL before releasing
			// the row. No sleep or process-local mutex supplies serialization.
			require.Eventually(t, func() bool {
				var waiting int
				queryErr := pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity
				 WHERE application_name=ANY($1::text[]) AND wait_event_type='Lock'`, names).Scan(&waiting)
				return queryErr == nil && waiting == 2
			}, 5*time.Second, 10*time.Millisecond)
			require.NoError(t, blocker.Commit(ctx))
			var winners, denied int
			for range 2 {
				select {
				case joinErr := <-results:
					if joinErr == nil {
						winners++
					} else {
						require.True(t, errors.Is(joinErr, ErrMemberCapReached), "unexpected join error: %v", joinErr)
						denied++
					}
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
			}
			require.Equal(t, 1, winners)
			require.Equal(t, 1, denied)
			var cached, actual int
			require.NoError(t, pool.QueryRow(ctx, `SELECT member_count,(SELECT count(*) FROM space_members WHERE space_id=$1) FROM spaces WHERE id=$1`, space.ID).Scan(&cached, &actual))
			require.Equal(t, 50, cached)
			require.Equal(t, 50, actual)
		})
	}
}
