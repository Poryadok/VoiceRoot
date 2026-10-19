package partysnapshot

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func componentForeignCut(t *testing.T, pool *pgxpool.Pool, subject Subject, native Candidate) foreignCut {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	cut, err := readForeignCut(ctx, tx, subject, native)
	require.NoError(t, err)
	return cut
}

func seedForeignRoom(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	room := uuid.New()
	_, err := pool.Exec(context.Background(), `INSERT INTO voice_room_instances
 (room_id,room_type,purpose,chat_id,livekit_room_name,state,roster_version,created_at,updated_at)
 VALUES($1,'group_voice','ORDINARY',$2,$3,'active',0,clock_timestamp(),clock_timestamp())`, room, uuid.New(), "foreign-"+room.String())
	require.NoError(t, err)
	return room
}

// Each successful absence here is only a foreign component witness. No issued
// coverage certificate or complete NativeSource activation is manufactured.
func TestForeignNativeComponentRejectsLegacyPendingAndAmbiguousFences(t *testing.T) {
	pool := nativePostgres(t)
	ctx := context.Background()
	t.Run("creation is not membership and legacy NULL is never adopted", func(t *testing.T) {
		subject := testSubject()
		subject.AccountID, subject.ProfileID = uuid.New(), uuid.New()
		room := seedForeignRoom(t, pool)
		before := componentForeignCut(t, pool, subject, Candidate{Disposition: Absent})
		require.False(t, before.Unresolved)
		_, err := pool.Exec(ctx, `INSERT INTO voice_room_memberships
 (profile_id,room_id,media_epoch,space_access_epoch,role_policy_epoch,authorization_digest,
 can_join,can_publish_audio,can_publish_video,can_publish_screen_share,can_subscribe,
 can_mute_others,can_deafen_others,can_move_others,can_use_ptt,priority_speaker,joined_at,updated_at)
 VALUES($1,$2,$3,NULL,NULL,$4,true,true,false,false,true,false,false,false,false,false,clock_timestamp(),clock_timestamp())`, subject.ProfileID, room, uuid.New(), make([]byte, 32))
		require.NoError(t, err)
		legacy := componentForeignCut(t, pool, subject, Candidate{Disposition: Absent})
		require.True(t, legacy.Unresolved, "unknown legacy identity cannot become SOLO or native roster")
		require.NotEqual(t, before.Witness, legacy.Witness)
		_, err = pool.Exec(ctx, `UPDATE voice_room_memberships SET account_id=$2,session_epoch=3,membership_state='LEFT' WHERE profile_id=$1`, subject.ProfileID, subject.AccountID)
		require.NoError(t, err)
		terminal := componentForeignCut(t, pool, subject, Candidate{Disposition: Absent})
		require.False(t, terminal.Unresolved, "retained terminal membership is not a current entitlement")
		require.NotEqual(t, before.Witness, terminal.Witness, "terminal native evidence remains fingerprinted")
	})
	t.Run("pending foreign member operation survives absent membership", func(t *testing.T) {
		subject := testSubject()
		subject.AccountID, subject.ProfileID = uuid.New(), uuid.New()
		room := seedForeignRoom(t, pool)
		_, err := pool.Exec(ctx, `INSERT INTO voice_match_squad_member_operations
 (operation_id,match_id,room_id,profile_id,account_id,session_epoch,media_epoch,method,request_sha256,request_bytes,state,created_at,updated_at)
 VALUES($1,$2,$3,$4,$5,7,$6,'join',$7,$8,'pending',clock_timestamp(),clock_timestamp())`, uuid.New(), uuid.New(), room, subject.ProfileID, subject.AccountID, uuid.New(), make([]byte, 32), []byte{1})
		require.NoError(t, err)
		cut := componentForeignCut(t, pool, subject, Candidate{Disposition: Absent})
		require.True(t, cut.Unresolved)
	})
	t.Run("same RR cut and exact native fence ownership", func(t *testing.T) {
		subject := testSubject()
		subject.AccountID, subject.ProfileID = uuid.New(), uuid.New()
		_, err := pool.Exec(ctx, `INSERT INTO voice_profile_account_mappings(profile_id,account_id) VALUES($1,$2)`, subject.ProfileID, subject.AccountID)
		require.NoError(t, err)
		tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
		require.NoError(t, err)
		defer tx.Rollback(ctx)
		first, err := readForeignCut(ctx, tx, subject, Candidate{Disposition: Absent})
		require.NoError(t, err)
		room, operation, generation := uuid.New(), uuid.New(), uuid.New().String()
		_, err = pool.Exec(ctx, `INSERT INTO voice_account_voice_fences
 (account_id,profile_id,room_id,state,admission_operation_id,admission_generation)
 VALUES($1,$2,$3,'active',$4,$5)`, subject.AccountID, subject.ProfileID, room.String(), operation, generation)
		require.NoError(t, err, "read-only cut must not block the native writer")
		again, err := readForeignCut(ctx, tx, subject, Candidate{Disposition: Absent})
		require.NoError(t, err)
		require.Equal(t, first, again, "concurrent committed fence cannot mix into the pinned cut")
		require.NoError(t, tx.Commit(ctx))
		fresh := componentForeignCut(t, pool, subject, Candidate{Disposition: Absent})
		require.True(t, fresh.Unresolved, "a fence without a native locator is not absence")
		native := Candidate{Disposition: Present, SourceKind: "ORDINARY", RoomID: room, Members: []Member{{AccountID: subject.AccountID, ProfileID: subject.ProfileID, OperationID: operation, Generation: generation, SessionEpoch: 7, State: Joined}}}
		require.False(t, componentForeignCut(t, pool, subject, native).Unresolved)
		preparedAccount, preparedProfile := uuid.New(), uuid.New()
		native.Members = append(native.Members, Member{AccountID: preparedAccount, ProfileID: preparedProfile, OperationID: uuid.New(), Generation: "prepared", SessionEpoch: 8, State: Joining})
		require.False(t, componentForeignCut(t, pool, subject, native).Unresolved, "a prepared peer without a fence is excluded from eligibility, not an ambiguous active member")
		native.Members[1].State = Leaving
		require.False(t, componentForeignCut(t, pool, subject, native).Unresolved, "an already released departing peer must not require a current fence")
		native.Members = native.Members[:1]
		_, err = pool.Exec(ctx, `UPDATE voice_account_voice_fences SET admission_generation='changed' WHERE account_id=$1`, subject.AccountID)
		require.NoError(t, err)
		require.True(t, componentForeignCut(t, pool, subject, native).Unresolved)
		alias := uuid.New()
		_, err = pool.Exec(ctx, `INSERT INTO voice_profile_account_mappings(profile_id,account_id) VALUES($1,$2)`, alias, subject.AccountID)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `UPDATE voice_account_voice_fences SET admission_generation=$2,profile_id=$3 WHERE account_id=$1`, subject.AccountID, generation, alias)
		require.NoError(t, err)
		require.True(t, componentForeignCut(t, pool, subject, native).Unresolved, "another profile on the same account cannot be ignored")
	})
	require.Zero(t, pool.Stat().AcquiredConns())
}
