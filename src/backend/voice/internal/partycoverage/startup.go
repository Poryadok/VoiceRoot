package partycoverage

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"github.com/jackc/pgx/v5"
)

// CheckStartup must run before any legacy/native fallback decision, including
// when an operator removed the binding configuration after a cutover.
func CheckStartup(ctx context.Context, tx pgx.Tx, binding *RuntimeBinding) error {
	if tx == nil {
		return ErrUnavailable
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT to_regclass('voice_party_coverage_head') IS NOT NULL`).Scan(&exists); err != nil {
		return ErrUnavailable
	}
	if !exists {
		if binding != nil {
			return ErrUnavailable
		}
		return nil
	}
	if binding == nil {
		var begun bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM voice_party_coverage_head)`).Scan(&begun); err != nil || begun {
			return ErrUnavailable
		}
		return nil
	}
	if err := CheckRuntimePrivileges(ctx, tx); err != nil {
		return err
	}
	identity, err := ObserveDatabaseIdentity(ctx, tx)
	if err != nil || identity != binding.DatabaseIdentity {
		return ErrUnavailable
	}
	_, err = ReadCoverage(ctx, tx, *binding)
	return err
}

// ObserveDatabaseIdentity binds the physical PostgreSQL storage lineage and
// actual database, not its reusable DSN/hostname. A restore still requires the
// independently rotated external AuthorityIncarnation. Restricted runtime roles
// get EXECUTE only on this metadata function, never pg_monitor or owner rights.
func ObserveDatabaseIdentity(ctx context.Context, tx pgx.Tx) ([32]byte, error) {
	var data struct{ SystemIdentifier, DatabaseOID, DatabaseName string }
	if tx == nil {
		return [32]byte{}, ErrUnavailable
	}
	err := tx.QueryRow(ctx, `SELECT s.system_identifier::text,d.oid::text,d.datname FROM pg_catalog.pg_control_system() s JOIN pg_catalog.pg_database d ON d.datname=pg_catalog.current_database()`).Scan(&data.SystemIdentifier, &data.DatabaseOID, &data.DatabaseName)
	if err != nil || data.SystemIdentifier == "" || data.DatabaseOID == "" || data.DatabaseName == "" {
		return [32]byte{}, ErrUnavailable
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return [32]byte{}, ErrUnavailable
	}
	return sha256.Sum256(append([]byte("voice.party.database.v1\x00"), raw...)), nil
}
