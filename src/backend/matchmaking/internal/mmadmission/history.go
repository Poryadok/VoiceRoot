package mmadmission

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	wire "voice/backend/pkg/mmadmissionwire"
)

// HistoryCut is a validated, transaction-local history view. It grants no
// external installation authority and never substitutes for fresh readback.
type HistoryCut struct {
	stamp    wire.AdmissionStampV1
	receipts []wire.ArmedReceiptV1
	records  []wire.ContinuityRecordV1 // current -> genesis
}

func (h HistoryCut) Stamp() wire.AdmissionStampV1 { return h.stamp }

// Hash commits explicit canonical receipt and continuity evidence, rather than
// depending on incidental party-row JSON or treating the head as sufficient.
func (h HistoryCut) Hash() wire.Hash {
	if h.stamp.Validate() != nil || len(h.receipts) == 0 {
		return wire.Hash{}
	}
	body, err := json.Marshal(struct {
		Stamp      wire.AdmissionStampV1
		Receipts   []wire.ArmedReceiptV1
		Continuity []wire.ContinuityRecordV1
	}{h.stamp, h.receipts, h.records})
	if err != nil {
		return wire.Hash{}
	}
	return wire.Hash(sha256.Sum256(append([]byte("voice.mm.validated-history-cut.v1\x00"), body...)))
}

// Authority identifies the current receipt issuer for the root's independent
// descriptor/readback comparison. These data are not installed custody proof.
func (h HistoryCut) Authority() (wire.Scope, wire.ID, wire.ID) {
	if len(h.receipts) == 0 {
		return "", wire.ID{}, wire.ID{}
	}
	r := h.receipts[0]
	return r.AuthorityScope, r.ScopeID, r.ControllerID
}

func ReadHistory(ctx context.Context, tx pgx.Tx, expected wire.Binding) (HistoryCut, error) {
	return readHistory(ctx, tx, expected, false)
}

// ReadTerminalHistory validates retained provenance for an exact terminal
// decision. CLOSED is not forward admission; positive readers still use
// ReadHistory and transitions must use Acquire, never this function as a grant.
func ReadTerminalHistory(ctx context.Context, tx pgx.Tx, expected wire.Binding) (HistoryCut, error) {
	return readHistory(ctx, tx, expected, true)
}

func readHistory(ctx context.Context, tx pgx.Tx, expected wire.Binding, terminal bool) (HistoryCut, error) {
	if tx == nil || expected.Validate() != nil {
		return HistoryCut{}, ErrUnavailable
	}
	if checkHistoryView(ctx, tx) != nil {
		return HistoryCut{}, ErrUnavailable
	}
	actual, lineage, root, err := readHistoryHead(ctx, tx, terminal)
	if err != nil || actual != expected {
		return HistoryCut{}, ErrUnavailable
	}
	bindingHash, err := wire.BindingHash(expected)
	if err != nil {
		return HistoryCut{}, ErrUnavailable
	}
	cut := HistoryCut{stamp: wire.AdmissionStampV1{Version: 1, BindingHash: bindingHash, DatabaseLineageID: lineage, ContinuityHistoryRootHash: root}}
	if cut.stamp.Validate() != nil {
		return HistoryCut{}, ErrUnavailable
	}
	current := expected.AdmissionReceiptHash
	seen := map[wire.Hash]bool{}
	for range 128 {
		if seen[current] {
			return HistoryCut{}, ErrUnavailable
		}
		seen[current] = true
		receipt, err := readHistoryReceipt(ctx, tx, current)
		if err != nil || receipt.DatabaseLineageID != lineage || receipt.Binding.EnvironmentID != expected.EnvironmentID || receipt.Binding.InstallationID != expected.InstallationID || receipt.Binding.AuthorityIncarnation != expected.AuthorityIncarnation {
			return HistoryCut{}, ErrUnavailable
		}
		if len(cut.receipts) > 0 && (receipt.AuthorityScope != cut.receipts[0].AuthorityScope || receipt.ScopeID != cut.receipts[0].ScopeID) {
			return HistoryCut{}, ErrUnavailable
		}
		bound, err := wire.ReceiptBinding(receipt)
		if err != nil || (len(cut.receipts) == 0 && bound != expected) {
			return HistoryCut{}, ErrUnavailable
		}
		if len(cut.receipts) > 0 && receipt.Binding.AdmissionGeneration >= cut.receipts[len(cut.receipts)-1].Binding.AdmissionGeneration {
			return HistoryCut{}, ErrUnavailable
		}
		cut.receipts = append(cut.receipts, receipt)
		var raw, recordHash, predecessor, successor, recordRoot []byte
		err = tx.QueryRow(ctx, `SELECT record_bytes,record_sha256,predecessor_receipt_sha256,successor_receipt_sha256,history_root_sha256 FROM public.mm_native_continuity_records WHERE successor_receipt_sha256=$1`, current[:]).Scan(&raw, &recordHash, &predecessor, &successor, &recordRoot)
		if errors.Is(err, pgx.ErrNoRows) {
			genesis, e := wire.HistoryGenesisHash(current, lineage)
			if e != nil || genesis != root || receipt.PredecessorReceiptHash != nil || receipt.PreviousReceiptHash != nil {
				return HistoryCut{}, ErrUnavailable
			}
			return cut, nil
		}
		var record wire.ContinuityRecordV1
		if err != nil || wire.Decode(raw, &record) != nil {
			return HistoryCut{}, ErrUnavailable
		}
		hash, e := wire.ContinuityHash(record)
		if e != nil || hash != root || !sameHistoryHash(recordHash, hash) || !sameHistoryHash(recordRoot, hash) || !sameHistoryHash(predecessor, record.PredecessorReceiptHash) || !sameHistoryHash(successor, current) || record.SuccessorReceiptHash != current || record.EnvironmentID != expected.EnvironmentID || record.InstallationID != expected.InstallationID || record.AuthorityIncarnation != expected.AuthorityIncarnation || record.DatabaseLineageID != lineage {
			return HistoryCut{}, ErrUnavailable
		}
		if receipt.PredecessorReceiptHash == nil || *receipt.PredecessorReceiptHash != record.PredecessorReceiptHash || receipt.PreviousReceiptHash == nil || *receipt.PreviousReceiptHash != record.PredecessorReceiptHash {
			return HistoryCut{}, ErrUnavailable
		}
		cut.records = append(cut.records, record)
		current = record.PredecessorReceiptHash
		root = record.PreviousHistoryRootHash
	}
	return HistoryCut{}, ErrUnavailable
}

func sameHistoryHash(raw []byte, expected wire.Hash) bool {
	return len(raw) == 32 && slices.Equal(raw, expected[:])
}

func checkHistoryView(ctx context.Context, tx pgx.Tx) error {
	if tx == nil {
		return ErrUnavailable
	}
	var isolation, readOnly string
	if tx.QueryRow(ctx, `SHOW transaction_isolation`).Scan(&isolation) != nil || tx.QueryRow(ctx, `SHOW transaction_read_only`).Scan(&readOnly) != nil {
		return ErrUnavailable
	}
	switch isolation {
	case "repeatable read":
		if readOnly != "on" {
			return ErrUnavailable
		}
	case "read committed":
		// Nonblocking: verification cannot wait behind a closer after taking
		// row locks. Positive writers must still acquire Acquire FIRST.
		var acquired bool
		if tx.QueryRow(ctx, `SELECT pg_catalog.pg_try_advisory_xact_lock_shared($1)`, nativeGateKey()).Scan(&acquired) != nil || !acquired {
			return ErrUnavailable
		}
	default:
		return ErrUnavailable
	}
	return nil
}

// CheckPrecutCut admits only a genuinely never-begun transaction view. It is
// solely for historical v1 capture/verification, never native enrollment.
func CheckPrecutCut(ctx context.Context, tx pgx.Tx) error {
	if checkHistoryView(ctx, tx) != nil {
		return ErrUnavailable
	}
	var exists bool
	if tx.QueryRow(ctx, `SELECT pg_catalog.to_regclass('public.mm_native_admission_head') IS NOT NULL`).Scan(&exists) != nil {
		return ErrUnavailable
	}
	if !exists {
		return nil
	}
	var begun bool
	if tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.mm_native_admission_head)`).Scan(&begun) != nil || begun {
		return ErrUnavailable
	}
	return nil
}

func readHistoryReceipt(ctx context.Context, tx pgx.Tx, hash wire.Hash) (wire.ArmedReceiptV1, error) {
	var raw []byte
	var lineage, authority, transition, environment, installation uuid.UUID
	var generation uint64
	err := tx.QueryRow(ctx, `SELECT receipt_bytes,database_lineage_id,authority_incarnation,transition_id,admission_generation,environment_id,installation_id FROM public.mm_native_admission_receipts WHERE receipt_sha256=$1`, hash[:]).Scan(&raw, &lineage, &authority, &transition, &generation, &environment, &installation)
	var r wire.ArmedReceiptV1
	if err != nil || wire.Decode(raw, &r) != nil {
		return r, ErrUnavailable
	}
	actual, e := wire.ArmedHash(r)
	if e != nil || actual != hash || r.DatabaseLineageID != wire.ID(lineage) || r.Binding.AuthorityIncarnation != wire.ID(authority) || r.TransitionID != wire.ID(transition) || r.Binding.AdmissionGeneration != generation || r.Binding.EnvironmentID != wire.ID(environment) || r.Binding.InstallationID != wire.ID(installation) {
		return wire.ArmedReceiptV1{}, ErrUnavailable
	}
	return r, nil
}

func readHistoryHead(ctx context.Context, tx pgx.Tx, terminal bool) (wire.Binding, wire.ID, wire.Hash, error) {
	var b Binding
	var lineage uuid.UUID
	var root []byte
	hashes := make([][]byte, 6)
	err := tx.QueryRow(ctx, `SELECT protocol_version,environment_id,installation_id,authority_incarnation,transition_id,admission_generation,credential_cohort_id,database_binding_sha256,schema_contract_sha256,source_factory_manifest_sha256,fresh_route_manifest_sha256,admission_receipt_sha256,launch_manifest_sha256,expected_login_role_oid,expected_login_role_name,database_lineage_id,continuity_history_root_sha256 FROM public.mm_native_admission_head WHERE singleton AND (state='ADMITTED' OR ($1 AND state='CLOSED'))`, terminal).Scan(&b.ProtocolVersion, &b.EnvironmentID, &b.InstallationID, &b.AuthorityIncarnation, &b.TransitionID, &b.AdmissionGeneration, &b.CredentialCohortID, &hashes[0], &hashes[1], &hashes[2], &hashes[3], &hashes[4], &hashes[5], &b.ExpectedLoginRoleOID, &b.ExpectedLoginRoleName, &lineage, &root)
	if err != nil || lineage == uuid.Nil || len(root) != 32 {
		return wire.Binding{}, wire.ID{}, wire.Hash{}, ErrUnavailable
	}
	destinations := []*[32]byte{&b.DatabaseBindingHash, &b.SchemaContractHash, &b.SourceFactoryManifestHash, &b.FreshRouteManifestHash, &b.AdmissionReceiptHash, &b.LaunchManifestHash}
	for i, raw := range hashes {
		if len(raw) != 32 {
			return wire.Binding{}, wire.ID{}, wire.Hash{}, ErrUnavailable
		}
		copy(destinations[i][:], raw)
	}
	if b.Validate() != nil {
		return wire.Binding{}, wire.ID{}, wire.Hash{}, ErrUnavailable
	}
	return WireBinding(b), wire.ID(lineage), wire.Hash(root), nil
}

const NativeSourceVersion = "NATIVE_VOICE_C16_V1_ORIGIN_V1"

// AllowsOrigin validates stable original authority through every intervening
// compatible rollout. It never changes or re-enrolls the original origin.
// This checks lineage only: callers MUST separately bind the origin's exact
// party, operation, request, owner capture and private source manifest.
func (h HistoryCut) AllowsOrigin(origin wire.OriginV1, version string) bool {
	if origin.Validate() != nil || h.stamp.Validate() != nil || version != NativeSourceVersion || origin.SourceKind != "NATIVE_VOICE" {
		return false
	}
	for i, r := range h.receipts {
		hash, err := wire.ArmedHash(r)
		if err != nil {
			return false
		}
		if hash != origin.OriginAdmissionReceiptHash {
			continue
		}
		if origin.EnvironmentID != r.Binding.EnvironmentID || origin.InstallationID != r.Binding.InstallationID || origin.AuthorityIncarnation != r.Binding.AuthorityIncarnation || origin.DatabaseLineageID != r.DatabaseLineageID || origin.OriginTransitionID != r.TransitionID || origin.OriginAdmissionGeneration != r.Binding.AdmissionGeneration {
			return false
		}
		for _, record := range h.records[:i] {
			if !slices.Contains(record.SupportedSourceVersions, version) {
				return false
			}
		}
		return true
	}
	return false
}

// WireBinding adapts the existing local guard without redefining wire hashes.
func WireBinding(b Binding) wire.Binding {
	return wire.Binding{ProtocolVersion: b.ProtocolVersion,
		EnvironmentID: wire.ID(b.EnvironmentID), InstallationID: wire.ID(b.InstallationID),
		AuthorityIncarnation: wire.ID(b.AuthorityIncarnation), TransitionID: wire.ID(b.TransitionID),
		AdmissionGeneration: b.AdmissionGeneration, CredentialCohortID: wire.ID(b.CredentialCohortID),
		DatabaseBindingHash: wire.Hash(b.DatabaseBindingHash), SchemaContractHash: wire.Hash(b.SchemaContractHash),
		SourceFactoryManifestHash: wire.Hash(b.SourceFactoryManifestHash), FreshRouteManifestHash: wire.Hash(b.FreshRouteManifestHash),
		AdmissionReceiptHash: wire.Hash(b.AdmissionReceiptHash), LaunchManifestHash: wire.Hash(b.LaunchManifestHash),
		ExpectedLoginRoleOID: b.ExpectedLoginRoleOID, ExpectedLoginRoleName: b.ExpectedLoginRoleName}
}
