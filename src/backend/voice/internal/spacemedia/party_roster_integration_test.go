package spacemedia

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/voice/internal/rosteroutbox"
)

func TestSpacePartyRosterCommitsOnlyEligibilityDeltas(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	ctx := context.Background()
	pool := startSpaceMediaAdmissionPostgres(t, ctx)
	st := NewPostgresAdmissionStore(pool)
	a := admissionFixture()
	ctx = ownAdmission(ctx, &a)
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
	delivery := &rosteroutbox.Store{Pool: pool}
	firstLease, err := delivery.Claim(ctx)
	require.NoError(t, err)
	require.NotNil(t, firstLease)
	require.Equal(t, firstID, firstLease.ID.String())
	require.Equal(t, firstPayload, firstLease.Payload)
	require.NoError(t, delivery.Retry(ctx, *firstLease, 0))
	newLease, err := delivery.Claim(ctx)
	require.NoError(t, err)
	require.NotNil(t, newLease)
	require.Equal(t, firstLease.ID, newLease.ID)
	require.Equal(t, firstLease.Payload, newLease.Payload)
	require.NotEqual(t, firstLease.LeaseToken, newLease.LeaseToken)
	require.ErrorIs(t, delivery.Delivered(ctx, *firstLease), rosteroutbox.ErrStaleLease)
	require.NoError(t, delivery.Delivered(ctx, *newLease))
	loss := PartyEligibilityLoss{RoomID: uuid.MustParse(a.RoomID), Incarnation: a.OperationID, AccountID: a.AccountID, ProfileID: a.ProfileID, OperationID: a.OperationID, Generation: a.Generation, SessionEpoch: int64(a.SessionEpoch)}
	stale := loss
	stale.SessionEpoch++
	require.ErrorIs(t, st.ApplyPartyEligibilityLoss(ctx, stale), ErrPartySourceChanged)
	stale = loss
	stale.Generation += "-replacement"
	require.ErrorIs(t, st.ApplyPartyEligibilityLoss(ctx, stale), ErrPartySourceChanged)
	check(1, []string{"JOINED"})
	_, err = pool.Exec(ctx, `ALTER TABLE voice_roster_outbox ADD CONSTRAINT reject_test_left CHECK(change <> 'LEFT')`)
	require.NoError(t, err)
	require.Error(t, fixtureBeginOwnedLeave(ctx, st, a.OperationID, a.Generation))
	check(1, []string{"JOINED"})
	require.NoError(t, st.ConfirmProjection(ctx, a.OperationID, a.Generation), "failed outbox must roll back revocation")
	_, err = pool.Exec(ctx, `ALTER TABLE voice_roster_outbox DROP CONSTRAINT reject_test_left`)
	require.NoError(t, err)
	require.NoError(t, st.ApplyPartyEligibilityLoss(ctx, loss))
	check(2, []string{"JOINED", "LEFT"})
	require.ErrorIs(t, st.ApplyPartyEligibilityLoss(ctx, loss), ErrPartySourceChanged)
	require.ErrorIs(t, st.MarkProjectionApplied(ctx, a.OperationID, a.Generation), ErrRoomHeadPending,
		"late projection confirmation cannot resurrect a departing last member")
	require.NoError(t, fixtureBeginOwnedLeave(ctx, st, a.OperationID, a.Generation))
	check(2, []string{"JOINED", "LEFT"})
}

func TestSpacePartyRoomCloseSharesCallerTransactionAndOrdersDepartures(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	ctx := context.Background()
	pool := startSpaceMediaAdmissionPostgres(t, ctx)
	st := NewPostgresAdmissionStore(pool)
	first := admissionFixture()
	ctx = ownAdmission(ctx, &first)
	require.NoError(t, prepareAdmission(ctx, st, &first))
	require.NoError(t, st.Fence(ctx, first))
	require.NoError(t, st.MarkCommitted(ctx, first.OperationID, first.Generation))
	require.NoError(t, st.MarkProjectionApplied(ctx, first.OperationID, first.Generation))
	second := admissionFixture()
	ctx = ownAdmission(ctx, &second)
	second.RoomID = first.RoomID
	second.SpaceID = first.SpaceID
	second.VoiceRoomID = first.VoiceRoomID
	second.CreatedRoom = false
	second.Events = second.Events[1:]
	require.NoError(t, prepareAdmission(ctx, st, &second))
	require.NoError(t, st.Fence(ctx, second))
	require.NoError(t, st.MarkCommitted(ctx, second.OperationID, second.Generation))
	require.NoError(t, st.MarkProjectionApplied(ctx, second.OperationID, second.Generation))
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, BeginPartyRoomClose(ctx, tx, first.VoiceRoomID, first.RoomGeneration))
	require.NoError(t, tx.Rollback(ctx))
	require.NoError(t, st.ConfirmProjection(ctx, first.OperationID, first.Generation))
	var version int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT roster_version FROM voice_space_media_room_heads WHERE call_room_id=$1`, first.RoomID).Scan(&version))
	require.EqualValues(t, 2, version)
	tx, err = pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, BeginPartyRoomClose(ctx, tx, first.VoiceRoomID, first.RoomGeneration))
	require.NoError(t, BeginPartyRoomClose(ctx, tx, first.VoiceRoomID, first.RoomGeneration))
	require.NoError(t, tx.Commit(ctx))
	require.NoError(t, pool.QueryRow(ctx, `SELECT roster_version FROM voice_space_media_room_heads WHERE call_room_id=$1`, first.RoomID).Scan(&version))
	require.EqualValues(t, 4, version)
	rows, err := pool.Query(ctx, `SELECT profile_id::text FROM voice_roster_outbox WHERE room_id=$1 AND change='LEFT' ORDER BY roster_version`, first.RoomID)
	require.NoError(t, err)
	var profiles []string
	for rows.Next() {
		var id string
		require.NoError(t, rows.Scan(&id))
		profiles = append(profiles, id)
	}
	require.NoError(t, rows.Err())
	rows.Close()
	require.Len(t, profiles, 2)
	require.IsIncreasing(t, profiles)
	require.ErrorIs(t, st.ConfirmProjection(ctx, first.OperationID, first.Generation), ErrAdmissionConflict)
	require.ErrorIs(t, st.ConfirmProjection(ctx, second.OperationID, second.Generation), ErrAdmissionConflict)
}
