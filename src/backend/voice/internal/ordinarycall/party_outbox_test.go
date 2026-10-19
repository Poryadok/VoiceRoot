package ordinarycall

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPostgresPartyOutboxPreservesCompatibilityAndBatchVersions(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	pool := ordinaryPostgres(t)
	ctx := context.Background()
	st := NewPostgresStore(pool, storageTestEncoder)
	caller, callee := identity(), identity()
	caller.SessionEpoch = 7
	op := uuid.New()
	room, err := NewPendingRoom(CreateIntent{OperationID: op, RoomID: uuid.New(), ChatID: uuid.New(), Initiator: caller, Callee: &callee, MediaKind: 1}, testNow)
	require.NoError(t, err)
	require.NoError(t, st.Create(ctx, room))
	_, err = st.Mutate(ctx, room.Intent.RoomID, func(r *Room) error {
		if e := r.RecordEnsureResult(op, true, testNow); e != nil {
			return e
		}
		return r.CommitAdmission(op, testNow)
	})
	require.NoError(t, err)
	accept := uuid.New()
	_, err = st.Mutate(ctx, room.Intent.RoomID, func(r *Room) error {
		if e := r.BeginAccept(accept, callee.ProfileID, testNow); e != nil {
			return e
		}
		return r.CommitAdmission(accept, testNow)
	})
	require.NoError(t, err)
	var versions []int64
	var labels []string
	rows, err := pool.Query(ctx, `SELECT roster_version,change FROM voice_roster_outbox WHERE room_id=$1 ORDER BY roster_version`, room.Intent.RoomID)
	require.NoError(t, err)
	for rows.Next() {
		var v int64
		var label string
		require.NoError(t, rows.Scan(&v, &label))
		versions = append(versions, v)
		labels = append(labels, label)
	}
	require.NoError(t, rows.Err())
	rows.Close()
	require.Equal(t, []int64{1, 2}, versions)
	require.Equal(t, []string{"JOINED", "JOINED"}, labels)
	var collisions, compat int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_roster_outbox a JOIN voice_ordinary_outbox b USING(event_id) WHERE a.room_id=$1`, room.Intent.RoomID).Scan(&collisions))
	require.Zero(t, collisions, "NATS stream dedup must not suppress either topic")
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_ordinary_outbox WHERE room_id=$1 AND roster_version IS NOT NULL`, room.Intent.RoomID).Scan(&compat))
	require.Equal(t, 2, compat)
	_, err = st.Mutate(ctx, room.Intent.RoomID, func(r *Room) error { return r.CommitAdmission(accept, testNow) })
	require.NoError(t, err)
	current, err := st.Load(ctx, room.Intent.RoomID)
	require.NoError(t, err)
	m := current.Members[caller.ProfileID.String()]
	loss := PartyEligibilityLoss{RoomID: room.Intent.RoomID, Incarnation: op, AccountID: caller.AccountID, ProfileID: caller.ProfileID, OperationID: m.FenceOperationID, Generation: m.Generation, SessionEpoch: caller.SessionEpoch}
	stale := loss
	stale.Generation = uuid.New()
	require.ErrorIs(t, st.ApplyPartyEligibilityLoss(ctx, stale), ErrPartySourceChanged)
	require.NoError(t, st.ApplyPartyEligibilityLoss(ctx, loss))
	require.ErrorIs(t, st.ApplyPartyEligibilityLoss(ctx, loss), ErrPartySourceChanged)
	_, err = st.Mutate(ctx, room.Intent.RoomID, func(r *Room) error { return r.DecideEnd(caller.ProfileID, "hangup", testNow) })
	require.NoError(t, err)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_roster_outbox WHERE room_id=$1`, room.Intent.RoomID).Scan(&count))
	require.Equal(t, 4, count, "two joined and two left facts, no replay fact")
}
