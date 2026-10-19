package ordinarycall

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

// The owner must seal work and elect native closure in ONE caller transaction.
// A second independently committed Mutate would leave an unowned crash window.
func TestOrdinaryCutoverCloseSharesCallerTransactionAndNativeFacts(t *testing.T) {
	pool := ordinaryPostgres(t)
	ctx := context.Background()
	store := NewPostgresStore(pool, storageTestEncoder)
	c := &Coordinator{Journal: store, Media: &mediaFake{}, Projection: &projectionFake{}, Now: func() time.Time { return testNow }}
	pending, _, _ := pendingGroup(t)
	active, err := c.Start(ctx, pending.Intent, nil)
	require.NoError(t, err)
	closeNative := func(r *Room) error { return r.terminate(uuid.Nil, "maintenance_retirement", r.TransactionTime) }

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(context.Background()) }()
	closed, err := store.mutateTx(ctx, tx, active.Intent.RoomID, closeNative)
	require.NoError(t, err)
	require.Equal(t, StageClosing, closed.Stage)
	require.Equal(t, active.RosterVersion+1, closed.RosterVersion)
	require.NoError(t, tx.Rollback(ctx))
	rolledBack, err := store.Load(ctx, active.Intent.RoomID)
	require.NoError(t, err)
	require.Equal(t, StageActive, rolledBack.Stage)
	require.Equal(t, active.Version, rolledBack.Version)
	require.Equal(t, active.RosterVersion, rolledBack.RosterVersion)
	var leftCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_roster_outbox WHERE room_id=$1 AND change='LEFT'`, active.Intent.RoomID).Scan(&leftCount))
	require.Zero(t, leftCount)

	tx, err = pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(context.Background()) }()
	closed, err = store.mutateTx(ctx, tx, active.Intent.RoomID, closeNative)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	committed, err := store.Load(ctx, active.Intent.RoomID)
	require.NoError(t, err)
	require.Equal(t, StageClosing, committed.Stage)
	require.Equal(t, active.RosterVersion+1, committed.RosterVersion)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_roster_outbox WHERE room_id=$1 AND change='LEFT'`, active.Intent.RoomID).Scan(&leftCount))
	require.Equal(t, 1, leftCount)
	var factsBefore, factsAfter int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_ordinary_outbox WHERE room_id=$1`, active.Intent.RoomID).Scan(&factsBefore))
	tx, err = pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(context.Background()) }()
	replayed, err := store.mutateTx(ctx, tx, active.Intent.RoomID, closeNative)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	require.Equal(t, closed.Version, replayed.Version)
	require.Equal(t, closed.RosterVersion, replayed.RosterVersion)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_ordinary_outbox WHERE room_id=$1`, active.Intent.RoomID).Scan(&factsAfter))
	require.Equal(t, factsBefore, factsAfter)
}
