package spacemedia

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSpacePartyRosterCommitsOnlyEligibilityDeltas(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	ctx := context.Background()
	pool := startSpaceMediaAdmissionPostgres(t, ctx)
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	migration := filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations", "voice_db", "000013_a3_roster_outbox.up.sql")
	body, err := os.ReadFile(migration)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(body))
	require.NoError(t, err)
	st := NewPostgresAdmissionStore(pool)
	a := admissionFixture()
	require.NoError(t, prepareAdmission(ctx, st, &a))
	check := func(version int64, changes []string) {
		t.Helper()
		var got int64
		require.NoError(t, pool.QueryRow(ctx, `SELECT roster_version FROM voice_space_media_room_heads WHERE call_room_id=$1`, a.RoomID).Scan(&got))
		require.Equal(t, version, got)
		rows, err := pool.Query(ctx, `SELECT change FROM voice_roster_outbox WHERE room_id=$1 ORDER BY roster_version`, a.RoomID)
		require.NoError(t, err)
		defer rows.Close()
		actual := []string{}
		for rows.Next() {
			var c string
			require.NoError(t, rows.Scan(&c))
			actual = append(actual, c)
		}
		require.NoError(t, rows.Err())
		require.Equal(t, changes, actual)
	}
	check(0, []string{})
	require.NoError(t, st.Fence(ctx, a))
	require.NoError(t, st.MarkCommitted(ctx, a.OperationID, a.Generation))
	check(0, []string{})
	require.NoError(t, st.MarkProjectionApplied(ctx, a.OperationID, a.Generation))
	check(1, []string{"JOINED"})
	var firstID string
	var firstPayload []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT event_id,payload FROM voice_roster_outbox WHERE room_id=$1`, a.RoomID).Scan(&firstID, &firstPayload))
	require.NotEmpty(t, firstPayload)
	require.NoError(t, st.MarkProjectionApplied(ctx, a.OperationID, a.Generation))
	check(1, []string{"JOINED"})
	var replayID string
	var replayPayload []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT event_id,payload FROM voice_roster_outbox WHERE room_id=$1`, a.RoomID).Scan(&replayID, &replayPayload))
	require.Equal(t, firstID, replayID)
	require.Equal(t, firstPayload, replayPayload)
	_, err = pool.Exec(ctx, `ALTER TABLE voice_roster_outbox ADD CONSTRAINT reject_test_left CHECK(change <> 'LEFT')`)
	require.NoError(t, err)
	require.Error(t, st.BeginRoomLeave(ctx, a.OperationID, a.Generation))
	check(1, []string{"JOINED"})
	require.NoError(t, st.ConfirmProjection(ctx, a.OperationID, a.Generation), "failed outbox must roll back revocation")
	_, err = pool.Exec(ctx, `ALTER TABLE voice_roster_outbox DROP CONSTRAINT reject_test_left`)
	require.NoError(t, err)
	require.NoError(t, st.BeginRoomLeave(ctx, a.OperationID, a.Generation))
	check(2, []string{"JOINED", "LEFT"})
	require.ErrorIs(t, st.MarkProjectionApplied(ctx, a.OperationID, a.Generation), ErrRoomHeadPending,
		"late projection confirmation cannot resurrect a departing last member")
	require.NoError(t, st.BeginRoomLeave(ctx, a.OperationID, a.Generation))
	check(2, []string{"JOINED", "LEFT"})
}
