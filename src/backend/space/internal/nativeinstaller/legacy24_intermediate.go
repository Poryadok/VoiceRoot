package nativeinstaller

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"

	"github.com/jackc/pgx/v5"
	"voice/backend/space/internal/store"
)

// verifyLegacy24Intermediate observes the real migration 25 in one snapshot.
// The originating observation must come from observeLegacy24Origin before DDL;
// the owning controller is responsible for custody and maintenance authority.
func verifyLegacy24Intermediate(ctx context.Context, conn *pgx.Conn, origin *legacy24OriginObservation) (_ *Legacy24Intermediate, err error) {
	if origin == nil || origin.Target == nil || origin.Derived == nil {
		return nil, errors.New("HOLD: missing legacy24 origin")
	}
	tx, err := beginLegacySourceRead(ctx, conn)
	if err != nil {
		return nil, err
	}
	defer finishLegacy24Read(tx, &err)
	catalog, err := inspectLegacy25CatalogTx(ctx, tx)
	if err != nil {
		return nil, errors.New("HOLD: intermediate25 catalog mismatch")
	}
	var noNativeSchema bool
	if err = tx.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM pg_namespace WHERE nspname IN ('native_membership_guard','space_account_ban_guard'))`).Scan(&noNativeSchema); err != nil || !noNativeSchema {
		return nil, errors.New("HOLD: intermediate25 native schema already exists")
	}
	var name, role, current, system string
	var oid uint32
	if err = tx.QueryRow(ctx, `SELECT d.datname,d.oid,session_user,current_user,(SELECT system_identifier::text FROM pg_catalog.pg_control_system()) FROM pg_catalog.pg_database d WHERE d.datname='space_db'`).Scan(&name, &oid, &role, &current, &system); err != nil || role != current {
		return nil, errors.New("HOLD: intermediate25 identity unavailable")
	}
	target := &store.NativeInstallerSourceFacts{Version: 25, DatabaseName: name, DatabaseOID: oid, DatabaseSystemID: system, InstallerRole: role}
	if !sameDatabase(origin.Target, target) {
		return nil, errors.New("HOLD: intermediate25 database identity changed")
	}
	rows, err := captureLegacy25ProjectionTx(ctx, tx, origin.Rows)
	if err != nil || !reflect.DeepEqual(rows, origin.Rows) {
		return nil, errors.New("HOLD: intermediate25 source data changed")
	}
	derived, err := captureLegacy24DerivedTx(ctx, tx)
	currentDerived, currentErr := json.Marshal(derived)
	priorDerived, priorErr := json.Marshal(origin.Derived)
	if err != nil || currentErr != nil || priorErr != nil || string(currentDerived) != string(priorDerived) {
		return nil, errors.New("HOLD: intermediate25 derived data changed")
	}
	extras, err := captureLegacy24ExtrasTx(ctx, tx)
	if err != nil || !reflect.DeepEqual(extras, origin.Extras) {
		return nil, errors.New("HOLD: intermediate25 independent data changed")
	}
	access, err := captureLegacyAccessTx(ctx, tx)
	if err != nil || access != origin.AccessSHA256 {
		return nil, errors.New("HOLD: intermediate25 access changed")
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, errors.New("HOLD: intermediate25 observation failed")
	}
	return &Legacy24Intermediate{Target: target, Clean: true, MigrationSHA256: selectedSQL["000025_tree_audit_actions.up.sql"], CatalogSHA256: catalog, PreimageSHA256: origin.PreimageSHA256, AccessSHA256: access}, nil
}
