package sourcecustody

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CreationRequest supplies independent cluster pins and an intended new name.
// It is trusted operator input, not proof of deployment custody by itself.
type CreationRequest struct {
	SourceID        string
	Epoch           int64
	ControlSystemID string
	SourceSystemID  string
	Database        string
}

const creationSchema = `CREATE TABLE public.user_custody_creation(singleton boolean PRIMARY KEY CHECK(singleton),phase text NOT NULL CHECK(phase IN ('CREATING','CREATED_CLOSED','MIGRATING','MIGRATED','INITIALIZED_CLOSED','EXISTING_INITIALIZED_CLOSED','PREPARING_OPEN','ACTIVE','RECOVERY_REQUIRED','RECOVERED_CLOSED','QUARANTINING','QUARANTINED_EXISTING','VERIFYING_IDENTITY','IDENTITY_CHECKED_CLOSED')),source_id text NOT NULL,epoch bigint NOT NULL,control_system_id text NOT NULL,source_system_id text NOT NULL,database_name text NOT NULL,database_oid oid,history_generation uuid,user_source_generation uuid,keyset_digest bytea);
CREATE TABLE public.user_custody_physical_source(source_id text NOT NULL,epoch bigint NOT NULL,system_identifier text NOT NULL,database_name text NOT NULL,database_oid oid NOT NULL,PRIMARY KEY(source_id,epoch));
CREATE TABLE public.user_custody_open_evidence(challenge bytea PRIMARY KEY CHECK(octet_length(challenge)=32),source_id text NOT NULL,epoch bigint NOT NULL,writer_role text NOT NULL,payload bytea NOT NULL,signature bytea NOT NULL CHECK(octet_length(signature)=64),key_digest bytea NOT NULL CHECK(octet_length(key_digest)=32),recorded_at timestamptz NOT NULL DEFAULT clock_timestamp());`

// CreateClosedSource creates a new, inaccessible source and records its
// physical identity. It never adopts, migrates, initializes, or opens a source.
func CreateClosedSource(ctx context.Context, control, admin *pgxpool.Pool, request CreationRequest) (PhysicalSource, error) {
	var empty PhysicalSource
	if ctx == nil || ctx.Err() != nil || control == nil || admin == nil || !sourceIDPattern.MatchString(request.SourceID) || request.Epoch <= 0 || !databaseName.MatchString(request.Database) || !systemID.MatchString(request.ControlSystemID) || !systemID.MatchString(request.SourceSystemID) || request.ControlSystemID == request.SourceSystemID {
		return empty, ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	source, err := admin.Acquire(ctx)
	if err != nil {
		return empty, ErrUnavailable
	}
	defer source.Release()
	var sourceSystem, sourceDatabase string
	if source.QueryRow(ctx, `SELECT system_identifier::text,current_database() FROM pg_control_system()`).Scan(&sourceSystem, &sourceDatabase) != nil || sourceSystem != request.SourceSystemID || sourceDatabase == request.Database {
		return empty, ErrUnavailable
	}
	var exists bool
	if source.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname=$1)`, request.Database).Scan(&exists) != nil || exists {
		return empty, ErrUnavailable
	}
	owner, err := control.Acquire(ctx)
	if err != nil {
		return empty, ErrUnavailable
	}
	var locked bool
	if err = owner.QueryRow(ctx, `SELECT pg_try_advisory_lock($1::bigint)`, leaderKey).Scan(&locked); err != nil || !locked {
		if err != nil {
			destroy(owner)
		} else {
			owner.Release()
		}
		return empty, ErrUnavailable
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
	var controlSystem string
	if owner.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&controlSystem) != nil || controlSystem != request.ControlSystemID || controlSystem == sourceSystem {
		return empty, ErrUnavailable
	}
	if owner.QueryRow(ctx, `SELECT to_regclass('public.user_custody_creation') IS NOT NULL OR to_regclass('public.user_custody_state') IS NOT NULL OR to_regclass('public.user_custody_physical_source') IS NOT NULL`).Scan(&exists) != nil || exists {
		return empty, ErrUnavailable
	}
	// Commit the closed intent before the nontransactional CREATE DATABASE.
	tx, err := owner.Begin(ctx)
	if err != nil {
		return empty, ErrUnavailable
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		_ = tx.Rollback(cleanup)
	}()
	_, err = tx.Exec(ctx, creationSchema)
	if err != nil {
		return empty, ErrUnavailable
	}
	if _, err = tx.Exec(ctx, `INSERT INTO public.user_custody_creation(singleton,phase,source_id,epoch,control_system_id,source_system_id,database_name) VALUES(true,'CREATING',$1,$2,$3,$4,$5)`, request.SourceID, request.Epoch, request.ControlSystemID, request.SourceSystemID, request.Database); err != nil || tx.Commit(ctx) != nil {
		return empty, ErrUnavailable
	}
	// A conflict or unknown acknowledgement leaves the intent closed and cannot
	// be repaired by reading and adopting a database with the same name.
	if _, err = source.Exec(ctx, `CREATE DATABASE `+pgx.Identifier{request.Database}.Sanitize()+` WITH TEMPLATE template0 ALLOW_CONNECTIONS false`); err != nil {
		return empty, ErrUnavailable
	}
	// Keep the default PUBLIC grant from becoming active during any later,
	// separately authorized maintenance opening.
	if _, err = source.Exec(ctx, `REVOKE CONNECT ON DATABASE `+pgx.Identifier{request.Database}.Sanitize()+` FROM PUBLIC`); err != nil {
		return empty, ErrUnavailable
	}
	physical := PhysicalSource{SystemIdentifier: sourceSystem, Database: request.Database}
	var allowed bool
	if source.QueryRow(ctx, `SELECT oid,datallowconn FROM pg_database WHERE datname=$1`, request.Database).Scan(&physical.DatabaseOID, &allowed) != nil || physical.DatabaseOID == 0 || allowed {
		return empty, ErrUnavailable
	}
	var publicConnect bool
	if source.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_database d CROSS JOIN LATERAL aclexplode(COALESCE(d.datacl,acldefault('d',d.datdba))) a WHERE d.oid=$1 AND a.grantee=0 AND a.privilege_type='CONNECT')`, physical.DatabaseOID).Scan(&publicConnect) != nil || publicConnect {
		return empty, ErrUnavailable
	}
	var sessions, prepared int
	if source.QueryRow(ctx, `SELECT (SELECT count(*) FROM pg_stat_activity WHERE datid=$1),(SELECT count(*) FROM pg_prepared_xacts WHERE database=$2)`, physical.DatabaseOID, physical.Database).Scan(&sessions, &prepared) != nil || sessions != 0 || prepared != 0 {
		return empty, ErrUnavailable
	}
	if source.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&sourceSystem) != nil || sourceSystem != physical.SystemIdentifier {
		return empty, ErrUnavailable
	}
	final, err := owner.Begin(ctx)
	if err != nil {
		return empty, ErrUnavailable
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		_ = final.Rollback(cleanup)
	}()
	if _, err = final.Exec(ctx, `INSERT INTO public.user_custody_physical_source VALUES($1,$2,$3,$4,$5)`, request.SourceID, request.Epoch, physical.SystemIdentifier, physical.Database, physical.DatabaseOID); err != nil {
		return empty, ErrUnavailable
	}
	tag, err := final.Exec(ctx, `UPDATE public.user_custody_creation SET phase='CREATED_CLOSED',database_oid=$1 WHERE singleton AND phase='CREATING' AND source_id=$2 AND epoch=$3 AND control_system_id=$4 AND source_system_id=$5 AND database_name=$6 AND database_oid IS NULL`, physical.DatabaseOID, request.SourceID, request.Epoch, request.ControlSystemID, request.SourceSystemID, request.Database)
	if err != nil || tag.RowsAffected() != 1 || final.Commit(ctx) != nil {
		return empty, ErrUnavailable
	}
	return physical, nil
}
