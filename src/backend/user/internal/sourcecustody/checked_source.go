package sourcecustody

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MaterializeCheckedSource publishes only CLOSED for a previously quarantined
// and identity-checked source. It does not attest currentness or open capture;
// exclusive administrator, endpoint and restore custody remain prerequisites.
func MaterializeCheckedSource(ctx context.Context, control, maintenance *pgxpool.Pool, request CreationRequest, physical PhysicalSource, binding Binding) error {
	if ctx == nil || ctx.Err() != nil || control == nil || maintenance == nil || !binding.valid() || binding.SourceID != request.SourceID || binding.Epoch != request.Epoch || !systemID.MatchString(request.ControlSystemID) || !systemID.MatchString(request.SourceSystemID) || request.ControlSystemID == request.SourceSystemID || !databaseName.MatchString(request.Database) || physical.SystemIdentifier != request.SourceSystemID || physical.Database != request.Database || physical.DatabaseOID == 0 {
		return ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	admin, err := maintenance.Acquire(ctx)
	if err != nil {
		return ErrUnavailable
	}
	defer admin.Release()
	checkSource := func() error {
		var system, database string
		var oid uint32
		var superuser, allowed, publicConnect, unsafe bool
		var sessions, prepared int
		if admin.QueryRow(ctx, `SELECT system_identifier::text,current_database(),(SELECT rolsuper FROM pg_roles WHERE rolname=current_user) FROM pg_control_system()`).Scan(&system, &database, &superuser) != nil || system != physical.SystemIdentifier || database == physical.Database || !superuser || admin.QueryRow(ctx, `SELECT oid,datallowconn,EXISTS(SELECT 1 FROM aclexplode(COALESCE(datacl,acldefault('d',datdba))) a WHERE a.grantee=0 AND a.privilege_type='CONNECT') FROM pg_database WHERE datname=$1`, physical.Database).Scan(&oid, &allowed, &publicConnect) != nil || oid != physical.DatabaseOID || allowed || publicConnect {
			return ErrUnavailable
		}
		if admin.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_roles r WHERE r.rolcanlogin AND NOT r.rolsuper AND has_database_privilege(r.oid,$1::oid,'CONNECT')),(SELECT count(*) FROM pg_stat_activity WHERE datid=$1),(SELECT count(*) FROM pg_prepared_xacts WHERE database=$2)`, oid, physical.Database).Scan(&unsafe, &sessions, &prepared) != nil || unsafe || sessions != 0 || prepared != 0 {
			return ErrUnavailable
		}
		return nil
	}
	if checkSource() != nil {
		return ErrUnavailable
	}
	owner, err := control.Acquire(ctx)
	if err != nil {
		return ErrUnavailable
	}
	var locked bool
	if owner.QueryRow(ctx, `SELECT pg_try_advisory_lock($1::bigint)`, leaderKey).Scan(&locked) != nil {
		destroy(owner)
		return ErrUnavailable
	}
	if !locked {
		owner.Release()
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
	tx, err := owner.Begin(ctx)
	if err != nil {
		return ErrUnavailable
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		_ = tx.Rollback(cleanup)
	}()
	var system, database string
	var controlOID uint32
	if tx.QueryRow(ctx, `SELECT system_identifier::text,current_database(),(SELECT oid FROM pg_database WHERE datname=current_database()) FROM pg_control_system()`).Scan(&system, &database, &controlOID) != nil || system != request.ControlSystemID || database != control.Config().ConnConfig.Database || controlOID == 0 {
		return ErrUnavailable
	}
	var phase, storedControl, storedSource, storedDatabase string
	var oid uint32
	var history, generation uuid.UUID
	var digest []byte
	if tx.QueryRow(ctx, `SELECT phase,control_system_id,source_system_id,database_name,database_oid,history_generation,user_source_generation,keyset_digest FROM public.user_custody_creation WHERE singleton AND source_id=$1 AND epoch=$2 FOR UPDATE`, binding.SourceID, binding.Epoch).Scan(&phase, &storedControl, &storedSource, &storedDatabase, &oid, &history, &generation, &digest) != nil || phase != "IDENTITY_CHECKED_CLOSED" || storedControl != system || storedSource != physical.SystemIdentifier || storedDatabase != physical.Database || oid != physical.DatabaseOID || !same(binding, binding.Epoch, binding.SourceID, history, generation, digest) {
		return ErrUnavailable
	}
	var pinned PhysicalSource
	if tx.QueryRow(ctx, `SELECT system_identifier,database_name,database_oid FROM public.user_custody_physical_source WHERE source_id=$1 AND epoch=$2 FOR UPDATE`, binding.SourceID, binding.Epoch).Scan(&pinned.SystemIdentifier, &pinned.Database, &pinned.DatabaseOID) != nil || pinned != physical || checkSource() != nil {
		return ErrUnavailable
	}
	if _, err = tx.Exec(ctx, `CREATE TABLE public.user_custody_state(singleton boolean PRIMARY KEY CHECK(singleton),state text NOT NULL CHECK(state IN ('OPEN','CLOSED')),epoch bigint NOT NULL,source_id text NOT NULL,history_generation uuid NOT NULL,user_source_generation uuid NOT NULL,keyset_digest bytea NOT NULL)`); err != nil {
		return ErrUnavailable
	}
	if _, err = tx.Exec(ctx, `INSERT INTO public.user_custody_state VALUES(true,'CLOSED',$1,$2,$3,$4,$5)`, binding.Epoch, binding.SourceID, binding.HistoryGeneration, binding.UserSourceGeneration, binding.KeysetDigest[:]); err != nil {
		return ErrUnavailable
	}
	tag, err := tx.Exec(ctx, `UPDATE public.user_custody_creation SET phase='EXISTING_INITIALIZED_CLOSED' WHERE singleton AND phase='IDENTITY_CHECKED_CLOSED' AND source_id=$1 AND epoch=$2`, binding.SourceID, binding.Epoch)
	if err != nil || tag.RowsAffected() != 1 || checkSource() != nil || tx.Commit(ctx) != nil {
		return ErrUnavailable
	}
	return nil
}
