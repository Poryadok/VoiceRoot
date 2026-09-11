package roomlifecycle

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

var r22VoiceTables = []string{
	"voice_event_outbox",
	"voice_lifecycle_effects",
	"voice_lifecycle_operations",
	"voice_media_epoch_denials",
	"voice_room_instances",
	"voice_room_memberships",
}

var r22ValidGrants = [10]bool{true, false, true, false, true, false, true, false, true, false}

type r22MigrationRows struct {
	spaceID                  uuid.UUID
	sourceRoomID             uuid.UUID
	destinationRoomID        uuid.UUID
	sourceLogicalRoomID      uuid.UUID
	destinationLogicalRoomID uuid.UUID
	membershipID             uuid.UUID
	membershipEpoch          uuid.UUID
	actorID                  uuid.UUID
	operationID              uuid.UUID
}

type r22RowMutation struct {
	table  string
	column string
	value  any
}

func (m *r22RowMutation) valueFor(table, column string, valid any) any {
	if m != nil && m.table == table && m.column == column {
		return m.value
	}
	return valid
}

func TestVoiceDBMigration_A01_FilesAndExactTables(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL migration contract requires testcontainers")
	}
	ctx := context.Background()
	require.NotEmpty(t, r22ReadVoiceMigration(t, "up"))
	require.NotEmpty(t, r22ReadVoiceMigration(t, "down"))
	pool := r22StartVoicePostgres(t, ctx, "r22voicea01")

	rows, err := pool.Query(ctx, `
SELECT tablename
FROM pg_tables
WHERE schemaname = current_schema()
ORDER BY tablename`)
	require.NoError(t, err)
	defer rows.Close()

	var got []string
	for rows.Next() {
		var table string
		require.NoError(t, rows.Scan(&table))
		got = append(got, table)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, r22VoiceTables, got)
}

func TestVoiceDBMigration_A02_A09_CoreConstraints(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL migration contract requires testcontainers")
	}
	ctx := context.Background()
	pool := r22StartVoicePostgres(t, ctx, "r22voicea02a09")

	t.Run("A02 every UUID rejects nil and required identities reject null", func(t *testing.T) {
		required := map[string]map[string]bool{
			"voice_room_instances":       {"room_id": true, "space_id": true, "voice_room_id": true},
			"voice_room_memberships":     {"profile_id": true, "room_id": true, "media_epoch": true},
			"voice_lifecycle_operations": {"actor_profile_id": true, "operation_id": true, "actor_account_id": true, "subject_profile_id": true, "space_id": true},
			"voice_lifecycle_effects":    {"effect_id": true, "actor_profile_id": true, "operation_id": true, "target_voice_room_id": true},
			"voice_media_epoch_denials":  {"media_epoch": true, "profile_id": true, "room_id": true},
			"voice_event_outbox":         {"event_id": true, "actor_profile_id": true, "operation_id": true, "subject_profile_id": true, "room_id": true, "voice_room_id": true, "space_id": true},
		}

		type uuidColumn struct {
			table, column, nullable string
		}
		rows, err := pool.Query(ctx, `
SELECT table_name,column_name,is_nullable
FROM information_schema.columns
WHERE table_schema=current_schema() AND table_name=ANY($1) AND udt_name='uuid'
ORDER BY table_name,ordinal_position`, r22VoiceTables)
		require.NoError(t, err)
		var columns []uuidColumn
		for rows.Next() {
			var column uuidColumn
			require.NoError(t, rows.Scan(&column.table, &column.column, &column.nullable))
			columns = append(columns, column)
		}
		require.NoError(t, rows.Err())
		rows.Close()
		require.NotEmpty(t, columns)

		seen := make(map[string]map[string]bool)
		for _, column := range columns {
			if seen[column.table] == nil {
				seen[column.table] = make(map[string]bool)
			}
			seen[column.table][column.column] = true
			definitions := r22ConstraintDefinitions(t, ctx, pool, column.table)
			r22RequireColumnCheck(t, definitions, column.column, "00000000-0000-0000-0000-000000000000")
			if column.column == "lease_owner" && (column.table == "voice_lifecycle_operations" || column.table == "voice_lifecycle_effects" || column.table == "voice_event_outbox") {
				continue
			}

			t.Run(column.table+"."+column.column+" rejects nil UUID", func(t *testing.T) {
				r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
					_, seedErr := r22TrySeedMigrationRows(ctx, tx, nil)
					require.NoError(t, seedErr, "control fixture must be valid")
					_, writeErr := r22TrySeedMigrationRows(ctx, tx, &r22RowMutation{column.table, column.column, uuid.Nil})
					r22RequireSQLState(t, writeErr, "23514")
				})
			})

			if required[column.table][column.column] {
				require.Equal(t, "NO", column.nullable, "%s.%s is a required identity", column.table, column.column)
				t.Run(column.table+"."+column.column+" rejects null", func(t *testing.T) {
					r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
						_, seedErr := r22TrySeedMigrationRows(ctx, tx, nil)
						require.NoError(t, seedErr, "control fixture must be valid")
						_, writeErr := r22TrySeedMigrationRows(ctx, tx, &r22RowMutation{column.table, column.column, nil})
						r22RequireSQLState(t, writeErr, "23502")
					})
				})
			}
		}
		for table, requiredColumns := range required {
			for column := range requiredColumns {
				require.True(t, seen[table][column], "missing required UUID column %s.%s", table, column)
			}
		}
		for _, table := range []string{"voice_lifecycle_operations", "voice_lifecycle_effects", "voice_event_outbox"} {
			t.Run(table+".lease_owner rejects nil on an otherwise valid lease", func(t *testing.T) {
				r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
					seed, seedErr := r22TrySeedMigrationRows(ctx, tx, nil)
					require.NoError(t, seedErr, "control fixture must be valid")
					validOwner := uuid.New()
					require.NoError(t, r22InsertValidLeasedRow(ctx, tx, table, seed, validOwner))
					command := fmt.Sprintf("UPDATE %s SET lease_owner=$1 WHERE lease_owner=$2", pgx.Identifier{table}.Sanitize())
					_, writeErr := tx.Exec(ctx, command, uuid.Nil, validOwner)
					r22RequireSQLState(t, writeErr, "23514")
				})
			})
		}
	})

	t.Run("A03 every epoch and roster boundary rejects zero and negative", func(t *testing.T) {
		positive := map[string][]string{
			"voice_room_memberships": {"space_access_epoch", "role_policy_epoch"},
			"voice_lifecycle_operations": {
				"space_access_epoch", "subject_role_policy_epoch",
				"actor_source_role_policy_epoch", "actor_destination_role_policy_epoch",
				"receipt_space_access_epoch", "receipt_role_policy_epoch",
			},
		}
		nonnegative := map[string][]string{
			"voice_room_instances":       {"roster_version"},
			"voice_lifecycle_operations": {"receipt_source_roster_version", "receipt_destination_roster_version"},
			"voice_event_outbox":         {"roster_version"},
		}
		for table, columns := range positive {
			definitions := r22ConstraintDefinitions(t, ctx, pool, table)
			for _, column := range columns {
				r22RequireColumnCheck(t, definitions, column, column+">0")
				for _, invalid := range []int64{0, -1} {
					t.Run(fmt.Sprintf("%s.%s rejects %d", table, column, invalid), func(t *testing.T) {
						r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
							_, seedErr := r22TrySeedMigrationRows(ctx, tx, nil)
							require.NoError(t, seedErr, "control fixture must be valid")
							_, writeErr := r22TrySeedMigrationRows(ctx, tx, &r22RowMutation{table, column, invalid})
							r22RequireSQLState(t, writeErr, "23514")
						})
					})
				}
			}
		}
		for table, columns := range nonnegative {
			definitions := r22ConstraintDefinitions(t, ctx, pool, table)
			for _, column := range columns {
				r22RequireColumnCheck(t, definitions, column, column+">=0")
				t.Run(fmt.Sprintf("%s.%s accepts zero", table, column), func(t *testing.T) {
					r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
						_, writeErr := r22TrySeedMigrationRows(ctx, tx, &r22RowMutation{table, column, int64(0)})
						require.NoError(t, writeErr)
					})
				})
				for _, invalid := range []int64{-1, -2} {
					t.Run(fmt.Sprintf("%s.%s rejects %d", table, column, invalid), func(t *testing.T) {
						r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
							_, seedErr := r22TrySeedMigrationRows(ctx, tx, nil)
							require.NoError(t, seedErr, "control fixture must be valid")
							_, writeErr := r22TrySeedMigrationRows(ctx, tx, &r22RowMutation{table, column, invalid})
							r22RequireSQLState(t, writeErr, "23514")
						})
					})
				}
			}
		}
	})

	t.Run("A04 every hash digest and owner token is exactly 32 bytes", func(t *testing.T) {
		exact32 := map[string][]string{
			"voice_room_memberships":     {"authorization_digest"},
			"voice_lifecycle_operations": {"fingerprint", "binding_hash", "authorization_digest", "redis_owner_token", "receipt_authorization_digest", "receipt_hash"},
			"voice_lifecycle_effects":    {"request_digest"},
			"voice_event_outbox":         {"payload_hash"},
		}
		for table, columns := range exact32 {
			definitions := r22ConstraintDefinitions(t, ctx, pool, table)
			for _, column := range columns {
				r22RequireColumnCheck(t, definitions, column, "octet_length("+column+")=32")
				for _, length := range []int{0, 31, 33} {
					t.Run(fmt.Sprintf("%s.%s rejects %d bytes", table, column, length), func(t *testing.T) {
						r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
							_, seedErr := r22TrySeedMigrationRows(ctx, tx, nil)
							require.NoError(t, seedErr, "control fixture must be valid")
							_, writeErr := r22TrySeedMigrationRows(ctx, tx, &r22RowMutation{table, column, make([]byte, length)})
							r22RequireSQLState(t, writeErr, "23514")
						})
					})
				}
			}
		}
	})

	t.Run("A05 room state and closed timestamp are equivalent", func(t *testing.T) {
		for _, tc := range []struct {
			name     string
			state    string
			closedAt any
		}{
			{"unknown state", "unknown", nil},
			{"active with closed timestamp", "active", time.Now().UTC()},
			{"closed without closed timestamp", "closed", nil},
		} {
			t.Run(tc.name, func(t *testing.T) {
				_, err := pool.Exec(ctx, `
INSERT INTO voice_room_instances(
  room_id,space_id,voice_room_id,livekit_room_name,state,roster_version,
  created_at,updated_at,closed_at
) VALUES($1,$2,$3,$4,$5,0,now(),now(),$6)`,
					uuid.New(), uuid.New(), uuid.New(), "r22-a05-"+uuid.NewString(), tc.state, tc.closedAt)
				r22RequireSQLState(t, err, "23514")
			})
		}
	})

	t.Run("A06 room runtime uniqueness allows historical closed instances", func(t *testing.T) {
		spaceID, logicalRoomID := uuid.New(), uuid.New()
		firstRoomID := r22InsertRoom(t, ctx, pool, spaceID, logicalRoomID, "r22-a06-livekit", "active", 0, nil)
		_, err := pool.Exec(ctx, `
INSERT INTO voice_room_instances(room_id,space_id,voice_room_id,livekit_room_name,state,roster_version,created_at,updated_at)
VALUES($1,$2,$3,$4,'active',0,now(),now())`, uuid.New(), spaceID, uuid.New(), "r22-a06-livekit")
		r22RequireSQLState(t, err, "23505")
		_, err = pool.Exec(ctx, `
INSERT INTO voice_room_instances(room_id,space_id,voice_room_id,livekit_room_name,state,roster_version,created_at,updated_at)
VALUES($1,$2,$3,$4,'active',0,now(),now())`, uuid.New(), spaceID, logicalRoomID, "r22-a06-second-active")
		r22RequireSQLState(t, err, "23505")

		closedAt := time.Now().UTC()
		_, err = pool.Exec(ctx, `UPDATE voice_room_instances SET state='closed',closed_at=$2,updated_at=$2 WHERE room_id=$1`, firstRoomID, closedAt)
		require.NoError(t, err)
		r22InsertRoom(t, ctx, pool, spaceID, logicalRoomID, "r22-a06-history-2", "closed", 0, closedAt)
		r22InsertRoom(t, ctx, pool, spaceID, logicalRoomID, "r22-a06-new-active", "active", 0, nil)
	})

	t.Run("A07 profile PK and media epoch uniqueness reject valid duplicates", func(t *testing.T) {
		rows, err := pool.Query(ctx, `
SELECT c.contype::text,string_agg(a.attname,',' ORDER BY key.ord)
FROM pg_constraint c
JOIN pg_class r ON r.oid=c.conrelid
JOIN pg_namespace n ON n.oid=r.relnamespace
JOIN unnest(c.conkey) WITH ORDINALITY AS key(attnum,ord) ON true
JOIN pg_attribute a ON a.attrelid=r.oid AND a.attnum=key.attnum
WHERE n.nspname=current_schema() AND r.relname='voice_room_memberships' AND c.contype IN ('p','u')
GROUP BY c.oid,c.contype
ORDER BY c.contype`)
		require.NoError(t, err)
		var keys []string
		for rows.Next() {
			var kind, columns string
			require.NoError(t, rows.Scan(&kind, &columns))
			keys = append(keys, kind+":"+columns)
		}
		require.NoError(t, rows.Err())
		rows.Close()
		require.Equal(t, []string{"p:profile_id", "u:media_epoch"}, keys)

		spaceID := uuid.New()
		roomA := r22InsertRoom(t, ctx, pool, spaceID, uuid.New(), "r22-a07-a", "active", 0, nil)
		roomB := r22InsertRoom(t, ctx, pool, spaceID, uuid.New(), "r22-a07-b", "active", 0, nil)
		profileID, mediaEpoch := uuid.New(), uuid.New()
		r22InsertMembership(t, ctx, pool, profileID, roomA, mediaEpoch, r22ValidGrants)
		r22RequireSQLState(t, r22TryInsertMembership(ctx, pool, profileID, roomB, uuid.New(), r22ValidGrants), "23505")
		r22RequireSQLState(t, r22TryInsertMembership(ctx, pool, uuid.New(), roomB, mediaEpoch, r22ValidGrants), "23505")
	})

	t.Run("A08 all foreign keys are exact Voice owned restricted edges", func(t *testing.T) {
		r22RequireSQLState(t, r22TryInsertMembership(ctx, pool, uuid.New(), uuid.New(), uuid.New(), r22ValidGrants), "23503")

		rows, err := pool.Query(ctx, `
SELECT r.relname,
       string_agg(a.attname,',' ORDER BY source_key.ord),
       referenced.relname,
       string_agg(referenced_attribute.attname,',' ORDER BY source_key.ord),
       c.confmatchtype::text,c.confdeltype::text
FROM pg_constraint c
JOIN pg_class r ON r.oid=c.conrelid
JOIN pg_namespace n ON n.oid=r.relnamespace
JOIN pg_class referenced ON referenced.oid=c.confrelid
JOIN unnest(c.conkey) WITH ORDINALITY AS source_key(attnum,ord) ON true
JOIN unnest(c.confkey) WITH ORDINALITY AS target_key(attnum,ord) ON target_key.ord=source_key.ord
JOIN pg_attribute a ON a.attrelid=r.oid AND a.attnum=source_key.attnum
JOIN pg_attribute referenced_attribute ON referenced_attribute.attrelid=referenced.oid AND referenced_attribute.attnum=target_key.attnum
WHERE n.nspname=current_schema() AND r.relname=ANY($1) AND c.contype='f'
GROUP BY c.oid,r.relname,referenced.relname,c.confmatchtype,c.confdeltype
ORDER BY r.relname,2,referenced.relname`, r22VoiceTables)
		require.NoError(t, err)
		var edges []string
		for rows.Next() {
			var sourceTable, sourceColumns, targetTable, targetColumns, matchType, deleteAction string
			require.NoError(t, rows.Scan(&sourceTable, &sourceColumns, &targetTable, &targetColumns, &matchType, &deleteAction))
			edges = append(edges, fmt.Sprintf("%s(%s)->%s(%s):%s:%s", sourceTable, sourceColumns, targetTable, targetColumns, matchType, deleteAction))
		}
		require.NoError(t, rows.Err())
		rows.Close()
		require.Equal(t, []string{
			"voice_event_outbox(actor_profile_id,operation_id)->voice_lifecycle_operations(actor_profile_id,operation_id):s:r",
			"voice_event_outbox(room_id)->voice_room_instances(room_id):s:r",
			"voice_lifecycle_effects(actor_profile_id,operation_id)->voice_lifecycle_operations(actor_profile_id,operation_id):s:r",
			"voice_lifecycle_operations(destination_room_id)->voice_room_instances(room_id):s:r",
			"voice_lifecycle_operations(source_room_id)->voice_room_instances(room_id):s:r",
			"voice_media_epoch_denials(operation_actor_profile_id,operation_id)->voice_lifecycle_operations(actor_profile_id,operation_id):f:r",
			"voice_media_epoch_denials(room_id)->voice_room_instances(room_id):s:r",
			"voice_room_memberships(room_id)->voice_room_instances(room_id):s:r",
		}, edges, "the exact edge set proves cross-service IDs have no foreign keys")
	})

	t.Run("A09 all ten explicit grants are required and round trip", func(t *testing.T) {
		grantColumns := []string{
			"can_join", "can_publish_audio", "can_publish_video",
			"can_publish_screen_share", "can_subscribe", "can_mute_others",
			"can_deafen_others", "can_move_others", "can_use_ptt", "priority_speaker",
		}
		rows, err := pool.Query(ctx, `
SELECT column_name,is_nullable
FROM information_schema.columns
WHERE table_schema=current_schema() AND table_name='voice_room_memberships'
  AND column_name=ANY($1)
ORDER BY ordinal_position`, grantColumns)
		require.NoError(t, err)
		var gotColumns []string
		for rows.Next() {
			var column, nullable string
			require.NoError(t, rows.Scan(&column, &nullable))
			require.Equal(t, "NO", nullable, "%s must be NOT NULL", column)
			gotColumns = append(gotColumns, column)
		}
		require.NoError(t, rows.Err())
		rows.Close()
		require.ElementsMatch(t, grantColumns, gotColumns)

		spaceID := uuid.New()
		roomID := r22InsertRoom(t, ctx, pool, spaceID, uuid.New(), "r22-a09", "active", 0, nil)
		profileID := uuid.New()
		r22InsertMembership(t, ctx, pool, profileID, roomID, uuid.New(), r22ValidGrants)
		var got [10]bool
		err = pool.QueryRow(ctx, `
SELECT can_join,can_publish_audio,can_publish_video,can_publish_screen_share,
       can_subscribe,can_mute_others,can_deafen_others,can_move_others,
       can_use_ptt,priority_speaker
FROM voice_room_memberships WHERE profile_id=$1`, profileID).Scan(
			&got[0], &got[1], &got[2], &got[3], &got[4],
			&got[5], &got[6], &got[7], &got[8], &got[9],
		)
		require.NoError(t, err)
		require.Equal(t, r22ValidGrants, got)
	})
}

func r22WithMigrationTx(t *testing.T, ctx context.Context, pool *pgxpool.Pool, check func(pgx.Tx)) {
	t.Helper()
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, tx.Rollback(cleanupCtx))
	}()
	check(tx)
}

func r22TrySeedMigrationRows(ctx context.Context, tx pgx.Tx, mutation *r22RowMutation) (r22MigrationRows, error) {
	now := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)
	seed := r22MigrationRows{
		spaceID:                  uuid.New(),
		sourceRoomID:             uuid.New(),
		destinationRoomID:        uuid.New(),
		sourceLogicalRoomID:      uuid.New(),
		destinationLogicalRoomID: uuid.New(),
		membershipID:             uuid.New(),
		membershipEpoch:          uuid.New(),
		actorID:                  uuid.New(),
		operationID:              uuid.New(),
	}
	spaceID := seed.spaceID
	insertRoom := func(roomID, logicalRoomID uuid.UUID, suffix string) error {
		_, err := tx.Exec(ctx, `
INSERT INTO voice_room_instances(room_id,space_id,voice_room_id,livekit_room_name,state,roster_version,created_at,updated_at)
VALUES(@room_id,@space_id,@voice_room_id,@livekit_room_name,'active',@roster_version,@now,@now)`, pgx.NamedArgs{
			"room_id":           mutation.valueFor("voice_room_instances", "room_id", roomID),
			"space_id":          mutation.valueFor("voice_room_instances", "space_id", spaceID),
			"voice_room_id":     mutation.valueFor("voice_room_instances", "voice_room_id", logicalRoomID),
			"livekit_room_name": "r22-seed-" + suffix + "-" + roomID.String(),
			"roster_version":    mutation.valueFor("voice_room_instances", "roster_version", int64(7)),
			"now":               now,
		})
		return err
	}
	if err := insertRoom(seed.sourceRoomID, seed.sourceLogicalRoomID, "source"); err != nil {
		return seed, err
	}
	if err := insertRoom(seed.destinationRoomID, seed.destinationLogicalRoomID, "destination"); err != nil {
		return seed, err
	}

	_, err := tx.Exec(ctx, `
INSERT INTO voice_room_memberships(
  profile_id,room_id,media_epoch,space_access_epoch,role_policy_epoch,authorization_digest,
  can_join,can_publish_audio,can_publish_video,can_publish_screen_share,can_subscribe,
  can_mute_others,can_deafen_others,can_move_others,can_use_ptt,priority_speaker,
  latest_grant_expires_at,joined_at,updated_at
) VALUES(
  @profile_id,@room_id,@media_epoch,@space_epoch,@role_epoch,@auth_digest,
  @g0,@g1,@g2,@g3,@g4,@g5,@g6,@g7,@g8,@g9,@grant_expiry,@now,@now
)`, pgx.NamedArgs{
		"profile_id":  mutation.valueFor("voice_room_memberships", "profile_id", seed.membershipID),
		"room_id":     mutation.valueFor("voice_room_memberships", "room_id", seed.destinationRoomID),
		"media_epoch": mutation.valueFor("voice_room_memberships", "media_epoch", seed.membershipEpoch),
		"space_epoch": mutation.valueFor("voice_room_memberships", "space_access_epoch", int64(11)),
		"role_epoch":  mutation.valueFor("voice_room_memberships", "role_policy_epoch", int64(12)),
		"auth_digest": mutation.valueFor("voice_room_memberships", "authorization_digest", bytesOfLength(32)),
		"g0":          r22ValidGrants[0], "g1": r22ValidGrants[1], "g2": r22ValidGrants[2], "g3": r22ValidGrants[3], "g4": r22ValidGrants[4],
		"g5": r22ValidGrants[5], "g6": r22ValidGrants[6], "g7": r22ValidGrants[7], "g8": r22ValidGrants[8], "g9": r22ValidGrants[9],
		"grant_expiry": now.Add(time.Hour), "now": now,
	})
	if err != nil {
		return seed, err
	}

	subjectID, sourceEpoch, destinationEpoch := uuid.New(), uuid.New(), seed.membershipEpoch
	_, err = tx.Exec(ctx, `
INSERT INTO voice_lifecycle_operations(
  actor_profile_id,operation_id,schema_version,method,fingerprint,binding_bytes,binding_hash,
  actor_account_id,subject_profile_id,space_id,source_voice_room_id,destination_voice_room_id,
  source_room_id,destination_room_id,source_media_epoch,destination_media_epoch,
  space_access_epoch,subject_role_policy_epoch,actor_source_role_policy_epoch,
  actor_destination_role_policy_epoch,authorization_digest,
  can_join,can_publish_audio,can_publish_video,can_publish_screen_share,can_subscribe,
  can_mute_others,can_deafen_others,can_move_others,can_use_ptt,priority_speaker,
  actor_can_move_source,actor_can_move_destination,decided_at,created_at,updated_at,
  redis_owner_token,state,lease_owner,lease_fence,receipt_outcome,
  receipt_source_voice_room_id,receipt_destination_voice_room_id,receipt_room_id,
  receipt_source_roster_version,receipt_destination_roster_version,
  receipt_media_epoch,receipt_space_access_epoch,
  receipt_role_policy_epoch,receipt_authorization_digest,receipt_bytes,receipt_hash,
  completed_at,replay_until
) VALUES(
  @actor_id,@operation_id,1,'moderator_move',@fingerprint,@binding_bytes,@binding_hash,
  @account_id,@subject_id,@space_id,@source_logical,@destination_logical,
  @source_room,@destination_room,@source_media,@destination_media,
  @space_epoch,@subject_role_epoch,@actor_source_epoch,@actor_destination_epoch,@auth_digest,
  @g0,@g1,@g2,@g3,@g4,@g5,@g6,@g7,@g8,@g9,true,true,@now,@now,@now,
  @owner_token,'completed',@lease_owner,0,'moved',
  @receipt_source_logical,@receipt_destination_logical,@receipt_room,
  @receipt_source_roster,@receipt_destination_roster,@receipt_media,
  @receipt_space_epoch,@receipt_role_epoch,@receipt_digest,@receipt_bytes,@receipt_hash,@now,@replay_until
)`, pgx.NamedArgs{
		"actor_id":                mutation.valueFor("voice_lifecycle_operations", "actor_profile_id", seed.actorID),
		"operation_id":            mutation.valueFor("voice_lifecycle_operations", "operation_id", seed.operationID),
		"fingerprint":             mutation.valueFor("voice_lifecycle_operations", "fingerprint", bytesOfLength(32)),
		"binding_bytes":           []byte("binding-v1"),
		"binding_hash":            mutation.valueFor("voice_lifecycle_operations", "binding_hash", bytesOfLength(32)),
		"account_id":              mutation.valueFor("voice_lifecycle_operations", "actor_account_id", uuid.New()),
		"subject_id":              mutation.valueFor("voice_lifecycle_operations", "subject_profile_id", subjectID),
		"space_id":                mutation.valueFor("voice_lifecycle_operations", "space_id", spaceID),
		"source_logical":          mutation.valueFor("voice_lifecycle_operations", "source_voice_room_id", seed.sourceLogicalRoomID),
		"destination_logical":     mutation.valueFor("voice_lifecycle_operations", "destination_voice_room_id", seed.destinationLogicalRoomID),
		"source_room":             mutation.valueFor("voice_lifecycle_operations", "source_room_id", seed.sourceRoomID),
		"destination_room":        mutation.valueFor("voice_lifecycle_operations", "destination_room_id", seed.destinationRoomID),
		"source_media":            mutation.valueFor("voice_lifecycle_operations", "source_media_epoch", sourceEpoch),
		"destination_media":       mutation.valueFor("voice_lifecycle_operations", "destination_media_epoch", destinationEpoch),
		"space_epoch":             mutation.valueFor("voice_lifecycle_operations", "space_access_epoch", int64(21)),
		"subject_role_epoch":      mutation.valueFor("voice_lifecycle_operations", "subject_role_policy_epoch", int64(22)),
		"actor_source_epoch":      mutation.valueFor("voice_lifecycle_operations", "actor_source_role_policy_epoch", int64(23)),
		"actor_destination_epoch": mutation.valueFor("voice_lifecycle_operations", "actor_destination_role_policy_epoch", int64(24)),
		"auth_digest":             mutation.valueFor("voice_lifecycle_operations", "authorization_digest", bytesOfLength(32)),
		"g0":                      r22ValidGrants[0], "g1": r22ValidGrants[1], "g2": r22ValidGrants[2], "g3": r22ValidGrants[3], "g4": r22ValidGrants[4],
		"g5": r22ValidGrants[5], "g6": r22ValidGrants[6], "g7": r22ValidGrants[7], "g8": r22ValidGrants[8], "g9": r22ValidGrants[9],
		"now":         now,
		"owner_token": mutation.valueFor("voice_lifecycle_operations", "redis_owner_token", bytesOfLength(32)),
		"lease_owner": mutation.valueFor("voice_lifecycle_operations", "lease_owner", nil),
		"receipt_source_logical": mutation.valueFor(
			"voice_lifecycle_operations", "receipt_source_voice_room_id", seed.sourceLogicalRoomID,
		),
		"receipt_destination_logical": mutation.valueFor(
			"voice_lifecycle_operations", "receipt_destination_voice_room_id", seed.destinationLogicalRoomID,
		),
		"receipt_room": mutation.valueFor(
			"voice_lifecycle_operations", "receipt_room_id", seed.destinationRoomID,
		),
		"receipt_source_roster": mutation.valueFor(
			"voice_lifecycle_operations", "receipt_source_roster_version", int64(6),
		),
		"receipt_destination_roster": mutation.valueFor(
			"voice_lifecycle_operations", "receipt_destination_roster_version", int64(7),
		),
		"receipt_media": mutation.valueFor(
			"voice_lifecycle_operations", "receipt_media_epoch", destinationEpoch,
		),
		"receipt_space_epoch": mutation.valueFor(
			"voice_lifecycle_operations", "receipt_space_access_epoch", int64(21),
		),
		"receipt_role_epoch": mutation.valueFor(
			"voice_lifecycle_operations", "receipt_role_policy_epoch", int64(22),
		),
		"receipt_digest": mutation.valueFor("voice_lifecycle_operations", "receipt_authorization_digest", bytesOfLength(32)),
		"receipt_bytes":  []byte("receipt-v1"),
		"receipt_hash":   mutation.valueFor("voice_lifecycle_operations", "receipt_hash", bytesOfLength(32)),
		"replay_until":   now.Add(24 * time.Hour),
	})
	if err != nil {
		return seed, err
	}

	_, err = tx.Exec(ctx, `
INSERT INTO voice_lifecycle_effects(
  effect_id,actor_profile_id,operation_id,ordinal,kind,schema_version,
  target_voice_room_id,target_livekit_room_name,target_profile_id,target_participant_identity,
  target_media_epoch,request_bytes,request_digest,state,attempt_count,next_attempt_at,
  lease_owner,lease_fence,applied_at,created_at,updated_at
) VALUES(
  @effect_id,@actor_id,@operation_id,0,'livekit_eject_participant',1,
  @voice_room_id,@livekit_room_name,@profile_id,@participant_identity,
  @media_epoch,@request_bytes,@request_digest,'applied',1,@now,
  @lease_owner,0,@now,@now,@now
)`, pgx.NamedArgs{
		"effect_id":            mutation.valueFor("voice_lifecycle_effects", "effect_id", uuid.New()),
		"actor_id":             mutation.valueFor("voice_lifecycle_effects", "actor_profile_id", seed.actorID),
		"operation_id":         mutation.valueFor("voice_lifecycle_effects", "operation_id", seed.operationID),
		"voice_room_id":        mutation.valueFor("voice_lifecycle_effects", "target_voice_room_id", seed.sourceLogicalRoomID),
		"livekit_room_name":    "r22-seed-source-" + seed.sourceRoomID.String(),
		"profile_id":           mutation.valueFor("voice_lifecycle_effects", "target_profile_id", subjectID),
		"participant_identity": "profile:" + subjectID.String() + ":media:" + sourceEpoch.String(),
		"media_epoch":          mutation.valueFor("voice_lifecycle_effects", "target_media_epoch", sourceEpoch),
		"request_bytes":        []byte("effect-v1"),
		"request_digest":       mutation.valueFor("voice_lifecycle_effects", "request_digest", bytesOfLength(32)),
		"now":                  now,
		"lease_owner":          mutation.valueFor("voice_lifecycle_effects", "lease_owner", nil),
	})
	if err != nil {
		return seed, err
	}

	_, err = tx.Exec(ctx, `
INSERT INTO voice_media_epoch_denials(
  media_epoch,profile_id,room_id,livekit_room_name,participant_identity,
  grant_expires_at,accepted_clock_skew,deny_until,reason,
  operation_actor_profile_id,operation_id,created_at,updated_at
) VALUES(
  @media_epoch,@profile_id,@room_id,@livekit_room_name,@participant_identity,
  @grant_expiry,interval '2 seconds',@deny_until,'leave',@actor_id,@operation_id,@now,@now
)`, pgx.NamedArgs{
		"media_epoch":          mutation.valueFor("voice_media_epoch_denials", "media_epoch", sourceEpoch),
		"profile_id":           mutation.valueFor("voice_media_epoch_denials", "profile_id", subjectID),
		"room_id":              mutation.valueFor("voice_media_epoch_denials", "room_id", seed.sourceRoomID),
		"livekit_room_name":    "r22-seed-source-" + seed.sourceRoomID.String(),
		"participant_identity": "profile:" + subjectID.String() + ":media:" + sourceEpoch.String(),
		"grant_expiry":         now.Add(time.Minute),
		"deny_until":           now.Add(time.Minute + 2*time.Second),
		"actor_id":             mutation.valueFor("voice_media_epoch_denials", "operation_actor_profile_id", seed.actorID),
		"operation_id":         mutation.valueFor("voice_media_epoch_denials", "operation_id", seed.operationID),
		"now":                  now,
	})
	if err != nil {
		return seed, err
	}

	_, err = tx.Exec(ctx, `
INSERT INTO voice_event_outbox(
  event_id,actor_profile_id,operation_id,ordinal,subject,schema_version,payload_bytes,payload_hash,
  subject_profile_id,room_id,voice_room_id,space_id,roster_version,state,attempt_count,
  next_attempt_at,lease_owner,lease_fence,created_at,updated_at
) VALUES(
  @event_id,@actor_id,@operation_id,0,'voice.test.r22.lifecycle.v1',1,@payload,@payload_hash,
  @subject_id,@room_id,@voice_room_id,@space_id,@roster_version,'ready',0,
  @now,@lease_owner,0,@now,@now
)`, pgx.NamedArgs{
		"event_id":       mutation.valueFor("voice_event_outbox", "event_id", uuid.New()),
		"actor_id":       mutation.valueFor("voice_event_outbox", "actor_profile_id", seed.actorID),
		"operation_id":   mutation.valueFor("voice_event_outbox", "operation_id", seed.operationID),
		"payload":        []byte("fixture-v1"),
		"payload_hash":   mutation.valueFor("voice_event_outbox", "payload_hash", bytesOfLength(32)),
		"subject_id":     mutation.valueFor("voice_event_outbox", "subject_profile_id", subjectID),
		"room_id":        mutation.valueFor("voice_event_outbox", "room_id", seed.destinationRoomID),
		"voice_room_id":  mutation.valueFor("voice_event_outbox", "voice_room_id", seed.destinationLogicalRoomID),
		"space_id":       mutation.valueFor("voice_event_outbox", "space_id", spaceID),
		"roster_version": mutation.valueFor("voice_event_outbox", "roster_version", int64(7)),
		"now":            now,
		"lease_owner":    mutation.valueFor("voice_event_outbox", "lease_owner", nil),
	})
	return seed, err
}

func r22InsertValidLeasedRow(ctx context.Context, tx pgx.Tx, table string, seed r22MigrationRows, leaseOwner uuid.UUID) error {
	now := time.Date(2026, 9, 11, 1, 0, 0, 0, time.UTC)
	leaseUntil := now.Add(time.Minute)
	switch table {
	case "voice_lifecycle_operations":
		grants := r22ValidGrants
		_, err := tx.Exec(ctx, `
INSERT INTO voice_lifecycle_operations(
  actor_profile_id,operation_id,schema_version,method,fingerprint,binding_bytes,binding_hash,
  actor_account_id,subject_profile_id,space_id,destination_voice_room_id,destination_room_id,
  destination_media_epoch,space_access_epoch,subject_role_policy_epoch,authorization_digest,
  can_join,can_publish_audio,can_publish_video,can_publish_screen_share,can_subscribe,
  can_mute_others,can_deafen_others,can_move_others,can_use_ptt,priority_speaker,
  decided_at,created_at,updated_at,redis_owner_token,state,lease_owner,lease_until,lease_fence
) VALUES(
  @actor_id,@operation_id,1,'join',@fingerprint,@binding_bytes,@binding_hash,
  @account_id,@subject_id,@space_id,@voice_room_id,@room_id,@media_epoch,31,32,@auth_digest,
  @g0,@g1,@g2,@g3,@g4,@g5,@g6,@g7,@g8,@g9,
  @now,@now,@now,@owner_token,'decided',@lease_owner,@lease_until,1
)`, pgx.NamedArgs{
			"actor_id":      uuid.New(),
			"operation_id":  uuid.New(),
			"fingerprint":   bytesOfLength(32),
			"binding_bytes": []byte("leased-operation-v1"),
			"binding_hash":  bytesOfLength(32),
			"account_id":    uuid.New(),
			"subject_id":    uuid.New(),
			"space_id":      seed.spaceID,
			"voice_room_id": seed.destinationLogicalRoomID,
			"room_id":       seed.destinationRoomID,
			"media_epoch":   uuid.New(),
			"auth_digest":   bytesOfLength(32),
			"g0":            grants[0], "g1": grants[1], "g2": grants[2], "g3": grants[3], "g4": grants[4],
			"g5": grants[5], "g6": grants[6], "g7": grants[7], "g8": grants[8], "g9": grants[9],
			"now": now, "owner_token": bytesOfLength(32), "lease_owner": leaseOwner, "lease_until": leaseUntil,
		})
		return err
	case "voice_lifecycle_effects":
		_, err := tx.Exec(ctx, `
INSERT INTO voice_lifecycle_effects(
  effect_id,actor_profile_id,operation_id,ordinal,kind,schema_version,
  target_voice_room_id,target_livekit_room_name,request_bytes,request_digest,
  state,attempt_count,next_attempt_at,lease_owner,lease_until,lease_fence,created_at,updated_at
) VALUES(
  @effect_id,@actor_id,@operation_id,1,'livekit_ensure_room',1,
  @voice_room_id,@livekit_room_name,@request_bytes,@request_digest,
  'ready',0,@now,@lease_owner,@lease_until,1,@now,@now
)`, pgx.NamedArgs{
			"effect_id":         uuid.New(),
			"actor_id":          seed.actorID,
			"operation_id":      seed.operationID,
			"voice_room_id":     seed.destinationLogicalRoomID,
			"livekit_room_name": "r22-seed-destination-" + seed.destinationRoomID.String(),
			"request_bytes":     []byte("leased-effect-v1"),
			"request_digest":    bytesOfLength(32),
			"now":               now,
			"lease_owner":       leaseOwner,
			"lease_until":       leaseUntil,
		})
		return err
	case "voice_event_outbox":
		_, err := tx.Exec(ctx, `
INSERT INTO voice_event_outbox(
  event_id,actor_profile_id,operation_id,ordinal,subject,schema_version,payload_bytes,payload_hash,
  subject_profile_id,room_id,voice_room_id,space_id,roster_version,state,attempt_count,
  next_attempt_at,lease_owner,lease_until,lease_fence,created_at,updated_at
) VALUES(
  @event_id,@actor_id,@operation_id,1,'voice.test.r22.lifecycle.v1',1,@payload,@payload_hash,
  @subject_id,@room_id,@voice_room_id,@space_id,7,'ready',0,
  @now,@lease_owner,@lease_until,1,@now,@now
)`, pgx.NamedArgs{
			"event_id":      uuid.New(),
			"actor_id":      seed.actorID,
			"operation_id":  seed.operationID,
			"payload":       []byte("leased-outbox-v1"),
			"payload_hash":  bytesOfLength(32),
			"subject_id":    uuid.New(),
			"room_id":       seed.destinationRoomID,
			"voice_room_id": seed.destinationLogicalRoomID,
			"space_id":      seed.spaceID,
			"now":           now,
			"lease_owner":   leaseOwner,
			"lease_until":   leaseUntil,
		})
		return err
	default:
		return fmt.Errorf("unsupported leased table %q", table)
	}
}

func r22RequireSQLState(t *testing.T, err error, state string) {
	t.Helper()
	require.Error(t, err)
	var pgErr *pgconn.PgError
	require.True(t, errors.As(err, &pgErr), "expected PostgreSQL error, got %T: %v", err, err)
	require.Equal(t, state, pgErr.Code)
}

func bytesOfLength(length int) []byte {
	return []byte(strings.Repeat("x", length))
}

func r22InsertRoom(t *testing.T, ctx context.Context, pool *pgxpool.Pool, spaceID, logicalRoomID uuid.UUID, liveKitName, state string, rosterVersion int64, closedAt any) uuid.UUID {
	t.Helper()
	roomID := uuid.New()
	_, err := pool.Exec(ctx, `
INSERT INTO voice_room_instances(room_id,space_id,voice_room_id,livekit_room_name,state,roster_version,created_at,updated_at,closed_at)
VALUES($1,$2,$3,$4,$5,$6,now(),now(),$7)`, roomID, spaceID, logicalRoomID, liveKitName, state, rosterVersion, closedAt)
	require.NoError(t, err)
	return roomID
}

func r22InsertMembership(t *testing.T, ctx context.Context, pool *pgxpool.Pool, profileID, roomID, mediaEpoch uuid.UUID, grants [10]bool) {
	t.Helper()
	require.NoError(t, r22TryInsertMembership(ctx, pool, profileID, roomID, mediaEpoch, grants))
}

func r22TryInsertMembership(ctx context.Context, pool *pgxpool.Pool, profileID, roomID, mediaEpoch uuid.UUID, grants [10]bool) error {
	_, err := pool.Exec(ctx, `
INSERT INTO voice_room_memberships(
  profile_id,room_id,media_epoch,space_access_epoch,role_policy_epoch,authorization_digest,
  can_join,can_publish_audio,can_publish_video,can_publish_screen_share,can_subscribe,
  can_mute_others,can_deafen_others,can_move_others,can_use_ptt,priority_speaker,joined_at,updated_at
) VALUES($1,$2,$3,1,1,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,now(),now())`,
		profileID, roomID, mediaEpoch, bytesOfLength(32),
		grants[0], grants[1], grants[2], grants[3], grants[4],
		grants[5], grants[6], grants[7], grants[8], grants[9])
	return err
}
