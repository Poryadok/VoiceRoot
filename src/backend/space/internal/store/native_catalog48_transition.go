package store

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"strings"
)

type NativeMembershipCatalog48Receipt struct {
	NativeMembershipCatalog45Receipt
	BaselineSchema int64 `json:"baseline_schema"`
}

// TransitionNativeMembershipCatalog48 is an explicit trusted admitted47
// installer action. Its predecessor47 receipt must bind the admitted46 chain.
// No serving method or startup path invokes it or repairs unknown provenance.
func (s *SpaceStore) TransitionNativeMembershipCatalog48(ctx context.Context) (*NativeMembershipCatalog48Receipt, error) {
	if s == nil || s.Pool == nil || s.tx != nil || ctx == nil || ctx.Err() != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, err
	}
	defer rollbackLifecycleTx(ctx, tx)
	r := &NativeMembershipCatalog48Receipt{BaselineSchema: 47}
	r.MigrationSHA256 = nativeMinimal48MigrationSHA
	var installer bool
	if err = tx.QueryRow(ctx, `SELECT current_user=session_user AND session_user=pg_get_userbyid(c.relowner),session_user,current_database(),(SELECT oid FROM pg_database WHERE datname=current_database()) FROM pg_class c WHERE c.oid='native_membership_guard.epoch'::regclass`).Scan(&installer, &r.InstallerRole, &r.DatabaseName, &r.DatabaseOID); err != nil || !installer {
		return nil, ErrMemberAuthorityUnavailable
	}
	if _, err = tx.Exec(ctx, `LOCK TABLE public.schema_migrations IN ACCESS EXCLUSIVE MODE; LOCK TABLE native_membership_guard.epoch IN SHARE ROW EXCLUSIVE MODE`); err != nil {
		return nil, err
	}
	names := []string{"native_membership_guard.spaces", "native_membership_guard.frames", "native_membership_guard.profiles", "native_membership_guard.bot_terminals", "native_membership_guard.bot_removal_receipts", "native_membership_guard.catalog46_transition", "native_membership_guard.terminal_maintenance_scope", "native_membership_guard.catalog47_transition", "native_membership_guard.bot_terminal_registrations", "native_membership_guard.bot_removal_receipts48", "native_membership_guard.catalog48_transition", "public.space_join_operations"}
	for _, table := range nativeMembershipGuardCoverage {
		names = append(names, pgx.Identifier{"public", table}.Sanitize())
	}
	if _, err = tx.Exec(ctx, "LOCK TABLE "+strings.Join(names, ",")+" IN ACCESS EXCLUSIVE MODE"); err != nil {
		return nil, err
	}
	var version int64
	if err = tx.QueryRow(ctx, `SELECT version FROM public.schema_migrations WHERE NOT dirty`).Scan(&version); err != nil || version != 48 {
		return nil, ErrMemberAuthorityUnavailable
	}
	if err = checkMemberAuthorityCatalog(ctx, tx); err != nil {
		return nil, err
	}
	var tables []uint32
	for _, name := range []string{"bot_terminal_registrations", "bot_removal_receipts48", "catalog48_transition"} {
		var oid uint32
		if err = tx.QueryRow(ctx, `SELECT to_regclass($1)::oid`, "native_membership_guard."+name).Scan(&oid); err != nil {
			return nil, err
		}
		tables = append(tables, oid)
	}
	var indexes []uint32
	if err = tx.QueryRow(ctx, `SELECT array_agg(indexrelid::oid ORDER BY indexrelid) FROM pg_index WHERE indrelid=ANY($1::oid[])`, tables).Scan(&indexes); err != nil || len(indexes) != 6 {
		return nil, ErrMemberAuthorityUnavailable
	}
	tables = append(tables, indexes...)
	var functions []uint32
	for _, pin := range nativeReadinessMinimal48Pins {
		name, _, _ := strings.Cut(pin.signature, "(")
		if _, old := nativeReadinessPre48Bodies[name]; old {
			continue
		}
		var oid uint32
		if err = tx.QueryRow(ctx, `SELECT to_regprocedure($1)::oid`, pin.signature).Scan(&oid); err != nil {
			return nil, err
		}
		functions = append(functions, oid)
	}
	var cleanup uint32
	if err = tx.QueryRow(ctx, `SELECT oid FROM pg_trigger WHERE tgrelid='public.spaces'::regclass AND tgname='zzzzzzz_bot_minimal48_receipt_terminal'`).Scan(&cleanup); err != nil {
		return nil, err
	}
	var old, current, retained []byte
	if err = tx.QueryRow(ctx, nativeReadinessPre48ProjectionSQL, tables, functions, []uint32{cleanup}, nativeReadinessPre48Bodies).Scan(&old); err != nil {
		return nil, fmt.Errorf("%w: fixed48 old47 projection: %v", ErrMemberAuthorityUnavailable, err)
	}
	if err = tx.QueryRow(ctx, `SELECT native_membership_guard.catalog()`).Scan(&current); err != nil {
		return nil, err
	}
	var epoch uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT epoch,catalog_sha256 FROM native_membership_guard.epoch WHERE singleton FOR UPDATE`).Scan(&epoch, &retained); err != nil || epoch.String() != "94db7e08-e958-44fa-94b2-988410ab0044" || len(old) != 32 || len(current) != 32 || bytes.Equal(old, current) {
		return nil, ErrMemberAuthorityUnavailable
	}
	var chain bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM native_membership_guard.catalog47_transition r47 JOIN native_membership_guard.catalog46_transition r46 ON r46.singleton AND r46.epoch=r47.epoch AND r46.new_seal_sha256=r47.old_seal_sha256 WHERE r47.singleton AND r47.epoch=$1 AND r47.migration_sha256=decode($2,'hex') AND r47.new_seal_sha256=$3 AND r46.migration_sha256=decode($4,'hex'))`, epoch, nativeTerminal47MigrationSHA, old, nativeBotGlobal46MigrationSHA).Scan(&chain); err != nil || !chain {
		return nil, ErrMemberAuthorityUnavailable
	}
	migration, _ := hex.DecodeString(nativeMinimal48MigrationSHA)
	var savedEpoch uuid.UUID
	var savedMigration, savedOld, savedNew []byte
	err = tx.QueryRow(ctx, `SELECT epoch,migration_sha256,old_seal_sha256,new_seal_sha256 FROM native_membership_guard.catalog48_transition WHERE singleton`).Scan(&savedEpoch, &savedMigration, &savedOld, &savedNew)
	switch err {
	case nil:
		if savedEpoch != epoch || !bytes.Equal(savedMigration, migration) || !bytes.Equal(savedOld, old) || !bytes.Equal(savedNew, current) || !bytes.Equal(retained, current) {
			return nil, ErrMemberAuthorityUnavailable
		}
	case pgx.ErrNoRows:
		if !bytes.Equal(retained, old) {
			return nil, ErrMemberAuthorityUnavailable
		}
		// No unknown source is adopted. Exact admitted old46 immutable facts are
		// projected under all affected table locks; receipt/time are newly issued.
		var invalid bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM native_membership_guard.bot_removal_receipts r LEFT JOIN native_membership_guard.bot_terminals t USING(bot_id) WHERE t.bot_id IS NULL OR t.decision_id<>r.decision_id OR t.decision_sha256<>r.decision_sha256) OR EXISTS(SELECT 1 FROM native_membership_guard.bot_terminal_registrations) OR EXISTS(SELECT 1 FROM native_membership_guard.bot_removal_receipts48)`).Scan(&invalid); err != nil || invalid {
			return nil, ErrMemberAuthorityUnavailable
		}
		if _, err = tx.Exec(ctx, `INSERT INTO native_membership_guard.bot_terminal_registrations(bot_id,decision_id,receipt_id,registered_at) SELECT bot_id,decision_id,gen_random_uuid(),clock_timestamp() FROM native_membership_guard.bot_terminals ORDER BY bot_id`); err != nil {
			return nil, ErrMemberAuthorityUnavailable
		}
		if _, err = tx.Exec(ctx, `INSERT INTO native_membership_guard.catalog48_transition VALUES(true,$1,$2,$3,$4,clock_timestamp())`, epoch, migration, old, current); err != nil {
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
	r.OldSealSHA256 = hex.EncodeToString(old)
	r.NewSealSHA256 = hex.EncodeToString(current)
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
