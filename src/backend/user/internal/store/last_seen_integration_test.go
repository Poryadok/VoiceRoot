package store

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

func TestLastSeenPostgresMonotonicRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "userdb", "")
	integrationtest.ApplyUserDBMigrations(t, ctx, pool, userStoreRepoRoot(t))
	s := NewLastSeenStore(pool)
	id, missing := uuid.New(), uuid.New()
	at := time.Unix(1700000000, 0).UTC()
	require.NoError(t, s.Observe(ctx, id, at))
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) { defer wg.Done(); errs <- s.Observe(ctx, id, at.Add(time.Duration(i)*time.Minute)) }(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.NoError(t, s.Observe(ctx, id, at.Add(-time.Hour)))
	got, err := NewLastSeenStore(pool).GetMany(ctx, []uuid.UUID{id, missing})
	require.NoError(t, err)
	require.Equal(t, at.Add(7*time.Minute).Unix(), got[id])
	require.NotContains(t, got, missing)
	_, err = pool.Exec(ctx, `DROP TABLE profile_last_seen`)
	require.NoError(t, err)
	_, err = s.GetMany(ctx, []uuid.UUID{id})
	require.Error(t, err, "missing migration must not silently serve Redis only")
}
