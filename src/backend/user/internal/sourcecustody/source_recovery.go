package sourcecustody

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RecoverInterruptedInitialization fences an interrupted initialization or
// quarantine. It cannot adopt the source or authorize capture, and requires
// exclusive administrative custody of both clusters throughout the operation.
func RecoverInterruptedInitialization(ctx context.Context, control, admin *pgxpool.Pool, request CreationRequest, physical PhysicalSource) error {
	if ctx == nil || ctx.Err() != nil || control == nil || admin == nil || !sourceIDPattern.MatchString(request.SourceID) || request.Epoch <= 0 || !databaseName.MatchString(request.Database) || physical.Database != request.Database || physical.SystemIdentifier != request.SourceSystemID || physical.DatabaseOID == 0 || !systemID.MatchString(request.ControlSystemID) || !systemID.MatchString(request.SourceSystemID) || request.ControlSystemID == request.SourceSystemID {
		return ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
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
	var controlSystem string
	if owner.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&controlSystem) != nil || controlSystem != request.ControlSystemID || controlSystem == physical.SystemIdentifier {
		return ErrUnavailable
	}
	tx, err := owner.Begin(ctx)
	if err != nil {
		return ErrUnavailable
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		_ = tx.Rollback(cleanup)
	}()
	var phase string
	var stateExists bool
	if tx.QueryRow(ctx, `SELECT phase,to_regclass('public.user_custody_state') IS NOT NULL FROM user_custody_creation WHERE singleton AND source_id=$1 AND epoch=$2 AND control_system_id=$3 AND source_system_id=$4 AND database_name=$5 AND database_oid=$6 FOR UPDATE`, request.SourceID, request.Epoch, request.ControlSystemID, request.SourceSystemID, request.Database, physical.DatabaseOID).Scan(&phase, &stateExists) != nil || stateExists || (phase != "MIGRATING" && phase != "MIGRATED" && phase != "QUARANTINING" && phase != "VERIFYING_IDENTITY") {
		return ErrUnavailable
	}
	var pinned PhysicalSource
	if tx.QueryRow(ctx, `SELECT system_identifier,database_name,database_oid FROM user_custody_physical_source WHERE source_id=$1 AND epoch=$2 FOR UPDATE`, request.SourceID, request.Epoch).Scan(&pinned.SystemIdentifier, &pinned.Database, &pinned.DatabaseOID) != nil || pinned != physical {
		return ErrUnavailable
	}
	source, err := admin.Acquire(ctx)
	if err != nil {
		return ErrUnavailable
	}
	defer source.Release()
	check := func() error {
		var system, current string
		var oid uint32
		if source.QueryRow(ctx, `SELECT system_identifier::text,current_database() FROM pg_control_system()`).Scan(&system, &current) != nil || system != physical.SystemIdentifier || current == physical.Database || source.QueryRow(ctx, `SELECT oid FROM pg_database WHERE datname=$1`, physical.Database).Scan(&oid) != nil || oid != physical.DatabaseOID {
			return ErrUnavailable
		}
		return nil
	}
	if check() != nil {
		return ErrUnavailable
	}
	if _, err = source.Exec(ctx, `ALTER DATABASE `+pgx.Identifier{physical.Database}.Sanitize()+` WITH ALLOW_CONNECTIONS false`); err != nil {
		return ErrUnavailable
	}
	var terminated bool
	if source.QueryRow(ctx, `SELECT COALESCE(bool_and(pg_terminate_backend(pid)),true) FROM pg_stat_activity WHERE datid=$1`, physical.DatabaseOID).Scan(&terminated) != nil || !terminated {
		return ErrUnavailable
	}
	for {
		var allowed bool
		var sessions, prepared int
		if check() != nil || source.QueryRow(ctx, `SELECT datallowconn,(SELECT count(*) FROM pg_stat_activity WHERE datid=$1),(SELECT count(*) FROM pg_prepared_xacts WHERE database=$2) FROM pg_database WHERE oid=$1`, physical.DatabaseOID, physical.Database).Scan(&allowed, &sessions, &prepared) != nil || allowed || prepared != 0 {
			return ErrUnavailable
		}
		if sessions == 0 {
			break
		}
		select {
		case <-ctx.Done():
			return ErrUnavailable
		case <-time.After(25 * time.Millisecond):
		}
	}
	tag, err := tx.Exec(ctx, `UPDATE user_custody_creation SET phase='RECOVERY_REQUIRED' WHERE singleton AND phase=$1 AND source_id=$2 AND epoch=$3`, phase, request.SourceID, request.Epoch)
	if err != nil || tag.RowsAffected() != 1 || tx.Commit(ctx) != nil {
		return ErrUnavailable
	}
	return nil
}
