package sourcecustody

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var errRecoveryLeaderBusy = fmt.Errorf("%w: recovery leader busy", ErrUnavailable)

// RecoverInterruptedActivation closes admission, disables the independently
// recorded writer, and fences the exact source. It does not requalify or reopen
// the source, and requires exclusive custodian control of both clusters.
func RecoverInterruptedActivation(ctx context.Context, control, maintenance *pgxpool.Pool, request CreationRequest, physical PhysicalSource, writer string) error {
	if ctx == nil || ctx.Err() != nil || control == nil || maintenance == nil || !sourceIDPattern.MatchString(request.SourceID) || request.Epoch <= 0 || !databaseName.MatchString(request.Database) || !databaseName.MatchString(writer) || physical.Database != request.Database || physical.SystemIdentifier != request.SourceSystemID || physical.DatabaseOID == 0 || !systemID.MatchString(request.ControlSystemID) || !systemID.MatchString(request.SourceSystemID) || request.ControlSystemID == request.SourceSystemID {
		return ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
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
		return errRecoveryLeaderBusy
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
	return recoverActivationLocked(ctx, owner, control, maintenance, request, physical, writer)
}

// recoverActivationLocked requires the caller to retain leaderKey until it
// returns. The independent monitor uses the same lock to prevent reopening
// between detecting leader loss and completing the physical fence.
func recoverActivationLocked(ctx context.Context, owner *pgxpool.Conn, control, maintenance *pgxpool.Pool, request CreationRequest, physical PhysicalSource, writer string) error {
	var system, database string
	var controlOID uint32
	if owner.QueryRow(ctx, `SELECT system_identifier::text,current_database(),(SELECT oid FROM pg_database WHERE datname=current_database()) FROM pg_control_system()`).Scan(&system, &database, &controlOID) != nil || system != request.ControlSystemID || system == physical.SystemIdentifier || database != control.Config().ConnConfig.Database || controlOID == 0 {
		return ErrUnavailable
	}
	var state string
	if owner.QueryRow(ctx, `UPDATE public.user_custody_state SET state='CLOSED' WHERE singleton AND source_id=$1 AND epoch=$2 RETURNING state`, request.SourceID, request.Epoch).Scan(&state) != nil || state != "CLOSED" {
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
	var phase, recordedWriter string
	if tx.QueryRow(ctx, `SELECT phase FROM public.user_custody_creation WHERE singleton AND source_id=$1 AND epoch=$2 AND control_system_id=$3 AND source_system_id=$4 AND database_name=$5 AND database_oid=$6 FOR UPDATE`, request.SourceID, request.Epoch, request.ControlSystemID, request.SourceSystemID, request.Database, physical.DatabaseOID).Scan(&phase) != nil || (phase != "PREPARING_OPEN" && phase != "ACTIVE" && phase != "RECOVERY_REQUIRED" && phase != "RECOVERED_CLOSED") {
		return ErrUnavailable
	}
	var pinned PhysicalSource
	var evidenceCount, writerCount int
	// A live permit holds a shared lock on this binding. A shared pin still
	// prevents the binding from changing during the fence, without letting a
	// stalled permit prevent the writer and source from being fenced.
	if tx.QueryRow(ctx, `SELECT system_identifier,database_name,database_oid FROM public.user_custody_physical_source WHERE source_id=$1 AND epoch=$2 FOR SHARE`, request.SourceID, request.Epoch).Scan(&pinned.SystemIdentifier, &pinned.Database, &pinned.DatabaseOID) != nil || pinned != physical || tx.QueryRow(ctx, `SELECT count(*),count(DISTINCT writer_role),COALESCE(min(writer_role),'') FROM public.user_custody_open_evidence WHERE source_id=$1 AND epoch=$2`, request.SourceID, request.Epoch).Scan(&evidenceCount, &writerCount, &recordedWriter) != nil || evidenceCount == 0 || writerCount != 1 || recordedWriter != writer {
		return ErrUnavailable
	}
	admin, err := maintenance.Acquire(ctx)
	if err != nil {
		return ErrUnavailable
	}
	defer admin.Release()
	check := func() error {
		var sourceSystem, current, user string
		var oid uint32
		var superuser bool
		if admin.QueryRow(ctx, `SELECT system_identifier::text,current_database(),current_user,(SELECT rolsuper FROM pg_roles WHERE rolname=current_user) FROM pg_control_system()`).Scan(&sourceSystem, &current, &user, &superuser) != nil || sourceSystem != physical.SystemIdentifier || current == physical.Database || user == writer || !superuser || admin.QueryRow(ctx, `SELECT oid FROM pg_database WHERE datname=$1`, physical.Database).Scan(&oid) != nil || oid != physical.DatabaseOID {
			return ErrUnavailable
		}
		return nil
	}
	if check() != nil {
		return ErrUnavailable
	}
	if _, err = admin.Exec(ctx, `ALTER ROLE `+pgx.Identifier{writer}.Sanitize()+` NOLOGIN`); err != nil {
		return ErrUnavailable
	}
	if _, err = admin.Exec(ctx, `ALTER DATABASE `+pgx.Identifier{physical.Database}.Sanitize()+` WITH ALLOW_CONNECTIONS false`); err != nil {
		return ErrUnavailable
	}
	var terminated bool
	if admin.QueryRow(ctx, `SELECT COALESCE(bool_and(pg_terminate_backend(pid)),true) FROM pg_stat_activity WHERE pid<>pg_backend_pid() AND (datid=$1 OR usename=$2)`, physical.DatabaseOID, writer).Scan(&terminated) != nil || !terminated {
		return ErrUnavailable
	}
	for {
		var allowed, login bool
		var sessions, prepared int
		if check() != nil || admin.QueryRow(ctx, `SELECT datallowconn,(SELECT rolcanlogin FROM pg_roles WHERE rolname=$3),(SELECT count(*) FROM pg_stat_activity WHERE datid=$1 OR usename=$3),(SELECT count(*) FROM pg_prepared_xacts WHERE database=$2 OR owner=$3) FROM pg_database WHERE oid=$1`, physical.DatabaseOID, physical.Database, writer).Scan(&allowed, &login, &sessions, &prepared) != nil || allowed || login || prepared != 0 {
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
	tag, err := tx.Exec(ctx, `UPDATE public.user_custody_creation SET phase='RECOVERED_CLOSED' WHERE singleton AND phase=$1 AND source_id=$2 AND epoch=$3`, phase, request.SourceID, request.Epoch)
	if err != nil || tag.RowsAffected() != 1 || tx.Commit(ctx) != nil {
		return ErrUnavailable
	}
	return nil
}
