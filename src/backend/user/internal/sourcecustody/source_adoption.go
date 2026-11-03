package sourcecustody

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"voice/backend/user/internal/identityassociation"
)

// AdoptPristineExistingSource records and initializes a newly adopted, exact v22
// pristine identity domain. All pins and the evidence digest are independently
// reviewed custodian input, never historical creation/migration or proof that
// custody happened. Exclusive administrative/endpoint/restore custody and a
// verified backup/restore remain prerequisites. No guards or admission open.
// Any error is HOLD: an attempted adoption must not be retried or reclassified.
func AdoptPristineExistingSource(ctx context.Context, control, admin *pgxpool.Pool, request CreationRequest, physical PhysicalSource, expected Binding, keys *identityassociation.Keyset, receiptID uuid.UUID, evidenceDigest [32]byte) error {
	if receiptID == uuid.Nil || evidenceDigest == [32]byte{} || ctx == nil || ctx.Err() != nil || control == nil || admin == nil || keys == nil || !expected.valid() || expected.HistoryGeneration == expected.UserSourceGeneration || expected.SourceID != request.SourceID || expected.Epoch != request.Epoch || !databaseName.MatchString(request.Database) || physical.Database != request.Database || physical.SystemIdentifier != request.SourceSystemID || physical.DatabaseOID == 0 || !systemID.MatchString(request.ControlSystemID) || !systemID.MatchString(request.SourceSystemID) || request.ControlSystemID == request.SourceSystemID {
		return ErrUnavailable
	}
	digest, err := keys.CustodyDigest()
	if err != nil || digest != expected.KeysetDigest || keys.Snapshot().Revision != 1 {
		return ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	source, err := admin.Acquire(ctx)
	if err != nil {
		return ErrUnavailable
	}
	defer source.Release()
	check := func(expectAllowed bool) error {
		var system, current string
		var oid uint32
		var allowed, owner, publicConnect bool
		if source.QueryRow(ctx, `SELECT system_identifier::text,current_database() FROM pg_control_system()`).Scan(&system, &current) != nil || system != physical.SystemIdentifier || current == physical.Database || source.QueryRow(ctx, `SELECT oid,datallowconn,pg_get_userbyid(datdba)=current_user,EXISTS(SELECT 1 FROM aclexplode(COALESCE(datacl,acldefault('d',datdba))) a WHERE a.grantee=0 AND a.privilege_type='CONNECT') FROM pg_database WHERE datname=$1`, physical.Database).Scan(&oid, &allowed, &owner, &publicConnect) != nil || oid != physical.DatabaseOID || allowed != expectAllowed || !owner || publicConnect {
			return ErrUnavailable
		}
		var untrusted int
		if source.QueryRow(ctx, `SELECT count(*) FROM pg_roles r WHERE r.rolcanlogin AND NOT r.rolsuper AND r.rolname<>current_user AND has_database_privilege(r.oid,$1::oid,'CONNECT')`, physical.DatabaseOID).Scan(&untrusted) != nil || untrusted != 0 {
			return ErrUnavailable
		}
		return nil
	}
	if check(false) != nil {
		return ErrUnavailable
	}
	var sessions, prepared int
	if source.QueryRow(ctx, `SELECT (SELECT count(*) FROM pg_stat_activity WHERE datid=$1),(SELECT count(*) FROM pg_prepared_xacts WHERE database=$2)`, physical.DatabaseOID, physical.Database).Scan(&sessions, &prepared) != nil || sessions != 0 || prepared != 0 {
		return ErrUnavailable
	}
	owner, err := control.Acquire(ctx)
	if err != nil {
		return ErrUnavailable
	}
	var locked bool
	if err = owner.QueryRow(ctx, `SELECT pg_try_advisory_lock($1::bigint)`, leaderKey).Scan(&locked); err != nil || !locked {
		if err != nil {
			destroy(owner)
		} else {
			owner.Release()
		}
		return ErrUnavailable
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		var unlocked bool
		if owner.QueryRow(cleanup, `SELECT pg_advisory_unlock($1::bigint)`, leaderKey).Scan(&unlocked) != nil || !unlocked {
			destroy(owner)
		} else {
			owner.Release()
		}
	}()
	var system, database string
	var databaseOID uint32
	if owner.QueryRow(ctx, `SELECT system_identifier::text,current_database(),(SELECT oid FROM pg_database WHERE datname=current_database()) FROM pg_control_system()`).Scan(&system, &database, &databaseOID) != nil || system != request.ControlSystemID || system == physical.SystemIdentifier || database == "" || databaseOID == 0 {
		return ErrUnavailable
	}
	pin, err := control.Begin(ctx)
	if err != nil {
		return ErrUnavailable
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		_ = pin.Rollback(cleanup)
	}()
	var pinnedSystem, pinnedDatabase string
	var pinnedDatabaseOID uint32
	if pin.QueryRow(ctx, `SELECT system_identifier::text,current_database(),(SELECT oid FROM pg_database WHERE datname=current_database()) FROM pg_control_system()`).Scan(&pinnedSystem, &pinnedDatabase, &pinnedDatabaseOID) != nil || pinnedSystem != system || pinnedDatabase != database || pinnedDatabaseOID != databaseOID {
		return ErrUnavailable
	}
	var pinned PhysicalSource
	if pin.QueryRow(ctx, `SELECT system_identifier,database_name,database_oid FROM user_custody_physical_source WHERE source_id=$1 AND epoch=$2 FOR UPDATE`, request.SourceID, request.Epoch).Scan(&pinned.SystemIdentifier, &pinned.Database, &pinned.DatabaseOID) != nil || pinned != physical {
		return ErrUnavailable
	}
	// Persist all pins outside the source before enabling maintenance. A
	// surviving attempt, even with a lost acknowledgement, rejects every retry.
	attempt, err := owner.Begin(ctx)
	if err != nil {
		return ErrUnavailable
	}
	defer func() { _ = attempt.Rollback(context.Background()) }()
	if _, err = attempt.Exec(ctx, `CREATE TABLE public.user_custody_adoption (
 singleton boolean PRIMARY KEY CHECK(singleton), receipt_id uuid NOT NULL UNIQUE,
 phase text NOT NULL CHECK(phase IN ('ADOPTING','ADOPTED_CLOSED')),
 source_id text NOT NULL, epoch bigint NOT NULL, control_system_id text NOT NULL,
 source_system_id text NOT NULL, database_name text NOT NULL, database_oid oid NOT NULL,
 history_generation uuid NOT NULL, user_source_generation uuid NOT NULL,
 keyset_digest bytea NOT NULL CHECK(octet_length(keyset_digest)=32),
 evidence_digest bytea NOT NULL CHECK(octet_length(evidence_digest)=32),
 recorded_at timestamptz NOT NULL DEFAULT clock_timestamp())`); err != nil {
		return ErrUnavailable
	}
	if _, err = attempt.Exec(ctx, `INSERT INTO public.user_custody_adoption VALUES(true,$1,'ADOPTING',$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,clock_timestamp())`, receiptID, request.SourceID, request.Epoch, request.ControlSystemID, request.SourceSystemID, physical.Database, physical.DatabaseOID, expected.HistoryGeneration, expected.UserSourceGeneration, digest[:], evidenceDigest[:]); err != nil {
		return ErrUnavailable
	}
	tag, err := attempt.Exec(ctx, `UPDATE user_custody_creation SET phase='VERIFYING_IDENTITY',history_generation=$1,user_source_generation=$2,keyset_digest=$3 WHERE singleton AND phase='QUARANTINED_EXISTING' AND source_id=$4 AND epoch=$5 AND control_system_id=$6 AND source_system_id=$7 AND database_name=$8 AND database_oid=$9 AND history_generation IS NULL AND user_source_generation IS NULL AND keyset_digest IS NULL AND to_regclass('public.user_custody_state') IS NULL`, expected.HistoryGeneration, expected.UserSourceGeneration, digest[:], request.SourceID, request.Epoch, request.ControlSystemID, request.SourceSystemID, request.Database, physical.DatabaseOID)
	if err != nil || tag.RowsAffected() != 1 || attempt.Commit(ctx) != nil || ctx.Err() != nil {
		return ErrUnavailable
	}

	closed := false
	defer func() {
		if !closed {
			cleanup, stop := context.WithTimeout(context.Background(), 10*time.Second)
			defer stop()
			_, _ = source.Exec(cleanup, `ALTER DATABASE `+pgx.Identifier{physical.Database}.Sanitize()+` WITH ALLOW_CONNECTIONS false`)
			_, _ = source.Exec(cleanup, `SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datid=$1`, physical.DatabaseOID)
		}
	}()
	if check(false) != nil {
		return ErrUnavailable
	}
	if _, err = source.Exec(ctx, `ALTER DATABASE `+pgx.Identifier{physical.Database}.Sanitize()+` WITH ALLOW_CONNECTIONS true`); err != nil || check(true) != nil {
		return ErrUnavailable
	}
	config := admin.Config().Copy()
	config.ConnConfig.Database = physical.Database
	if config.ConnConfig.RuntimeParams == nil {
		config.ConnConfig.RuntimeParams = make(map[string]string)
	}
	config.ConnConfig.RuntimeParams["statement_timeout"] = "10000"
	config.ConnConfig.RuntimeParams["lock_timeout"] = "2000"
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return ErrUnavailable
	}
	defer pool.Close()
	var version int
	var dirty bool
	if pool.QueryRow(ctx, `SELECT version,dirty FROM schema_migrations`).Scan(&version, &dirty) != nil || version != 22 || dirty {
		return ErrUnavailable
	}
	_, err = identityassociation.NewAdoptedStore(ctx, pool, keys, identityassociation.AdoptionSourcePin{
		ReceiptID: receiptID, EvidenceDigest: evidenceDigest,
		HistoryGeneration: expected.HistoryGeneration, UserSourceGeneration: expected.UserSourceGeneration,
		KeysetDigest: digest, Physical: identityassociation.PhysicalSourceIdentity{SystemIdentifier: physical.SystemIdentifier, Database: physical.Database, DatabaseOID: physical.DatabaseOID},
	})
	if err != nil {
		return ErrUnavailable
	}
	pool.Close()
	if _, err = source.Exec(ctx, `ALTER DATABASE `+pgx.Identifier{physical.Database}.Sanitize()+` WITH ALLOW_CONNECTIONS false`); err != nil || check(false) != nil {
		return ErrUnavailable
	}
	if source.QueryRow(ctx, `SELECT (SELECT count(*) FROM pg_stat_activity WHERE datid=$1),(SELECT count(*) FROM pg_prepared_xacts WHERE database=$2)`, physical.DatabaseOID, physical.Database).Scan(&sessions, &prepared) != nil || sessions != 0 || prepared != 0 || pin.QueryRow(ctx, `SELECT system_identifier,database_name,database_oid FROM user_custody_physical_source WHERE source_id=$1 AND epoch=$2`, request.SourceID, request.Epoch).Scan(&pinned.SystemIdentifier, &pinned.Database, &pinned.DatabaseOID) != nil || pinned != physical {
		return ErrUnavailable
	}
	// Publish CLOSED only after re-fencing, while holding the physical pin.
	if _, err = pin.Exec(ctx, `CREATE TABLE public.user_custody_state(singleton boolean PRIMARY KEY CHECK(singleton),state text NOT NULL CHECK(state IN ('OPEN','CLOSED')),epoch bigint NOT NULL,source_id text NOT NULL,history_generation uuid NOT NULL,user_source_generation uuid NOT NULL,keyset_digest bytea NOT NULL)`); err != nil {
		return ErrUnavailable
	}
	if _, err = pin.Exec(ctx, `INSERT INTO public.user_custody_state VALUES(true,'CLOSED',$1,$2,$3,$4,$5)`, expected.Epoch, expected.SourceID, expected.HistoryGeneration, expected.UserSourceGeneration, digest[:]); err != nil {
		return ErrUnavailable
	}
	tag, err = pin.Exec(ctx, `UPDATE user_custody_creation SET phase='EXISTING_INITIALIZED_CLOSED' WHERE singleton AND phase='VERIFYING_IDENTITY' AND source_id=$1 AND epoch=$2 AND history_generation=$3 AND user_source_generation=$4 AND keyset_digest=$5`, request.SourceID, request.Epoch, expected.HistoryGeneration, expected.UserSourceGeneration, digest[:])
	if err != nil || tag.RowsAffected() != 1 {
		return ErrUnavailable
	}
	tag, err = pin.Exec(ctx, `UPDATE user_custody_adoption SET phase='ADOPTED_CLOSED' WHERE singleton AND phase='ADOPTING' AND receipt_id=$1 AND evidence_digest=$2`, receiptID, evidenceDigest[:])
	if err != nil || tag.RowsAffected() != 1 || check(false) != nil || pin.Commit(ctx) != nil || ctx.Err() != nil {
		return ErrUnavailable
	}
	closed = true
	return nil
}
