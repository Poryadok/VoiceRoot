package mmadmissionctl

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	w "voice/backend/pkg/mmadmissionwire"
)

func acquireCloseGate(ctx context.Context, tx pgx.Tx) error {
	var isolation string
	if tx.QueryRow(ctx, `SHOW transaction_isolation`).Scan(&isolation) != nil || isolation != "read committed" {
		return ErrPending
	}
	h := sha256.Sum256([]byte("voice.mm.native-admission.gate.v1\x00"))
	_, e := tx.Exec(ctx, `SELECT pg_catalog.pg_advisory_xact_lock($1)`, int64(binary.BigEndian.Uint64(h[:8])))
	return e
}
func validateHistory(s Snapshot) error {
	if s.Spec == nil || s.Spec.AuthorityScope != w.LocalOwned || len(s.ReceiptHistory) == 0 || len(s.ReceiptHistory) > 128 || len(s.ContinuityHistory) != len(s.ReceiptHistory)-1 || s.ArmedReceipt == nil {
		return ErrPending
	}
	var previous w.Hash
	var root w.Hash
	var generation uint64
	transitions := map[w.ID]bool{}
	for i, r := range s.ReceiptHistory {
		hash, e := w.ArmedHash(r)
		if e != nil || r.AuthorityScope != w.LocalOwned || r.ScopeID != s.Identity.ScopeID || r.ControllerID != s.Identity.ControllerID || r.DatabaseLineageID != s.Identity.DatabaseLineageID || r.Binding.EnvironmentID != s.Identity.EnvironmentID || r.Binding.InstallationID != s.Identity.InstallationID || r.Binding.AuthorityIncarnation != s.Identity.AuthorityIncarnation || r.Binding.AdmissionGeneration <= generation || transitions[r.TransitionID] {
			return ErrPending
		}
		transitions[r.TransitionID] = true
		generation = r.Binding.AdmissionGeneration
		if i == 0 {
			if r.PredecessorReceiptHash != nil || r.PreviousReceiptHash != nil {
				return ErrPending
			}
			root, e = w.HistoryGenesisHash(hash, r.DatabaseLineageID)
		} else {
			if r.PredecessorReceiptHash == nil || r.PreviousReceiptHash == nil || *r.PredecessorReceiptHash != previous || *r.PreviousReceiptHash != previous {
				return ErrPending
			}
			c := s.ContinuityHistory[i-1]
			if c.EnvironmentID != s.Identity.EnvironmentID || c.InstallationID != s.Identity.InstallationID || c.AuthorityIncarnation != s.Identity.AuthorityIncarnation || c.DatabaseLineageID != s.Identity.DatabaseLineageID || c.PredecessorReceiptHash != previous || c.SuccessorReceiptHash != hash || c.PreviousHistoryRootHash != root || len(c.SupportedSourceVersions) != 1 || c.SupportedSourceVersions[0] != "NATIVE_VOICE_C16_V1_ORIGIN_V1" {
				return ErrPending
			}
			root, e = w.ContinuityHash(c)
		}
		if e != nil {
			return e
		}
		previous = hash
	}
	final, e := w.ReceiptBinding(s.ReceiptHistory[len(s.ReceiptHistory)-1])
	if e != nil || final != s.Binding || previous != s.Binding.AdmissionReceiptHash || root != s.HistoryRoot {
		return ErrPending
	}
	hash, e := w.ArmedHash(*s.ArmedReceipt)
	if e != nil || hash != previous {
		return ErrPending
	}
	return nil
}

// installMMHead uses only the separately granted controller login. No serving
// identity or control credential is inferred from a role name or local marker.
// The owning execution driver must first verify its exact external resource.
func installMMHead(ctx context.Context, conn *pgx.Conn, s Snapshot) error {
	if validateHistory(s) != nil || s.Binding.Validate() != nil || s.State != w.Armed {
		return ErrPending
	}
	tx, e := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if e = acquireCloseGate(ctx, tx); e != nil {
		return e
	}
	var user string
	var privileged, delegated bool
	if e = tx.QueryRow(ctx, `SELECT session_user,(r.rolsuper OR r.rolcreatedb OR r.rolcreaterole OR r.rolreplication OR r.rolbypassrls),session_user<>current_user OR EXISTS(SELECT 1 FROM pg_auth_members WHERE member=r.oid) FROM pg_roles r WHERE r.rolname=current_user`).Scan(&user, &privileged, &delegated); e != nil || privileged || delegated || user == s.Binding.ExpectedLoginRoleName {
		return ErrPending
	}
	var servingOID uint32
	var login, servingPrivileged, member bool
	if e = tx.QueryRow(ctx, `SELECT oid,rolcanlogin,(rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls),EXISTS(SELECT 1 FROM pg_auth_members WHERE member=pg_roles.oid) FROM pg_roles WHERE rolname=$1`, s.Binding.ExpectedLoginRoleName).Scan(&servingOID, &login, &servingPrivileged, &member); e != nil || servingOID != s.Binding.ExpectedLoginRoleOID || !login || servingPrivileged || member {
		return ErrPending
	}
	var state string
	var environment, installation, incarnation, lineage *string
	var generation *int64
	var currentReceipt []byte
	e = tx.QueryRow(ctx, `SELECT state,environment_id::text,installation_id::text,authority_incarnation::text,admission_generation,admission_receipt_sha256,database_lineage_id::text FROM public.mm_native_admission_head WHERE singleton FOR UPDATE`).Scan(&state, &environment, &installation, &incarnation, &generation, &currentReceipt, &lineage)
	if e != nil && !errors.Is(e, pgx.ErrNoRows) {
		return e
	}
	b := s.Binding
	if generation != nil {
		if environment == nil || installation == nil || incarnation == nil || lineage == nil || *environment != b.EnvironmentID.String() || *installation != b.InstallationID.String() || *incarnation != b.AuthorityIncarnation.String() || *lineage != s.Identity.DatabaseLineageID.String() || uint64(*generation) > b.AdmissionGeneration {
			return ErrPending
		}
		if uint64(*generation) == b.AdmissionGeneration {
			if !bytes.Equal(currentReceipt, b.AdmissionReceiptHash[:]) {
				return ErrPending
			}
		} else {
			if state != "CLOSED" || len(s.ReceiptHistory) < 2 {
				return ErrPending
			}
			previous := s.ReceiptHistory[len(s.ReceiptHistory)-2]
			hash, _ := w.ArmedHash(previous)
			if uint64(*generation) != previous.Binding.AdmissionGeneration || !bytes.Equal(currentReceipt, hash[:]) {
				return ErrPending
			}
		}
	} else if len(s.ReceiptHistory) != 1 || b.AdmissionGeneration != 1 {
		return ErrPending
	}
	for i, r := range s.ReceiptHistory {
		hash, err := w.ArmedHash(r)
		if err != nil {
			return err
		}
		raw, err := json.Marshal(r)
		if err != nil {
			return err
		}
		var saved []byte
		err = tx.QueryRow(ctx, `SELECT receipt_bytes FROM public.mm_native_admission_receipts WHERE receipt_sha256=$1`, hash[:]).Scan(&saved)
		if err == nil {
			if !bytes.Equal(saved, raw) {
				return ErrConflict
			}
			continue
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if i != len(s.ReceiptHistory)-1 {
			return ErrPending
		}
		if _, err = tx.Exec(ctx, `INSERT INTO public.mm_native_admission_receipts(receipt_sha256,receipt_bytes,database_lineage_id,authority_incarnation,transition_id,admission_generation,environment_id,installation_id)VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, hash[:], raw, r.DatabaseLineageID.String(), r.Binding.AuthorityIncarnation.String(), r.TransitionID.String(), r.Binding.AdmissionGeneration, r.Binding.EnvironmentID.String(), r.Binding.InstallationID.String()); err != nil {
			return err
		}
	}
	for i, r := range s.ContinuityHistory {
		hash, err := w.ContinuityHash(r)
		if err != nil {
			return err
		}
		raw, err := json.Marshal(r)
		if err != nil {
			return err
		}
		var saved []byte
		err = tx.QueryRow(ctx, `SELECT record_bytes FROM public.mm_native_continuity_records WHERE record_sha256=$1`, hash[:]).Scan(&saved)
		if err == nil {
			if !bytes.Equal(saved, raw) {
				return ErrConflict
			}
			continue
		}
		if !errors.Is(err, pgx.ErrNoRows) || i != len(s.ContinuityHistory)-1 {
			return ErrPending
		}
		if _, err = tx.Exec(ctx, `INSERT INTO public.mm_native_continuity_records(record_sha256,record_bytes,predecessor_receipt_sha256,successor_receipt_sha256,history_root_sha256)VALUES($1,$2,$3,$4,$1)`, hash[:], raw, r.PredecessorReceiptHash[:], r.SuccessorReceiptHash[:]); err != nil {
			return err
		}
	}
	_, e = tx.Exec(ctx, `INSERT INTO public.mm_native_admission_head(singleton,protocol_version,state,environment_id,installation_id,authority_incarnation,transition_id,admission_generation,credential_cohort_id,database_binding_sha256,schema_contract_sha256,source_factory_manifest_sha256,fresh_route_manifest_sha256,admission_receipt_sha256,launch_manifest_sha256,expected_login_role_oid,expected_login_role_name,database_lineage_id,continuity_history_root_sha256)VALUES(TRUE,1,'ADMITTED',$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16) ON CONFLICT(singleton)DO UPDATE SET protocol_version=1,state='ADMITTED',environment_id=EXCLUDED.environment_id,installation_id=EXCLUDED.installation_id,authority_incarnation=EXCLUDED.authority_incarnation,transition_id=EXCLUDED.transition_id,admission_generation=EXCLUDED.admission_generation,credential_cohort_id=EXCLUDED.credential_cohort_id,database_binding_sha256=EXCLUDED.database_binding_sha256,schema_contract_sha256=EXCLUDED.schema_contract_sha256,source_factory_manifest_sha256=EXCLUDED.source_factory_manifest_sha256,fresh_route_manifest_sha256=EXCLUDED.fresh_route_manifest_sha256,admission_receipt_sha256=EXCLUDED.admission_receipt_sha256,launch_manifest_sha256=EXCLUDED.launch_manifest_sha256,expected_login_role_oid=EXCLUDED.expected_login_role_oid,expected_login_role_name=EXCLUDED.expected_login_role_name,database_lineage_id=EXCLUDED.database_lineage_id,continuity_history_root_sha256=EXCLUDED.continuity_history_root_sha256`, b.EnvironmentID.String(), b.InstallationID.String(), b.AuthorityIncarnation.String(), b.TransitionID.String(), b.AdmissionGeneration, b.CredentialCohortID.String(), b.DatabaseBindingHash[:], b.SchemaContractHash[:], b.SourceFactoryManifestHash[:], b.FreshRouteManifestHash[:], b.AdmissionReceiptHash[:], b.LaunchManifestHash[:], b.ExpectedLoginRoleOID, b.ExpectedLoginRoleName, s.Identity.DatabaseLineageID.String(), s.HistoryRoot[:])
	if e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func closeMMHead(ctx context.Context, conn *pgx.Conn, expected w.Binding) error {
	if expected.Validate() != nil {
		return ErrPending
	}
	tx, e := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if e = acquireCloseGate(ctx, tx); e != nil {
		return e
	}
	result, e := tx.Exec(ctx, `UPDATE public.mm_native_admission_head SET state='CLOSED' WHERE singleton AND environment_id=$1 AND installation_id=$2 AND authority_incarnation=$3 AND transition_id=$4 AND admission_generation=$5 AND admission_receipt_sha256=$6`, expected.EnvironmentID.String(), expected.InstallationID.String(), expected.AuthorityIncarnation.String(), expected.TransitionID.String(), expected.AdmissionGeneration, expected.AdmissionReceiptHash[:])
	if e != nil {
		return e
	}
	if result.RowsAffected() != 1 {
		return ErrPending
	}
	return tx.Commit(ctx)
}
