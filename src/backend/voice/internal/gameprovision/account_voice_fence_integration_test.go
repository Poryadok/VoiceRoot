package gameprovision

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestPostgresAccountVoiceFenceSerializesProfilesAndSurvivesStoreRestart(t *testing.T) {
	ctx := context.Background()
	pool := startGameSessionPostgres(t, ctx)
	first := NewPostgresAccountVoiceFenceStore(pool)
	second := NewPostgresAccountVoiceFenceStore(pool) // independent store instance/DB connection pool
	accountID, profileA, profileB := uuid.New(), uuid.New(), uuid.New()
	roomA, roomB := uuid.NewString(), uuid.NewString()

	start := make(chan struct{})
	var wg sync.WaitGroup
	type reserveResult struct {
		newReservation bool
		err            error
	}
	results := make(chan reserveResult, 2)
	for _, attempt := range []struct {
		store   *PostgresAccountVoiceFenceStore
		profile uuid.UUID
		room    string
	}{
		{first, profileA, roomA}, {second, profileB, roomB},
	} {
		attempt := attempt
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			newReservation, err := attempt.store.Reserve(ctx, accountID, attempt.profile, attempt.room)
			results <- reserveResult{newReservation: newReservation, err: err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	var acquired int
	for result := range results {
		if result.err == nil {
			require.True(t, result.newReservation)
			acquired++
			continue
		}
		require.ErrorIs(t, result.err, ErrActiveAccountVoiceSession)
	}
	require.Equal(t, 1, acquired, "two profiles on one Auth account cannot reserve concurrent Voice sessions")

	var ownerProfile uuid.UUID
	var ownerRoom string
	require.NoError(t, pool.QueryRow(ctx, `SELECT profile_id,room_id FROM voice_account_voice_fences WHERE account_id=$1`, accountID).Scan(&ownerProfile, &ownerRoom))
	owner := first
	if ownerProfile == profileB {
		owner = second
	}
	require.NoError(t, owner.Commit(ctx, accountID, ownerProfile, ownerRoom))

	// A process/store restart observes the durable account fence and cannot
	// admit another profile or rewrite the trusted User profile mapping.
	restarted := NewPostgresAccountVoiceFenceStore(pool)
	otherProfile, otherRoom := profileB, roomB
	if ownerProfile == profileB {
		otherProfile, otherRoom = profileA, roomA
	}
	_, err := restarted.Reserve(ctx, accountID, otherProfile, otherRoom)
	require.ErrorIs(t, err, ErrActiveAccountVoiceSession)
	_, err = restarted.Reserve(ctx, uuid.New(), ownerProfile, uuid.NewString())
	require.ErrorIs(t, err, ErrAccountProfileMappingConflict)

	require.NoError(t, restarted.Release(ctx, accountID, ownerProfile, ownerRoom))
	newReservation, err := restarted.Reserve(ctx, accountID, otherProfile, otherRoom)
	require.NoError(t, err)
	require.True(t, newReservation)
	require.NoError(t, restarted.Commit(ctx, accountID, otherProfile, otherRoom))
	require.NoError(t, restarted.Release(ctx, accountID, otherProfile, otherRoom))
}

func TestPostgresAccountVoiceFenceReclaimsExpiredReservationOnly(t *testing.T) {
	ctx := context.Background()
	pool := startGameSessionPostgres(t, ctx)
	store := NewPostgresAccountVoiceFenceStore(pool)
	accountID, profileA, profileB := uuid.New(), uuid.New(), uuid.New()
	newReservation, err := store.Reserve(ctx, accountID, profileA, "reservation-a")
	require.NoError(t, err)
	require.True(t, newReservation)
	_, err = pool.Exec(ctx, `UPDATE voice_account_voice_fences SET reservation_expires_at=clock_timestamp()-interval '1 second' WHERE account_id=$1`, accountID)
	require.NoError(t, err)
	newReservation, err = store.Reserve(ctx, accountID, profileB, "reservation-b")
	require.NoError(t, err)
	require.True(t, newReservation)
	require.NoError(t, store.Commit(ctx, accountID, profileB, "reservation-b"))
	_, err = store.Reserve(ctx, accountID, profileA, "reservation-c")
	require.ErrorIs(t, err, ErrActiveAccountVoiceSession)
	var state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM voice_account_voice_fences WHERE account_id=$1`, accountID).Scan(&state))
	require.Equal(t, "active", state)
}

func TestPostgresAccountVoiceFenceTransfersExplicitlyAndIdempotently(t *testing.T) {
	ctx := context.Background()
	pool := startGameSessionPostgres(t, ctx)
	store := NewPostgresAccountVoiceFenceStore(pool)
	accountID, profileID, otherProfile := uuid.New(), uuid.New(), uuid.New()
	newReservation, err := store.Reserve(ctx, accountID, profileID, "old-room")
	require.NoError(t, err)
	require.True(t, newReservation)
	require.NoError(t, store.Commit(ctx, accountID, profileID, "old-room"))
	require.NoError(t, store.Transfer(ctx, accountID, profileID, "old-room", "new-room"))
	require.NoError(t, store.Transfer(ctx, accountID, profileID, "old-room", "new-room"), "retry after call-store success is idempotent")
	_, err = store.Reserve(ctx, accountID, otherProfile, "other-room")
	require.ErrorIs(t, err, ErrActiveAccountVoiceSession)
	require.NoError(t, store.Release(ctx, accountID, profileID, "new-room"))
	newReservation, err = store.Reserve(ctx, accountID, otherProfile, "other-room")
	require.NoError(t, err)
	require.True(t, newReservation)
}
