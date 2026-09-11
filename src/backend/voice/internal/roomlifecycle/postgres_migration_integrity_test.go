package roomlifecycle

import (
	"context"
	"crypto/sha256"
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

// This file freezes RED-A A10-A24 from R22.2-VOICE-DB-PLAN.md lines 258-520
// and 754-768. Every rejected write is exercised against PostgreSQL 16; catalog
// checks supplement the write assertions where the plan requires a named object.

type r22SQLExecer interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

type r22IntegritySeed struct {
	spaceID             uuid.UUID
	sourceRoomID        uuid.UUID
	destinationRoomID   uuid.UUID
	alternateRoomID     uuid.UUID
	sourceVoiceRoomID   uuid.UUID
	destinationVoiceID  uuid.UUID
	alternateVoiceID    uuid.UUID
	sourceLiveKitRoom   string
	destinationLiveKit  string
	sourceMediaEpoch    uuid.UUID
	destinationMedia    uuid.UUID
	subjectProfileID    uuid.UUID
	participantIdentity string
}

func r22NewIntegritySeed(t *testing.T, ctx context.Context, pool *pgxpool.Pool) r22IntegritySeed {
	t.Helper()
	seed := r22IntegritySeed{
		spaceID:            uuid.New(),
		sourceVoiceRoomID:  uuid.New(),
		destinationVoiceID: uuid.New(),
		alternateVoiceID:   uuid.New(),
		sourceMediaEpoch:   uuid.New(),
		destinationMedia:   uuid.New(),
		subjectProfileID:   uuid.New(),
	}
	seed.sourceLiveKitRoom = "r22-integrity-source-" + uuid.NewString()
	seed.destinationLiveKit = "r22-integrity-destination-" + uuid.NewString()
	seed.sourceRoomID = r22InsertRoom(t, ctx, pool, seed.spaceID, seed.sourceVoiceRoomID, seed.sourceLiveKitRoom, "active", 0, nil)
	seed.destinationRoomID = r22InsertRoom(t, ctx, pool, seed.spaceID, seed.destinationVoiceID, seed.destinationLiveKit, "active", 0, nil)
	seed.alternateRoomID = r22InsertRoom(t, ctx, pool, seed.spaceID, seed.alternateVoiceID, "r22-integrity-alternate-"+uuid.NewString(), "active", 0, nil)
	seed.participantIdentity = "profile:" + seed.subjectProfileID.String() + ":media:" + seed.sourceMediaEpoch.String()
	return seed
}

func r22CloneArgs(source pgx.NamedArgs) pgx.NamedArgs {
	clone := make(pgx.NamedArgs, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

func r22OperationArgs(seed r22IntegritySeed, method, state, outcome string) pgx.NamedArgs {
	now := time.Date(2026, 9, 11, 2, 0, 0, 0, time.UTC)
	args := pgx.NamedArgs{
		"actor_id": uuid.New(), "operation_id": uuid.New(), "schema_version": int16(1),
		"method": method, "fingerprint": bytesOfLength(32), "binding_bytes": []byte("binding-v1"), "binding_hash": bytesOfLength(32),
		"account_id": uuid.New(), "subject_id": seed.subjectProfileID, "space_id": seed.spaceID,
		"source_voice": nil, "destination_voice": nil, "source_room": nil, "destination_room": nil,
		"source_media": nil, "destination_media": nil,
		"space_epoch": nil, "subject_role_epoch": nil, "actor_source_epoch": nil, "actor_destination_epoch": nil,
		"auth_digest": nil,
		"g0":          nil, "g1": nil, "g2": nil, "g3": nil, "g4": nil,
		"g5": nil, "g6": nil, "g7": nil, "g8": nil, "g9": nil,
		"actor_can_source": nil, "actor_can_destination": nil,
		"decided_at": now, "created_at": now, "updated_at": now, "owner_token": bytesOfLength(32),
		"state": state, "lease_owner": nil, "lease_until": nil, "lease_fence": int64(0),
		"quarantine_class": nil, "quarantine_detail": nil, "quarantined_at": nil,
		"receipt_outcome": nil, "receipt_source_voice": nil, "receipt_destination_voice": nil, "receipt_room": nil,
		"receipt_source_roster": nil, "receipt_destination_roster": nil, "receipt_media": nil,
		"receipt_space_epoch": nil, "receipt_role_epoch": nil, "receipt_digest": nil,
		"receipt_bytes": nil, "receipt_hash": nil, "completed_at": nil, "replay_until": nil,
	}

	switch method {
	case "join":
		args["destination_voice"], args["destination_room"], args["destination_media"] = seed.destinationVoiceID, seed.destinationRoomID, seed.destinationMedia
		r22SetSubjectDecision(args)
	case "leave":
		args["source_voice"], args["source_room"], args["source_media"] = seed.sourceVoiceRoomID, seed.sourceRoomID, seed.sourceMediaEpoch
		if state == "completed" && outcome == "no_op" {
			args["source_room"], args["source_media"] = nil, nil
		}
	case "self_move":
		args["source_voice"], args["source_room"], args["source_media"] = seed.sourceVoiceRoomID, seed.sourceRoomID, seed.sourceMediaEpoch
		args["destination_voice"], args["destination_room"], args["destination_media"] = seed.destinationVoiceID, seed.destinationRoomID, seed.destinationMedia
		r22SetSubjectDecision(args)
	case "moderator_move":
		args["source_voice"], args["source_room"], args["source_media"] = seed.sourceVoiceRoomID, seed.sourceRoomID, seed.sourceMediaEpoch
		args["destination_voice"], args["destination_room"], args["destination_media"] = seed.destinationVoiceID, seed.destinationRoomID, seed.destinationMedia
		r22SetSubjectDecision(args)
		args["actor_source_epoch"], args["actor_destination_epoch"] = int64(43), int64(44)
		args["actor_can_source"], args["actor_can_destination"] = true, true
	}

	if state == "completed" {
		args["receipt_outcome"] = outcome
		args["receipt_bytes"], args["receipt_hash"] = []byte("receipt-v1"), bytesOfLength(32)
		args["completed_at"], args["replay_until"] = now, now.Add(24*time.Hour)
		r22SetReceiptShape(args, seed, method, outcome)
	}
	if state == "quarantined" {
		args["quarantine_class"], args["quarantined_at"] = "manual_repair_required", now
	}
	return args
}

func r22SetSubjectDecision(args pgx.NamedArgs) {
	args["space_epoch"], args["subject_role_epoch"], args["auth_digest"] = int64(41), int64(42), bytesOfLength(32)
	for index, value := range r22ValidGrants {
		args[fmt.Sprintf("g%d", index)] = value
	}
}

func r22SetReceiptShape(args pgx.NamedArgs, seed r22IntegritySeed, method, outcome string) {
	switch {
	case method == "join" && (outcome == "joined" || outcome == "no_op"):
		args["receipt_destination_voice"], args["receipt_room"] = seed.destinationVoiceID, seed.destinationRoomID
		args["receipt_destination_roster"], args["receipt_media"] = int64(8), seed.destinationMedia
		args["receipt_space_epoch"], args["receipt_role_epoch"], args["receipt_digest"] = int64(41), int64(42), bytesOfLength(32)
	case method == "leave" && outcome == "left":
		args["receipt_source_voice"], args["receipt_room"] = seed.sourceVoiceRoomID, seed.sourceRoomID
		args["receipt_source_roster"] = int64(8)
	case (method == "self_move" || method == "moderator_move") && outcome == "moved":
		args["receipt_source_voice"], args["receipt_destination_voice"] = seed.sourceVoiceRoomID, seed.destinationVoiceID
		args["receipt_room"] = seed.destinationRoomID
		args["receipt_source_roster"], args["receipt_destination_roster"] = int64(8), int64(9)
		args["receipt_media"] = seed.destinationMedia
		args["receipt_space_epoch"], args["receipt_role_epoch"], args["receipt_digest"] = int64(41), int64(42), bytesOfLength(32)
	}
}

func r22InsertOperation(ctx context.Context, execer r22SQLExecer, args pgx.NamedArgs) error {
	_, err := execer.Exec(ctx, `
INSERT INTO voice_lifecycle_operations(
  actor_profile_id,operation_id,schema_version,method,fingerprint,binding_bytes,binding_hash,
  actor_account_id,subject_profile_id,space_id,source_voice_room_id,destination_voice_room_id,
  source_room_id,destination_room_id,source_media_epoch,destination_media_epoch,
  space_access_epoch,subject_role_policy_epoch,actor_source_role_policy_epoch,actor_destination_role_policy_epoch,
  authorization_digest,can_join,can_publish_audio,can_publish_video,can_publish_screen_share,
  can_subscribe,can_mute_others,can_deafen_others,can_move_others,can_use_ptt,priority_speaker,
  actor_can_move_source,actor_can_move_destination,decided_at,created_at,updated_at,redis_owner_token,
  state,lease_owner,lease_until,lease_fence,quarantine_class,quarantine_detail,quarantined_at,
  receipt_outcome,receipt_source_voice_room_id,receipt_destination_voice_room_id,receipt_room_id,
  receipt_source_roster_version,receipt_destination_roster_version,receipt_media_epoch,
  receipt_space_access_epoch,receipt_role_policy_epoch,receipt_authorization_digest,
  receipt_bytes,receipt_hash,completed_at,replay_until
) VALUES(
  @actor_id,@operation_id,@schema_version,@method,@fingerprint,@binding_bytes,@binding_hash,
  @account_id,@subject_id,@space_id,@source_voice,@destination_voice,@source_room,@destination_room,
  @source_media,@destination_media,@space_epoch,@subject_role_epoch,@actor_source_epoch,@actor_destination_epoch,
  @auth_digest,@g0,@g1,@g2,@g3,@g4,@g5,@g6,@g7,@g8,@g9,@actor_can_source,@actor_can_destination,
  @decided_at,@created_at,@updated_at,@owner_token,@state,@lease_owner,@lease_until,@lease_fence,
  @quarantine_class,@quarantine_detail,@quarantined_at,@receipt_outcome,@receipt_source_voice,
  @receipt_destination_voice,@receipt_room,@receipt_source_roster,@receipt_destination_roster,
  @receipt_media,@receipt_space_epoch,@receipt_role_epoch,@receipt_digest,@receipt_bytes,@receipt_hash,
  @completed_at,@replay_until
)`, args)
	return err
}

func r22EffectArgs(seed r22IntegritySeed, actorID, operationID uuid.UUID, kind, state string) pgx.NamedArgs {
	now := time.Date(2026, 9, 11, 3, 0, 0, 0, time.UTC)
	args := pgx.NamedArgs{
		"effect_id": uuid.New(), "actor_id": actorID, "operation_id": operationID,
		"ordinal": int16(1), "kind": kind, "schema_version": int16(1),
		"target_voice": seed.destinationVoiceID, "target_livekit": seed.destinationLiveKit,
		"target_profile": nil, "participant_identity": nil, "target_media": nil,
		"request_bytes": []byte("effect-v1"), "request_digest": bytesOfLength(32),
		"state": state, "attempt_count": int64(0), "last_error_class": nil, "last_error_at": nil,
		"next_attempt_at": now, "lease_owner": nil, "lease_until": nil, "lease_fence": int64(0),
		"applied_at": nil, "quarantine_class": nil, "quarantine_detail": nil, "quarantined_at": nil,
		"created_at": now, "updated_at": now,
	}
	if kind == "livekit_eject_participant" {
		args["target_voice"], args["target_livekit"] = seed.sourceVoiceRoomID, seed.sourceLiveKitRoom
		args["target_profile"], args["participant_identity"], args["target_media"] = seed.subjectProfileID, seed.participantIdentity, seed.sourceMediaEpoch
	}
	if state == "applied" {
		args["applied_at"] = now
	}
	if state == "quarantined" {
		args["quarantine_class"], args["quarantined_at"] = "permanent_livekit_error", now
	}
	return args
}

func r22InsertEffect(ctx context.Context, execer r22SQLExecer, args pgx.NamedArgs) error {
	_, err := execer.Exec(ctx, `
INSERT INTO voice_lifecycle_effects(
  effect_id,actor_profile_id,operation_id,ordinal,kind,schema_version,target_voice_room_id,
  target_livekit_room_name,target_profile_id,target_participant_identity,target_media_epoch,
  request_bytes,request_digest,state,attempt_count,last_error_class,last_error_at,next_attempt_at,
  lease_owner,lease_until,lease_fence,applied_at,quarantine_class,quarantine_detail,quarantined_at,
  created_at,updated_at
) VALUES(
  @effect_id,@actor_id,@operation_id,@ordinal,@kind,@schema_version,@target_voice,@target_livekit,
  @target_profile,@participant_identity,@target_media,@request_bytes,@request_digest,@state,
  @attempt_count,@last_error_class,@last_error_at,@next_attempt_at,@lease_owner,@lease_until,
  @lease_fence,@applied_at,@quarantine_class,@quarantine_detail,@quarantined_at,@created_at,@updated_at
)`, args)
	return err
}

func r22DenialArgs(seed r22IntegritySeed, actorID, operationID uuid.UUID) pgx.NamedArgs {
	now := time.Date(2026, 9, 11, 4, 0, 0, 0, time.UTC)
	return pgx.NamedArgs{
		"media_epoch": seed.sourceMediaEpoch, "profile_id": seed.subjectProfileID, "room_id": seed.sourceRoomID,
		"livekit_room": seed.sourceLiveKitRoom, "participant_identity": seed.participantIdentity,
		"grant_expires_at": now.Add(time.Hour), "clock_skew": 2 * time.Second,
		"deny_until": now.Add(time.Hour + 2*time.Second), "reason": "leave",
		"actor_id": actorID, "operation_id": operationID, "absence_observed_at": nil,
		"created_at": now, "updated_at": now,
	}
}

func r22InsertDenial(ctx context.Context, execer r22SQLExecer, args pgx.NamedArgs) error {
	_, err := execer.Exec(ctx, `
INSERT INTO voice_media_epoch_denials(
  media_epoch,profile_id,room_id,livekit_room_name,participant_identity,grant_expires_at,
  accepted_clock_skew,deny_until,reason,operation_actor_profile_id,operation_id,
  absence_observed_at,created_at,updated_at
) VALUES(
  @media_epoch,@profile_id,@room_id,@livekit_room,@participant_identity,@grant_expires_at,
  @clock_skew,@deny_until,@reason,@actor_id,@operation_id,@absence_observed_at,@created_at,@updated_at
)`, args)
	return err
}

func r22OutboxArgs(seed r22IntegritySeed, actorID, operationID uuid.UUID, state string) pgx.NamedArgs {
	now := time.Date(2026, 9, 11, 5, 0, 0, 0, time.UTC)
	payload := []byte("payload-v1")
	payloadHash := sha256.Sum256(payload)
	args := pgx.NamedArgs{
		"event_id": uuid.New(), "actor_id": actorID, "operation_id": operationID,
		"ordinal": int16(1), "subject": "voice.test.r22.lifecycle.v1", "schema_version": int16(1),
		"payload_bytes": payload, "payload_hash": payloadHash[:],
		"subject_profile": seed.subjectProfileID, "room_id": seed.destinationRoomID,
		"voice_room_id": seed.destinationVoiceID, "space_id": seed.spaceID, "roster_version": int64(9),
		"state": state, "attempt_count": int64(0), "last_error_class": nil, "last_error_at": nil,
		"next_attempt_at": now, "lease_owner": nil, "lease_until": nil, "lease_fence": int64(0),
		"delivered_at": nil, "quarantine_class": nil, "quarantine_detail": nil, "quarantined_at": nil,
		"created_at": now, "updated_at": now,
	}
	if state == "delivered" {
		args["delivered_at"] = now
	}
	if state == "quarantined" {
		args["quarantine_class"], args["quarantined_at"] = "permanent_publish_error", now
	}
	return args
}

func r22InsertOutbox(ctx context.Context, execer r22SQLExecer, args pgx.NamedArgs) error {
	_, err := execer.Exec(ctx, `
INSERT INTO voice_event_outbox(
  event_id,actor_profile_id,operation_id,ordinal,subject,schema_version,payload_bytes,payload_hash,
  subject_profile_id,room_id,voice_room_id,space_id,roster_version,state,attempt_count,
  last_error_class,last_error_at,next_attempt_at,lease_owner,lease_until,lease_fence,delivered_at,
  quarantine_class,quarantine_detail,quarantined_at,created_at,updated_at
) VALUES(
  @event_id,@actor_id,@operation_id,@ordinal,@subject,@schema_version,@payload_bytes,@payload_hash,
  @subject_profile,@room_id,@voice_room_id,@space_id,@roster_version,@state,@attempt_count,
  @last_error_class,@last_error_at,@next_attempt_at,@lease_owner,@lease_until,@lease_fence,
  @delivered_at,@quarantine_class,@quarantine_detail,@quarantined_at,@created_at,@updated_at
)`, args)
	return err
}

func r22RequireRejectedWrite(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err, "the frozen migration integrity contract must reject this direct SQL write")
}

func r22NormalizedSQL(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(value)), "")
}

func TestVoiceDBMigration_A10_A16_OperationIntegrity(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL migration contract requires testcontainers")
	}
	ctx := context.Background()
	pool := r22StartVoicePostgres(t, ctx, "r22voicea10a16")
	seed := r22NewIntegritySeed(t, ctx, pool)

	t.Run("A10 method schema and state allowlists", func(t *testing.T) {
		for _, tc := range []struct {
			name   string
			column string
			value  any
		}{
			{"unknown method", "method", "unknown"},
			{"unknown state", "state", "unknown"},
			{"zero schema", "schema_version", int16(0)},
			{"negative schema", "schema_version", int16(-1)},
		} {
			t.Run(tc.name, func(t *testing.T) {
				r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
					args := r22OperationArgs(seed, "join", "decided", "")
					args[tc.column] = tc.value
					r22RequireRejectedWrite(t, r22InsertOperation(ctx, tx, args))
				})
			})
		}

		decisionFields := []struct {
			name    string
			nonnull any
		}{
			{"source_voice", seed.sourceVoiceRoomID}, {"destination_voice", seed.destinationVoiceID},
			{"source_room", seed.sourceRoomID}, {"destination_room", seed.destinationRoomID},
			{"source_media", seed.sourceMediaEpoch}, {"destination_media", seed.destinationMedia},
			{"space_epoch", int64(41)}, {"subject_role_epoch", int64(42)},
			{"actor_source_epoch", int64(43)}, {"actor_destination_epoch", int64(44)},
			{"auth_digest", bytesOfLength(32)},
			{"g0", true}, {"g1", false}, {"g2", true}, {"g3", false}, {"g4", true},
			{"g5", false}, {"g6", true}, {"g7", false}, {"g8", true}, {"g9", false},
			{"actor_can_source", true}, {"actor_can_destination", true},
		}
		for _, method := range []string{"join", "leave", "self_move", "moderator_move"} {
			t.Run(method+" accepts exact decision shape", func(t *testing.T) {
				r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
					valid := r22OperationArgs(seed, method, "decided", "")
					require.NoError(t, r22InsertOperation(ctx, tx, valid))
				})
			})
			for _, field := range decisionFields {
				t.Run(method+" rejects decision presence flip of "+field.name, func(t *testing.T) {
					r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
						args := r22OperationArgs(seed, method, "decided", "")
						if args[field.name] == nil {
							args[field.name] = field.nonnull
						} else {
							args[field.name] = nil
						}
						r22RequireRejectedWrite(t, r22InsertOperation(ctx, tx, args))
					})
				})
			}
		}
		for _, method := range []string{"self_move", "moderator_move"} {
			t.Run(method+" moved rejects equal source and destination", func(t *testing.T) {
				r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
					args := r22OperationArgs(seed, method, "completed", "moved")
					args["receipt_destination_voice"] = args["receipt_source_voice"]
					r22RequireSQLState(t, r22InsertOperation(ctx, tx, args), "23514")
				})
			})
		}
		t.Run("unknown receipt outcome is rejected", func(t *testing.T) {
			r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
				args := r22OperationArgs(seed, "join", "completed", "joined")
				args["receipt_outcome"] = "unknown"
				r22RequireSQLState(t, r22InsertOperation(ctx, tx, args), "23514")
			})
		})
	})

	t.Run("A11 operation key permanently binds method fingerprint and body", func(t *testing.T) {
		r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
			original := r22OperationArgs(seed, "join", "decided", "")
			require.NoError(t, r22InsertOperation(ctx, tx, original))
			r22RequireSQLState(t, r22InsertOperation(ctx, tx, r22CloneArgs(original)), "23505")
		})

		for _, tc := range []struct {
			column string
			value  any
		}{
			{"method", "leave"},
			{"fingerprint", []byte(strings.Repeat("z", 32))},
			{"binding_bytes", []byte("changed-binding")},
			{"binding_hash", []byte(strings.Repeat("z", 32))},
		} {
			t.Run(tc.column+" conflict leaves original stable", func(t *testing.T) {
				r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
					original := r22OperationArgs(seed, "join", "decided", "")
					require.NoError(t, r22InsertOperation(ctx, tx, original))
					changed := r22CloneArgs(original)
					changed[tc.column] = tc.value
					if tc.column == "method" {
						changed["source_voice"], changed["source_room"], changed["source_media"] = seed.sourceVoiceRoomID, seed.sourceRoomID, seed.sourceMediaEpoch
						changed["destination_voice"], changed["destination_room"], changed["destination_media"] = nil, nil, nil
						changed["space_epoch"], changed["subject_role_epoch"], changed["auth_digest"] = nil, nil, nil
						changed["actor_source_epoch"], changed["actor_destination_epoch"] = nil, nil
						changed["actor_can_source"], changed["actor_can_destination"] = nil, nil
						for index := range r22ValidGrants {
							changed[fmt.Sprintf("g%d", index)] = nil
						}
					}
					_, err := tx.Exec(ctx, "SAVEPOINT changed_binding")
					require.NoError(t, err)
					r22RequireSQLState(t, r22InsertOperation(ctx, tx, changed), "23505")
					_, err = tx.Exec(ctx, "ROLLBACK TO SAVEPOINT changed_binding")
					require.NoError(t, err)

					var method string
					var fingerprint, bindingBytes, bindingHash []byte
					err = tx.QueryRow(ctx, `
SELECT method,fingerprint,binding_bytes,binding_hash
FROM voice_lifecycle_operations
WHERE actor_profile_id=$1 AND operation_id=$2`, original["actor_id"], original["operation_id"]).Scan(
						&method, &fingerprint, &bindingBytes, &bindingHash,
					)
					require.NoError(t, err)
					require.Equal(t, original["method"], method)
					require.Equal(t, original["fingerprint"], fingerprint)
					require.Equal(t, original["binding_bytes"], bindingBytes)
					require.Equal(t, original["binding_hash"], bindingHash)
				})
			})
		}
	})

	t.Run("A12 exact named six row receipt shape", func(t *testing.T) {
		t.Run("named check exists", func(t *testing.T) {
			var constraintDefinition string
			err := pool.QueryRow(ctx, `
SELECT pg_get_constraintdef(c.oid)
FROM pg_constraint c
JOIN pg_class r ON r.oid=c.conrelid
JOIN pg_namespace n ON n.oid=r.relnamespace
WHERE n.nspname=current_schema()
  AND r.relname='voice_lifecycle_operations'
  AND c.conname='voice_lifecycle_operations_receipt_shape_check'`).Scan(&constraintDefinition)
			require.NoError(t, err, "plan lines 326-368 require the exact named receipt shape CHECK")
			require.Contains(t, r22NormalizedSQL(constraintDefinition), "state<>'completed'")
		})

		allowed := []struct{ method, outcome string }{
			{"join", "joined"}, {"join", "no_op"}, {"leave", "left"},
			{"leave", "no_op"}, {"self_move", "moved"}, {"moderator_move", "moved"},
		}
		receiptFields := []struct {
			name    string
			nonnull any
		}{
			{"receipt_source_voice", uuid.New()},
			{"receipt_destination_voice", uuid.New()},
			{"receipt_room", seed.destinationRoomID},
			{"receipt_source_roster", int64(1)},
			{"receipt_destination_roster", int64(1)},
			{"receipt_media", uuid.New()},
			{"receipt_space_epoch", int64(1)},
			{"receipt_role_epoch", int64(1)},
			{"receipt_digest", bytesOfLength(32)},
		}
		for _, row := range allowed {
			name := row.method + "_" + row.outcome
			t.Run(name+" accepts exact row", func(t *testing.T) {
				r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
					if row.method == "leave" && row.outcome == "no_op" {
						_, err := tx.Exec(ctx, `DELETE FROM voice_room_instances`)
						require.NoError(t, err)
						var roomCount int64
						require.NoError(t, tx.QueryRow(ctx, `SELECT count(*) FROM voice_room_instances`).Scan(&roomCount))
						require.Zero(t, roomCount, "absent leave must not require a manufactured runtime room")
					}
					require.NoError(t, r22InsertOperation(ctx, tx, r22OperationArgs(seed, row.method, "completed", row.outcome)))
				})
			})
			for _, field := range receiptFields {
				t.Run(name+" rejects presence flip of "+field.name, func(t *testing.T) {
					r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
						args := r22OperationArgs(seed, row.method, "completed", row.outcome)
						if args[field.name] == nil {
							args[field.name] = field.nonnull
						} else {
							args[field.name] = nil
						}
						r22RequireRejectedWrite(t, r22InsertOperation(ctx, tx, args))
					})
				})
			}
		}

		allowedPair := map[string]bool{}
		for _, row := range allowed {
			allowedPair[row.method+"/"+row.outcome] = true
		}
		for _, method := range []string{"join", "leave", "self_move", "moderator_move"} {
			for _, outcome := range []string{"joined", "left", "moved", "no_op"} {
				if allowedPair[method+"/"+outcome] {
					continue
				}
				t.Run(method+" rejects outcome "+outcome, func(t *testing.T) {
					r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
						r22RequireRejectedWrite(t, r22InsertOperation(ctx, tx, r22OperationArgs(seed, method, "completed", outcome)))
					})
				})
			}
		}

		t.Run("absent leave requires requested logical source and forbids runtime evidence", func(t *testing.T) {
			cases := []struct {
				name    string
				state   string
				outcome string
				set     pgx.NamedArgs
			}{
				{"completed no op missing requested logical source", "completed", "no_op", pgx.NamedArgs{"source_voice": nil}},
				{"completed no op runtime room only", "completed", "no_op", pgx.NamedArgs{"source_room": seed.sourceRoomID}},
				{"completed no op runtime media only", "completed", "no_op", pgx.NamedArgs{"source_media": seed.sourceMediaEpoch}},
				{"completed no op complete runtime pair", "completed", "no_op", pgx.NamedArgs{"source_room": seed.sourceRoomID, "source_media": seed.sourceMediaEpoch}},
			}
			for _, state := range []struct {
				name    string
				state   string
				outcome string
			}{
				{"decided", "decided", ""},
				{"quarantined", "quarantined", ""},
				{"completed left", "completed", "left"},
			} {
				cases = append(cases,
					struct {
						name    string
						state   string
						outcome string
						set     pgx.NamedArgs
					}{state.name + " missing runtime room", state.state, state.outcome, pgx.NamedArgs{"source_room": nil}},
					struct {
						name    string
						state   string
						outcome string
						set     pgx.NamedArgs
					}{state.name + " missing runtime media", state.state, state.outcome, pgx.NamedArgs{"source_media": nil}},
					struct {
						name    string
						state   string
						outcome string
						set     pgx.NamedArgs
					}{state.name + " missing runtime pair", state.state, state.outcome, pgx.NamedArgs{"source_room": nil, "source_media": nil}},
				)
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
						args := r22OperationArgs(seed, "leave", tc.state, tc.outcome)
						for key, value := range tc.set {
							args[key] = value
						}
						r22RequireSQLState(t, r22InsertOperation(ctx, tx, args), "23514")
					})
				})
			}
		})
	})

	t.Run("A13 replay window is at least twenty four hours", func(t *testing.T) {
		r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
			exact := r22OperationArgs(seed, "join", "completed", "joined")
			require.NoError(t, r22InsertOperation(ctx, tx, exact))
		})
		r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
			tooShort := r22OperationArgs(seed, "join", "completed", "joined")
			tooShort["replay_until"] = tooShort["completed_at"].(time.Time).Add(24*time.Hour - time.Microsecond)
			r22RequireRejectedWrite(t, r22InsertOperation(ctx, tx, tooShort))
		})
	})

	t.Run("A14 operation completion lease and quarantine evidence", func(t *testing.T) {
		now := time.Date(2026, 9, 11, 6, 0, 0, 0, time.UTC)
		cases := []struct {
			name  string
			state string
			set   pgx.NamedArgs
		}{
			{"lease owner without deadline", "decided", pgx.NamedArgs{"lease_owner": uuid.New()}},
			{"lease deadline without owner", "decided", pgx.NamedArgs{"lease_until": now}},
			{"negative lease fence", "decided", pgx.NamedArgs{"lease_fence": int64(-1)}},
			{"completed with lease", "completed", pgx.NamedArgs{"lease_owner": uuid.New(), "lease_until": now}},
			{"quarantined with lease", "quarantined", pgx.NamedArgs{"lease_owner": uuid.New(), "lease_until": now}},
			{"blank quarantine class", "quarantined", pgx.NamedArgs{"quarantine_class": " "}},
			{"blank quarantine detail", "quarantined", pgx.NamedArgs{"quarantine_detail": " "}},
			{"missing quarantine time", "quarantined", pgx.NamedArgs{"quarantined_at": nil}},
			{"quarantine evidence in decided", "decided", pgx.NamedArgs{"quarantine_class": "unexpected", "quarantined_at": now}},
			{"quarantine detail alone in decided", "decided", pgx.NamedArgs{"quarantine_detail": "unexpected"}},
			{"quarantine evidence in completed", "completed", pgx.NamedArgs{"quarantine_class": "unexpected", "quarantined_at": now}},
			{"quarantine detail alone in completed", "completed", pgx.NamedArgs{"quarantine_detail": "unexpected"}},
			{"completed missing timestamp", "completed", pgx.NamedArgs{"completed_at": nil}},
			{"completed missing replay deadline", "completed", pgx.NamedArgs{"replay_until": nil}},
			{"completed missing outcome", "completed", pgx.NamedArgs{"receipt_outcome": nil}},
			{"completed missing receipt bytes", "completed", pgx.NamedArgs{"receipt_bytes": nil}},
			{"completed empty receipt bytes", "completed", pgx.NamedArgs{"receipt_bytes": []byte{}}},
			{"completed missing receipt hash", "completed", pgx.NamedArgs{"receipt_hash": nil}},
			{"decided with completion evidence", "decided", pgx.NamedArgs{"completed_at": now}},
			{"quarantined with receipt evidence", "quarantined", pgx.NamedArgs{"receipt_outcome": "joined"}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
					outcome := ""
					if tc.state == "completed" {
						outcome = "joined"
					}
					args := r22OperationArgs(seed, "join", tc.state, outcome)
					for key, value := range tc.set {
						args[key] = value
					}
					r22RequireRejectedWrite(t, r22InsertOperation(ctx, tx, args))
				})
			})
		}
		nonterminalReceiptFields := []struct {
			name  string
			value any
		}{
			{"receipt_outcome", "joined"},
			{"receipt_source_voice", seed.sourceVoiceRoomID},
			{"receipt_destination_voice", seed.destinationVoiceID},
			{"receipt_room", seed.destinationRoomID},
			{"receipt_source_roster", int64(1)},
			{"receipt_destination_roster", int64(1)},
			{"receipt_media", seed.destinationMedia},
			{"receipt_space_epoch", int64(1)},
			{"receipt_role_epoch", int64(1)},
			{"receipt_digest", bytesOfLength(32)},
			{"receipt_bytes", []byte("unexpected-receipt")},
			{"receipt_hash", bytesOfLength(32)},
			{"completed_at", now},
			{"replay_until", now.Add(24 * time.Hour)},
		}
		for _, state := range []string{"decided", "quarantined"} {
			for _, field := range nonterminalReceiptFields {
				t.Run(state+" rejects lone "+field.name, func(t *testing.T) {
					r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
						args := r22OperationArgs(seed, "join", state, "")
						args[field.name] = field.value
						r22RequireSQLState(t, r22InsertOperation(ctx, tx, args), "23514")
					})
				})
			}
		}
	})

	t.Run("A15 every nonterminal state fences the subject", func(t *testing.T) {
		for _, states := range [][2]string{{"decided", "decided"}, {"decided", "quarantined"}, {"quarantined", "quarantined"}} {
			name := states[0] + "_then_" + states[1]
			t.Run(name, func(t *testing.T) {
				r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
					first := r22OperationArgs(seed, "join", states[0], "")
					require.NoError(t, r22InsertOperation(ctx, tx, first))
					second := r22OperationArgs(seed, "moderator_move", states[1], "")
					second["actor_id"] = uuid.New()
					second["operation_id"] = uuid.New()
					r22RequireSQLState(t, r22InsertOperation(ctx, tx, second), "23505")
				})
			})
		}
	})

	t.Run("A16 completion releases subject fence but not operation key", func(t *testing.T) {
		r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
			completed := r22OperationArgs(seed, "join", "completed", "joined")
			require.NoError(t, r22InsertOperation(ctx, tx, completed))
			later := r22OperationArgs(seed, "leave", "decided", "")
			require.NoError(t, r22InsertOperation(ctx, tx, later))
			r22RequireSQLState(t, r22InsertOperation(ctx, tx, r22CloneArgs(completed)), "23505")
		})
	})

	t.Run("A15 operation fence and recovery indexes are exact", func(t *testing.T) {
		rows, err := pool.Query(ctx, `
SELECT indexdef FROM pg_indexes
WHERE schemaname=current_schema() AND tablename='voice_lifecycle_operations'`)
		require.NoError(t, err)
		defer rows.Close()
		var definitions []string
		for rows.Next() {
			var definition string
			require.NoError(t, rows.Scan(&definition))
			definitions = append(definitions, r22NormalizedSQL(definition))
		}
		require.NoError(t, rows.Err())
		for _, index := range []struct {
			name      string
			fragments []string
		}{
			{"unique nonterminal subject fence", []string{"unique", "(subject_profile_id)", "where(completed_atisnull)"}},
			{"decided recovery", []string{"(state,lease_until,decided_at,actor_profile_id,operation_id)"}},
			{"subject completion lookup", []string{"(subject_profile_id,completed_atdesc)"}},
		} {
			t.Run(index.name, func(t *testing.T) {
				require.True(t, r22ContainsSQL(definitions, index.fragments...), "missing %s index", index.name)
			})
		}
	})
}

func r22ContainsSQL(definitions []string, fragments ...string) bool {
	for _, definition := range definitions {
		matched := true
		for _, fragment := range fragments {
			if !strings.Contains(definition, r22NormalizedSQL(fragment)) {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

func r22InsertIntegrityParent(t *testing.T, ctx context.Context, tx pgx.Tx, seed r22IntegritySeed) pgx.NamedArgs {
	t.Helper()
	parent := r22OperationArgs(seed, "join", "completed", "joined")
	require.NoError(t, r22InsertOperation(ctx, tx, parent))
	return parent
}

func TestVoiceDBMigration_A17_A24_WorkAndGuardIntegrity(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL migration contract requires testcontainers")
	}
	ctx := context.Background()
	pool := r22StartVoicePostgres(t, ctx, "r22voicea17a24")
	seed := r22NewIntegritySeed(t, ctx, pool)

	t.Run("A17 effect ordinal uniqueness and restricted operation ownership", func(t *testing.T) {
		r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
			parent := r22InsertIntegrityParent(t, ctx, tx, seed)
			first := r22EffectArgs(seed, parent["actor_id"].(uuid.UUID), parent["operation_id"].(uuid.UUID), "livekit_ensure_room", "ready")
			first["ordinal"] = int16(0)
			require.NoError(t, r22InsertEffect(ctx, tx, first))
			duplicate := r22EffectArgs(seed, parent["actor_id"].(uuid.UUID), parent["operation_id"].(uuid.UUID), "livekit_eject_participant", "ready")
			duplicate["ordinal"] = int16(0)
			r22RequireSQLState(t, r22InsertEffect(ctx, tx, duplicate), "23505")
		})
		r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
			missingParent := r22EffectArgs(seed, uuid.New(), uuid.New(), "livekit_ensure_room", "ready")
			r22RequireSQLState(t, r22InsertEffect(ctx, tx, missingParent), "23503")
		})
		r22RequireUniqueConstraint(t, ctx, pool, "voice_lifecycle_effects", "actor_profile_id,operation_id,ordinal")
	})

	t.Run("A18 ensure and eject target shapes are mutually exact", func(t *testing.T) {
		for _, kind := range []string{"livekit_ensure_room", "livekit_eject_participant"} {
			t.Run(kind+" valid control", func(t *testing.T) {
				r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
					parent := r22InsertIntegrityParent(t, ctx, tx, seed)
					require.NoError(t, r22InsertEffect(ctx, tx, r22EffectArgs(seed, parent["actor_id"].(uuid.UUID), parent["operation_id"].(uuid.UUID), kind, "ready")))
				})
			})
		}

		invalid := []struct {
			name string
			kind string
			set  pgx.NamedArgs
		}{
			{"ensure with profile", "livekit_ensure_room", pgx.NamedArgs{"target_profile": seed.subjectProfileID}},
			{"ensure with participant identity", "livekit_ensure_room", pgx.NamedArgs{"participant_identity": seed.participantIdentity}},
			{"ensure with media epoch", "livekit_ensure_room", pgx.NamedArgs{"target_media": seed.sourceMediaEpoch}},
			{"eject missing profile", "livekit_eject_participant", pgx.NamedArgs{"target_profile": nil}},
			{"eject missing participant identity", "livekit_eject_participant", pgx.NamedArgs{"participant_identity": nil}},
			{"eject missing media epoch", "livekit_eject_participant", pgx.NamedArgs{"target_media": nil}},
			{"eject identity for another media epoch", "livekit_eject_participant", pgx.NamedArgs{"target_media": uuid.New()}},
			{"eject identity for another profile", "livekit_eject_participant", pgx.NamedArgs{"target_profile": uuid.New()}},
			{"blank target livekit room", "livekit_ensure_room", pgx.NamedArgs{"target_livekit": " "}},
		}
		for _, tc := range invalid {
			t.Run(tc.name, func(t *testing.T) {
				r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
					parent := r22InsertIntegrityParent(t, ctx, tx, seed)
					args := r22EffectArgs(seed, parent["actor_id"].(uuid.UUID), parent["operation_id"].(uuid.UUID), tc.kind, "ready")
					for key, value := range tc.set {
						args[key] = value
					}
					r22RequireRejectedWrite(t, r22InsertEffect(ctx, tx, args))
				})
			})
		}
	})

	t.Run("A19 effect retry lease terminal and quarantine evidence", func(t *testing.T) {
		now := time.Date(2026, 9, 11, 7, 0, 0, 0, time.UTC)
		for _, state := range []string{"ready", "applied", "quarantined"} {
			t.Run(state+" valid control", func(t *testing.T) {
				r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
					parent := r22InsertIntegrityParent(t, ctx, tx, seed)
					require.NoError(t, r22InsertEffect(ctx, tx, r22EffectArgs(seed, parent["actor_id"].(uuid.UUID), parent["operation_id"].(uuid.UUID), "livekit_ensure_room", state)))
				})
			})
		}
		cases := []struct {
			name  string
			state string
			set   pgx.NamedArgs
		}{
			{"negative ordinal", "ready", pgx.NamedArgs{"ordinal": int16(-1)}},
			{"unknown kind", "ready", pgx.NamedArgs{"kind": "unknown"}},
			{"zero schema", "ready", pgx.NamedArgs{"schema_version": int16(0)}},
			{"unknown state", "ready", pgx.NamedArgs{"state": "unknown"}},
			{"negative attempts", "ready", pgx.NamedArgs{"attempt_count": int64(-1)}},
			{"error class without time", "ready", pgx.NamedArgs{"last_error_class": "timeout"}},
			{"error time without class", "ready", pgx.NamedArgs{"last_error_at": now}},
			{"blank error class", "ready", pgx.NamedArgs{"last_error_class": " ", "last_error_at": now}},
			{"lease owner without deadline", "ready", pgx.NamedArgs{"lease_owner": uuid.New()}},
			{"lease deadline without owner", "ready", pgx.NamedArgs{"lease_until": now}},
			{"negative lease fence", "ready", pgx.NamedArgs{"lease_fence": int64(-1)}},
			{"ready with applied time", "ready", pgx.NamedArgs{"applied_at": now}},
			{"applied missing time", "applied", pgx.NamedArgs{"applied_at": nil}},
			{"quarantined with applied time", "quarantined", pgx.NamedArgs{"applied_at": now}},
			{"applied with lease", "applied", pgx.NamedArgs{"lease_owner": uuid.New(), "lease_until": now}},
			{"quarantined with lease", "quarantined", pgx.NamedArgs{"lease_owner": uuid.New(), "lease_until": now}},
			{"quarantined blank class", "quarantined", pgx.NamedArgs{"quarantine_class": " "}},
			{"quarantined blank detail", "quarantined", pgx.NamedArgs{"quarantine_detail": " "}},
			{"quarantined missing time", "quarantined", pgx.NamedArgs{"quarantined_at": nil}},
			{"ready with quarantine evidence", "ready", pgx.NamedArgs{"quarantine_class": "unexpected", "quarantined_at": now}},
			{"ready with quarantine detail alone", "ready", pgx.NamedArgs{"quarantine_detail": "unexpected"}},
			{"applied with quarantine evidence", "applied", pgx.NamedArgs{"quarantine_class": "unexpected", "quarantined_at": now}},
			{"applied with quarantine detail alone", "applied", pgx.NamedArgs{"quarantine_detail": "unexpected"}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
					parent := r22InsertIntegrityParent(t, ctx, tx, seed)
					args := r22EffectArgs(seed, parent["actor_id"].(uuid.UUID), parent["operation_id"].(uuid.UUID), "livekit_ensure_room", tc.state)
					for key, value := range tc.set {
						args[key] = value
					}
					r22RequireRejectedWrite(t, r22InsertEffect(ctx, tx, args))
				})
			})
		}
	})

	t.Run("A20 denial reference skew deadline and reason", func(t *testing.T) {
		r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
			parent := r22InsertIntegrityParent(t, ctx, tx, seed)
			require.NoError(t, r22InsertDenial(ctx, tx, r22DenialArgs(seed, parent["actor_id"].(uuid.UUID), parent["operation_id"].(uuid.UUID))))
		})
		cases := []struct {
			name string
			set  pgx.NamedArgs
		}{
			{"negative accepted clock skew", pgx.NamedArgs{"clock_skew": -time.Microsecond}},
			{"deadline before expiry plus skew", pgx.NamedArgs{"deny_until": time.Date(2026, 9, 11, 5, 0, 1, 999999000, time.UTC)}},
			{"blank reason", pgx.NamedArgs{"reason": ""}},
			{"whitespace reason", pgx.NamedArgs{"reason": " "}},
			{"operation actor without operation id", pgx.NamedArgs{"operation_id": nil}},
			{"operation id without actor", pgx.NamedArgs{"actor_id": nil}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
					parent := r22InsertIntegrityParent(t, ctx, tx, seed)
					args := r22DenialArgs(seed, parent["actor_id"].(uuid.UUID), parent["operation_id"].(uuid.UUID))
					if tc.name == "deadline before expiry plus skew" {
						args["deny_until"] = args["grant_expires_at"].(time.Time).Add(args["clock_skew"].(time.Duration) - time.Microsecond)
					} else {
						for key, value := range tc.set {
							args[key] = value
						}
					}
					r22RequireRejectedWrite(t, r22InsertDenial(ctx, tx, args))
				})
			})
		}
	})

	t.Run("A21 denial identity deadline delete guard and indexes", func(t *testing.T) {
		immutable := []struct {
			column string
			value  any
		}{
			{"room_id", seed.destinationRoomID}, {"livekit_room_name", "changed-room"},
			{"participant_identity", "changed-participant-identity"},
			{"grant_expires_at", time.Date(2026, 9, 11, 4, 30, 0, 0, time.UTC)},
			{"accepted_clock_skew", time.Second}, {"reason", "move"},
		}
		for _, tc := range immutable {
			t.Run("immutable "+tc.column, func(t *testing.T) {
				r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
					parent := r22InsertIntegrityParent(t, ctx, tx, seed)
					args := r22DenialArgs(seed, parent["actor_id"].(uuid.UUID), parent["operation_id"].(uuid.UUID))
					require.NoError(t, r22InsertDenial(ctx, tx, args))
					command := fmt.Sprintf("UPDATE voice_media_epoch_denials SET %s=$1 WHERE media_epoch=$2", pgx.Identifier{tc.column}.Sanitize())
					_, err := tx.Exec(ctx, command, tc.value, args["media_epoch"])
					r22RequireRejectedWrite(t, err)
				})
			})
		}
		for _, identity := range []string{"media and participant", "profile and participant", "operation reference"} {
			t.Run("immutable "+identity, func(t *testing.T) {
				r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
					parent := r22InsertIntegrityParent(t, ctx, tx, seed)
					args := r22DenialArgs(seed, parent["actor_id"].(uuid.UUID), parent["operation_id"].(uuid.UUID))
					require.NoError(t, r22InsertDenial(ctx, tx, args))
					var err error
					switch identity {
					case "media and participant":
						newMedia := uuid.New()
						newIdentity := "profile:" + seed.subjectProfileID.String() + ":media:" + newMedia.String()
						_, err = tx.Exec(ctx, `UPDATE voice_media_epoch_denials SET media_epoch=$1,participant_identity=$2 WHERE media_epoch=$3`, newMedia, newIdentity, args["media_epoch"])
					case "profile and participant":
						newProfile := uuid.New()
						newIdentity := "profile:" + newProfile.String() + ":media:" + seed.sourceMediaEpoch.String()
						_, err = tx.Exec(ctx, `UPDATE voice_media_epoch_denials SET profile_id=$1,participant_identity=$2 WHERE media_epoch=$3`, newProfile, newIdentity, args["media_epoch"])
					case "operation reference":
						other := r22OperationArgs(seed, "leave", "completed", "left")
						other["subject_id"] = uuid.New()
						require.NoError(t, r22InsertOperation(ctx, tx, other))
						_, err = tx.Exec(ctx, `UPDATE voice_media_epoch_denials SET operation_actor_profile_id=$1,operation_id=$2 WHERE media_epoch=$3`, other["actor_id"], other["operation_id"], args["media_epoch"])
					}
					r22RequireRejectedWrite(t, err)
				})
			})
		}
		r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
			parent := r22InsertIntegrityParent(t, ctx, tx, seed)
			args := r22DenialArgs(seed, parent["actor_id"].(uuid.UUID), parent["operation_id"].(uuid.UUID))
			require.NoError(t, r22InsertDenial(ctx, tx, args))
			later := args["deny_until"].(time.Time).Add(time.Minute)
			_, err := tx.Exec(ctx, `UPDATE voice_media_epoch_denials SET deny_until=$1 WHERE media_epoch=$2`, later, args["media_epoch"])
			require.NoError(t, err, "deny_until may advance")
			_, err = tx.Exec(ctx, `UPDATE voice_media_epoch_denials SET deny_until=$1 WHERE media_epoch=$2`, args["deny_until"], args["media_epoch"])
			r22RequireRejectedWrite(t, err)
		})

		for _, observed := range []bool{false, true} {
			name := "expired without absence observation"
			if observed {
				name = "expired with absence observation"
			}
			t.Run(name, func(t *testing.T) {
				r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
					parent := r22InsertIntegrityParent(t, ctx, tx, seed)
					args := r22DenialArgs(seed, parent["actor_id"].(uuid.UUID), parent["operation_id"].(uuid.UUID))
					past := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
					args["grant_expires_at"], args["deny_until"] = past.Add(-2*time.Second), past
					if observed {
						args["absence_observed_at"] = past
					}
					require.NoError(t, r22InsertDenial(ctx, tx, args))
					_, err := tx.Exec(ctx, `DELETE FROM voice_media_epoch_denials WHERE media_epoch=$1`, args["media_epoch"])
					if observed {
						require.NoError(t, err)
					} else {
						r22RequireRejectedWrite(t, err)
					}
				})
			})
		}
		t.Run("unexpired deletion is refused", func(t *testing.T) {
			r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
				parent := r22InsertIntegrityParent(t, ctx, tx, seed)
				args := r22DenialArgs(seed, parent["actor_id"].(uuid.UUID), parent["operation_id"].(uuid.UUID))
				future := time.Now().UTC().Add(48 * time.Hour)
				args["grant_expires_at"] = future
				args["deny_until"] = future.Add(args["clock_skew"].(time.Duration))
				args["absence_observed_at"] = time.Now().UTC()
				require.NoError(t, r22InsertDenial(ctx, tx, args))
				_, err := tx.Exec(ctx, `DELETE FROM voice_media_epoch_denials WHERE media_epoch=$1`, args["media_epoch"])
				r22RequireRejectedWrite(t, err)
			})
		})
		r22RequireIndexFragments(t, ctx, pool, "voice_media_epoch_denials", []string{"(deny_until,absence_observed_at)", "(room_id,participant_identity)"})
	})

	t.Run("A22 outbox identity ordering subject and version", func(t *testing.T) {
		r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
			parent := r22InsertIntegrityParent(t, ctx, tx, seed)
			first := r22OutboxArgs(seed, parent["actor_id"].(uuid.UUID), parent["operation_id"].(uuid.UUID), "ready")
			first["ordinal"] = int16(0)
			require.NoError(t, r22InsertOutbox(ctx, tx, first))
			duplicateEvent := r22CloneArgs(first)
			r22RequireSQLState(t, r22InsertOutbox(ctx, tx, duplicateEvent), "23505")
		})
		r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
			parent := r22InsertIntegrityParent(t, ctx, tx, seed)
			first := r22OutboxArgs(seed, parent["actor_id"].(uuid.UUID), parent["operation_id"].(uuid.UUID), "ready")
			first["ordinal"] = int16(0)
			require.NoError(t, r22InsertOutbox(ctx, tx, first))
			duplicateOrdinal := r22OutboxArgs(seed, parent["actor_id"].(uuid.UUID), parent["operation_id"].(uuid.UUID), "ready")
			duplicateOrdinal["ordinal"] = int16(0)
			r22RequireSQLState(t, r22InsertOutbox(ctx, tx, duplicateOrdinal), "23505")
		})

		cases := []struct {
			name   string
			column string
			value  any
		}{
			{"negative ordinal", "ordinal", int16(-1)},
			{"blank subject", "subject", " "},
			{"foreign namespace", "subject", "chat.test.v1"},
			{"bare voice namespace", "subject", "voice."},
			{"zero schema", "schema_version", int16(0)},
			{"negative schema", "schema_version", int16(-1)},
			{"negative roster", "roster_version", int64(-1)},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
					parent := r22InsertIntegrityParent(t, ctx, tx, seed)
					args := r22OutboxArgs(seed, parent["actor_id"].(uuid.UUID), parent["operation_id"].(uuid.UUID), "ready")
					args[tc.column] = tc.value
					r22RequireRejectedWrite(t, r22InsertOutbox(ctx, tx, args))
				})
			})
		}
		r22RequireUniqueConstraint(t, ctx, pool, "voice_event_outbox", "actor_profile_id,operation_id,ordinal")
	})

	t.Run("A23 outbox retry lease terminal and quarantine evidence", func(t *testing.T) {
		now := time.Date(2026, 9, 11, 8, 0, 0, 0, time.UTC)
		for _, state := range []string{"ready", "delivered", "quarantined"} {
			t.Run(state+" valid control", func(t *testing.T) {
				r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
					parent := r22InsertIntegrityParent(t, ctx, tx, seed)
					require.NoError(t, r22InsertOutbox(ctx, tx, r22OutboxArgs(seed, parent["actor_id"].(uuid.UUID), parent["operation_id"].(uuid.UUID), state)))
				})
			})
		}
		cases := []struct {
			name  string
			state string
			set   pgx.NamedArgs
		}{
			{"unknown state", "ready", pgx.NamedArgs{"state": "unknown"}},
			{"negative attempts", "ready", pgx.NamedArgs{"attempt_count": int64(-1)}},
			{"error class without time", "ready", pgx.NamedArgs{"last_error_class": "timeout"}},
			{"error time without class", "ready", pgx.NamedArgs{"last_error_at": now}},
			{"blank error class", "ready", pgx.NamedArgs{"last_error_class": " ", "last_error_at": now}},
			{"lease owner without deadline", "ready", pgx.NamedArgs{"lease_owner": uuid.New()}},
			{"lease deadline without owner", "ready", pgx.NamedArgs{"lease_until": now}},
			{"negative lease fence", "ready", pgx.NamedArgs{"lease_fence": int64(-1)}},
			{"ready with delivered time", "ready", pgx.NamedArgs{"delivered_at": now}},
			{"delivered missing time", "delivered", pgx.NamedArgs{"delivered_at": nil}},
			{"quarantined with delivered time", "quarantined", pgx.NamedArgs{"delivered_at": now}},
			{"delivered with lease", "delivered", pgx.NamedArgs{"lease_owner": uuid.New(), "lease_until": now}},
			{"quarantined with lease", "quarantined", pgx.NamedArgs{"lease_owner": uuid.New(), "lease_until": now}},
			{"quarantined blank class", "quarantined", pgx.NamedArgs{"quarantine_class": " "}},
			{"quarantined blank detail", "quarantined", pgx.NamedArgs{"quarantine_detail": " "}},
			{"quarantined missing time", "quarantined", pgx.NamedArgs{"quarantined_at": nil}},
			{"ready with quarantine evidence", "ready", pgx.NamedArgs{"quarantine_class": "unexpected", "quarantined_at": now}},
			{"ready with quarantine detail alone", "ready", pgx.NamedArgs{"quarantine_detail": "unexpected"}},
			{"delivered with quarantine evidence", "delivered", pgx.NamedArgs{"quarantine_class": "unexpected", "quarantined_at": now}},
			{"delivered with quarantine detail alone", "delivered", pgx.NamedArgs{"quarantine_detail": "unexpected"}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
					parent := r22InsertIntegrityParent(t, ctx, tx, seed)
					args := r22OutboxArgs(seed, parent["actor_id"].(uuid.UUID), parent["operation_id"].(uuid.UUID), tc.state)
					for key, value := range tc.set {
						args[key] = value
					}
					r22RequireRejectedWrite(t, r22InsertOutbox(ctx, tx, args))
				})
			})
		}
	})

	t.Run("A24 direct SQL transition immutability and monotonic guards", func(t *testing.T) {
		r22TestRoomAndMembershipGuards(t, ctx, pool, seed)
		r22TestOperationGuards(t, ctx, pool, seed)
		r22TestEffectGuards(t, ctx, pool, seed)
		r22TestOutboxGuards(t, ctx, pool, seed)
		for _, trigger := range [][2]string{
			{"voice_lifecycle_operations", "voice_lifecycle_operation_transition_guard"},
			{"voice_lifecycle_effects", "voice_lifecycle_effect_transition_guard"},
			{"voice_event_outbox", "voice_event_outbox_transition_guard"},
		} {
			t.Run(trigger[1]+" exists", func(t *testing.T) {
				r22RequireTrigger(t, ctx, pool, trigger[0], trigger[1])
			})
		}
	})

	t.Run("exact plan object names", func(t *testing.T) {
		var bindingShape, oldBindingShape int
		require.NoError(t, pool.QueryRow(ctx, `
SELECT count(*) FILTER (WHERE conname='voice_lifecycle_operations_binding_shape_check'),
       count(*) FILTER (WHERE conname='voice_lifecycle_operations_decision_shape_check')
FROM pg_constraint c
JOIN pg_class r ON r.oid=c.conrelid
JOIN pg_namespace n ON n.oid=r.relnamespace
WHERE n.nspname=current_schema() AND r.relname='voice_lifecycle_operations'`).Scan(&bindingShape, &oldBindingShape))
		require.Equal(t, 1, bindingShape)
		require.Zero(t, oldBindingShape)

		var generationTrigger, oldExpiryTrigger, generationFunction, oldExpiryFunction int
		require.NoError(t, pool.QueryRow(ctx, `
SELECT
 (SELECT count(*) FROM pg_trigger t JOIN pg_class r ON r.oid=t.tgrelid JOIN pg_namespace n ON n.oid=r.relnamespace
  WHERE n.nspname=current_schema() AND r.relname='voice_room_memberships'
    AND t.tgname='voice_room_membership_generation_guard' AND NOT t.tgisinternal),
 (SELECT count(*) FROM pg_trigger t JOIN pg_class r ON r.oid=t.tgrelid JOIN pg_namespace n ON n.oid=r.relnamespace
  WHERE n.nspname=current_schema() AND r.relname='voice_room_memberships'
    AND t.tgname='voice_room_membership_expiry_guard' AND NOT t.tgisinternal),
 (SELECT count(*) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
  WHERE n.nspname=current_schema() AND p.proname='voice_room_membership_generation_guard_fn'),
 (SELECT count(*) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
  WHERE n.nspname=current_schema() AND p.proname='voice_room_membership_expiry_guard_fn')`).Scan(
			&generationTrigger, &oldExpiryTrigger, &generationFunction, &oldExpiryFunction))
		require.Equal(t, 1, generationTrigger)
		require.Zero(t, oldExpiryTrigger)
		require.Equal(t, 1, generationFunction)
		require.Zero(t, oldExpiryFunction)
	})
}

func r22RequireIndexFragments(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table string, fragments []string) {
	t.Helper()
	rows, err := pool.Query(ctx, `SELECT indexdef FROM pg_indexes WHERE schemaname=current_schema() AND tablename=$1`, table)
	require.NoError(t, err)
	defer rows.Close()
	var definitions []string
	for rows.Next() {
		var definition string
		require.NoError(t, rows.Scan(&definition))
		definitions = append(definitions, r22NormalizedSQL(definition))
	}
	require.NoError(t, rows.Err())
	for _, fragment := range fragments {
		t.Run(table+" index "+fragment, func(t *testing.T) {
			require.True(t, r22ContainsSQL(definitions, fragment), "missing %s index containing %s", table, fragment)
		})
	}
}

func r22RequireUniqueConstraint(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table, expectedColumns string) {
	t.Helper()
	t.Run(table+" exact operation ordinal unique key", func(t *testing.T) {
		rows, err := pool.Query(ctx, `
SELECT string_agg(attribute.attname,',' ORDER BY key.ord)
FROM pg_constraint constraint_row
JOIN pg_class relation ON relation.oid=constraint_row.conrelid
JOIN pg_namespace namespace ON namespace.oid=relation.relnamespace
JOIN unnest(constraint_row.conkey) WITH ORDINALITY AS key(attnum,ord) ON true
JOIN pg_attribute attribute ON attribute.attrelid=relation.oid AND attribute.attnum=key.attnum
WHERE namespace.nspname=current_schema() AND relation.relname=$1 AND constraint_row.contype='u'
GROUP BY constraint_row.oid`, table)
		require.NoError(t, err)
		defer rows.Close()
		var keys []string
		for rows.Next() {
			var columns string
			require.NoError(t, rows.Scan(&columns))
			keys = append(keys, columns)
		}
		require.NoError(t, rows.Err())
		require.Contains(t, keys, expectedColumns)
	})
}

func r22RequireTrigger(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table, trigger string) {
	t.Helper()
	var count int
	err := pool.QueryRow(ctx, `
SELECT count(*) FROM pg_trigger trigger
JOIN pg_class relation ON relation.oid=trigger.tgrelid
JOIN pg_namespace namespace ON namespace.oid=relation.relnamespace
WHERE namespace.nspname=current_schema() AND relation.relname=$1
  AND trigger.tgname=$2 AND NOT trigger.tgisinternal`, table, trigger).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func r22TestRoomAndMembershipGuards(t *testing.T, ctx context.Context, pool *pgxpool.Pool, seed r22IntegritySeed) {
	t.Helper()
	for _, tc := range []struct {
		column string
		value  any
	}{
		{"room_id", uuid.New()},
		{"space_id", uuid.New()},
		{"voice_room_id", uuid.New()},
		{"livekit_room_name", "changed-livekit-room"},
	} {
		t.Run("room identity "+tc.column+" is immutable", func(t *testing.T) {
			r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
				command := fmt.Sprintf("UPDATE voice_room_instances SET %s=$1 WHERE room_id=$2", pgx.Identifier{tc.column}.Sanitize())
				_, err := tx.Exec(ctx, command, tc.value, seed.sourceRoomID)
				r22RequireRejectedWrite(t, err)
			})
		})
	}

	t.Run("membership grant expiry advances but never shortens", func(t *testing.T) {
		r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
			profileID := uuid.New()
			_, err := tx.Exec(ctx, `
INSERT INTO voice_room_memberships(
 profile_id,room_id,media_epoch,space_access_epoch,role_policy_epoch,authorization_digest,
 can_join,can_publish_audio,can_publish_video,can_publish_screen_share,can_subscribe,
 can_mute_others,can_deafen_others,can_move_others,can_use_ptt,priority_speaker,
 latest_grant_expires_at,joined_at,updated_at
) VALUES($1,$2,$3,1,1,$4,true,false,true,false,true,false,true,false,true,false,NULL,$5,$5)`,
				profileID, seed.sourceRoomID, uuid.New(), bytesOfLength(32), time.Date(2026, 9, 11, 9, 0, 0, 0, time.UTC))
			require.NoError(t, err)
			first := time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)
			second := first.Add(time.Hour)
			_, err = tx.Exec(ctx, `UPDATE voice_room_memberships SET latest_grant_expires_at=$1,updated_at=$1 WHERE profile_id=$2`, first, profileID)
			require.NoError(t, err)
			_, err = tx.Exec(ctx, `UPDATE voice_room_memberships SET latest_grant_expires_at=$1,updated_at=$1 WHERE profile_id=$2`, second, profileID)
			require.NoError(t, err)
			_, err = tx.Exec(ctx, `UPDATE voice_room_memberships SET latest_grant_expires_at=$1,updated_at=$2 WHERE profile_id=$3`, first, second.Add(time.Minute), profileID)
			r22RequireSQLState(t, err, "P0001")
		})
	})
	t.Run("membership grant expiry cannot return from nonnull to null", func(t *testing.T) {
		r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
			profileID := uuid.New()
			expiresAt := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
			_, err := tx.Exec(ctx, `
INSERT INTO voice_room_memberships(
 profile_id,room_id,media_epoch,space_access_epoch,role_policy_epoch,authorization_digest,
 can_join,can_publish_audio,can_publish_video,can_publish_screen_share,can_subscribe,
 can_mute_others,can_deafen_others,can_move_others,can_use_ptt,priority_speaker,
 latest_grant_expires_at,joined_at,updated_at
) VALUES($1,$2,$3,1,1,$4,true,false,true,false,true,false,true,false,true,false,$5,$6,$6)`,
				profileID, seed.sourceRoomID, uuid.New(), bytesOfLength(32), expiresAt, expiresAt.Add(-time.Hour))
			require.NoError(t, err)
			_, err = tx.Exec(ctx, `UPDATE voice_room_memberships SET latest_grant_expires_at=NULL,updated_at=$1 WHERE profile_id=$2`, expiresAt.Add(time.Hour), profileID)
			r22RequireSQLState(t, err, "P0001")
		})
	})
	t.Run("membership updated_at cannot move backward", func(t *testing.T) {
		r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
			profileID := uuid.New()
			updatedAt := time.Date(2026, 9, 11, 9, 0, 0, 0, time.UTC)
			_, err := tx.Exec(ctx, `
INSERT INTO voice_room_memberships(
 profile_id,room_id,media_epoch,space_access_epoch,role_policy_epoch,authorization_digest,
 can_join,can_publish_audio,can_publish_video,can_publish_screen_share,can_subscribe,
 can_mute_others,can_deafen_others,can_move_others,can_use_ptt,priority_speaker,
 latest_grant_expires_at,joined_at,updated_at
) VALUES($1,$2,$3,1,1,$4,true,false,true,false,true,false,true,false,true,false,NULL,$5,$5)`,
				profileID, seed.sourceRoomID, uuid.New(), bytesOfLength(32), updatedAt)
			require.NoError(t, err)
			_, err = tx.Exec(ctx, `UPDATE voice_room_memberships SET updated_at=$1 WHERE profile_id=$2`, updatedAt.Add(-time.Second), profileID)
			r22RequireSQLState(t, err, "P0001")
		})
	})

	insertMembership := func(t *testing.T, tx pgx.Tx, profileID, mediaEpoch uuid.UUID, expiresAt any) {
		t.Helper()
		_, err := tx.Exec(ctx, `
INSERT INTO voice_room_memberships(
 profile_id,room_id,media_epoch,space_access_epoch,role_policy_epoch,authorization_digest,
 can_join,can_publish_audio,can_publish_video,can_publish_screen_share,can_subscribe,
 can_mute_others,can_deafen_others,can_move_others,can_use_ptt,priority_speaker,
 latest_grant_expires_at,joined_at,updated_at
) VALUES($1,$2,$3,1,1,$4,true,false,true,false,true,false,true,false,true,false,$5,$6,$6)`,
			profileID, seed.sourceRoomID, mediaEpoch, bytesOfLength(32), expiresAt,
			time.Date(2026, 9, 11, 9, 0, 0, 0, time.UTC))
		require.NoError(t, err)
	}

	for _, tc := range []struct {
		name   string
		column string
		value  any
	}{
		{"profile", "profile_id", uuid.New()},
		{"room", "room_id", seed.destinationRoomID},
		{"Space epoch", "space_access_epoch", int64(2)},
		{"Role epoch", "role_policy_epoch", int64(2)},
		{"authorization digest", "authorization_digest", []byte(strings.Repeat("z", 32))},
		{"can join", "can_join", false},
		{"can publish audio", "can_publish_audio", true},
		{"can publish video", "can_publish_video", false},
		{"can publish screen share", "can_publish_screen_share", true},
		{"can subscribe", "can_subscribe", false},
		{"can mute others", "can_mute_others", true},
		{"can deafen others", "can_deafen_others", false},
		{"can move others", "can_move_others", true},
		{"can use ptt", "can_use_ptt", false},
		{"priority speaker", "priority_speaker", true},
		{"joined at", "joined_at", time.Date(2026, 9, 11, 9, 1, 0, 0, time.UTC)},
	} {
		t.Run("same media epoch rejects changed "+tc.name, func(t *testing.T) {
			r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
				profileID, mediaEpoch := uuid.New(), uuid.New()
				insertMembership(t, tx, profileID, mediaEpoch, nil)
				command := fmt.Sprintf("UPDATE voice_room_memberships SET %s=$1,updated_at=$2 WHERE profile_id=$3", pgx.Identifier{tc.column}.Sanitize())
				_, err := tx.Exec(ctx, command, tc.value, time.Date(2026, 9, 11, 9, 2, 0, 0, time.UTC), profileID)
				r22RequireSQLState(t, err, "P0001")
			})
		})
	}

	t.Run("new media epoch rejects a prepopulated grant expiry", func(t *testing.T) {
		r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
			profileID, mediaEpoch := uuid.New(), uuid.New()
			insertMembership(t, tx, profileID, mediaEpoch, nil)
			_, err := tx.Exec(ctx, `
UPDATE voice_room_memberships SET
 media_epoch=$1,space_access_epoch=2,role_policy_epoch=3,authorization_digest=$2,
 can_join=false,can_publish_audio=true,can_publish_video=false,can_publish_screen_share=true,
 can_subscribe=false,can_mute_others=true,can_deafen_others=false,can_move_others=true,
 can_use_ptt=false,priority_speaker=true,latest_grant_expires_at=$3,joined_at=$4,updated_at=$4
WHERE profile_id=$5`, uuid.New(), []byte(strings.Repeat("z", 32)),
				time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC),
				time.Date(2026, 9, 11, 9, 5, 0, 0, time.UTC), profileID)
			r22RequireSQLState(t, err, "P0001")
		})
	})

	t.Run("full same-room generation replacement starts with null expiry", func(t *testing.T) {
		r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
			profileID, mediaEpoch, destinationMediaEpoch := uuid.New(), uuid.New(), uuid.New()
			oldExpiry := time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)
			insertMembership(t, tx, profileID, mediaEpoch, oldExpiry)
			joinedAt := time.Date(2026, 9, 11, 9, 5, 0, 0, time.UTC)
			_, err := tx.Exec(ctx, `
UPDATE voice_room_memberships SET
 media_epoch=$1,space_access_epoch=2,role_policy_epoch=3,authorization_digest=$2,
 can_join=false,can_publish_audio=true,can_publish_video=false,can_publish_screen_share=true,
 can_subscribe=false,can_mute_others=true,can_deafen_others=false,can_move_others=true,
 can_use_ptt=false,priority_speaker=true,latest_grant_expires_at=NULL,joined_at=$3,updated_at=$3
WHERE profile_id=$4`, destinationMediaEpoch, []byte(strings.Repeat("z", 32)), joinedAt, profileID)
			require.NoError(t, err)
			var gotRoom, gotMedia uuid.UUID
			var gotExpiry *time.Time
			require.NoError(t, tx.QueryRow(ctx, `SELECT room_id,media_epoch,latest_grant_expires_at FROM voice_room_memberships WHERE profile_id=$1`, profileID).
				Scan(&gotRoom, &gotMedia, &gotExpiry))
			require.Equal(t, seed.sourceRoomID, gotRoom, "generic generation replacement may reauthorize within one room")
			require.Equal(t, destinationMediaEpoch, gotMedia)
			require.Nil(t, gotExpiry)
			var fullSnapshot bool
			require.NoError(t, tx.QueryRow(ctx, `
SELECT room_id=$2 AND media_epoch=$3 AND space_access_epoch=2 AND role_policy_epoch=3
 AND authorization_digest=$4 AND can_join=false AND can_publish_audio=true
 AND can_publish_video=false AND can_publish_screen_share=true AND can_subscribe=false
 AND can_mute_others=true AND can_deafen_others=false AND can_move_others=true
 AND can_use_ptt=false AND priority_speaker=true AND latest_grant_expires_at IS NULL
 AND joined_at=$5 AND updated_at=$5
FROM voice_room_memberships WHERE profile_id=$1`, profileID, seed.sourceRoomID, destinationMediaEpoch,
				[]byte(strings.Repeat("z", 32)), joinedAt).Scan(&fullSnapshot))
			require.True(t, fullSnapshot, "the replacement must persist one complete generation snapshot")
		})
	})
}

func r22TestOperationGuards(t *testing.T, ctx context.Context, pool *pgxpool.Pool, seed r22IntegritySeed) {
	t.Helper()
	immutableDecision := []struct {
		column string
		value  any
	}{
		{"actor_profile_id", uuid.New()}, {"operation_id", uuid.New()}, {"schema_version", int16(2)},
		{"fingerprint", []byte(strings.Repeat("z", 32))},
		{"binding_bytes", []byte("changed-binding")}, {"binding_hash", []byte(strings.Repeat("z", 32))},
		{"actor_account_id", uuid.New()}, {"subject_profile_id", uuid.New()}, {"space_id", uuid.New()},
		{"source_voice_room_id", seed.alternateVoiceID}, {"destination_voice_room_id", seed.alternateVoiceID},
		{"source_room_id", seed.alternateRoomID}, {"destination_room_id", seed.alternateRoomID},
		{"source_media_epoch", uuid.New()}, {"destination_media_epoch", uuid.New()},
		{"space_access_epoch", int64(99)}, {"subject_role_policy_epoch", int64(99)},
		{"actor_source_role_policy_epoch", int64(99)}, {"actor_destination_role_policy_epoch", int64(99)},
		{"authorization_digest", []byte(strings.Repeat("z", 32))}, {"can_join", false},
		{"can_publish_audio", true}, {"can_publish_video", false}, {"can_publish_screen_share", true},
		{"can_subscribe", false}, {"can_mute_others", true}, {"can_deafen_others", false},
		{"can_move_others", true}, {"can_use_ptt", false}, {"priority_speaker", true},
		{"actor_can_move_source", false}, {"actor_can_move_destination", false},
		{"decided_at", time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)},
		{"created_at", time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)},
		{"redis_owner_token", []byte(strings.Repeat("z", 32))},
	}
	for _, tc := range immutableDecision {
		t.Run("operation decision "+tc.column+" is immutable", func(t *testing.T) {
			r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
				args := r22OperationArgs(seed, "moderator_move", "decided", "")
				require.NoError(t, r22InsertOperation(ctx, tx, args))
				command := fmt.Sprintf("UPDATE voice_lifecycle_operations SET %s=$1 WHERE actor_profile_id=$2 AND operation_id=$3", pgx.Identifier{tc.column}.Sanitize())
				_, err := tx.Exec(ctx, command, tc.value, args["actor_id"], args["operation_id"])
				r22RequireRejectedWrite(t, err)
			})
		})
	}
	for _, tc := range []struct {
		column string
		value  any
	}{
		{"source_voice_room_id", seed.alternateVoiceID},
		{"source_room_id", seed.sourceRoomID},
		{"source_media_epoch", seed.sourceMediaEpoch},
	} {
		t.Run("completed absent leave "+tc.column+" is immutable", func(t *testing.T) {
			r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
				args := r22OperationArgs(seed, "leave", "completed", "no_op")
				require.NoError(t, r22InsertOperation(ctx, tx, args))
				command := fmt.Sprintf("UPDATE voice_lifecycle_operations SET %s=$1 WHERE actor_profile_id=$2 AND operation_id=$3", pgx.Identifier{tc.column}.Sanitize())
				_, err := tx.Exec(ctx, command, tc.value, args["actor_id"], args["operation_id"])
				r22RequireSQLState(t, err, "P0001")
			})
		})
	}
	t.Run("operation method and method-specific actor evidence are immutable as one valid conversion", func(t *testing.T) {
		r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
			args := r22OperationArgs(seed, "self_move", "decided", "")
			require.NoError(t, r22InsertOperation(ctx, tx, args))
			_, err := tx.Exec(ctx, `
UPDATE voice_lifecycle_operations
SET method='moderator_move',actor_source_role_policy_epoch=43,actor_destination_role_policy_epoch=44,
    actor_can_move_source=true,actor_can_move_destination=true
WHERE actor_profile_id=$1 AND operation_id=$2`, args["actor_id"], args["operation_id"])
			r22RequireRejectedWrite(t, err)
		})
	})

	completedReceipt := []struct {
		column string
		value  any
	}{
		{"receipt_source_voice_room_id", seed.alternateVoiceID},
		{"receipt_destination_voice_room_id", seed.alternateVoiceID}, {"receipt_room_id", seed.alternateRoomID},
		{"receipt_source_roster_version", int64(1)}, {"receipt_destination_roster_version", int64(10)},
		{"receipt_media_epoch", uuid.New()}, {"receipt_space_access_epoch", int64(99)},
		{"receipt_role_policy_epoch", int64(99)}, {"receipt_authorization_digest", []byte(strings.Repeat("z", 32))},
		{"receipt_bytes", []byte("changed-receipt")}, {"receipt_hash", []byte(strings.Repeat("z", 32))},
		{"completed_at", time.Date(2026, 9, 11, 1, 0, 0, 0, time.UTC)},
		{"replay_until", time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)},
	}
	for _, tc := range completedReceipt {
		t.Run("completed receipt "+tc.column+" is immutable", func(t *testing.T) {
			r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
				args := r22OperationArgs(seed, "moderator_move", "completed", "moved")
				require.NoError(t, r22InsertOperation(ctx, tx, args))
				command := fmt.Sprintf("UPDATE voice_lifecycle_operations SET %s=$1 WHERE actor_profile_id=$2 AND operation_id=$3", pgx.Identifier{tc.column}.Sanitize())
				_, err := tx.Exec(ctx, command, tc.value, args["actor_id"], args["operation_id"])
				r22RequireRejectedWrite(t, err)
			})
		})
	}
	t.Run("completed receipt outcome is immutable across two valid join outcomes", func(t *testing.T) {
		r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
			args := r22OperationArgs(seed, "join", "completed", "joined")
			require.NoError(t, r22InsertOperation(ctx, tx, args))
			_, err := tx.Exec(ctx, `
UPDATE voice_lifecycle_operations SET receipt_outcome='no_op'
WHERE actor_profile_id=$1 AND operation_id=$2`, args["actor_id"], args["operation_id"])
			r22RequireRejectedWrite(t, err)
		})
	})

	for _, state := range []string{"completed", "quarantined"} {
		t.Run(state+" operation cannot regress", func(t *testing.T) {
			r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
				outcome := ""
				if state == "completed" {
					outcome = "joined"
				}
				args := r22OperationArgs(seed, "join", state, outcome)
				require.NoError(t, r22InsertOperation(ctx, tx, args))
				var err error
				if state == "completed" {
					_, err = tx.Exec(ctx, `
UPDATE voice_lifecycle_operations
SET state='decided',receipt_outcome=NULL,receipt_source_voice_room_id=NULL,
    receipt_destination_voice_room_id=NULL,receipt_room_id=NULL,
    receipt_source_roster_version=NULL,receipt_destination_roster_version=NULL,
    receipt_media_epoch=NULL,receipt_space_access_epoch=NULL,receipt_role_policy_epoch=NULL,
    receipt_authorization_digest=NULL,receipt_bytes=NULL,receipt_hash=NULL,
    completed_at=NULL,replay_until=NULL
WHERE actor_profile_id=$1 AND operation_id=$2`, args["actor_id"], args["operation_id"])
				} else {
					_, err = tx.Exec(ctx, `
UPDATE voice_lifecycle_operations
SET state='decided',quarantine_class=NULL,quarantine_detail=NULL,quarantined_at=NULL
WHERE actor_profile_id=$1 AND operation_id=$2`, args["actor_id"], args["operation_id"])
				}
				r22RequireRejectedWrite(t, err)
			})
		})
	}

	t.Run("decided same state lease update is allowed", func(t *testing.T) {
		r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
			args := r22OperationArgs(seed, "join", "decided", "")
			require.NoError(t, r22InsertOperation(ctx, tx, args))
			_, err := tx.Exec(ctx, `
UPDATE voice_lifecycle_operations
SET lease_owner=$1,lease_until=$2,lease_fence=lease_fence+1,updated_at=$3
WHERE actor_profile_id=$4 AND operation_id=$5`, uuid.New(), time.Now().UTC().Add(time.Minute), time.Now().UTC(), args["actor_id"], args["operation_id"])
			require.NoError(t, err)
		})
	})
}

func r22TestEffectGuards(t *testing.T, ctx context.Context, pool *pgxpool.Pool, seed r22IntegritySeed) {
	t.Helper()
	immutable := []struct {
		column string
		value  any
	}{
		{"effect_id", uuid.New()},
		{"ordinal", int16(2)}, {"schema_version", int16(2)},
		{"target_voice_room_id", seed.alternateVoiceID}, {"target_livekit_room_name", "changed-room"},
		{"request_bytes", []byte("changed-request")},
		{"request_digest", []byte(strings.Repeat("z", 32))},
	}
	for _, tc := range immutable {
		t.Run("effect "+tc.column+" is immutable", func(t *testing.T) {
			r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
				parent := r22InsertIntegrityParent(t, ctx, tx, seed)
				args := r22EffectArgs(seed, parent["actor_id"].(uuid.UUID), parent["operation_id"].(uuid.UUID), "livekit_eject_participant", "ready")
				require.NoError(t, r22InsertEffect(ctx, tx, args))
				command := fmt.Sprintf("UPDATE voice_lifecycle_effects SET %s=$1 WHERE effect_id=$2", pgx.Identifier{tc.column}.Sanitize())
				_, err := tx.Exec(ctx, command, tc.value, args["effect_id"])
				r22RequireRejectedWrite(t, err)
			})
		})
	}
	for _, identity := range []string{"operation reference", "profile and participant", "media and participant"} {
		t.Run("effect "+identity+" is immutable", func(t *testing.T) {
			r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
				parent := r22InsertIntegrityParent(t, ctx, tx, seed)
				args := r22EffectArgs(seed, parent["actor_id"].(uuid.UUID), parent["operation_id"].(uuid.UUID), "livekit_eject_participant", "ready")
				require.NoError(t, r22InsertEffect(ctx, tx, args))
				var err error
				switch identity {
				case "operation reference":
					other := r22OperationArgs(seed, "leave", "completed", "left")
					other["subject_id"] = uuid.New()
					require.NoError(t, r22InsertOperation(ctx, tx, other))
					_, err = tx.Exec(ctx, `UPDATE voice_lifecycle_effects SET actor_profile_id=$1,operation_id=$2 WHERE effect_id=$3`, other["actor_id"], other["operation_id"], args["effect_id"])
				case "profile and participant":
					profileID := uuid.New()
					identityValue := "profile:" + profileID.String() + ":media:" + seed.sourceMediaEpoch.String()
					_, err = tx.Exec(ctx, `UPDATE voice_lifecycle_effects SET target_profile_id=$1,target_participant_identity=$2 WHERE effect_id=$3`, profileID, identityValue, args["effect_id"])
				case "media and participant":
					mediaEpoch := uuid.New()
					identityValue := "profile:" + seed.subjectProfileID.String() + ":media:" + mediaEpoch.String()
					_, err = tx.Exec(ctx, `UPDATE voice_lifecycle_effects SET target_media_epoch=$1,target_participant_identity=$2 WHERE effect_id=$3`, mediaEpoch, identityValue, args["effect_id"])
				}
				r22RequireRejectedWrite(t, err)
			})
		})
	}
	t.Run("effect kind and exact target remain immutable together", func(t *testing.T) {
		r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
			parent := r22InsertIntegrityParent(t, ctx, tx, seed)
			args := r22EffectArgs(seed, parent["actor_id"].(uuid.UUID), parent["operation_id"].(uuid.UUID), "livekit_ensure_room", "ready")
			require.NoError(t, r22InsertEffect(ctx, tx, args))
			_, err := tx.Exec(ctx, `
UPDATE voice_lifecycle_effects
SET kind='livekit_eject_participant',target_voice_room_id=$1,target_livekit_room_name=$2,
    target_profile_id=$3,target_participant_identity=$4,target_media_epoch=$5
WHERE effect_id=$6`, seed.sourceVoiceRoomID, seed.sourceLiveKitRoom, seed.subjectProfileID,
				seed.participantIdentity, seed.sourceMediaEpoch, args["effect_id"])
			r22RequireRejectedWrite(t, err)
		})
	})

	for _, state := range []string{"applied", "quarantined"} {
		t.Run(state+" effect cannot regress", func(t *testing.T) {
			r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
				parent := r22InsertIntegrityParent(t, ctx, tx, seed)
				args := r22EffectArgs(seed, parent["actor_id"].(uuid.UUID), parent["operation_id"].(uuid.UUID), "livekit_ensure_room", state)
				require.NoError(t, r22InsertEffect(ctx, tx, args))
				var err error
				if state == "applied" {
					_, err = tx.Exec(ctx, `UPDATE voice_lifecycle_effects SET state='ready',applied_at=NULL WHERE effect_id=$1`, args["effect_id"])
				} else {
					_, err = tx.Exec(ctx, `
UPDATE voice_lifecycle_effects
SET state='ready',quarantine_class=NULL,quarantine_detail=NULL,quarantined_at=NULL
WHERE effect_id=$1`, args["effect_id"])
				}
				r22RequireRejectedWrite(t, err)
			})
		})
	}
	for _, state := range []string{"applied", "quarantined"} {
		t.Run(state+" effect rejects same-state retry mutation", func(t *testing.T) {
			r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
				parent := r22InsertIntegrityParent(t, ctx, tx, seed)
				args := r22EffectArgs(seed, parent["actor_id"].(uuid.UUID), parent["operation_id"].(uuid.UUID), "livekit_ensure_room", state)
				require.NoError(t, r22InsertEffect(ctx, tx, args))
				now := time.Now().UTC()
				_, err := tx.Exec(ctx, `
UPDATE voice_lifecycle_effects
SET attempt_count=attempt_count+1,last_error_class='late_retry',last_error_at=$1,
    next_attempt_at=$2,updated_at=$1
WHERE effect_id=$3`, now, now.Add(time.Minute), args["effect_id"])
				r22RequireRejectedWrite(t, err)
			})
		})
	}

	t.Run("ready effect same state retry and lease updates are allowed", func(t *testing.T) {
		r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
			parent := r22InsertIntegrityParent(t, ctx, tx, seed)
			args := r22EffectArgs(seed, parent["actor_id"].(uuid.UUID), parent["operation_id"].(uuid.UUID), "livekit_ensure_room", "ready")
			require.NoError(t, r22InsertEffect(ctx, tx, args))
			now := time.Now().UTC()
			_, err := tx.Exec(ctx, `
UPDATE voice_lifecycle_effects
SET attempt_count=1,last_error_class='timeout',last_error_at=$1,next_attempt_at=$2,
    lease_owner=$3,lease_until=$2,lease_fence=1,updated_at=$1
WHERE effect_id=$4`, now, now.Add(time.Minute), uuid.New(), args["effect_id"])
			require.NoError(t, err)
		})
	})
}

func r22TestOutboxGuards(t *testing.T, ctx context.Context, pool *pgxpool.Pool, seed r22IntegritySeed) {
	t.Helper()
	immutable := []struct {
		column string
		value  any
	}{
		{"event_id", uuid.New()},
		{"ordinal", int16(2)}, {"subject", "voice.test.changed.v1"}, {"schema_version", int16(2)},
		{"payload_bytes", []byte("changed-payload")}, {"payload_hash", []byte(strings.Repeat("z", 32))},
		{"subject_profile_id", uuid.New()}, {"room_id", seed.sourceRoomID},
		{"voice_room_id", seed.sourceVoiceRoomID}, {"space_id", uuid.New()}, {"roster_version", int64(10)},
	}
	for _, tc := range immutable {
		t.Run("outbox "+tc.column+" is immutable", func(t *testing.T) {
			r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
				parent := r22InsertIntegrityParent(t, ctx, tx, seed)
				args := r22OutboxArgs(seed, parent["actor_id"].(uuid.UUID), parent["operation_id"].(uuid.UUID), "ready")
				require.NoError(t, r22InsertOutbox(ctx, tx, args))
				command := fmt.Sprintf("UPDATE voice_event_outbox SET %s=$1 WHERE event_id=$2", pgx.Identifier{tc.column}.Sanitize())
				_, err := tx.Exec(ctx, command, tc.value, args["event_id"])
				r22RequireRejectedWrite(t, err)
			})
		})
	}
	t.Run("outbox operation reference is immutable as a valid pair", func(t *testing.T) {
		r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
			parent := r22InsertIntegrityParent(t, ctx, tx, seed)
			args := r22OutboxArgs(seed, parent["actor_id"].(uuid.UUID), parent["operation_id"].(uuid.UUID), "ready")
			require.NoError(t, r22InsertOutbox(ctx, tx, args))
			other := r22OperationArgs(seed, "leave", "completed", "left")
			other["subject_id"] = uuid.New()
			require.NoError(t, r22InsertOperation(ctx, tx, other))
			_, err := tx.Exec(ctx, `UPDATE voice_event_outbox SET actor_profile_id=$1,operation_id=$2 WHERE event_id=$3`, other["actor_id"], other["operation_id"], args["event_id"])
			r22RequireRejectedWrite(t, err)
		})
	})

	for _, state := range []string{"delivered", "quarantined"} {
		t.Run(state+" outbox cannot regress", func(t *testing.T) {
			r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
				parent := r22InsertIntegrityParent(t, ctx, tx, seed)
				args := r22OutboxArgs(seed, parent["actor_id"].(uuid.UUID), parent["operation_id"].(uuid.UUID), state)
				require.NoError(t, r22InsertOutbox(ctx, tx, args))
				var err error
				if state == "delivered" {
					_, err = tx.Exec(ctx, `UPDATE voice_event_outbox SET state='ready',delivered_at=NULL WHERE event_id=$1`, args["event_id"])
				} else {
					_, err = tx.Exec(ctx, `
UPDATE voice_event_outbox
SET state='ready',quarantine_class=NULL,quarantine_detail=NULL,quarantined_at=NULL
WHERE event_id=$1`, args["event_id"])
				}
				r22RequireRejectedWrite(t, err)
			})
		})
	}
	for _, state := range []string{"delivered", "quarantined"} {
		t.Run(state+" outbox rejects same-state retry mutation", func(t *testing.T) {
			r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
				parent := r22InsertIntegrityParent(t, ctx, tx, seed)
				args := r22OutboxArgs(seed, parent["actor_id"].(uuid.UUID), parent["operation_id"].(uuid.UUID), state)
				require.NoError(t, r22InsertOutbox(ctx, tx, args))
				now := time.Now().UTC()
				_, err := tx.Exec(ctx, `
UPDATE voice_event_outbox
SET attempt_count=attempt_count+1,last_error_class='late_retry',last_error_at=$1,
    next_attempt_at=$2,updated_at=$1
WHERE event_id=$3`, now, now.Add(time.Minute), args["event_id"])
				r22RequireRejectedWrite(t, err)
			})
		})
	}

	t.Run("ready outbox same state retry and lease updates are allowed", func(t *testing.T) {
		r22WithMigrationTx(t, ctx, pool, func(tx pgx.Tx) {
			parent := r22InsertIntegrityParent(t, ctx, tx, seed)
			args := r22OutboxArgs(seed, parent["actor_id"].(uuid.UUID), parent["operation_id"].(uuid.UUID), "ready")
			require.NoError(t, r22InsertOutbox(ctx, tx, args))
			now := time.Now().UTC()
			_, err := tx.Exec(ctx, `
UPDATE voice_event_outbox
SET attempt_count=1,last_error_class='timeout',last_error_at=$1,next_attempt_at=$2,
    lease_owner=$3,lease_until=$2,lease_fence=1,updated_at=$1
WHERE event_id=$4`, now, now.Add(time.Minute), uuid.New(), args["event_id"])
			require.NoError(t, err)
		})
	})
}
