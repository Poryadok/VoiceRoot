package nativeinstaller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"voice/backend/space/internal/store"
)

// Re-observes, in one read-only snapshot, that the 25..51 Space SQL left the
// original public data, its permitted M39-derived consequences, function rights
// and the access projection intact; independent schemas are retained by the
// construction of the 25..51 chain and are not digested here.
// The 25..51 Space SQL rewrites only public and guard members, so
// independent schemas are retained by construction; the legacy24
// extras predicate is not applied here.
func preserveLegacy25PublicCustody(ctx context.Context, conn *pgx.Conn, origin *legacy24OriginObservation) (_ string, err error) {
	if origin == nil || origin.Target == nil || origin.Derived == nil {
		return "", errors.New("HOLD: legacy25 origin unavailable")
	}
	tx, err := beginLegacySourceRead(ctx, conn)
	if err != nil {
		return "", err
	}
	defer finishLegacy24Read(tx, &err)
	var name, role, current, system string
	var oid uint32
	var clean bool
	if err = tx.QueryRow(ctx, `SELECT d.datname,d.oid,session_user,current_user,(SELECT system_identifier::text FROM pg_catalog.pg_control_system()),(SELECT count(*)=1 AND bool_and(version IN (44,51) AND NOT dirty) FROM public.schema_migrations) FROM pg_catalog.pg_database d WHERE d.datname=current_database() AND d.datname='space_db'`).Scan(&name, &oid, &role, &current, &system, &clean); err != nil || !clean || role != current {
		return "", errors.New("HOLD: legacy25 final target unavailable")
	}
	target := &store.NativeInstallerSourceFacts{DatabaseName: name, DatabaseOID: oid, DatabaseSystemID: system, InstallerRole: role}
	if !sameDatabase(origin.Target, target) {
		return "", errors.New("HOLD: legacy25 final target changed")
	}
	if err = verifyLegacy24DerivedTx(ctx, tx, origin.Rows, origin.Derived); err != nil {
		return "", err
	}
	if _, err = captureLegacy24FunctionsTx(ctx, tx, origin.Functions); err != nil {
		return "", err
	}
	access, err := captureLegacy24StableAccessTx(ctx, tx, origin.Rows)
	if err != nil || access != origin.StableAccessSHA256 {
		return "", errors.New("HOLD: original Space access changed")
	}
	value, err := json.Marshal(struct {
		Profile  string `json:"profile"`
		Preimage string `json:"preimage"`
		Access   string `json:"access"`
	}{"legacy25-preservation-v1", origin.PreimageSHA256, access})
	if err != nil {
		return "", errors.New("HOLD: preservation evidence unavailable")
	}
	if err = tx.Commit(ctx); err != nil {
		return "", errors.New("HOLD: preservation observation failed")
	}
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:]), nil
}
