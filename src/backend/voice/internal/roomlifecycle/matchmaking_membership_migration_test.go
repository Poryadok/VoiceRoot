package roomlifecycle

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

const mmKnownIdentity = `account_id='11111111-1111-1111-1111-111111111111',session_epoch=7,membership_state='JOINED'`

func mmReadMigration(t *testing.T, direction string) string {
	t.Helper()
	path := filepath.Join(r22VoiceRepoRoot(t), "src", "backend", "migrations", "voice_db", "000003_matchmaking_membership."+direction+".sql")
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NotEmpty(t, raw)
	return string(raw)
}

func TestMatchmakingMembershipMigrationFiles(t *testing.T) {
	for _, direction := range []string{"up", "down"} {
		t.Run(direction, func(t *testing.T) { require.NotEmpty(t, mmReadMigration(t, direction)) })
	}
}

func mmRequireRejected(t *testing.T, err error) {
	t.Helper()
	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	require.Contains(t, []string{"23514", "23502", "55000", "P0001"}, pgErr.Code, "rejection must be a constraint/guard, not malformed SQL")
}

func mmMembershipUpdate(ctx context.Context, tx pgx.Tx, profile uuid.UUID, set string) error {
	_, err := tx.Exec(ctx, "UPDATE voice_room_memberships SET "+set+" WHERE profile_id=$1", profile)
	return err
}

func mmInsertRoom(ctx context.Context, tx pgx.Tx, kind string, match bool, override pgx.NamedArgs) (uuid.UUID, error) {
	id := uuid.New()
	args := pgx.NamedArgs{"id": id, "name": "mm-schema-" + id.String(), "kind": kind,
		"purpose": "ORDINARY", "space": nil, "logical": nil, "chat": nil,
		"owner": nil, "operation": nil, "manifest": nil, "receipt": nil, "chat_receipt": nil}
	if kind == "voice_room" {
		args["space"], args["logical"] = uuid.New(), uuid.New()
	} else {
		args["chat"] = uuid.New()
	}
	if match {
		args["purpose"], args["owner"], args["operation"] = "MATCH_SQUAD", uuid.New(), uuid.New()
		args["manifest"], args["receipt"], args["chat_receipt"] = make([]byte, 32), uuid.New(), uuid.New()
	}
	for key, value := range override {
		args[key] = value
	}
	_, err := tx.Exec(ctx, `INSERT INTO voice_room_instances(
room_id,livekit_room_name,room_type,purpose,space_id,voice_room_id,chat_id,
owner_id,creation_operation_id,creation_manifest_hash,creation_receipt_id,
chat_creation_receipt_id,state,roster_version,created_at,updated_at)
VALUES(@id,@name,@kind,@purpose,@space,@logical,@chat,@owner,@operation,
@manifest,@receipt,@chat_receipt,'active',0,now(),now())`, args)
	return id, err
}

func TestMatchmakingMembershipMigrationConstraints(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx := context.Background()
	pool := r22StartVoicePostgres(t, ctx, "mmmembershipconstraints")
	require.NoError(t, NewPostgresLifecycleStore(pool).CheckSchema(ctx))
	_, err := pool.Exec(ctx, mmReadMigration(t, "up"))
	require.NoError(t, err)
	require.NoError(t, NewPostgresLifecycleStore(pool).CheckSchema(ctx))
	withMember := func(t *testing.T, check func(pgx.Tx, r22MigrationRows)) {
		t.Helper()
		r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
			seed, seedErr := r22TrySeedMigrationRows(ctx, tx, nil)
			require.NoError(t, seedErr, "legacy writers remain supported")
			check(tx, seed)
		})
	}
	t.Run("exact binding and reconnect updates remain valid", func(t *testing.T) {
		withMember(t, func(tx pgx.Tx, seed r22MigrationRows) {
			require.NoError(t, mmMembershipUpdate(ctx, tx, seed.membershipID, mmKnownIdentity))
			require.NoError(t, mmMembershipUpdate(ctx, tx, seed.membershipID, "membership_state='RECONNECTING',reconnect_started_at=now(),reconnect_deadline=now()+interval '30 seconds'"))
			require.NoError(t, mmMembershipUpdate(ctx, tx, seed.membershipID, "membership_state=membership_state,reconnect_started_at=reconnect_started_at,reconnect_deadline=reconnect_deadline"))
			id, err := mmInsertRoom(ctx, tx, "group_voice", true, nil)
			require.NoError(t, err)
			_, err = tx.Exec(ctx, `UPDATE voice_room_instances SET purpose=purpose,chat_id=chat_id,owner_id=owner_id,creation_operation_id=creation_operation_id,creation_manifest_hash=creation_manifest_hash,creation_receipt_id=creation_receipt_id,chat_creation_receipt_id=chat_creation_receipt_id,roster_version=roster_version+1 WHERE room_id=$1`, id)
			require.NoError(t, err)
		})
	})
	t.Run("parent room cannot change kind behind membership epochs", func(t *testing.T) {
		withMember(t, func(tx pgx.Tx, seed r22MigrationRows) {
			_, err := tx.Exec(ctx, `UPDATE voice_room_instances SET room_type='call',space_id=NULL,voice_room_id=NULL,chat_id=$1 WHERE room_id=$2`, uuid.New(), seed.destinationRoomID)
			mmRequireRejected(t, err)
		})
	})
	t.Run("legacy identity remains entirely unknown", func(t *testing.T) {
		withMember(t, func(tx pgx.Tx, seed r22MigrationRows) {
			var unknown bool
			require.NoError(t, tx.QueryRow(ctx, `SELECT account_id IS NULL AND session_epoch IS NULL AND membership_state IS NULL
AND reconnect_started_at IS NULL AND reconnect_deadline IS NULL
FROM voice_room_memberships WHERE profile_id=$1`, seed.membershipID).Scan(&unknown))
			require.True(t, unknown)
		})
	})
	for _, state := range []string{"JOINING", "JOINED", "RECONNECTING", "LEAVING", "LEFT", "EJECTED"} {
		t.Run("accepts "+state, func(t *testing.T) {
			withMember(t, func(tx pgx.Tx, seed r22MigrationRows) {
				set := "account_id='11111111-1111-1111-1111-111111111111',session_epoch=7,membership_state='" + state + "'"
				if state == "RECONNECTING" {
					set += ",reconnect_started_at=now(),reconnect_deadline=now()+interval '30 seconds'"
				}
				require.NoError(t, mmMembershipUpdate(ctx, tx, seed.membershipID, set))
				var got string
				require.NoError(t, tx.QueryRow(ctx, "SELECT membership_state FROM voice_room_memberships WHERE profile_id=$1", seed.membershipID).Scan(&got))
				require.Equal(t, state, got)
			})
		})
	}
	for name, set := range map[string]string{
		"account alone": "account_id='11111111-1111-1111-1111-111111111111'",
		"epoch alone":   "session_epoch=7", "state alone": "membership_state='JOINED'",
		"identity without state": "account_id='11111111-1111-1111-1111-111111111111',session_epoch=7",
		"nil account":            "account_id='00000000-0000-0000-0000-000000000000',session_epoch=7,membership_state='JOINED'",
		"zero epoch":             "account_id='11111111-1111-1111-1111-111111111111',session_epoch=0,membership_state='JOINED'",
		"negative epoch":         "account_id='11111111-1111-1111-1111-111111111111',session_epoch=-1,membership_state='JOINED'",
		"unknown state":          "account_id='11111111-1111-1111-1111-111111111111',session_epoch=7,membership_state='SOLO'",
		"legacy reconnect":       "reconnect_started_at=now(),reconnect_deadline=now()+interval '1 second'",
		"Space missing epochs":   "space_access_epoch=NULL,role_policy_epoch=NULL", "Space missing one epoch": "role_policy_epoch=NULL",
	} {
		t.Run("rejects "+name, func(t *testing.T) {
			withMember(t, func(tx pgx.Tx, seed r22MigrationRows) {
				mmRequireRejected(t, mmMembershipUpdate(ctx, tx, seed.membershipID, set))
			})
		})
	}
	for name, set := range map[string]string{
		"clear known":                     "account_id=NULL,session_epoch=NULL,membership_state=NULL",
		"change account":                  "account_id='22222222-2222-2222-2222-222222222222'",
		"regress session":                 "session_epoch=6,media_epoch='22222222-2222-2222-2222-222222222222'",
		"change session under same media": "session_epoch=8",
	} {
		t.Run("guards "+name, func(t *testing.T) {
			withMember(t, func(tx pgx.Tx, seed r22MigrationRows) {
				require.NoError(t, mmMembershipUpdate(ctx, tx, seed.membershipID, mmKnownIdentity))
				mmRequireRejected(t, mmMembershipUpdate(ctx, tx, seed.membershipID, set))
			})
		})
	}
	t.Run("new media epoch permits newer authenticated session identity", func(t *testing.T) {
		withMember(t, func(tx pgx.Tx, seed r22MigrationRows) {
			require.NoError(t, mmMembershipUpdate(ctx, tx, seed.membershipID, mmKnownIdentity))
			require.NoError(t, mmMembershipUpdate(ctx, tx, seed.membershipID, "session_epoch=8,media_epoch='22222222-2222-2222-2222-222222222222'"))
		})
	})
	for _, interval := range []string{"0 seconds", "-1 second", "30.000001 seconds"} {
		t.Run("rejects reconnect interval "+interval, func(t *testing.T) {
			withMember(t, func(tx pgx.Tx, seed r22MigrationRows) {
				require.NoError(t, mmMembershipUpdate(ctx, tx, seed.membershipID, mmKnownIdentity))
				mmRequireRejected(t, mmMembershipUpdate(ctx, tx, seed.membershipID, "membership_state='RECONNECTING',reconnect_started_at=now(),reconnect_deadline=now()+interval '"+interval+"'"))
			})
		})
	}
	for name, set := range map[string]string{
		"missing both": "membership_state='RECONNECTING'", "missing start": "membership_state='RECONNECTING',reconnect_deadline=now()",
		"missing deadline":   "membership_state='RECONNECTING',reconnect_started_at=now()",
		"interval on joined": "reconnect_started_at=now(),reconnect_deadline=now()+interval '1 second'",
	} {
		t.Run("rejects reconnect "+name, func(t *testing.T) {
			withMember(t, func(tx pgx.Tx, seed r22MigrationRows) {
				require.NoError(t, mmMembershipUpdate(ctx, tx, seed.membershipID, mmKnownIdentity))
				mmRequireRejected(t, mmMembershipUpdate(ctx, tx, seed.membershipID, set))
			})
		})
	}
	for _, set := range []string{"reconnect_deadline=reconnect_deadline+interval '1 second'", "reconnect_started_at=reconnect_started_at+interval '1 second'", "reconnect_started_at=reconnect_started_at+interval '1 second',reconnect_deadline=reconnect_deadline+interval '1 second'"} {
		t.Run("cannot move reconnect window "+set, func(t *testing.T) {
			withMember(t, func(tx pgx.Tx, seed r22MigrationRows) {
				require.NoError(t, mmMembershipUpdate(ctx, tx, seed.membershipID, mmKnownIdentity))
				require.NoError(t, mmMembershipUpdate(ctx, tx, seed.membershipID, "membership_state='RECONNECTING',reconnect_started_at=now(),reconnect_deadline=now()+interval '10 seconds'"))
				mmRequireRejected(t, mmMembershipUpdate(ctx, tx, seed.membershipID, set))
			})
		})
	}
	for _, kind := range []string{"voice_room", "call", "group_voice"} {
		t.Run("valid room "+kind, func(t *testing.T) {
			withMember(t, func(tx pgx.Tx, seed r22MigrationRows) {
				id, insertErr := mmInsertRoom(ctx, tx, kind, false, nil)
				require.NoError(t, insertErr)
				if kind != "voice_room" {
					_, moveErr := tx.Exec(ctx, `UPDATE voice_room_memberships SET room_id=$1,space_access_epoch=NULL,role_policy_epoch=NULL WHERE profile_id=$2`, id, seed.membershipID)
					require.NoError(t, moveErr)
					mmRequireRejected(t, mmMembershipUpdate(ctx, tx, seed.membershipID, "space_access_epoch=1,role_policy_epoch=1"))
				}
			})
		})
	}
	for _, tc := range []struct {
		name, kind string
		match      bool
		override   pgx.NamedArgs
	}{
		{"unknown kind", "invalid", false, nil}, {"Space without logical", "voice_room", false, pgx.NamedArgs{"logical": nil}},
		{"Space with chat", "voice_room", false, pgx.NamedArgs{"chat": uuid.New()}}, {"call without chat", "call", false, pgx.NamedArgs{"chat": nil}},
		{"group with Space", "group_voice", false, pgx.NamedArgs{"space": uuid.New()}}, {"nil chat", "group_voice", false, pgx.NamedArgs{"chat": uuid.Nil}},
		{"ordinary ownership", "group_voice", false, pgx.NamedArgs{"owner": uuid.New()}}, {"unknown purpose", "group_voice", false, pgx.NamedArgs{"purpose": "OTHER"}},
		{"match call", "call", true, nil}, {"match without owner", "group_voice", true, pgx.NamedArgs{"owner": nil}},
		{"match without operation", "group_voice", true, pgx.NamedArgs{"operation": nil}}, {"match without manifest", "group_voice", true, pgx.NamedArgs{"manifest": nil}},
		{"match short manifest", "group_voice", true, pgx.NamedArgs{"manifest": make([]byte, 31)}}, {"match without receipt", "group_voice", true, pgx.NamedArgs{"receipt": nil}},
		{"match without Chat receipt", "group_voice", true, pgx.NamedArgs{"chat_receipt": nil}},
	} {
		t.Run("rejects room "+tc.name, func(t *testing.T) {
			r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
				_, insertErr := mmInsertRoom(ctx, tx, tc.kind, tc.match, tc.override)
				mmRequireRejected(t, insertErr)
			})
		})
	}
	for _, column := range []string{"chat_id", "owner_id", "creation_operation_id", "creation_receipt_id", "chat_creation_receipt_id", "creation_manifest_hash"} {
		t.Run("match immutable "+column, func(t *testing.T) {
			r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
				id, insertErr := mmInsertRoom(ctx, tx, "group_voice", true, nil)
				require.NoError(t, insertErr)
				var replacement any = uuid.New()
				if column == "creation_manifest_hash" {
					replacement = []byte("11111111111111111111111111111111")
				}
				_, updateErr := tx.Exec(ctx, "UPDATE voice_room_instances SET "+column+"=$1 WHERE room_id=$2", replacement, id)
				mmRequireRejected(t, updateErr)
			})
		})
	}
	t.Run("cannot convert ordinary room to match squad", func(t *testing.T) {
		r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
			id, insertErr := mmInsertRoom(ctx, tx, "group_voice", false, nil)
			require.NoError(t, insertErr)
			_, updateErr := tx.Exec(ctx, `UPDATE voice_room_instances SET purpose='MATCH_SQUAD',owner_id=$2,creation_operation_id=$3,creation_manifest_hash=$4,creation_receipt_id=$5,chat_creation_receipt_id=$6 WHERE room_id=$1`, id, uuid.New(), uuid.New(), make([]byte, 32), uuid.New(), uuid.New())
			mmRequireRejected(t, updateErr)
		})
	})
}

func TestMatchmakingMembershipMigrationRollback(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	for _, scenario := range []string{"legacy", "identity", "call", "match"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			pool := r22StartVoicePostgres(t, ctx, "mmdown"+scenario)
			tx, err := pool.Begin(ctx)
			require.NoError(t, err)
			seed, err := r22TrySeedMigrationRows(ctx, tx, nil)
			require.NoError(t, err)
			require.NoError(t, tx.Commit(ctx))
			before := r22TableRowsSnapshot(t, ctx, pool, "voice_room_memberships")
			_, err = pool.Exec(ctx, mmReadMigration(t, "up"))
			require.NoError(t, err)
			if scenario == "identity" {
				_, err = pool.Exec(ctx, "UPDATE voice_room_memberships SET "+mmKnownIdentity+" WHERE profile_id=$1", seed.membershipID)
				require.NoError(t, err)
			}
			if scenario == "call" || scenario == "match" {
				tx, err = pool.Begin(ctx)
				require.NoError(t, err)
				kind := "call"
				if scenario == "match" {
					kind = "group_voice"
				}
				_, err = mmInsertRoom(ctx, tx, kind, scenario == "match", nil)
				require.NoError(t, err)
				require.NoError(t, tx.Commit(ctx))
			}
			if scenario == "legacy" {
				_, err = pool.Exec(ctx, mmReadMigration(t, "down"))
				require.NoError(t, err)
				require.Equal(t, before, r22TableRowsSnapshot(t, ctx, pool, "voice_room_memberships"))
				require.NoError(t, NewPostgresLifecycleStore(pool).CheckSchema(ctx))
				return
			}
			schema := r22VoiceSchemaSnapshot(t, ctx, pool)
			members, rooms := r22TableRowsSnapshot(t, ctx, pool, "voice_room_memberships"), r22TableRowsSnapshot(t, ctx, pool, "voice_room_instances")
			r22ExecuteDownRefusal(t, ctx, pool, mmReadMigration(t, "down"), true)
			require.Equal(t, schema, r22VoiceSchemaSnapshot(t, ctx, pool))
			require.Equal(t, members, r22TableRowsSnapshot(t, ctx, pool, "voice_room_memberships"))
			require.Equal(t, rooms, r22TableRowsSnapshot(t, ctx, pool, "voice_room_instances"))
		})
	}
}
