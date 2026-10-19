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

type NativeMembershipCatalog47Receipt struct {
	NativeMembershipCatalog45Receipt
	BaselineSchema int64 `json:"baseline_schema"`
}

// TransitionNativeMembershipCatalog47 is an explicit trusted installer action
// on an already admitted46 installation. It cannot attest unknown historical
// custody or repair a guard gap. Serving paths never invoke this operation.
func (s *SpaceStore) TransitionNativeMembershipCatalog47(ctx context.Context) (*NativeMembershipCatalog47Receipt, error) {
	if s == nil || s.Pool == nil || s.tx != nil || ctx == nil || ctx.Err() != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, err
	}
	defer rollbackLifecycleTx(ctx, tx)
	r := &NativeMembershipCatalog47Receipt{BaselineSchema: 46}
	r.MigrationSHA256 = nativeTerminal47MigrationSHA
	var installer bool
	if err = tx.QueryRow(ctx, `SELECT current_user=session_user AND session_user=pg_get_userbyid(c.relowner),
 session_user,current_database(),(SELECT oid FROM pg_database WHERE datname=current_database())
 FROM pg_class c WHERE c.oid='native_membership_guard.epoch'::regclass`).Scan(&installer, &r.InstallerRole, &r.DatabaseName, &r.DatabaseOID); err != nil || !installer {
		return nil, ErrMemberAuthorityUnavailable
	}
	if _, err = tx.Exec(ctx, `LOCK TABLE public.schema_migrations IN ACCESS EXCLUSIVE MODE;
 LOCK TABLE native_membership_guard.epoch IN SHARE ROW EXCLUSIVE MODE`); err != nil {
		return nil, err
	}
	names := []string{"native_membership_guard.spaces", "native_membership_guard.frames", "native_membership_guard.profiles", "native_membership_guard.bot_terminals", "native_membership_guard.bot_removal_receipts", "native_membership_guard.catalog46_transition", "native_membership_guard.terminal_maintenance_scope", "native_membership_guard.catalog47_transition", "public.space_join_operations"}
	for _, table := range nativeMembershipGuardCoverage {
		names = append(names, pgx.Identifier{"public", table}.Sanitize())
	}
	if _, err = tx.Exec(ctx, "LOCK TABLE "+strings.Join(names, ",")+" IN ACCESS EXCLUSIVE MODE"); err != nil {
		return nil, err
	}
	var version int64
	if err = tx.QueryRow(ctx, `SELECT version FROM public.schema_migrations WHERE NOT dirty`).Scan(&version); err != nil || version != 47 {
		return nil, ErrMemberAuthorityUnavailable
	}
	if err = checkMemberAuthorityCatalog(ctx, tx); err != nil {
		return nil, err
	}
	var tables []uint32
	for _, name := range []string{"terminal_maintenance_scope", "catalog47_transition"} {
		var oid uint32
		if err = tx.QueryRow(ctx, `SELECT to_regclass($1)::oid`, "native_membership_guard."+name).Scan(&oid); err != nil {
			return nil, err
		}
		tables = append(tables, oid)
	}
	var indexes []uint32
	if err = tx.QueryRow(ctx, `SELECT array_agg(indexrelid::oid ORDER BY indexrelid) FROM pg_index WHERE indrelid=ANY($1::oid[])`, tables).Scan(&indexes); err != nil || len(indexes) != 2 {
		return nil, ErrMemberAuthorityUnavailable
	}
	tables = append(tables, indexes...)
	var functions []uint32
	var sourceBefore uint32
	for _, pin := range nativeReadinessTerminal47Pins {
		var oid uint32
		if err = tx.QueryRow(ctx, `SELECT to_regprocedure($1)::oid`, pin.signature).Scan(&oid); err != nil {
			return nil, err
		}
		if pin.signature == "native_membership_guard.source_before()" {
			sourceBefore = oid
		} else {
			functions = append(functions, oid)
		}
	}
	var constraint uint32
	if err = tx.QueryRow(ctx, `SELECT oid FROM pg_constraint WHERE conrelid='native_membership_guard.spaces'::regclass AND conname='terminal_maintenance_phase_check'`).Scan(&constraint); err != nil {
		return nil, err
	}
	var old, current, retained []byte
	if err = tx.QueryRow(ctx, nativeReadinessPre47ProjectionSQL, tables, functions, constraint, nativeReadinessPre47SourceBefore, sourceBefore).Scan(&old); err != nil {
		return nil, fmt.Errorf("%w: native47 fixed old46 projection: %v", ErrMemberAuthorityUnavailable, err)
	}
	if err = tx.QueryRow(ctx, `SELECT native_membership_guard.catalog()`).Scan(&current); err != nil {
		return nil, err
	}
	var epoch uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT epoch,catalog_sha256 FROM native_membership_guard.epoch WHERE singleton FOR UPDATE`).Scan(&epoch, &retained); err != nil || epoch.String() != "94db7e08-e958-44fa-94b2-988410ab0044" || len(old) != 32 || len(current) != 32 || bytes.Equal(old, current) {
		return nil, ErrMemberAuthorityUnavailable
	}
	// The old admitted46 receipt itself must remain genuine, unchanged, and
	// bound to the exact projected old seal. Mere matching table shapes do not
	// establish a previously admitted installation.
	var oldReceipt bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM native_membership_guard.catalog46_transition WHERE singleton AND epoch=$1 AND migration_sha256=decode($2,'hex') AND new_seal_sha256=$3)`, epoch, nativeBotGlobal46MigrationSHA, old).Scan(&oldReceipt); err != nil || !oldReceipt {
		return nil, ErrMemberAuthorityUnavailable
	}
	migration, err := hex.DecodeString(nativeTerminal47MigrationSHA)
	if err != nil {
		return nil, err
	}
	var savedEpoch uuid.UUID
	var savedMigration, savedOld, savedNew []byte
	err = tx.QueryRow(ctx, `SELECT epoch,migration_sha256,old_seal_sha256,new_seal_sha256 FROM native_membership_guard.catalog47_transition WHERE singleton`).Scan(&savedEpoch, &savedMigration, &savedOld, &savedNew)
	switch err {
	case nil:
		if savedEpoch != epoch || !bytes.Equal(savedMigration, migration) || !bytes.Equal(savedOld, old) || !bytes.Equal(savedNew, current) || !bytes.Equal(retained, current) {
			return nil, ErrMemberAuthorityUnavailable
		}
	case pgx.ErrNoRows:
		if !bytes.Equal(retained, old) {
			return nil, fmt.Errorf("%w: native47 retained seal differs from fixed admitted46", ErrMemberAuthorityUnavailable)
		}
		if _, err = tx.Exec(ctx, `INSERT INTO native_membership_guard.catalog47_transition VALUES(true,$1,$2,$3,$4,clock_timestamp())`, epoch, migration, old, current); err != nil {
			return nil, err
		}
		tag, e := tx.Exec(ctx, `UPDATE native_membership_guard.epoch SET catalog_sha256=$1 WHERE singleton AND epoch=$2 AND catalog_sha256=$3`, current, epoch, old)
		if e != nil || tag.RowsAffected() != 1 {
			return nil, ErrMemberAuthorityUnavailable
		}
		r.Changed = true
	default:
		return nil, err
	}
	r.Epoch = epoch.String()
	r.OldSealSHA256, r.NewSealSHA256 = hex.EncodeToString(old), hex.EncodeToString(current)
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
