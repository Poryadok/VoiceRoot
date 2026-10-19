package legacycut

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"sort"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/voice/internal/retirementproof"
)

var r22Tables = []string{"voice_room_instances", "voice_room_memberships", "voice_lifecycle_operations", "voice_lifecycle_effects", "voice_media_epoch_denials", "voice_event_outbox", "voice_lifecycle_redis_divergences"}

func (o *Owner) PrepareCutoverDrain(ctx context.Context, cut retirementproof.CutBinding, id uuid.UUID) (retirementproof.OwnerWorkRef, error) {
	var zero retirementproof.OwnerWorkRef
	if id == uuid.Nil || !validCut(cut) {
		return zero, retirementproof.ErrConflict
	}
	if o == nil || o.Authority == nil || o.Inventory == nil {
		return zero, retirementproof.ErrUnavailable
	}
	if err := o.Authority.CheckMaintenance(ctx, cut); err != nil {
		return zero, err
	}
	if err := o.checkSession(ctx); err != nil {
		return zero, err
	}
	inventory, err := o.Inventory.ReadClosedLegacyInventory(ctx, cut)
	if err != nil {
		return zero, err
	}
	if err := validateInventory(inventory, cut); err != nil {
		return zero, err
	}
	request, err := json.Marshal(struct {
		Protocol  string
		ID        uuid.UUID
		Inventory Inventory
	}{Protocol, id, inventory})
	if err != nil {
		return zero, err
	}
	ref := retirementproof.OwnerWorkRef{Protocol: 1, OwnerProtocol: Protocol, WorkID: id, RequestHash: retirementproof.Hash(retirementproof.OwnerRequestDomain, request)}
	// Retry reads retained evidence before touching a possibly TTL-expired source.
	prior, err := o.ReadOwnerWork(ctx, ref)
	if err == nil {
		return prior.Ref, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return zero, err
	}
	targets := []target{{Kind: "CLOSED_INVENTORY", Inventory: &inventory}}
	for _, endpoint := range inventory.Endpoints {
		if o.Endpoints == nil {
			return zero, retirementproof.ErrUnavailable
		}
		snapshot, err := o.Endpoints.Capture(ctx, endpoint)
		if err != nil {
			return zero, err
		}
		if snapshot.Prefix != endpoint.Prefix {
			return zero, retirementproof.ErrConflict
		}
		if err := snapshot.Validate(); err != nil {
			return zero, err
		}
		targets = append(targets, target{Kind: "REDIS", Endpoint: &endpoint, Snapshot: &snapshot})
	}
	for _, table := range r22Tables {
		rows, err := readR22(ctx, o.Pool, table)
		if err != nil {
			return zero, err
		}
		targets = append(targets, target{Kind: "R22_RETAINED", Table: table, Rows: rows})
	}
	manifest, err := json.Marshal(targets)
	if err != nil {
		return zero, err
	}
	work := retirementproof.OwnerWork{Ref: ref, Purpose: retirementproof.ColdCutDrain, SourceKind: SourceKind, Cut: cut, CapturedHead: retirementproof.OwnerHeadBinding{State: retirementproof.HeadAbsent, Hash: retirementproof.Hash(retirementproof.HeadDomain, []byte(retirementproof.HeadAbsent))}, OwnerRequestDomain: retirementproof.OwnerRequestDomain, OwnerRequestBytes: request, SourceManifestHash: ContractHash(), SealedTargetManifestBytes: manifest, SealedTargetManifestHash: retirementproof.Hash(retirementproof.TargetDomain, manifest), SealedTargetCount: uint64(len(targets)), UnknownIssuanceMarkers: deriveUnknown(targets)}
	encoded, err := json.Marshal(work)
	if err != nil {
		return zero, err
	}
	digest := retirementproof.Hash(retirementproof.WorkDomain, encoded)
	tx, err := o.Pool.Begin(ctx)
	if err != nil {
		return zero, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockWork(ctx, tx, id); err != nil {
		return zero, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO voice_legacy_owner_works(work_id,request_sha256,work_bytes,work_sha256) VALUES($1,$2,$3,$4) ON CONFLICT(work_id) DO NOTHING`, id, ref.RequestHash[:], encoded, digest[:])
	if err != nil {
		return zero, err
	}
	stored, _, _, err := readWork(ctx, tx, ref)
	if err != nil {
		return zero, err
	}
	if !bytes.Equal(stored.OwnerRequestBytes, request) {
		return zero, retirementproof.ErrConflict
	}
	return stored.Ref, tx.Commit(ctx)
}

func (o *Owner) ReadOwnerWork(ctx context.Context, ref retirementproof.OwnerWorkRef) (retirementproof.OwnerWork, error) {
	if err := o.checkSession(ctx); err != nil {
		return retirementproof.OwnerWork{}, err
	}
	w, _, _, err := readWork(ctx, o.Pool, ref)
	if err != nil {
		return w, err
	}
	if o.Inventory == nil {
		return retirementproof.OwnerWork{}, retirementproof.ErrUnavailable
	}
	inventory, err := o.Inventory.ReadClosedLegacyInventory(ctx, w.Cut)
	if err != nil {
		return retirementproof.OwnerWork{}, err
	}
	targets, err := decodeTargets(w)
	if err != nil {
		return retirementproof.OwnerWork{}, err
	}
	if !sameJSON(targets[0].Inventory, inventory) {
		return retirementproof.OwnerWork{}, retirementproof.ErrConflict
	}
	return w, nil
}

func (o *Owner) ReadCutoverDrain(ctx context.Context, ref retirementproof.OwnerWorkRef) (Receipt, error) {
	if err := o.checkSession(ctx); err != nil {
		return Receipt{}, err
	}
	w, state, receipt, err := readWork(ctx, o.Pool, ref)
	if err != nil {
		return Receipt{}, err
	}
	if state != "COMPLETE" {
		return Receipt{}, retirementproof.ErrPending
	}
	return decodeReceipt(w, receipt)
}

// Resume accepts reference identities only. The trusted verifier resolves both
// immutable journals and reobserves installation/resources on EVERY attempt,
// including a completed receipt read after a lost acknowledgement.
func (o *Owner) ResumeCutoverDrain(ctx context.Context, ref retirementproof.OwnerWorkRef, proof retirementproof.ProofRef) (Receipt, error) {
	if o == nil || o.Authority == nil || o.Inventory == nil || o.Verifier == nil {
		return Receipt{}, retirementproof.ErrUnavailable
	}
	w, err := o.ReadOwnerWork(ctx, ref)
	if err != nil {
		return Receipt{}, err
	}
	if err = o.Authority.CheckMaintenance(ctx, w.Cut); err != nil {
		return Receipt{}, err
	}
	inventory, err := o.Inventory.ReadClosedLegacyInventory(ctx, w.Cut)
	if err != nil {
		return Receipt{}, err
	}
	targets, err := decodeTargets(w)
	if err != nil {
		return Receipt{}, err
	}
	if !sameJSON(targets[0].Inventory, inventory) {
		return Receipt{}, retirementproof.ErrConflict
	}
	verified, err := o.Verifier.VerifyForWork(ctx, proof, ref)
	if err != nil {
		return Receipt{}, err
	}
	if err = verified.ValidateFor(w); err != nil {
		return Receipt{}, err
	}
	acceptance, err := json.Marshal(struct {
		Work, Target   retirementproof.Digest
		Proof          retirementproof.ProofRef
		Resolution     []byte
		ResolutionHash retirementproof.Digest
	}{verified.WorkHash(), verified.TargetManifestHash(), proof, verified.DomainResolutionBytes(), verified.DomainResolutionHash()})
	if err != nil {
		return Receipt{}, err
	}
	acceptanceHash := retirementproof.Hash("voice.legacy.acceptance.v1", acceptance)
	// Persist acceptance BEFORE any cleanup; crash recovery always reuses it.
	tx, err := o.Pool.Begin(ctx)
	if err != nil {
		return Receipt{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = lockWork(ctx, tx, ref.WorkID); err != nil {
		return Receipt{}, err
	}
	_, state, receipt, err := readWork(ctx, tx, ref)
	if err != nil {
		return Receipt{}, err
	}
	if state == "COMPLETE" {
		r, err := decodeReceipt(w, receipt)
		if err != nil || r.Proof != proof {
			return Receipt{}, retirementproof.ErrConflict
		}
		return r, tx.Commit(ctx)
	}
	if state == "PREPARED" {
		if _, err = tx.Exec(ctx, `INSERT INTO voice_legacy_owner_acceptances(work_id,acceptance_bytes,acceptance_sha256) VALUES($1,$2,$3)`, ref.WorkID, acceptance, acceptanceHash[:]); err != nil {
			return Receipt{}, err
		}
		if _, err = tx.Exec(ctx, `UPDATE voice_legacy_owner_works SET state='ACCEPTED' WHERE work_id=$1`, ref.WorkID); err != nil {
			return Receipt{}, err
		}
	} else {
		var saved, digest []byte
		if err = tx.QueryRow(ctx, `SELECT acceptance_bytes,acceptance_sha256 FROM voice_legacy_owner_acceptances WHERE work_id=$1`, ref.WorkID).Scan(&saved, &digest); err != nil {
			return Receipt{}, err
		}
		if !bytes.Equal(saved, acceptance) || !bytes.Equal(digest, acceptanceHash[:]) {
			return Receipt{}, retirementproof.ErrConflict
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return Receipt{}, err
	}
	// Each endpoint compares its complete captured bytes atomically. A later
	// endpoint failure leaves ACCEPTED work; already cleaned endpoints exact-retry.
	for _, t := range targets {
		switch t.Kind {
		case "REDIS":
			if o.Endpoints == nil {
				return Receipt{}, retirementproof.ErrUnavailable
			}
			if err = o.Endpoints.CompareCleanup(ctx, *t.Endpoint, *t.Snapshot); err != nil {
				return Receipt{}, err
			}
		case "R22_RETAINED":
			current, e := readR22(ctx, o.Pool, t.Table)
			if e != nil {
				return Receipt{}, e
			}
			if !sameJSON(t.Rows, current) {
				return Receipt{}, retirementproof.ErrConflict
			}
		}
	}
	// Reobserve after bounded cleanup, before recording a terminal receipt.
	verified, err = o.Verifier.VerifyForWork(ctx, proof, ref)
	if err != nil {
		return Receipt{}, err
	}
	if err = verified.ValidateFor(w); err != nil {
		return Receipt{}, err
	}
	if err = o.Authority.CheckMaintenance(ctx, w.Cut); err != nil {
		return Receipt{}, err
	}
	raw, err := json.Marshal(struct {
		Protocol                      string
		Work                          retirementproof.OwnerWorkRef
		Proof                         retirementproof.ProofRef
		WorkHash, Targets, Acceptance retirementproof.Digest
	}{Protocol, ref, proof, verified.WorkHash(), w.SealedTargetManifestHash, acceptanceHash})
	if err != nil {
		return Receipt{}, err
	}
	hash := retirementproof.Hash("voice.legacy.receipt.v1", raw)
	tx, err = o.Pool.Begin(ctx)
	if err != nil {
		return Receipt{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = lockWork(ctx, tx, ref.WorkID); err != nil {
		return Receipt{}, err
	}
	_, state, existing, err := readWork(ctx, tx, ref)
	if err != nil {
		return Receipt{}, err
	}
	if state == "COMPLETE" {
		if !bytes.Equal(existing, raw) {
			return Receipt{}, retirementproof.ErrConflict
		}
	} else if _, err = tx.Exec(ctx, `UPDATE voice_legacy_owner_works SET state='COMPLETE',receipt_bytes=$2,receipt_sha256=$3 WHERE work_id=$1 AND state='ACCEPTED'`, ref.WorkID, raw, hash[:]); err != nil {
		return Receipt{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Receipt{}, err
	}
	return Receipt{ref, proof, raw, hash}, nil
}

type query interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func (o *Owner) checkSession(ctx context.Context) error {
	if o == nil || o.Pool == nil {
		return retirementproof.ErrUnavailable
	}
	var safe bool
	err := o.Pool.QueryRow(ctx, `SELECT current_user=session_user AND NOT EXISTS(SELECT 1 FROM pg_roles r WHERE (r.rolsuper OR r.rolbypassrls OR r.rolcreaterole) AND pg_has_role(session_user,r.oid,'MEMBER')) AND NOT EXISTS(SELECT 1 FROM pg_class c WHERE c.relname IN ('voice_legacy_owner_works','voice_legacy_owner_acceptances') AND c.relowner=(SELECT oid FROM pg_roles WHERE rolname=current_user))`).Scan(&safe)
	if err != nil {
		return err
	}
	if !safe {
		return retirementproof.ErrUnavailable
	}
	return nil
}

func lockWork(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::integer,hashtext($2))`, int32(0x4c474359), id.String())
	return err
}

func readR22(ctx context.Context, q query, table string) ([]json.RawMessage, error) {
	allowed := false
	for _, name := range r22Tables {
		if name == table {
			allowed = true
		}
	}
	if !allowed {
		return nil, retirementproof.ErrConflict
	}
	rows, err := q.Query(ctx, `SELECT to_jsonb(t)::text FROM `+table+` t ORDER BY to_jsonb(t)::text`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []json.RawMessage{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		result = append(result, json.RawMessage(raw))
	}
	return result, rows.Err()
}

func readWork(ctx context.Context, q query, ref retirementproof.OwnerWorkRef) (retirementproof.OwnerWork, string, []byte, error) {
	var w retirementproof.OwnerWork
	var raw, digest, request, receipt, receiptHash []byte
	var state string
	if ref.Protocol != 1 || ref.OwnerProtocol != Protocol || ref.WorkID == uuid.Nil {
		return w, "", nil, retirementproof.ErrConflict
	}
	err := q.QueryRow(ctx, `SELECT work_bytes,work_sha256,request_sha256,state,receipt_bytes,receipt_sha256 FROM voice_legacy_owner_works WHERE work_id=$1`, ref.WorkID).Scan(&raw, &digest, &request, &state, &receipt, &receiptHash)
	if err != nil {
		return w, "", nil, err
	}
	h := retirementproof.Hash(retirementproof.WorkDomain, raw)
	if !bytes.Equal(h[:], digest) || json.Unmarshal(raw, &w) != nil || w.Ref != ref || !bytes.Equal(request, ref.RequestHash[:]) {
		return w, "", nil, retirementproof.ErrConflict
	}
	canonical, err := json.Marshal(w)
	if err != nil || !bytes.Equal(raw, canonical) {
		return w, "", nil, retirementproof.ErrConflict
	}
	if _, err = decodeTargets(w); err != nil {
		return w, "", nil, err
	}
	if state == "COMPLETE" {
		h = retirementproof.Hash("voice.legacy.receipt.v1", receipt)
		if !bytes.Equal(h[:], receiptHash) {
			return w, "", nil, retirementproof.ErrConflict
		}
	}
	return w, state, receipt, nil
}

func decodeTargets(w retirementproof.OwnerWork) ([]target, error) {
	if w.SourceKind != SourceKind || w.SourceManifestHash != ContractHash() || w.Purpose != retirementproof.ColdCutDrain || w.ExistingP3 != nil || w.SpaceID != uuid.Nil || !validCut(w.Cut) || w.OwnerRequestDomain != retirementproof.OwnerRequestDomain || retirementproof.Hash(w.OwnerRequestDomain, w.OwnerRequestBytes) != w.Ref.RequestHash || retirementproof.Hash(retirementproof.TargetDomain, w.SealedTargetManifestBytes) != w.SealedTargetManifestHash {
		return nil, retirementproof.ErrConflict
	}
	var targets []target
	if json.Unmarshal(w.SealedTargetManifestBytes, &targets) != nil || uint64(len(targets)) != w.SealedTargetCount || len(targets) == 0 || targets[0].Kind != "CLOSED_INVENTORY" || targets[0].Inventory == nil {
		return nil, retirementproof.ErrConflict
	}
	inventory := *targets[0].Inventory
	if err := validateInventory(inventory, w.Cut); err != nil {
		return nil, err
	}
	request, _ := json.Marshal(struct {
		Protocol  string
		ID        uuid.UUID
		Inventory Inventory
	}{Protocol, w.Ref.WorkID, inventory})
	if !bytes.Equal(w.OwnerRequestBytes, request) || len(targets) != 1+len(inventory.Endpoints)+len(r22Tables) {
		return nil, retirementproof.ErrConflict
	}
	for i, endpoint := range inventory.Endpoints {
		t := targets[i+1]
		if t.Kind != "REDIS" || t.Endpoint == nil || *t.Endpoint != endpoint || t.Snapshot == nil || t.Snapshot.Prefix != endpoint.Prefix || t.Snapshot.Validate() != nil {
			return nil, retirementproof.ErrConflict
		}
	}
	for i, table := range r22Tables {
		t := targets[i+1+len(inventory.Endpoints)]
		if t.Kind != "R22_RETAINED" || t.Table != table || t.Rows == nil {
			return nil, retirementproof.ErrConflict
		}
		for _, r := range t.Rows {
			if !json.Valid(r) {
				return nil, retirementproof.ErrConflict
			}
		}
	}
	// Derive exhaustive issuance inputs from RAW retained manifest on every read.
	if len(w.CapturedProviderBindings) != 0 || !sameJSON(deriveUnknown(targets), w.UnknownIssuanceMarkers) {
		return nil, retirementproof.ErrConflict
	}
	return targets, nil
}

func deriveUnknown(targets []target) []retirementproof.Digest {
	result := make([]retirementproof.Digest, 0, len(targets))
	for _, t := range targets {
		raw, _ := json.Marshal(t)
		result = append(result, retirementproof.Hash("voice.legacy.unknown-issuance.v1", raw))
	}
	sort.Slice(result, func(i, j int) bool { return bytes.Compare(result[i][:], result[j][:]) < 0 })
	return result
}

func validateInventory(i Inventory, cut retirementproof.CutBinding) error {
	if i.Cut != cut {
		return retirementproof.ErrConflict
	}
	if !validCut(cut) || i.Endpoints == nil || i.ExecutionCohorts == nil || !json.Valid(i.CanonicalDeploymentBytes) {
		return retirementproof.ErrUnavailable
	}
	seen := map[string]bool{}
	for _, e := range i.Endpoints {
		if e.ID == "" || e.Prefix == "" || e.Database < 0 || e.ResourceIdentity == (retirementproof.Digest{}) || seen[e.ID] {
			return retirementproof.ErrConflict
		}
		seen[e.ID] = true
	}
	cohorts := map[uuid.UUID]bool{}
	for _, c := range i.ExecutionCohorts {
		if c.ID == uuid.Nil || c.Implementation == (retirementproof.Digest{}) || !json.Valid(c.MemoryInventoryBytes) || cohorts[c.ID] {
			return retirementproof.ErrConflict
		}
		cohorts[c.ID] = true
	}
	return nil
}

func validCut(c retirementproof.CutBinding) bool {
	if c.EnvironmentID == uuid.Nil || c.AuthorityIncarnation == uuid.Nil || c.CutoverOperationID == uuid.Nil || c.ClosedAdmissionBarrierID == uuid.Nil || c.CoverageEpoch == 0 || c.CoverageEpoch > math.MaxInt64 {
		return false
	}
	for _, d := range []retirementproof.Digest{c.PlanHash, c.DatabaseIdentity, c.SourceCatalogHash, c.SchemaContractHash, c.WriterManifestHash, c.ProviderManifestHash, c.CredentialManifestHash, c.ClosedAdmissionBarrierHash, c.IssuerImplementationManifestHash} {
		if d == (retirementproof.Digest{}) {
			return false
		}
	}
	return true
}

func sameJSON(a, b any) bool {
	x, e := json.Marshal(a)
	y, f := json.Marshal(b)
	return e == nil && f == nil && bytes.Equal(x, y)
}
func decodeReceipt(w retirementproof.OwnerWork, raw []byte) (Receipt, error) {
	var r struct {
		Protocol                      string
		Work                          retirementproof.OwnerWorkRef
		Proof                         retirementproof.ProofRef
		WorkHash, Targets, Acceptance retirementproof.Digest
	}
	if json.Unmarshal(raw, &r) != nil || r.Protocol != Protocol || r.Work != w.Ref || r.Targets != w.SealedTargetManifestHash || r.Proof.ProofID == uuid.Nil {
		return Receipt{}, retirementproof.ErrConflict
	}
	encoded, _ := json.Marshal(w)
	if r.WorkHash != retirementproof.Hash(retirementproof.WorkDomain, encoded) {
		return Receipt{}, retirementproof.ErrConflict
	}
	return Receipt{w.Ref, r.Proof, append([]byte(nil), raw...), retirementproof.Hash("voice.legacy.receipt.v1", raw)}, nil
}
