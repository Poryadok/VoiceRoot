package partysnapshot

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/voice/internal/ordinarycall"
)

func nativePostgres(t *testing.T) *pgxpool.Pool {
	t.Helper()
	admin, _ := postgresFixture(t)
	database := "native_party_" + uuid.New().String()[:8]
	_, err := admin.Exec(context.Background(), `CREATE DATABASE `+database)
	require.NoError(t, err)
	config := admin.Config()
	config.ConnConfig.Database = database
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close(); _, _ = admin.Exec(context.Background(), `DROP DATABASE `+database) })
	files, err := filepath.Glob("../../../migrations/voice_db/*.up.sql")
	require.NoError(t, err)
	require.Len(t, files, 12, "pin the native schema under test rather than silently omit new migrations")
	sort.Strings(files)
	for _, file := range files {
		body, err := os.ReadFile(file)
		require.NoError(t, err)
		_, err = pool.Exec(context.Background(), string(body))
		require.NoError(t, err, file)
	}
	return pool
}

func seedOrdinary(t *testing.T, pool *pgxpool.Pool, active bool) (*ordinarycall.Room, ordinarycall.Identity) {
	t.Helper()
	identity := ordinarycall.Identity{AccountID: uuid.New(), ProfileID: uuid.New(), SessionEpoch: 7}
	intent := ordinarycall.CreateIntent{OperationID: uuid.New(), RoomID: uuid.New(), ChatID: uuid.New(), Group: true, Initiator: identity, MediaKind: 1}
	room, err := ordinarycall.NewPendingRoom(intent, time.Now().UTC())
	require.NoError(t, err)
	journal := ordinarycall.NewPostgresStore(pool, func(_ *ordinarycall.Room, e ordinarycall.Event) (ordinarycall.EncodedEvent, error) {
		body, err := json.Marshal(e)
		subject := "voice.participant_joined"
		if e.Kind == ordinarycall.EventAccepted {
			subject = "voice.call_accepted"
		}
		if e.Kind == ordinarycall.EventCallStarted {
			subject = "voice.call_started"
		}
		return ordinarycall.EncodedEvent{ID: e.ID, Subject: subject, Payload: body}, err
	})
	require.NoError(t, journal.Create(context.Background(), room))
	if active {
		room, err = journal.Mutate(context.Background(), room.Intent.RoomID, func(r *ordinarycall.Room) error {
			if err := r.RecordEnsureResult(intent.OperationID, true, time.Now().UTC()); err != nil {
				return err
			}
			return r.CommitAdmission(intent.OperationID, time.Now().UTC())
		})
		require.NoError(t, err)
	}
	return room, identity
}

func ordinaryCut(t *testing.T, pool *pgxpool.Pool, identity ordinarycall.Identity) (ordinarycall.NativePartyCut, error) {
	t.Helper()
	tx, err := pool.BeginTx(context.Background(), pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	require.NoError(t, err)
	defer tx.Rollback(context.Background())
	return ordinarycall.ReadPartyCut(context.Background(), tx, identity.AccountID, identity.ProfileID)
}

func TestNativeOrdinaryCutUsesFinalizedDocumentAndExactOwnership(t *testing.T) {
	pool := nativePostgres(t)
	room, identity := seedOrdinary(t, pool, true)
	cut, err := ordinaryCut(t, pool, identity)
	require.NoError(t, err)
	require.NotNil(t, cut.Room)
	require.Equal(t, room.Intent.OperationID, cut.Room.Intent.OperationID)
	require.Equal(t, room.RosterVersion, cut.Room.RosterVersion)
	require.Equal(t, int64(7), cut.Room.Members[identity.ProfileID.String()].SessionEpoch)
	require.NotEmpty(t, cut.Witness)
}

func TestNativeOrdinaryCutDoesNotConcealPendingSource(t *testing.T) {
	pool := nativePostgres(t)
	_, identity := seedOrdinary(t, pool, false)
	cut, err := ordinaryCut(t, pool, identity)
	require.NoError(t, err)
	require.NotNil(t, cut.Room)
	require.Equal(t, ordinarycall.StagePreparing, cut.Room.Stage)
}

func TestNativeOrdinaryMissingIndexIsUnavailableNotAbsence(t *testing.T) {
	pool := nativePostgres(t)
	room, identity := seedOrdinary(t, pool, true)
	_, err := pool.Exec(context.Background(), `DELETE FROM voice_ordinary_memberships WHERE room_id=$1`, room.Intent.RoomID)
	require.NoError(t, err)
	_, err = ordinaryCut(t, pool, identity)
	require.ErrorIs(t, err, ordinarycall.ErrUnavailable)
}

func TestNativeOrdinaryMissingFenceIsUnavailableNotAbsence(t *testing.T) {
	pool := nativePostgres(t)
	_, identity := seedOrdinary(t, pool, true)
	_, err := pool.Exec(context.Background(), `DELETE FROM voice_account_voice_fences WHERE account_id=$1`, identity.AccountID)
	require.NoError(t, err)
	_, err = ordinaryCut(t, pool, identity)
	require.ErrorIs(t, err, ordinarycall.ErrUnavailable)
}

func TestNativeOrdinaryAbsenceIsOnlyAComponentWitness(t *testing.T) {
	pool := nativePostgres(t)
	cut, err := ordinaryCut(t, pool, ordinarycall.Identity{AccountID: uuid.New(), ProfileID: uuid.New(), SessionEpoch: 7})
	require.NoError(t, err)
	require.Nil(t, cut.Room)
	require.NotEmpty(t, cut.Witness)
}
