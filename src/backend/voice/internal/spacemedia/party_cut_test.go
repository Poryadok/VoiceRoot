package spacemedia

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func spacePartyDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("BE011_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("real PostgreSQL unavailable: set BE011_TEST_PG_DSN")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	database := "space_party_" + uuid.New().String()[:8]
	_, err = admin.Exec(ctx, `CREATE DATABASE `+database)
	require.NoError(t, err)
	config, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	config.ConnConfig.Database = database
	config.MaxConns = 3
	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), `DROP DATABASE `+database)
		admin.Close()
	})
	files, err := filepath.Glob("../../../migrations/voice_db/*.up.sql")
	require.NoError(t, err)
	sort.Strings(files)
	for _, file := range files {
		if strings.Compare(filepath.Base(file)[:6], "000013") > 0 {
			continue
		}
		body, err := os.ReadFile(file)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(body))
		require.NoError(t, err, file)
	}
	return pool
}

func readSpaceParty(t *testing.T, pool *pgxpool.Pool, a Admission) (NativePartyCut, error) {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	return ReadPartyCut(ctx, tx, a.AccountID, a.ProfileID)
}

func finalizePartyAdmission(t *testing.T, st *PostgresAdmissionStore, a *Admission) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, prepareAdmission(ctx, st, a))
	require.NoError(t, st.Fence(ctx, *a))
	require.NoError(t, st.MarkCommitted(ctx, a.OperationID, a.Generation))
	require.NoError(t, st.MarkProjectionApplied(ctx, a.OperationID, a.Generation))
}

func TestNativeSpacePartyCutRequiresFinalizedCreatorAndExactActiveIdentity(t *testing.T) {
	pool := spacePartyDatabase(t)
	st := NewPostgresAdmissionStore(pool)
	ctx := context.Background()
	a := admissionFixture()
	a.Generation = "native text/epoch-alpha"
	absent, err := readSpaceParty(t, pool, a)
	require.NoError(t, err)
	require.Nil(t, absent.Head)
	require.NotEmpty(t, absent.Witness)
	require.NoError(t, prepareAdmission(ctx, st, &a))
	_, err = readSpaceParty(t, pool, a)
	require.ErrorIs(t, err, ErrRoomHeadPending, "tentative creation cannot become SOLO absence")
	require.NoError(t, st.Fence(ctx, a))
	require.NoError(t, st.MarkCommitted(ctx, a.OperationID, a.Generation))
	_, err = readSpaceParty(t, pool, a)
	require.ErrorIs(t, err, ErrRoomHeadPending)
	require.NoError(t, st.MarkProjectionApplied(ctx, a.OperationID, a.Generation))
	cut, err := readSpaceParty(t, pool, a)
	require.NoError(t, err)
	require.NotNil(t, cut.Head)
	require.Equal(t, uuid.MustParse(a.RoomID), cut.Head.RoomID)
	require.Equal(t, a.OperationID, cut.Head.CreatorOperationID)
	require.EqualValues(t, 1, cut.Head.RosterVersion)
	require.Len(t, cut.Members, 1)
	require.Equal(t, a.Generation, cut.Members[0].Generation)
	require.EqualValues(t, a.SessionEpoch, cut.Members[0].SessionEpoch)
	require.Equal(t, "ACTIVE", cut.Members[0].State)
	require.Equal(t, "COMMITTED", cut.Members[0].AdmissionState)
	require.True(t, cut.Members[0].ProjectionApplied)
	_, err = pool.Exec(ctx, `UPDATE voice_account_voice_fences SET admission_generation='replacement' WHERE account_id=$1`, a.AccountID)
	require.NoError(t, err)
	_, err = readSpaceParty(t, pool, a)
	require.ErrorIs(t, err, ErrAdmissionUnavailable)
	_, err = pool.Exec(ctx, `UPDATE voice_account_voice_fences SET admission_generation=$2 WHERE account_id=$1`, a.AccountID, a.Generation)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE voice_profile_account_mappings SET account_id=$2 WHERE profile_id=$1`, a.ProfileID, uuid.New())
	require.NoError(t, err)
	_, err = readSpaceParty(t, pool, a)
	require.ErrorIs(t, err, ErrAdmissionUnavailable)
}

func TestNativeSpacePartyCutRetainsDepartedCreatorAndExcludesEndedLocator(t *testing.T) {
	pool := spacePartyDatabase(t)
	st := NewPostgresAdmissionStore(pool)
	ctx := context.Background()
	creator := admissionFixture()
	finalizePartyAdmission(t, st, &creator)
	peer := admissionFixture()
	peer.RoomID = creator.RoomID
	peer.SpaceID = creator.SpaceID
	peer.VoiceRoomID = creator.VoiceRoomID
	peer.CreatedRoom = false
	peer.Events = peer.Events[1:]
	finalizePartyAdmission(t, st, &peer)
	require.NoError(t, st.BeginRoomLeave(ctx, creator.OperationID, creator.Generation))
	require.NoError(t, st.CompleteRoomLeave(ctx, creator))
	cut, err := readSpaceParty(t, pool, peer)
	require.NoError(t, err)
	require.NotNil(t, cut.Head)
	require.Equal(t, creator.OperationID, cut.Head.CreatorOperationID)
	require.EqualValues(t, 3, cut.Head.RosterVersion)
	require.Equal(t, "OPEN", cut.Head.State)
	require.Len(t, cut.Members, 2)
	states := map[uuid.UUID]string{}
	for _, m := range cut.Members {
		states[m.ProfileID] = m.State
	}
	require.Equal(t, "ENDED", states[creator.ProfileID])
	require.Equal(t, "ACTIVE", states[peer.ProfileID])
	departed, err := readSpaceParty(t, pool, creator)
	require.NoError(t, err)
	require.Nil(t, departed.Head, "ended locator is not the surviving peer's authority")
}

func TestNativeSpacePartyCutPinsMVCCWithoutBlockingNativeLeave(t *testing.T) {
	pool := spacePartyDatabase(t)
	st := NewPostgresAdmissionStore(pool)
	ctx := context.Background()
	creator := admissionFixture()
	finalizePartyAdmission(t, st, &creator)
	peer := admissionFixture()
	peer.RoomID = creator.RoomID
	peer.SpaceID = creator.SpaceID
	peer.VoiceRoomID = creator.VoiceRoomID
	peer.CreatedRoom = false
	peer.Events = peer.Events[1:]
	finalizePartyAdmission(t, st, &peer)
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	before, err := ReadPartyCut(ctx, tx, peer.AccountID, peer.ProfileID)
	require.NoError(t, err)
	require.NotNil(t, before.Head)
	bounded, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	require.NoError(t, st.BeginRoomLeave(bounded, peer.OperationID, peer.Generation), "read must not retain writer locks")
	again, err := ReadPartyCut(ctx, tx, peer.AccountID, peer.ProfileID)
	require.NoError(t, err)
	require.Equal(t, before, again)
	require.NoError(t, tx.Commit(ctx))
	after, err := readSpaceParty(t, pool, peer)
	require.NoError(t, err)
	require.NotNil(t, after.Head)
	require.Equal(t, before.Head.RosterVersion+1, after.Head.RosterVersion)
	require.NotEqual(t, before.Witness, after.Witness)
}
