package identityassociation

import (
	"bytes"
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func freshSourcePin(t *testing.T, pool *pgxpool.Pool) FreshSourcePin {
	t.Helper()
	var pin FreshSourcePin
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT history_generation,user_source_generation FROM identity_meta WHERE singleton`).Scan(&pin.HistoryGeneration, &pin.UserSourceGeneration))
	return pin
}

func TestPristineProvisioningRejectsEqualGenerations(t *testing.T) {
	keys := keysetForTest(t, 1, map[uint32][]byte{1: bytes.Repeat([]byte{0x47}, 32)})
	id := uuid.New()
	_, err := NewFreshStore(context.Background(), nil, keys, FreshSourcePin{HistoryGeneration: id, UserSourceGeneration: id})
	require.ErrorIs(t, err, ErrIncomplete)
	_, err = NewMigratedStore(context.Background(), nil, keys, MigrationSourcePin{HistoryGeneration: id, UserSourceGeneration: id})
	require.ErrorIs(t, err, ErrIncomplete)
}

func TestNativeFreshBootstrapRequiresBothPinnedGenerationsAndPristineSource(t *testing.T) {
	pool, profile, account := nativeDatabase(t)
	ctx := context.Background()
	secret := bytes.Repeat([]byte{0x47}, 32)
	keys := keysetForTest(t, 1, map[uint32][]byte{1: secret})
	pinned := freshSourcePin(t, pool)

	wrongHistory := pinned
	wrongHistory.HistoryGeneration = uuid.New()
	_, err := NewFreshStore(ctx, pool, keys, wrongHistory)
	require.ErrorIs(t, err, ErrIncomplete)
	wrongSource := pinned
	wrongSource.UserSourceGeneration = uuid.New()
	_, err = NewFreshStore(ctx, pool, keys, wrongSource)
	require.ErrorIs(t, err, ErrIncomplete)
	_, err = NewFreshStore(ctx, pool, keys, FreshSourcePin{})
	require.ErrorIs(t, err, ErrIncomplete)
	_, err = NewMigratedStore(ctx, pool, keys, MigrationSourcePin{HistoryGeneration: pinned.HistoryGeneration, UserSourceGeneration: pinned.HistoryGeneration})
	require.ErrorIs(t, err, ErrIncomplete)

	store, err := NewFreshStore(ctx, pool, keys, pinned)
	require.NoError(t, err)
	_, err = NewFreshStore(ctx, pool, keys, pinned)
	require.ErrorIs(t, err, ErrIncomplete, "a provisioned source cannot be reopened through the fresh-only path")
	require.NoError(t, store.ActivateGuards(ctx))
	prepared, err := keys.PrepareSelectors(profile, account)
	require.NoError(t, err)
	tx, err := store.Begin(ctx)
	require.NoError(t, err)
	fact, err := store.CaptureRetainedTx(ctx, tx, prepared)
	require.NoError(t, err)
	require.Equal(t, OriginRetainedNow, fact.Origin)
	require.Empty(t, fact.ParentHandle)
	require.NoError(t, tx.Commit(ctx))

	_, err = NewFreshStore(ctx, pool, keys, pinned)
	require.ErrorIs(t, err, ErrIncomplete, "fresh bootstrap must not adopt an existing source")
}

func TestNativeFreshBootstrapConcurrentCallersCommitOnce(t *testing.T) {
	pool, _, _ := nativeDatabase(t)
	ctx := context.Background()
	keys := keysetForTest(t, 1, map[uint32][]byte{1: bytes.Repeat([]byte{0x48}, 32)})
	pinned := freshSourcePin(t, pool)
	start := make(chan struct{})
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			<-start
			_, err := NewFreshStore(ctx, pool, keys, pinned)
			results <- err
		}()
	}
	close(start)
	first, second := <-results, <-results
	require.True(t, (first == nil && second == ErrIncomplete) || (second == nil && first == ErrIncomplete), "exactly one fresh bootstrap may commit: %v / %v", first, second)
	var head uint64
	var versions int
	require.NoError(t, pool.QueryRow(ctx, `SELECT history_head,(SELECT count(*) FROM identity_key_versions) FROM identity_meta WHERE singleton`).Scan(&head, &versions))
	require.EqualValues(t, 1, head)
	require.Equal(t, 1, versions)
}
