package store

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type NativeMembershipCatalog49Receipt struct {
	NativeMembershipCatalog45Receipt
	BaselineSchema int64 `json:"baseline_schema"`
}

// Explicit trusted admitted48 -> fixed49 transition. New objects alone are
// excluded from the exact old projection; no old function body is rewritten.
// This is never called by a serving reader or startup repair path.
func (s *SpaceStore) TransitionNativeMembershipCatalog49(ctx context.Context) (*NativeMembershipCatalog49Receipt, error) {
	if s == nil || s.Pool == nil || s.tx != nil || ctx == nil || ctx.Err() != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, err
	}
	defer rollbackLifecycleTx(ctx, tx)
	r := &NativeMembershipCatalog49Receipt{BaselineSchema: 48}
	r.MigrationSHA256 = nativePurgeChild49MigrationSHA
	var installer bool
	if err = tx.QueryRow(ctx, `SELECT current_user=session_user AND session_user=pg_get_userbyid(c.relowner),session_user,current_database(),(SELECT oid FROM pg_database WHERE datname=current_database()) FROM pg_class c WHERE c.oid='native_membership_guard.epoch'::regclass`).Scan(&installer, &r.InstallerRole, &r.DatabaseName, &r.DatabaseOID); err != nil || !installer {
		return nil, ErrMemberAuthorityUnavailable
	}
	if _, err = tx.Exec(ctx, `LOCK TABLE public.schema_migrations IN ACCESS EXCLUSIVE MODE; LOCK TABLE native_membership_guard.epoch IN SHARE ROW EXCLUSIVE MODE`); err != nil {
		return nil, err
	}
	names := []string{"native_membership_guard.spaces", "native_membership_guard.frames", "native_membership_guard.profiles", "native_membership_guard.bot_terminals", "native_membership_guard.bot_removal_receipts", "native_membership_guard.catalog46_transition", "native_membership_guard.terminal_maintenance_scope", "native_membership_guard.catalog47_transition", "native_membership_guard.bot_terminal_registrations", "native_membership_guard.bot_removal_receipts48", "native_membership_guard.catalog48_transition", "native_membership_guard.bot_purge_children", "native_membership_guard.catalog49_transition", "public.space_join_operations"}
	for _, table := range nativeMembershipGuardCoverage {
		names = append(names, pgx.Identifier{"public", table}.Sanitize())
	}
	if _, err = tx.Exec(ctx, "LOCK TABLE "+strings.Join(names, ",")+" IN ACCESS EXCLUSIVE MODE"); err != nil {
		return nil, err
	}
	var version int64
	if err = tx.QueryRow(ctx, `SELECT version FROM public.schema_migrations WHERE NOT dirty`).Scan(&version); err != nil || version != 49 {
		return nil, ErrMemberAuthorityUnavailable
	}
	if err = checkMemberAuthorityCatalog(ctx, tx); err != nil {
		return nil, err
	}
	var tables []uint32
	for _, name := range []string{"bot_purge_children", "catalog49_transition"} {
		var oid uint32
		if err = tx.QueryRow(ctx, `SELECT to_regclass($1)::oid`, "native_membership_guard."+name).Scan(&oid); err != nil {
			return nil, err
		}
		tables = append(tables, oid)
	}
	var indexes []uint32
	if err = tx.QueryRow(ctx, `SELECT array_agg(indexrelid::oid ORDER BY indexrelid) FROM pg_index WHERE indrelid=ANY($1::oid[])`, tables).Scan(&indexes); err != nil || len(indexes) != 4 {
		return nil, fmt.Errorf("%w: child49 new index closure: %v count=%d", ErrMemberAuthorityUnavailable, err, len(indexes))
	}
	tables = append(tables, indexes...)
	functions := make([]uint32, 0, len(nativeReadinessPurgeChild49Pins))
	for _, pin := range nativeReadinessPurgeChild49Pins {
		var oid uint32
		if err = tx.QueryRow(ctx, `SELECT to_regprocedure($1)::oid`, pin.signature).Scan(&oid); err != nil {
			return nil, err
		}
		functions = append(functions, oid)
	}
	var terminal []uint32
	if err = tx.QueryRow(ctx, `SELECT array_agg(oid ORDER BY oid) FROM pg_trigger WHERE (tgrelid='public.spaces'::regclass AND tgname='zzzzzzzz_bot_purge49_parent_terminal') OR (tgrelid='public.space_lifecycle_aggregates'::regclass AND tgname='zzzzzzzz_bot_purge49_parent_minimize')`).Scan(&terminal); err != nil || len(terminal) != 2 {
		return nil, err
	}
	var old, current, retained []byte
	// This fixed projection operator is identical to the old catalog SQL.
	// The replacement-body map is explicitly empty: all48 bodies stay exact.
	// Match catalog()'s fixed deparse context: pg_get_functiondef includes
	// private composite return types introduced in48. This trusted transaction
	// alone uses the native schema; the final catalog check restores the
	// serving read path before commit.
	if _, err = tx.Exec(ctx, `SET LOCAL search_path=pg_catalog,public,native_membership_guard,pg_temp`); err != nil {
		return nil, err
	}
	if err = tx.QueryRow(ctx, nativeReadinessPre48ProjectionSQL, tables, functions, terminal, map[string]string{}).Scan(&old); err != nil {
		return nil, fmt.Errorf("%w: child49 fixed old projection: %v", ErrMemberAuthorityUnavailable, err)
	}
	if err = tx.QueryRow(ctx, `SELECT native_membership_guard.catalog()`).Scan(&current); err != nil {
		return nil, err
	}
	var epoch uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT epoch,catalog_sha256 FROM native_membership_guard.epoch WHERE singleton FOR UPDATE`).Scan(&epoch, &retained); err != nil || epoch.String() != "94db7e08-e958-44fa-94b2-988410ab0044" || len(old) != 32 || len(current) != 32 || bytes.Equal(old, current) {
		return nil, ErrMemberAuthorityUnavailable
	}
	var chain bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM native_membership_guard.catalog48_transition r48 JOIN native_membership_guard.catalog47_transition r47 ON r47.singleton AND r47.epoch=r48.epoch AND r47.new_seal_sha256=r48.old_seal_sha256 WHERE r48.singleton AND r48.epoch=$1 AND r48.migration_sha256=decode($2,'hex') AND r48.new_seal_sha256=$3 AND r47.migration_sha256=decode($4,'hex'))`, epoch, nativeMinimal48MigrationSHA, old, nativeTerminal47MigrationSHA).Scan(&chain); err != nil || !chain {
		return nil, fmt.Errorf("%w: child49 admitted48 chain: %v", ErrMemberAuthorityUnavailable, err)
	}
	migration, _ := hex.DecodeString(nativePurgeChild49MigrationSHA)
	var savedEpoch uuid.UUID
	var savedMigration, savedOld, savedNew []byte
	err = tx.QueryRow(ctx, `SELECT epoch,migration_sha256,old_seal_sha256,new_seal_sha256 FROM native_membership_guard.catalog49_transition WHERE singleton`).Scan(&savedEpoch, &savedMigration, &savedOld, &savedNew)
	switch err {
	case nil:
		if savedEpoch != epoch || !bytes.Equal(savedMigration, migration) || !bytes.Equal(savedOld, old) || !bytes.Equal(savedNew, current) || !bytes.Equal(retained, current) {
			return nil, ErrMemberAuthorityUnavailable
		}
	case pgx.ErrNoRows:
		if !bytes.Equal(retained, old) {
			return nil, fmt.Errorf("%w: child49 retained48 seal differs from fixed projection", ErrMemberAuthorityUnavailable)
		}
		var unexpected bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM native_membership_guard.bot_purge_children)`).Scan(&unexpected); err != nil || unexpected {
			return nil, ErrMemberAuthorityUnavailable
		}
		if _, err = tx.Exec(ctx, `INSERT INTO native_membership_guard.catalog49_transition VALUES(true,$1,$2,$3,$4,clock_timestamp())`, epoch, migration, old, current); err != nil {
			return nil, err
		}
		command, e := tx.Exec(ctx, `UPDATE native_membership_guard.epoch SET catalog_sha256=$1 WHERE singleton AND epoch=$2 AND catalog_sha256=$3`, current, epoch, old)
		if e != nil || command.RowsAffected() != 1 {
			return nil, ErrMemberAuthorityUnavailable
		}
		r.Changed = true
	default:
		return nil, err
	}
	r.Epoch, r.OldSealSHA256, r.NewSealSHA256 = epoch.String(), hex.EncodeToString(old), hex.EncodeToString(current)
	if err = tx.QueryRow(ctx, `SELECT CASE WHEN has_function_privilege(session_user,'pg_catalog.pg_control_system()','EXECUTE') THEN (SELECT system_identifier::text FROM pg_catalog.pg_control_system()) ELSE '' END,pg_current_xact_id()::text`).Scan(&r.DatabaseSystemID, &r.TransactionID); err != nil {
		return nil, err
	}
	if err = checkMemberAuthorityCatalog(ctx, tx); err != nil {
		return nil, err
	}
	var final []byte
	if err = tx.QueryRow(ctx, `SELECT native_membership_guard.catalog()`).Scan(&final); err != nil || !bytes.Equal(final, current) {
		return nil, ErrMemberAuthorityUnavailable
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return r, nil
}
