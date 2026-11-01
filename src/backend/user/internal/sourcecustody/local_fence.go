package sourcecustody

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// restrictLocalSource performs a preliminary SQL restriction using only
// source maintenance access. A successful return is NOT a completed fence:
// PostgreSQL startup backends may have passed access checks before appearing
// in pg_stat_activity. The caller must confirm an independent source process
// or host stop and fail-closed restart before declaring a physical fence.
// The caller must also exclusively control source administration and reopen.
// A lost reply does not prove that an earlier COMMIT was rolled back.
func restrictLocalSource(ctx context.Context, admin *pgxpool.Pool, source PhysicalSource, writer string) error {
	if ctx == nil || ctx.Err() != nil || admin == nil || !systemID.MatchString(source.SystemIdentifier) || !databaseName.MatchString(source.Database) || source.DatabaseOID == 0 || !databaseName.MatchString(writer) {
		return ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	conn, err := admin.Acquire(ctx)
	if err != nil {
		return ErrUnavailable
	}
	defer conn.Release()
	check := func() (bool, bool, error) {
		var system, database, adminRole string
		var oid uint32
		var superuser, writerSuperuser, allowed, login bool
		if conn.QueryRow(ctx, `SELECT system_identifier::text,current_database(),current_user,(SELECT rolsuper FROM pg_roles WHERE rolname=current_user) FROM pg_control_system()`).Scan(&system, &database, &adminRole, &superuser) != nil || system != source.SystemIdentifier || database == source.Database || adminRole == writer || !superuser ||
			conn.QueryRow(ctx, `SELECT oid,datallowconn FROM pg_database WHERE datname=$1`, source.Database).Scan(&oid, &allowed) != nil || oid != source.DatabaseOID ||
			conn.QueryRow(ctx, `SELECT rolcanlogin,rolsuper FROM pg_roles WHERE rolname=$1`, writer).Scan(&login, &writerSuperuser) != nil || writerSuperuser {
			return false, false, ErrUnavailable
		}
		return allowed, login, nil
	}
	if _, _, err = check(); err != nil {
		return ErrUnavailable
	}
	// Each step is restrictive and remains in force after any later failure.
	if _, err = conn.Exec(ctx, `ALTER ROLE `+pgx.Identifier{writer}.Sanitize()+` NOLOGIN`); err != nil {
		return ErrUnavailable
	}
	if _, err = conn.Exec(ctx, `ALTER DATABASE `+pgx.Identifier{source.Database}.Sanitize()+` WITH ALLOW_CONNECTIONS false`); err != nil {
		return ErrUnavailable
	}
	var terminated bool
	if conn.QueryRow(ctx, `SELECT COALESCE(bool_and(pg_terminate_backend(pid)),true) FROM pg_stat_activity WHERE pid<>pg_backend_pid() AND (datid=$1 OR usename=$2)`, source.DatabaseOID, writer).Scan(&terminated) != nil || !terminated {
		return ErrUnavailable
	}
	for {
		var sessions, prepared int
		if conn.QueryRow(ctx, `SELECT (SELECT count(*) FROM pg_stat_activity WHERE pid<>pg_backend_pid() AND (datid=$1 OR usename=$2)),(SELECT count(*) FROM pg_prepared_xacts WHERE database=$3 OR owner=$2)`, source.DatabaseOID, writer, source.Database).Scan(&sessions, &prepared) != nil || prepared != 0 {
			return ErrUnavailable
		}
		if sessions == 0 {
			allowed, login, checkErr := check()
			if checkErr != nil || allowed || login || ctx.Err() != nil {
				return ErrUnavailable
			}
			return nil
		}
		timer := time.NewTimer(25 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ErrUnavailable
		case <-timer.C:
		}
	}
}
