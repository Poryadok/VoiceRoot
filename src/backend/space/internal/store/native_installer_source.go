package store

import (
	"bytes"
	"context"
	"encoding/hex"

	"github.com/jackc/pgx/v5"
)

// NativeInstallerSourceFacts are read-only deployment observations, not history
// or authority to adopt a previously unknown installation.
type NativeInstallerSourceFacts struct {
	Version          int64  `json:"version"`
	DatabaseSystemID string `json:"database_system_id"`
	DatabaseName     string `json:"database_name"`
	DatabaseOID      uint32 `json:"database_oid"`
	InstallerRole    string `json:"installer_role"`
	Epoch            string `json:"epoch"`
	SealSHA256       string `json:"seal_sha256"`
}

// InspectNativeInstallerSource authenticates the existing full compiled source
// profile and actual retained seal. It never writes an epoch, runs a transition
// or proves installation provenance. Only the owning installer consumes it.
func (s *SpaceStore) InspectNativeInstallerSource(ctx context.Context, version int64) (*NativeInstallerSourceFacts, error) {
	if s == nil || s.Pool == nil || s.tx != nil || ctx == nil || ctx.Err() != nil || (version != 44 && version != 45 && version != 50 && version != 51) {
		return nil, ErrMemberAuthorityUnavailable
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	defer rollbackLifecycleTx(ctx, tx)
	if err = checkMemberAuthorityCatalog(ctx, tx); err != nil {
		return nil, err
	}
	f := &NativeInstallerSourceFacts{Version: version}
	var valid bool
	if err = tx.QueryRow(ctx, `SELECT count(*)=1 AND bool_and(version=$1 AND NOT dirty) FROM public.schema_migrations`, version).Scan(&valid); err != nil || !valid {
		return nil, ErrMemberAuthorityUnavailable
	}
	if err = tx.QueryRow(ctx, `SELECT current_user=session_user AND session_user=pg_get_userbyid(c.relowner),session_user,current_database(),(SELECT oid FROM pg_database WHERE datname=current_database()), CASE WHEN has_function_privilege(session_user,'pg_catalog.pg_control_system()','EXECUTE') THEN (SELECT system_identifier::text FROM pg_catalog.pg_control_system()) ELSE '' END FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='native_membership_guard' AND c.relname='epoch'`).Scan(&valid, &f.InstallerRole, &f.DatabaseName, &f.DatabaseOID, &f.DatabaseSystemID); err != nil || !valid {
		return nil, ErrMemberAuthorityUnavailable
	}
	var count int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM native_membership_guard.epoch`).Scan(&count); err != nil || count != 1 {
		return nil, ErrMemberAuthorityUnavailable
	}
	var retained, current []byte
	if err = tx.QueryRow(ctx, `SELECT epoch::text,catalog_sha256,native_membership_guard.catalog() FROM native_membership_guard.epoch WHERE singleton`).Scan(&f.Epoch, &retained, &current); err != nil || f.Epoch != "94db7e08-e958-44fa-94b2-988410ab0044" || len(retained) != 32 || !bytes.Equal(retained, current) {
		return nil, ErrMemberAuthorityUnavailable
	}
	if err = tx.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM native_membership_guard.frames)`).Scan(&valid); err != nil || !valid {
		return nil, ErrMemberAuthorityUnavailable
	}
	f.SealSHA256 = hex.EncodeToString(retained)
	if err = tx.Commit(ctx); err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	return f, nil
}
