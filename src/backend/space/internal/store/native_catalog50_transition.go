package store

import (
	"bytes"
	"context"
	"encoding/hex"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// NativeMembershipCatalog50Receipt describes this fixed trusted installer
// transition. It is neither caller authority nor a privileged-DDL history audit.
type NativeMembershipCatalog50Receipt NativeMembershipCatalog45Receipt

// This is M44's exact catalog expression, retaining every M45 field and OID.
// Only source-qualified M50 FK trigger rows are omitted, and ONLY the exact
// source_after and catalog definitions use their compiled original45 texts. Every
// function OID/owner/ACL and every other original catalog field is retained.
// Both full50 definition and complete FK semantics/closure must be authenticated
// BEFORE this projection is permitted to select an old seal.
const nativeReadinessCatalog45From50ProjectionSQL = `
 SELECT sha256(convert_to(jsonb_build_object(
  'tables',(SELECT jsonb_agg(jsonb_build_array(c.oid,c.relname,c.relowner,c.relkind,c.relpersistence,c.relrowsecurity,c.relforcerowsecurity,
    CASE WHEN n.nspname='native_membership_guard' THEN c.relacl END) ORDER BY c.oid) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
   WHERE (n.nspname='public' AND c.relname=ANY(native_membership_guard.coverage())) OR (n.nspname='native_membership_guard' AND c.relkind='r')),
  'columns',(SELECT jsonb_agg(to_jsonb(a) ORDER BY a.attrelid,a.attnum) FROM pg_attribute a JOIN pg_class c ON c.oid=a.attrelid JOIN pg_namespace n ON n.oid=c.relnamespace
   WHERE a.attnum>0 AND NOT a.attisdropped AND ((n.nspname='public' AND c.relname=ANY(native_membership_guard.coverage())) OR n.nspname='native_membership_guard')),
  'triggers',(SELECT jsonb_agg(to_jsonb(t) ORDER BY t.oid) FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid JOIN pg_namespace n ON n.oid=c.relnamespace
   WHERE ((n.nspname='public' AND c.relname=ANY(native_membership_guard.coverage())) OR n.nspname='native_membership_guard')
   AND NOT EXISTS(SELECT 1 FROM pg_constraint fk JOIN pg_class fc ON fc.oid=fk.conrelid JOIN pg_namespace fn ON fn.oid=fc.relnamespace
    WHERE fk.oid=t.tgconstraint AND fk.contype='f' AND fn.nspname='public' AND fc.relname=ANY($1::text[]))),
  'constraints',(SELECT jsonb_agg(to_jsonb(k) ORDER BY k.oid) FROM pg_constraint k JOIN pg_class c ON c.oid=k.conrelid JOIN pg_namespace n ON n.oid=c.relnamespace
   WHERE (n.nspname='public' AND c.relname=ANY(native_membership_guard.coverage())) OR n.nspname='native_membership_guard'),
  'functions',(SELECT jsonb_agg(jsonb_build_array(p.oid,CASE WHEN p.oid=$2::oid THEN $3::text WHEN p.oid=$4::oid THEN $5::text ELSE pg_get_functiondef(p.oid) END,p.proowner,p.proacl) ORDER BY p.oid)
   FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
   WHERE p.prokind='f' AND n.nspname IN ('public','native_membership_guard'))
 )::text,'UTF8'))`

// TransitionNativeMembershipCatalog50 is installer-only maintenance after an
// established trusted45 baseline and controlled exact M50 execution. The
// installer must retain that provenance and drain writers, as for Transition45.
// Unknown provenance or a known enforcement gap is HOLD, never endpoint repair.
// The trusted installer must exclusively own privileged function/role/ACL DDL
// during that window; relation/advisory locks cannot fence hostile DDL sessions.
// Run before enrolling legacy Spaces and granting the new serving table ACLs;
// idempotent repetition is supported within that same installer window.
// No RPC, ordinary reader, worker or request path may invoke this method.
func (s *SpaceStore) TransitionNativeMembershipCatalog50(ctx context.Context) (*NativeMembershipCatalog50Receipt, error) {
	if s == nil || s.Pool == nil || s.tx != nil || ctx == nil || ctx.Err() != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	defer rollbackLifecycleTx(ctx, tx)
	receipt := &NativeMembershipCatalog50Receipt{MigrationSHA256: "d52240e13e70eead705db61cf462cefc860957d40f7f52bb0bf40962c73dc3f3"}
	var installer bool
	if err = tx.QueryRow(ctx, `SELECT current_user=session_user AND session_user=pg_get_userbyid(c.relowner),
 session_user,current_database(),(SELECT oid FROM pg_database WHERE datname=current_database())
 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='native_membership_guard' AND c.relname='epoch'`).Scan(&installer, &receipt.InstallerRole, &receipt.DatabaseName, &receipt.DatabaseOID); err != nil || !installer {
		return nil, ErrMemberAuthorityUnavailable
	}
	if _, err = tx.Exec(ctx, `LOCK TABLE public.schema_migrations IN ACCESS EXCLUSIVE MODE;
 LOCK TABLE native_membership_guard.epoch IN SHARE ROW EXCLUSIVE MODE`); err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	names := []string{`native_membership_guard.spaces`, `native_membership_guard.frames`, `native_membership_guard.profiles`, `public.space_join_operations`}
	for _, table := range nativeMembershipGuardCoverage {
		names = append(names, pgx.Identifier{"public", table}.Sanitize())
	}
	for _, table := range accountBanTables {
		names = append(names, pgx.Identifier{"public", table}.Sanitize())
	}
	if _, err = tx.Exec(ctx, "LOCK TABLE "+strings.Join(names, ",")+" IN ACCESS EXCLUSIVE MODE"); err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	var version int64
	if err = tx.QueryRow(ctx, `SELECT version FROM public.schema_migrations WHERE NOT dirty`).Scan(&version); err != nil || version != 50 {
		return nil, ErrMemberAuthorityUnavailable
	}
	// Includes the complete added FK-trigger projection. No unexplained trigger
	// can be dropped from the inherited preimage or silently enrolled here.
	if err = checkMemberAuthorityCatalog(ctx, tx); err != nil {
		return nil, err
	}
	observerOID, originalDefinition, err := requireNativeObserver50Definition(ctx, tx)
	if err != nil {
		return nil, err
	}
	catalogOID, originalCatalogDefinition, err := requireNativeCatalog50Definition(ctx, tx)
	if err != nil {
		return nil, err
	}
	var drained bool
	if err = tx.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM native_membership_guard.frames)`).Scan(&drained); err != nil || !drained {
		return nil, ErrMemberAuthorityUnavailable
	}
	var before, current, retained []byte
	var epoch uuid.UUID
	if err = tx.QueryRow(ctx, nativeReadinessCatalog45From50ProjectionSQL, accountBanTables, observerOID, originalDefinition, catalogOID, originalCatalogDefinition).Scan(&before); err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	var equal bool
	if err = tx.QueryRow(ctx, "SELECT native_membership_guard.catalog()=("+nativeCatalog44LiteralExpression+")").Scan(&equal); err != nil || !equal {
		return nil, ErrMemberAuthorityUnavailable
	}
	if err = tx.QueryRow(ctx, `SELECT native_membership_guard.catalog()`).Scan(&current); err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	if err = tx.QueryRow(ctx, `SELECT epoch,catalog_sha256 FROM native_membership_guard.epoch WHERE singleton FOR UPDATE`).Scan(&epoch, &retained); err != nil || epoch.String() != "94db7e08-e958-44fa-94b2-988410ab0044" || len(before) != 32 || len(current) != 32 || bytes.Equal(before, current) {
		return nil, ErrMemberAuthorityUnavailable
	}
	if !bytes.Equal(retained, before) && !bytes.Equal(retained, current) {
		return nil, ErrMemberAuthorityUnavailable
	}
	receipt.Epoch = epoch.String()
	receipt.OldSealSHA256 = hex.EncodeToString(before)
	receipt.NewSealSHA256 = hex.EncodeToString(current)
	if !bytes.Equal(retained, current) {
		tag, err := tx.Exec(ctx, `UPDATE native_membership_guard.epoch SET catalog_sha256=$1 WHERE singleton AND epoch=$2 AND catalog_sha256=$3`, current, epoch, before)
		if err != nil || tag.RowsAffected() != 1 {
			return nil, ErrMemberAuthorityUnavailable
		}
		receipt.Changed = true
	}
	if err = tx.QueryRow(ctx, `SELECT CASE WHEN has_function_privilege(session_user,'pg_catalog.pg_control_system()','EXECUTE')
 THEN (SELECT system_identifier::text FROM pg_catalog.pg_control_system()) ELSE '' END,pg_current_xact_id()::text`).Scan(&receipt.DatabaseSystemID, &receipt.TransactionID); err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	if err = checkMemberAuthorityCatalog(ctx, tx); err != nil {
		return nil, err
	}
	var final []byte
	if err = tx.QueryRow(ctx, `SELECT native_membership_guard.catalog()`).Scan(&final); err != nil || !bytes.Equal(current, final) {
		return nil, ErrMemberAuthorityUnavailable
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	return receipt, nil
}
