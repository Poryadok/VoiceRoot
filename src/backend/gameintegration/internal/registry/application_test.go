package registry

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

func TestCreateApplicationIdempotencyAndIsolation(t *testing.T) {
	ctx := context.Background()
	migration := filepath.Join("..", "..", "..", "migrations", "game_integration_db", "000001_init.up.sql")
	pool := integrationtest.StartPostgres(t, ctx, "game_integration_test", migration)
	store := &Store{Pool: pool}
	ownerA := uuid.New()
	ownerB := uuid.New()

	first, err := store.CreateApplication(ctx, CreateApplicationInput{
		OwnerAccountID: ownerA,
		Name:           "HerdTrip",
		IdempotencyKey: "create-herdtrip-1",
	})
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, first.ID)
	require.Equal(t, ownerA, first.OwnerAccountID)
	require.Equal(t, "draft", first.Status)

	retry, err := store.CreateApplication(ctx, CreateApplicationInput{
		OwnerAccountID: ownerA,
		Name:           "HerdTrip",
		IdempotencyKey: "create-herdtrip-1",
	})
	require.NoError(t, err)
	require.Equal(t, first.ID, retry.ID)

	_, err = store.CreateApplication(ctx, CreateApplicationInput{
		OwnerAccountID: ownerA,
		Name:           "Changed name",
		IdempotencyKey: "create-herdtrip-1",
	})
	require.ErrorIs(t, err, ErrIdempotencyConflict)

	other, err := store.CreateApplication(ctx, CreateApplicationInput{
		OwnerAccountID: ownerB,
		Name:           "HerdTrip",
		IdempotencyKey: "create-herdtrip-1",
	})
	require.NoError(t, err)
	require.NotEqual(t, first.ID, other.ID)
	require.Equal(t, ownerB, other.OwnerAccountID)
}

func TestCreateApplicationConcurrentRetryHasOneResult(t *testing.T) {
	ctx := context.Background()
	migration := filepath.Join("..", "..", "..", "migrations", "game_integration_db", "000001_init.up.sql")
	pool := integrationtest.StartPostgres(t, ctx, "game_integration_test", migration)
	store := &Store{Pool: pool}
	owner := uuid.New()
	const clients = 8
	ids := make(chan uuid.UUID, clients)
	errs := make(chan error, clients)
	var wg sync.WaitGroup
	for range clients {
		wg.Add(1)
		go func() {
			defer wg.Done()
			application, err := store.CreateApplication(ctx, CreateApplicationInput{
				OwnerAccountID: owner,
				Name:           "Dejavu",
				IdempotencyKey: "same-concurrent-request",
			})
			ids <- application.ID
			errs <- err
		}()
	}
	wg.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var expected uuid.UUID
	for id := range ids {
		if expected == uuid.Nil {
			expected = id
		}
		require.Equal(t, expected, id)
	}
	var count int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM applications WHERE owner_account_id=$1", owner).Scan(&count))
	require.Equal(t, 1, count)
}

func TestCreateApplicationRejectsInvalidInput(t *testing.T) {
	store := &Store{}
	_, err := store.CreateApplication(context.Background(), CreateApplicationInput{
		OwnerAccountID: uuid.New(),
		Name:           " ",
		IdempotencyKey: "request-1",
	})
	require.True(t, errors.Is(err, ErrInvalidApplication))
	_, err = store.CreateApplication(context.Background(), CreateApplicationInput{
		OwnerAccountID: uuid.Nil,
		Name:           "Game",
		IdempotencyKey: "request-1",
	})
	require.ErrorIs(t, err, ErrInvalidApplication)
}
