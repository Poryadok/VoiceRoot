package partycoverage

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
)

var ErrUnavailable = errors.New("voice party coverage unavailable")

// RetirePostgresRole retires an explicitly inventoried predecessor login role.
// The caller must own the isolated/deployment resource window and use a distinct
// administrative connection. It must never target a shared/default role.
func RetirePostgresRole(ctx context.Context, admin *pgx.Conn, role string) error {
	if admin == nil || role == "" || role == admin.Config().User {
		return ErrUnavailable
	}
	var oid uint32
	var privileged, prepared, delegated bool
	err := admin.QueryRow(ctx, `SELECT oid,rolsuper OR rolcreaterole OR rolcreatedb OR rolreplication OR rolbypassrls FROM pg_roles WHERE rolname=$1`, role).Scan(&oid, &privileged)
	if err != nil || privileged {
		return fmt.Errorf("retire predecessor role: %w", ErrUnavailable)
	}
	// Prepared transactions survive connection termination. Indirect login via
	// another role can survive NOLOGIN. Neither is safe to guess or auto-clean.
	if err = admin.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_prepared_xacts WHERE owner=$1),EXISTS(SELECT 1 FROM pg_auth_members WHERE roleid=$2)`, role, oid).Scan(&prepared, &delegated); err != nil {
		return err
	}
	if prepared || delegated {
		return fmt.Errorf("predecessor role has unresolved authority: %w", ErrUnavailable)
	}
	// Commit NOLOGIN before terminating sessions; existing transactions must be
	// rolled back by the server, not merely asked to stop by the old process.
	if _, err = admin.Exec(ctx, "ALTER ROLE "+pgx.Identifier{role}.Sanitize()+" NOLOGIN"); err != nil {
		return err
	}
	for {
		rows, err := admin.Query(ctx, `SELECT pg_terminate_backend(pid,5000) FROM pg_stat_activity WHERE usesysid=$1 AND pid<>pg_backend_pid()`, oid)
		if err != nil {
			return err
		}
		all := true
		for rows.Next() {
			var terminated bool
			if err = rows.Scan(&terminated); err != nil {
				rows.Close()
				return err
			}
			all = all && terminated
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if !all {
			return ErrUnavailable
		}
		var active, login bool
		if err = admin.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE usesysid=$1),rolcanlogin FROM pg_roles WHERE oid=$1`, oid).Scan(&active, &login); err != nil {
			return err
		}
		if login {
			return ErrUnavailable
		}
		if !active {
			return nil
		}
		if err = ctx.Err(); err != nil {
			return err
		}
	}
}
