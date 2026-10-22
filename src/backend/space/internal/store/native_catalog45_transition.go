package store

import (
	"bytes"
	"context"
	"encoding/hex"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// NativeMembershipCatalog45Receipt is deployment evidence for this one fixed
// transition, never caller authority or a historical privileged-DDL audit.
type NativeMembershipCatalog45Receipt struct {
	DatabaseSystemID string `json:"database_system_id"`
	DatabaseName     string `json:"database_name"`
	DatabaseOID      uint32 `json:"database_oid"`
	TransactionID    string `json:"transaction_id"`
	InstallerRole    string `json:"installer_role"`
	Epoch            string `json:"epoch"`
	MigrationSHA256  string `json:"migration_sha256"`
	OldSealSHA256    string `json:"old_seal_sha256"`
	NewSealSHA256    string `json:"new_seal_sha256"`
	Changed          bool   `json:"changed"`
}

// TransitionNativeMembershipCatalog45 is installer-only maintenance for an
// established trusted44 baseline and controlled exact M45 execution (410c127).
// Unknown installation provenance or a known enforcement gap is a deployment
// HOLD, never repaired by this endpoint comparison. No RPC/reader invokes it.
func (s *SpaceStore) TransitionNativeMembershipCatalog45(ctx context.Context) (*NativeMembershipCatalog45Receipt, error) {
	if s == nil || s.Pool == nil || s.tx != nil || ctx == nil || ctx.Err() != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	defer rollbackLifecycleTx(ctx, tx)
	receipt := &NativeMembershipCatalog45Receipt{MigrationSHA256: "f424f4f8afbf5a2a0aeb0530a1987ada0db6046a76aad769f736f19dce39ff62"}
	var installer bool
	if err = tx.QueryRow(ctx, `SELECT current_user=session_user AND session_user=pg_get_userbyid(c.relowner),
 session_user,current_database(),(SELECT oid FROM pg_database WHERE datname=current_database())
 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='native_membership_guard' AND c.relname='epoch'`).Scan(&installer, &receipt.InstallerRole, &receipt.DatabaseName, &receipt.DatabaseOID); err != nil || !installer {
		return nil, ErrMemberAuthorityUnavailable
	}
	// Marker AEX drains qualified Go writers and serializes the trusted
	// installer. Epoch SRX serializes seal CAS while letting stale scope reads
	// fail closed; the following source locks drain all remaining mutations.
	if _, err = tx.Exec(ctx, `LOCK TABLE public.schema_migrations IN ACCESS EXCLUSIVE MODE;
 LOCK TABLE native_membership_guard.epoch IN SHARE ROW EXCLUSIVE MODE`); err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	names := []string{`native_membership_guard.spaces`, `native_membership_guard.frames`, `native_membership_guard.profiles`, `public.space_join_operations`}
	for _, table := range nativeMembershipGuardCoverage {
		names = append(names, pgx.Identifier{"public", table}.Sanitize())
	}
	if _, err = tx.Exec(ctx, "LOCK TABLE "+strings.Join(names, ",")+" IN ACCESS EXCLUSIVE MODE"); err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	var version int64
	if err = tx.QueryRow(ctx, `SELECT version FROM public.schema_migrations WHERE NOT dirty`).Scan(&version); err != nil || version != 45 {
		return nil, ErrMemberAuthorityUnavailable
	}
	if err = checkMemberAuthorityCatalog(ctx, tx); err != nil {
		return nil, err
	}
	var helperOID uint32
	if err = tx.QueryRow(ctx, `SELECT 'public.guard_join_legacy_selection_immutable()'::regprocedure::oid`).Scan(&helperOID); err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	var oldProjection, newCatalog, retained []byte
	var epoch uuid.UUID
	if err = tx.QueryRow(ctx, nativeReadinessCatalog44ProjectionSQL, helperOID).Scan(&oldProjection); err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	if err = tx.QueryRow(ctx, `SELECT native_membership_guard.catalog()`).Scan(&newCatalog); err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	if err = tx.QueryRow(ctx, `SELECT epoch,catalog_sha256 FROM native_membership_guard.epoch WHERE singleton FOR UPDATE`).Scan(&epoch, &retained); err != nil || epoch.String() != "94db7e08-e958-44fa-94b2-988410ab0044" || len(oldProjection) != 32 || len(newCatalog) != 32 || bytes.Equal(oldProjection, newCatalog) {
		return nil, ErrMemberAuthorityUnavailable
	}
	if !bytes.Equal(retained, oldProjection) && !bytes.Equal(retained, newCatalog) {
		return nil, ErrMemberAuthorityUnavailable
	}
	receipt.Epoch = epoch.String()
	receipt.OldSealSHA256, receipt.NewSealSHA256 = hex.EncodeToString(oldProjection), hex.EncodeToString(newCatalog)
	if !bytes.Equal(retained, newCatalog) {
		tag, err := tx.Exec(ctx, `UPDATE native_membership_guard.epoch SET catalog_sha256=$1 WHERE singleton AND epoch=$2 AND catalog_sha256=$3`, newCatalog, epoch, oldProjection)
		if err != nil || tag.RowsAffected() != 1 {
			return nil, ErrMemberAuthorityUnavailable
		}
		receipt.Changed = true
	}
	// pg_control_system may require stronger installer monitoring permission.
	// Never invent a cluster identity; deployment evidence must bind the actual
	// target when that optional identity cannot be observed by its owner role.
	if err = tx.QueryRow(ctx, `SELECT CASE WHEN has_function_privilege(session_user,'pg_catalog.pg_control_system()','EXECUTE')
	 THEN (SELECT system_identifier::text FROM pg_catalog.pg_control_system()) ELSE '' END,pg_current_xact_id()::text`).Scan(&receipt.DatabaseSystemID, &receipt.TransactionID); err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	if err = checkMemberAuthorityCatalog(ctx, tx); err != nil {
		return nil, err
	}
	var finalCatalog []byte
	if err = tx.QueryRow(ctx, `SELECT native_membership_guard.catalog()`).Scan(&finalCatalog); err != nil || !bytes.Equal(newCatalog, finalCatalog) {
		return nil, ErrMemberAuthorityUnavailable
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	return receipt, nil
}
