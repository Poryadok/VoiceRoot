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

type NativeMembershipCatalog46Receipt struct {
	NativeMembershipCatalog45Receipt
	BaselineSchema int64 `json:"baseline_schema"`
}

// TransitionNativeMembershipCatalog46 extends a trusted controlled44/45
// installation. It cannot establish unknown historical installation custody,
// repair a known enforcement gap, or run from a reader/main startup path.
func (s *SpaceStore) TransitionNativeMembershipCatalog46(ctx context.Context) (*NativeMembershipCatalog46Receipt, error) {
	if s == nil || s.Pool == nil || s.tx != nil || ctx == nil || ctx.Err() != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	defer rollbackLifecycleTx(ctx, tx)
	r := &NativeMembershipCatalog46Receipt{}
	r.MigrationSHA256 = nativeBotGlobal46MigrationSHA
	var installer bool
	if err = tx.QueryRow(ctx, `SELECT current_user=session_user AND session_user=pg_get_userbyid(c.relowner),
 session_user,current_database(),(SELECT oid FROM pg_database WHERE datname=current_database())
 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='native_membership_guard' AND c.relname='epoch'`).Scan(&installer, &r.InstallerRole, &r.DatabaseName, &r.DatabaseOID); err != nil || !installer {
		return nil, ErrMemberAuthorityUnavailable
	}
	if _, err = tx.Exec(ctx, `LOCK TABLE public.schema_migrations IN ACCESS EXCLUSIVE MODE;
 LOCK TABLE native_membership_guard.epoch IN SHARE ROW EXCLUSIVE MODE`); err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	names := []string{"native_membership_guard.spaces", "native_membership_guard.frames", "native_membership_guard.profiles", "native_membership_guard.bot_terminals", "native_membership_guard.bot_removal_receipts", "native_membership_guard.catalog46_transition", "public.space_join_operations"}
	for _, table := range nativeMembershipGuardCoverage {
		names = append(names, pgx.Identifier{"public", table}.Sanitize())
	}
	if _, err = tx.Exec(ctx, "LOCK TABLE "+strings.Join(names, ",")+" IN ACCESS EXCLUSIVE MODE"); err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	var version int64
	if err = tx.QueryRow(ctx, `SELECT version FROM public.schema_migrations WHERE NOT dirty`).Scan(&version); err != nil || version != 46 {
		return nil, ErrMemberAuthorityUnavailable
	}
	if err = checkMemberAuthorityCatalog(ctx, tx); err != nil {
		return nil, err
	}
	tables := make([]uint32, 0, 3)
	for _, name := range []string{"bot_terminals", "bot_removal_receipts", "catalog46_transition"} {
		var oid uint32
		if err = tx.QueryRow(ctx, `SELECT to_regclass($1)::oid`, "native_membership_guard."+name).Scan(&oid); err != nil {
			return nil, ErrMemberAuthorityUnavailable
		}
		tables = append(tables, oid)
	}
	// The fixed old catalog's columns aggregate includes private index
	// attributes too. Their exact definitions/counts were authenticated by the
	// closed46 structural projection above; exclude only indexes of these three
	// new tables, never any preexisting relation or a caller-selected name.
	var indexes []uint32
	if err = tx.QueryRow(ctx, `SELECT array_agg(indexrelid::oid ORDER BY indexrelid) FROM pg_index WHERE indrelid=ANY($1::oid[])`, tables).Scan(&indexes); err != nil || len(indexes) != 5 {
		return nil, ErrMemberAuthorityUnavailable
	}
	tables = append(tables, indexes...)
	functions := make([]uint32, 0, len(nativeReadinessGlobalRemovalPins)+1)
	for _, pin := range nativeReadinessGlobalRemovalPins {
		var oid uint32
		if err = tx.QueryRow(ctx, `SELECT to_regprocedure($1)::oid`, pin.signature).Scan(&oid); err != nil {
			return nil, ErrMemberAuthorityUnavailable
		}
		functions = append(functions, oid)
	}
	triggers := make([]uint32, 0, 2)
	for _, target := range []struct{ table, name string }{{"public.spaces", "zzzzzz_bot_global_receipt_terminal"}, {"public.space_bot_installation_attempts", "a001_bot_global_terminal"}} {
		var oid uint32
		if err = tx.QueryRow(ctx, `SELECT oid FROM pg_trigger WHERE tgrelid=to_regclass($1) AND tgname=$2`, target.table, target.name).Scan(&oid); err != nil {
			return nil, ErrMemberAuthorityUnavailable
		}
		triggers = append(triggers, oid)
	}
	var old45, old44, current, retained []byte
	if err = tx.QueryRow(ctx, nativeReadinessPre46ProjectionSQL, tables, functions, triggers).Scan(&old45); err != nil {
		return nil, fmt.Errorf("%w: native46 old45 projection: %v", ErrMemberAuthorityUnavailable, err)
	}
	var helper uint32
	if err = tx.QueryRow(ctx, `SELECT 'public.guard_join_legacy_selection_immutable()'::regprocedure::oid`).Scan(&helper); err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	functions = append(functions, helper)
	if err = tx.QueryRow(ctx, nativeReadinessPre46ProjectionSQL, tables, functions, triggers).Scan(&old44); err != nil {
		return nil, fmt.Errorf("%w: native46 old44 projection: %v", ErrMemberAuthorityUnavailable, err)
	}
	if err = tx.QueryRow(ctx, `SELECT native_membership_guard.catalog()`).Scan(&current); err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	var epoch uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT epoch,catalog_sha256 FROM native_membership_guard.epoch WHERE singleton FOR UPDATE`).Scan(&epoch, &retained); err != nil || epoch.String() != "94db7e08-e958-44fa-94b2-988410ab0044" || len(current) != 32 || len(old44) != 32 || len(old45) != 32 || bytes.Equal(current, old45) || bytes.Equal(current, old44) {
		return nil, ErrMemberAuthorityUnavailable
	}
	var savedEpoch uuid.UUID
	var savedMigration, savedOld, savedNew []byte
	err = tx.QueryRow(ctx, `SELECT epoch,migration_sha256,old_seal_sha256,new_seal_sha256 FROM native_membership_guard.catalog46_transition WHERE singleton`).Scan(&savedEpoch, &savedMigration, &savedOld, &savedNew)
	migration, decodeErr := hex.DecodeString(nativeBotGlobal46MigrationSHA)
	if decodeErr != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	switch err {
	case nil:
		if savedEpoch != epoch || !bytes.Equal(savedMigration, migration) || !bytes.Equal(savedNew, current) || !bytes.Equal(retained, current) || (!bytes.Equal(savedOld, old44) && !bytes.Equal(savedOld, old45)) {
			return nil, ErrMemberAuthorityUnavailable
		}
		retained = savedOld
	case pgx.ErrNoRows:
		if !bytes.Equal(retained, old44) && !bytes.Equal(retained, old45) {
			return nil, fmt.Errorf("%w: native46 retained seal differs from both fixed old projections", ErrMemberAuthorityUnavailable)
		}
		if _, err = tx.Exec(ctx, `INSERT INTO native_membership_guard.catalog46_transition(singleton,epoch,migration_sha256,old_seal_sha256,new_seal_sha256,transitioned_at) VALUES(true,$1,$2,$3,$4,clock_timestamp())`, epoch, migration, retained, current); err != nil {
			return nil, ErrMemberAuthorityUnavailable
		}
		tag, e := tx.Exec(ctx, `UPDATE native_membership_guard.epoch SET catalog_sha256=$1 WHERE singleton AND epoch=$2 AND catalog_sha256=$3`, current, epoch, retained)
		if e != nil || tag.RowsAffected() != 1 {
			return nil, ErrMemberAuthorityUnavailable
		}
		r.Changed = true
	default:
		return nil, ErrMemberAuthorityUnavailable
	}
	r.BaselineSchema = 45
	if bytes.Equal(retained, old44) {
		r.BaselineSchema = 44
	}
	r.Epoch = epoch.String()
	r.OldSealSHA256, r.NewSealSHA256 = hex.EncodeToString(retained), hex.EncodeToString(current)
	if err = tx.QueryRow(ctx, `SELECT CASE WHEN has_function_privilege(session_user,'pg_catalog.pg_control_system()','EXECUTE') THEN (SELECT system_identifier::text FROM pg_catalog.pg_control_system()) ELSE '' END,pg_current_xact_id()::text`).Scan(&r.DatabaseSystemID, &r.TransactionID); err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	if err = checkMemberAuthorityCatalog(ctx, tx); err != nil {
		return nil, err
	}
	var final []byte
	if err = tx.QueryRow(ctx, `SELECT native_membership_guard.catalog()`).Scan(&final); err != nil || !bytes.Equal(final, current) {
		return nil, ErrMemberAuthorityUnavailable
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	return r, nil
}
