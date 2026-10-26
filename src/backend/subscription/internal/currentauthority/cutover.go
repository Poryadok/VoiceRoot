// Package currentauthority provides the database fence for Subscription's authority kernel.
// A privilege fence is only one prerequisite: it never certifies shadow rows,
// account eligibility, provider provenance, or runtime activation by itself.
package currentauthority

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"regexp"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUnsafeRole    = errors.New("unsafe Subscription authority role")
	ErrFenceConflict = errors.New("subscription authority fence conflict")
	commitPattern    = regexp.MustCompile(`^[0-9a-f]{40}$`)
)

// FenceRequest is installer-owned immutable source/role inventory. InstallFence
// must never be exposed as a runtime RPC or called using a serving credential.
type FenceRequest struct {
	Generation   string
	SourceCommit string
	WriterRole   string
	ReaderRole   string
	LegacyRoles  []string
}

// Fixed names intentionally include every source-owned legacy/canonical writer
// table. A new authoritative writer table must join this inventory before use.
var authorityTables = []string{
	"subscription_auth_account_observations", "subscription_auth_lifecycle_facts", "subscription_auth_observation_receipts",
	"billing_events", "space_subscriptions", "subscription_entitlement_aggregates",
	"subscription_event_outbox", "subscription_grace_reminders",
	"subscription_kernel_writes",
	"subscription_provider_bindings", "subscription_provider_conflicts",
	"subscription_provider_event_fences", "subscription_provider_outcomes",
	"subscription_provider_versions", "subscription_space_lifecycle_fences",
	"subscription_space_lifecycle_operations", "subscriptions",
}

func tableName(name string) string { return pgx.Identifier{"public", name}.Sanitize() }

// InstallFence drains transactions which touched authority tables, revokes old
// runtime DML, and records only FENCED. Existing and newly opened old connections
// lose the same effective privileges. Failure rolls back grants and evidence.
// An owner/superuser runtime cannot be fenced by REVOKE and is refused.
func InstallFence(ctx context.Context, pool *pgxpool.Pool, request FenceRequest) error {
	id, err := uuid.Parse(request.Generation)
	if err != nil || id == uuid.Nil || id.String() != request.Generation || !commitPattern.MatchString(request.SourceCommit) || request.WriterRole == "" || request.ReaderRole == "" || request.WriterRole == request.ReaderRole || len(request.LegacyRoles) == 0 {
		return ErrUnsafeRole
	}
	request.LegacyRoles = slices.Clone(request.LegacyRoles)
	slices.Sort(request.LegacyRoles)
	for i, role := range request.LegacyRoles {
		if role == "" || role == request.WriterRole || role == request.ReaderRole || (i > 0 && role == request.LegacyRoles[i-1]) {
			return ErrUnsafeRole
		}
	}
	body, err := json.Marshal(request)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(body)
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(17006)"); err != nil {
		return err
	}
	var saved []byte
	err = tx.QueryRow(ctx, "SELECT request_hash FROM public.subscription_authority_fence WHERE singleton FOR UPDATE").Scan(&saved)
	if err == nil {
		if !bytes.Equal(saved, hash[:]) {
			return ErrFenceConflict
		}
		if err = verifyPrivileges(ctx, tx, request); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	roles := append([]string{request.WriterRole, request.ReaderRole}, request.LegacyRoles...)
	for _, role := range roles {
		if err = limitedRole(ctx, tx, role); err != nil {
			return err
		}
	}
	for _, old := range request.LegacyRoles {
		var inherits bool
		if err = tx.QueryRow(ctx, "SELECT pg_has_role($1,$2,'MEMBER') OR pg_has_role($1,$3,'MEMBER')", old, request.WriterRole, request.ReaderRole).Scan(&inherits); err != nil {
			return err
		}
		if inherits {
			return ErrUnsafeRole
		}
	}
	// ACCESS EXCLUSIVE waits for pre-cutover DML/reads. Holding every table lock
	// until commit excludes delayed old transactions throughout privilege changes.
	for _, table := range authorityTables {
		if _, err = tx.Exec(ctx, "LOCK TABLE "+tableName(table)+" IN ACCESS EXCLUSIVE MODE"); err != nil {
			return err
		}
	}
	for _, role := range append([]string{"PUBLIC"}, request.LegacyRoles...) {
		identifier := pgx.Identifier{role}.Sanitize()
		if role == "PUBLIC" {
			identifier = "PUBLIC"
		}
		for _, table := range append(slices.Clone(authorityTables), "subscription_authority_fence") {
			if _, err = tx.Exec(ctx, "REVOKE ALL ON TABLE "+tableName(table)+" FROM "+identifier); err != nil {
				return err
			}
		}
		if _, err = tx.Exec(ctx, "REVOKE CREATE ON SCHEMA public FROM "+identifier); err != nil {
			return err
		}
	}
	for _, role := range request.LegacyRoles {
		for _, table := range authorityTables {
			var writable bool
			if err = tx.QueryRow(ctx, "SELECT has_table_privilege($1,$2,'INSERT,UPDATE,DELETE,TRUNCATE,REFERENCES,TRIGGER')", role, "public."+table).Scan(&writable); err != nil {
				return err
			}
			if writable {
				return ErrUnsafeRole
			}
		}
	}
	for _, role := range []string{request.WriterRole, request.ReaderRole} {
		identifier := pgx.Identifier{role}.Sanitize()
		if _, err = tx.Exec(ctx, "GRANT USAGE ON SCHEMA public TO "+identifier); err != nil {
			return err
		}
		for _, table := range authorityTables {
			if _, err = tx.Exec(ctx, "REVOKE ALL ON TABLE "+tableName(table)+" FROM "+identifier); err != nil {
				return err
			}
			privilege := "SELECT"
			if role == request.WriterRole {
				privilege += ",INSERT,UPDATE,DELETE"
			}
			if _, err = tx.Exec(ctx, "GRANT "+privilege+" ON TABLE "+tableName(table)+" TO "+identifier); err != nil {
				return err
			}
		}
		if _, err = tx.Exec(ctx, "REVOKE ALL ON TABLE public.subscription_authority_fence FROM "+identifier); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, "GRANT SELECT ON TABLE public.subscription_authority_fence TO "+identifier); err != nil {
			return err
		}
		var controlsFence bool
		if err = tx.QueryRow(ctx, "SELECT has_table_privilege($1,'public.subscription_authority_fence','INSERT,UPDATE,DELETE,TRUNCATE,REFERENCES,TRIGGER')", role).Scan(&controlsFence); err != nil {
			return err
		}
		if controlsFence {
			return ErrUnsafeRole
		}
		if role == request.ReaderRole {
			for _, table := range authorityTables {
				var writable bool
				if err = tx.QueryRow(ctx, "SELECT has_table_privilege($1,$2,'INSERT,UPDATE,DELETE,TRUNCATE,REFERENCES,TRIGGER')", role, "public."+table).Scan(&writable); err != nil {
					return err
				}
				if writable {
					return ErrUnsafeRole
				}
			}
		}
	}
	if err = verifyPrivileges(ctx, tx, request); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.subscription_authority_fence(singleton,generation,source_commit,writer_role,reader_role,request_hash,state)
 VALUES(true,$1,$2,(SELECT oid FROM pg_roles WHERE rolname=$3),(SELECT oid FROM pg_roles WHERE rolname=$4),$5,'FENCED')`, request.Generation, request.SourceCommit, request.WriterRole, request.ReaderRole, hash[:])
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// The immutable request is an evidence locator, not proof that grants have not
// drifted since installation. Every repeat must inspect PostgreSQL again.
func verifyPrivileges(ctx context.Context, tx pgx.Tx, request FenceRequest) error {
	// Do not accept an incomplete legacy manifest. The installing session is the
	// sole administrative exception; every other login must be either one of the
	// exact replacement roles or incapable of changing this authority domain.
	rows, err := tx.Query(ctx, "SELECT rolname FROM pg_roles WHERE rolcanlogin AND rolname<>current_user AND rolname<>$1 AND rolname<>$2", request.WriterRole, request.ReaderRole)
	if err != nil {
		return err
	}
	logins, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return err
	}
	for _, role := range logins {
		if err := limitedRole(ctx, tx, role); err != nil {
			return err
		}
		for _, table := range append(slices.Clone(authorityTables), "subscription_authority_fence") {
			if err := noWritePrivilege(ctx, tx, role, table); err != nil {
				return err
			}
		}
	}
	for _, role := range append([]string{request.WriterRole, request.ReaderRole}, request.LegacyRoles...) {
		if err := limitedRole(ctx, tx, role); err != nil {
			return err
		}
	}
	for _, role := range request.LegacyRoles {
		var inherits bool
		if err := tx.QueryRow(ctx, "SELECT pg_has_role($1,$2,'MEMBER') OR pg_has_role($1,$3,'MEMBER')", role, request.WriterRole, request.ReaderRole).Scan(&inherits); err != nil {
			return err
		}
		if inherits {
			return ErrUnsafeRole
		}
		for _, table := range append(slices.Clone(authorityTables), "subscription_authority_fence") {
			if err := noWritePrivilege(ctx, tx, role, table); err != nil {
				return err
			}
		}
	}
	for _, role := range []string{request.WriterRole, request.ReaderRole} {
		if err := noWritePrivilege(ctx, tx, role, "subscription_authority_fence"); err != nil {
			return err
		}
	}
	for _, table := range authorityTables {
		if err := noWritePrivilege(ctx, tx, request.ReaderRole, table); err != nil {
			return err
		}
	}
	return nil
}

func noWritePrivilege(ctx context.Context, tx pgx.Tx, role, table string) error {
	var writable bool
	// MEMBER deliberately over-approximates SET reachability. INHERIT FALSE does
	// not prevent SET ROLE; rejecting every membership ancestor with capabilities
	// avoids certifying an attacker-controlled session after a role switch.
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_roles ancestor
 WHERE pg_has_role((SELECT oid FROM pg_roles WHERE rolname=$1),ancestor.oid,'MEMBER')
 AND has_table_privilege(ancestor.oid,$2,'INSERT,UPDATE,DELETE,TRUNCATE,REFERENCES,TRIGGER'))`, role, "public."+table).Scan(&writable); err != nil {
		return err
	}
	if writable {
		return ErrUnsafeRole
	}
	return nil
}

func limitedRole(ctx context.Context, tx pgx.Tx, role string) error {
	var unsafe bool
	err := tx.QueryRow(ctx, `SELECT r.rolsuper OR r.rolcreaterole OR r.rolcreatedb OR r.rolreplication OR r.rolbypassrls
 OR EXISTS(SELECT 1 FROM pg_roles inherited WHERE pg_has_role(r.oid,inherited.oid,'MEMBER')
	AND (inherited.rolsuper OR inherited.rolcreaterole OR inherited.rolcreatedb OR inherited.rolreplication OR inherited.rolbypassrls
	OR inherited.rolname IN ('pg_read_server_files','pg_write_server_files','pg_execute_server_program')))
 OR pg_has_role(r.oid,(SELECT datdba FROM pg_database WHERE datname=current_database()),'MEMBER')
	OR EXISTS(SELECT 1 FROM pg_namespace n WHERE n.nspname='public' AND pg_has_role(r.oid,n.nspowner,'MEMBER'))
 OR EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
 WHERE n.nspname='public' AND c.relkind IN ('r','p','v','m','S','f') AND pg_has_role(r.oid,c.relowner,'MEMBER'))
 FROM pg_roles r WHERE r.rolname=$1`, role).Scan(&unsafe)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && unsafe) {
		return ErrUnsafeRole
	}
	return err
}
