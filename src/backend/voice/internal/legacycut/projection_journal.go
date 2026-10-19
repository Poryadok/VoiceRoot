package legacycut

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"voice/backend/voice/internal/retirementproof"
	"voice/backend/voice/internal/store"
)

type OwnershipPlan struct {
	AdditionalObligations []Obligation
	Assignments           []OwnershipAssignment
}
type OwnerPartitionAcceptance struct {
	OwnerWork      retirementproof.OwnerWorkRef
	OwnerWorkHash  retirementproof.Digest
	LegacyWork     retirementproof.OwnerWorkRef
	CaptureHash    retirementproof.Digest
	Partition      PartitionRef
	Proof          retirementproof.ProofRef
	CanonicalBytes []byte
}
type NativeProjectionCommand struct {
	Linkage            ProjectionLinkage
	ResourceID         string
	MethodContractHash retirementproof.Digest
	TargetHashes       []retirementproof.Digest
	Step               store.LegacyProjectionStep
}
type ProjectionState map[string]store.LegacyKeyImage

// ProjectionAuthority is installed trusted owner/source code reading protected
// journals, not a public uploaded partition/command/JSON certificate interface.
// Additional obligations enumerate ALL native targets even with absent Redis.
// Native acceptance verifies its exact W/C/P/proof and native method footprint.
// Resource scope validation observes actual enduring lineage, evidence ACL and
// persistence, cut/external incarnation and qualified closed writer custody.
type ProjectionAuthority interface {
	ResolveOwnership(context.Context, retirementproof.OwnerWork, []Obligation) (OwnershipPlan, error)
	ReadAuthoritativeOwnerWork(context.Context, retirementproof.OwnerWorkRef) (retirementproof.OwnerWork, error)
	ReadOwnerPartitionAcceptance(context.Context, retirementproof.OwnerWorkRef, PartitionRef) (OwnerPartitionAcceptance, error)
	ResolveNativeCommand(context.Context, ProjectionLinkage, ProjectionState) (NativeProjectionCommand, error)
	ValidateResourceScope(context.Context, retirementproof.CutBinding, retirementproof.ProofRef, Endpoint) error
}

type ProjectionJournal struct {
	Owner     *Owner
	Authority ProjectionAuthority
	// Routing only. Authority validates exact physical scope independently.
	Clients map[string]*redis.Client
}

// ReadPartition is the protected native-owner intake reader. It rederives the
// complete partition from retained raw capture and authoritative owner journals,
// and re-verifies every work/proof binding before returning it. Native owners
// call it before writing their own W/C/P acknowledgement; acknowledgements are
// intentionally not a prerequisite of this read. It grants no mutation right.
func (j *ProjectionJournal) ReadPartition(ctx context.Context, ref PartitionRef) (Partition, error) {
	p, err := j.readPartition(ctx, ref)
	if err != nil {
		return Partition{}, err
	}
	w, err := j.Owner.ReadOwnerWork(ctx, p.LegacyWork)
	if err != nil {
		return Partition{}, err
	}
	if _, err = j.verifyOwners(ctx, p, w, false); err != nil {
		return Partition{}, err
	}
	return p, nil
}

func (j *ProjectionJournal) RegisterPartition(ctx context.Context, legacy retirementproof.OwnerWorkRef, proof retirementproof.ProofRef, id uuid.UUID) (PartitionRef, error) {
	if j == nil || j.Owner == nil || j.Authority == nil || id == uuid.Nil {
		return PartitionRef{}, retirementproof.ErrUnavailable
	}
	w, err := j.Owner.ReadOwnerWork(ctx, legacy)
	if err != nil {
		return PartitionRef{}, err
	}
	if err = j.verifyWork(ctx, w, proof); err != nil {
		return PartitionRef{}, err
	}
	obligations, assignments, err := j.ownership(ctx, w)
	if err != nil {
		return PartitionRef{}, err
	}
	p := Partition{ID: id, Cut: w.Cut, LegacyWork: legacy, CaptureHash: w.SealedTargetManifestHash, Proof: proof, Assignments: assignments, Obligations: obligations}
	if err = ValidatePartitionShape(p, obligations); err != nil {
		return PartitionRef{}, err
	}
	if _, err = j.verifyOwners(ctx, p, w, false); err != nil {
		return PartitionRef{}, err
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return PartitionRef{}, err
	}
	if len(raw) > 16*1024*1024 {
		return PartitionRef{}, retirementproof.ErrUnsupported
	}
	hash, _ := PartitionHash(p)
	ref := PartitionRef{id, hash}
	_, err = j.Owner.Pool.Exec(ctx, `INSERT INTO voice_legacy_partitions(partition_id,work_id,canonical_bytes,canonical_sha256) VALUES($1,$2,$3,$4) ON CONFLICT(partition_id) DO NOTHING`, id, legacy.WorkID, raw, hash[:])
	if err != nil {
		return PartitionRef{}, err
	}
	stored, err := j.readPartition(ctx, ref)
	if err != nil {
		return PartitionRef{}, err
	}
	if !sameCanonical(stored, p) {
		return PartitionRef{}, retirementproof.ErrConflict
	}
	return ref, nil
}

func (j *ProjectionJournal) AcceptLegacyPartition(ctx context.Context, ref PartitionRef) error {
	p, err := j.readPartition(ctx, ref)
	if err != nil {
		return err
	}
	w, err := j.Owner.ReadOwnerWork(ctx, p.LegacyWork)
	if err != nil {
		return err
	}
	if _, err = j.verifyOwners(ctx, p, w, false); err != nil {
		return err
	}
	accept := OwnerPartitionAcceptance{OwnerWork: w.Ref, OwnerWorkHash: workHash(w), LegacyWork: w.Ref, CaptureHash: p.CaptureHash, Partition: ref, Proof: p.Proof}
	raw, _ := json.Marshal(accept)
	hash := retirementproof.Hash("voice.legacy.partition-acceptance.v1", raw)
	tx, err := j.Owner.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = lockWork(ctx, tx, w.Ref.WorkID); err != nil {
		return err
	}
	_, state, _, err := readWork(ctx, tx, w.Ref)
	if err != nil {
		return err
	}
	if state == "PREPARED" {
		if _, err = tx.Exec(ctx, `INSERT INTO voice_legacy_owner_acceptances(work_id,acceptance_bytes,acceptance_sha256) VALUES($1,$2,$3)`, w.Ref.WorkID, raw, hash[:]); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE voice_legacy_owner_works SET state='ACCEPTED' WHERE work_id=$1`, w.Ref.WorkID); err != nil {
			return err
		}
	} else {
		var saved, digest []byte
		if err = tx.QueryRow(ctx, `SELECT acceptance_bytes,acceptance_sha256 FROM voice_legacy_owner_acceptances WHERE work_id=$1`, w.Ref.WorkID).Scan(&saved, &digest); err != nil {
			return err
		}
		if !bytes.Equal(saved, raw) || !bytes.Equal(digest, hash[:]) {
			return retirementproof.ErrConflict
		}
	}
	return tx.Commit(ctx)
}

func (j *ProjectionJournal) PrepareProjection(ctx context.Context, link ProjectionLinkage) (store.LegacyProjectionStepRef, error) {
	p, w, acceptances, err := j.qualified(ctx, link)
	if err != nil {
		return store.LegacyProjectionStepRef{}, err
	}
	// Exact replay returns the already fixed intent before any new time/nonce.
	var savedID uuid.UUID
	var savedHash []byte
	err = j.Owner.Pool.QueryRow(ctx, `SELECT step_id,step_sha256 FROM voice_legacy_projection_steps WHERE step_id=$1`, link.NativeCommandRef).Scan(&savedID, &savedHash)
	if err == nil {
		var h retirementproof.Digest
		copy(h[:], savedHash)
		ref := store.LegacyProjectionStepRef{ID: savedID, Hash: h}
		_, storedLink, _, err := j.readStep(ctx, ref)
		if err != nil || storedLink != link {
			return store.LegacyProjectionStepRef{}, retirementproof.ErrConflict
		}
		if _, err = j.ResolveProjectionStep(ctx, ref); err != nil {
			return store.LegacyProjectionStepRef{}, err
		}
		return ref, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return store.LegacyProjectionStepRef{}, err
	}
	// The semantic owner resolves its immutable command with state from C/T,
	// never an external read-before / unmodified native method / read-after wrap.
	state, previous, ordinal, err := j.currentState(ctx, p, w, "", nil)
	if err != nil {
		return store.LegacyProjectionStepRef{}, err
	}
	command, err := j.Authority.ResolveNativeCommand(ctx, link, state)
	if err != nil {
		return store.LegacyProjectionStepRef{}, err
	}
	endpoint, err := findEndpoint(w, command.ResourceID)
	if err != nil {
		return store.LegacyProjectionStepRef{}, err
	}
	state, previous, ordinal, err = j.currentState(ctx, p, w, command.ResourceID, nil)
	if err != nil {
		return store.LegacyProjectionStepRef{}, err
	}
	if command.Linkage != link || command.MethodContractHash == (retirementproof.Digest{}) || !digestContains(command.TargetHashes, link.NativeTargetHash) {
		return store.LegacyProjectionStepRef{}, retirementproof.ErrConflict
	}
	native, err := j.ownerWork(ctx, link.NativeOwnerWorkRef, w)
	if err != nil {
		return store.LegacyProjectionStepRef{}, err
	}
	if command.MethodContractHash != native.SourceManifestHash {
		return store.LegacyProjectionStepRef{}, retirementproof.ErrConflict
	}
	if err = j.Authority.ValidateResourceScope(ctx, p.Cut, p.Proof, endpoint); err != nil {
		return store.LegacyProjectionStepRef{}, err
	}
	step := command.Step
	step.ID = link.NativeCommandRef
	step.CaptureID = w.Ref.WorkID
	step.Ordinal = ordinal
	step.PreviousReceipt = previous
	if step.ScopePrefix != endpoint.Prefix || endpoint.EvidencePrefix == "" || step.EvidencePrefix != endpoint.EvidencePrefix {
		return store.LegacyProjectionStepRef{}, retirementproof.ErrConflict
	}
	if retirementproof.Hash("voice.legacy.redis-instance.v1", []byte(step.ExpectedRunID)) != endpoint.ResourceIdentity {
		return store.LegacyProjectionStepRef{}, retirementproof.ErrConflict
	}
	if err = validateFootprint(p, link, command, state); err != nil {
		return store.LegacyProjectionStepRef{}, err
	}
	step.BindingBytes, err = json.Marshal(struct {
		Link        ProjectionLinkage
		Partition   PartitionRef
		Capture     retirementproof.Digest
		Command     NativeProjectionCommand
		Acceptances []OwnerPartitionAcceptance
		Endpoint    Endpoint
	}{link, link.PartitionRef, p.CaptureHash, command, acceptances, endpoint})
	if err != nil {
		return store.LegacyProjectionStepRef{}, err
	}
	if err = store.ValidateLegacyProjectionIntent(step); err != nil {
		return store.LegacyProjectionStepRef{}, err
	}
	hash, err := store.LegacyProjectionStepHash(step)
	if err != nil {
		return store.LegacyProjectionStepRef{}, err
	}
	raw, _ := json.Marshal(step)
	linkRaw, _ := json.Marshal(link)
	_, err = j.Owner.Pool.Exec(ctx, `INSERT INTO voice_legacy_projection_steps(step_id,partition_id,resource_id,ordinal,linkage_bytes,step_bytes,step_sha256) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(step_id) DO NOTHING`, step.ID, p.ID, command.ResourceID, int64(ordinal), linkRaw, raw, hash[:])
	if err != nil {
		return store.LegacyProjectionStepRef{}, err
	}
	ref := store.LegacyProjectionStepRef{ID: step.ID, Hash: hash}
	stored, storedLink, _, err := j.readStep(ctx, ref)
	if err != nil || storedLink != link || !sameCanonical(step, stored) {
		return store.LegacyProjectionStepRef{}, retirementproof.ErrConflict
	}
	return ref, nil
}

func (j *ProjectionJournal) ResumeProjection(ctx context.Context, link ProjectionLinkage, ref store.LegacyProjectionStepRef) (ProjectionProgress, error) {
	step, storedLink, resource, err := j.readStep(ctx, ref)
	if err != nil || storedLink != link {
		return ProjectionProgress{}, retirementproof.ErrConflict
	}
	if _, err = j.ResolveProjectionStep(ctx, ref); err != nil {
		return ProjectionProgress{}, err
	}
	client := j.Clients[resource]
	if client == nil {
		return ProjectionProgress{}, retirementproof.ErrUnavailable
	}
	// No native/global PostgreSQL lock is held across this external command.
	receipt, err := store.NewLegacyProjectionExecutor(client, j).Apply(ctx, ref)
	if err != nil {
		return ProjectionProgress{}, err
	}
	hash := retirementproof.Hash("voice.legacy.projection-receipt.v1", receipt)
	_, err = j.Owner.Pool.Exec(ctx, `UPDATE voice_legacy_projection_steps SET receipt_bytes=$2,receipt_sha256=$3 WHERE step_id=$1 AND receipt_bytes IS NULL`, step.ID, receipt, hash[:])
	if err != nil {
		return ProjectionProgress{}, err
	}
	return j.ReadProjection(ctx, link, ref)
}

func (j *ProjectionJournal) ReadProjection(ctx context.Context, link ProjectionLinkage, ref store.LegacyProjectionStepRef) (ProjectionProgress, error) {
	_, storedLink, _, err := j.readStep(ctx, ref)
	if err != nil || storedLink != link {
		return ProjectionProgress{}, retirementproof.ErrConflict
	}
	if _, err = j.ResolveProjectionStep(ctx, ref); err != nil {
		return ProjectionProgress{}, err
	}
	var raw, digest []byte
	err = j.Owner.Pool.QueryRow(ctx, `SELECT receipt_bytes,receipt_sha256 FROM voice_legacy_projection_steps WHERE step_id=$1`, ref.ID).Scan(&raw, &digest)
	if err != nil {
		return ProjectionProgress{}, err
	}
	if raw == nil {
		return ProjectionProgress{}, retirementproof.ErrPending
	}
	hash := retirementproof.Hash("voice.legacy.projection-receipt.v1", raw)
	if !bytes.Equal(hash[:], digest) {
		return ProjectionProgress{}, retirementproof.ErrConflict
	}
	return ProjectionProgress{Linkage: link, Step: ref, CanonicalReceipt: raw, ReceiptHash: hash}, nil
}

func (j *ProjectionJournal) ResolveProjectionStep(ctx context.Context, ref store.LegacyProjectionStepRef) (store.LegacyProjectionStep, error) {
	step, link, resource, err := j.readStep(ctx, ref)
	if err != nil {
		return step, err
	}
	p, w, acceptances, err := j.qualified(ctx, link)
	if err != nil {
		return store.LegacyProjectionStep{}, err
	}
	endpoint, err := findEndpoint(w, resource)
	if err != nil {
		return store.LegacyProjectionStep{}, err
	}
	if err = j.Authority.ValidateResourceScope(ctx, p.Cut, p.Proof, endpoint); err != nil {
		return store.LegacyProjectionStep{}, err
	}
	state, previous, ordinal, err := j.currentState(ctx, p, w, resource, &step.Ordinal)
	if err != nil {
		return store.LegacyProjectionStep{}, err
	}
	if ordinal != step.Ordinal || !bytes.Equal(previous, step.PreviousReceipt) {
		return store.LegacyProjectionStep{}, retirementproof.ErrConflict
	}
	var binding struct {
		Link     ProjectionLinkage
		Command  NativeProjectionCommand
		Endpoint Endpoint
	}
	if json.Unmarshal(step.BindingBytes, &binding) != nil || binding.Link != link || binding.Endpoint != endpoint {
		return store.LegacyProjectionStep{}, retirementproof.ErrConflict
	}
	command, err := j.Authority.ResolveNativeCommand(ctx, link, state)
	if err != nil {
		return store.LegacyProjectionStep{}, err
	}
	if !sameCanonical(command, binding.Command) {
		return store.LegacyProjectionStep{}, retirementproof.ErrConflict
	}
	native, err := j.ownerWork(ctx, link.NativeOwnerWorkRef, w)
	if err != nil {
		return store.LegacyProjectionStep{}, err
	}
	if command.Linkage != link || command.ResourceID != resource || command.MethodContractHash != native.SourceManifestHash || !digestContains(command.TargetHashes, link.NativeTargetHash) || step.ScopePrefix != endpoint.Prefix || step.EvidencePrefix != endpoint.EvidencePrefix || retirementproof.Hash("voice.legacy.redis-instance.v1", []byte(step.ExpectedRunID)) != endpoint.ResourceIdentity {
		return store.LegacyProjectionStep{}, retirementproof.ErrConflict
	}
	expectedBinding, err := json.Marshal(struct {
		Link        ProjectionLinkage
		Partition   PartitionRef
		Capture     retirementproof.Digest
		Command     NativeProjectionCommand
		Acceptances []OwnerPartitionAcceptance
		Endpoint    Endpoint
	}{link, link.PartitionRef, p.CaptureHash, command, acceptances, endpoint})
	if err != nil || !bytes.Equal(expectedBinding, step.BindingBytes) {
		return store.LegacyProjectionStep{}, retirementproof.ErrConflict
	}
	if err = validateFootprint(p, link, command, state); err != nil {
		return store.LegacyProjectionStep{}, err
	}
	expected := command.Step
	expected.ID = step.ID
	expected.CaptureID = w.Ref.WorkID
	expected.Ordinal = ordinal
	expected.PreviousReceipt = previous
	expected.BindingBytes = step.BindingBytes
	if !sameCanonical(expected, step) {
		return store.LegacyProjectionStep{}, retirementproof.ErrConflict
	}
	if err = store.ValidateLegacyProjectionIntent(step); err != nil {
		return store.LegacyProjectionStep{}, err
	}
	return step, nil
}

func (j *ProjectionJournal) qualified(ctx context.Context, link ProjectionLinkage) (Partition, retirementproof.OwnerWork, []OwnerPartitionAcceptance, error) {
	var w retirementproof.OwnerWork
	p, err := j.readPartition(ctx, link.PartitionRef)
	if err != nil {
		return p, w, nil, err
	}
	if p.LegacyWork != link.LegacyWorkRef || p.Proof != link.ResourceProofRef || link.NativeOwnerWorkRef.WorkID == uuid.Nil || link.NativeCommandRef == uuid.Nil || link.NativeTargetHash == (retirementproof.Digest{}) {
		return p, w, nil, retirementproof.ErrConflict
	}
	w, err = j.Owner.ReadOwnerWork(ctx, p.LegacyWork)
	if err != nil {
		return p, w, nil, err
	}
	present := link.NativeOwnerWorkRef == w.Ref
	for _, a := range p.Assignments {
		if a.Owner == link.NativeOwnerWorkRef && a.TargetHash == link.NativeTargetHash {
			present = true
		}
	}
	if !present {
		return p, w, nil, retirementproof.ErrConflict
	}
	acks, err := j.verifyOwners(ctx, p, w, true)
	return p, w, acks, err
}

func (j *ProjectionJournal) readPartition(ctx context.Context, ref PartitionRef) (Partition, error) {
	var p Partition
	if j == nil || j.Owner == nil || j.Authority == nil {
		return p, retirementproof.ErrUnavailable
	}
	if err := j.Owner.checkSession(ctx); err != nil {
		return p, err
	}
	var raw, digest []byte
	err := j.Owner.Pool.QueryRow(ctx, `SELECT canonical_bytes,canonical_sha256 FROM voice_legacy_partitions WHERE partition_id=$1`, ref.ID).Scan(&raw, &digest)
	if err != nil {
		return p, err
	}
	if json.Unmarshal(raw, &p) != nil {
		return p, retirementproof.ErrConflict
	}
	hash, _ := PartitionHash(p)
	if p.ID != ref.ID || hash != ref.Hash || !bytes.Equal(hash[:], digest) {
		return p, retirementproof.ErrConflict
	}
	canonical, _ := json.Marshal(p)
	if !bytes.Equal(canonical, raw) {
		return p, retirementproof.ErrConflict
	}
	w, err := j.Owner.ReadOwnerWork(ctx, p.LegacyWork)
	if err != nil {
		return p, err
	}
	if p.Cut != w.Cut || p.CaptureHash != w.SealedTargetManifestHash {
		return p, retirementproof.ErrConflict
	}
	obligations, assignments, err := j.ownership(ctx, w)
	if err != nil {
		return p, err
	}
	if !sameCanonical(obligations, p.Obligations) || !sameCanonical(assignments, p.Assignments) {
		return p, retirementproof.ErrConflict
	}
	return p, ValidatePartitionShape(p, obligations)
}

func (j *ProjectionJournal) ownership(ctx context.Context, w retirementproof.OwnerWork) ([]Obligation, []OwnershipAssignment, error) {
	raw, err := DeriveObligations(w)
	if err != nil {
		return nil, nil, err
	}
	plan, err := j.Authority.ResolveOwnership(ctx, w, raw)
	if err != nil {
		return nil, nil, err
	}
	all := append(append([]Obligation{}, raw...), plan.AdditionalObligations...)
	if len(all) > 65536 {
		return nil, nil, retirementproof.ErrUnsupported
	}
	for _, o := range all {
		if len(o.SourceBytes) == 0 || retirementproof.Hash("voice.legacy.obligation-value.v1", o.SourceBytes) != o.ValueHash {
			return nil, nil, retirementproof.ErrConflict
		}
	}
	sort.Slice(all, func(i, k int) bool { return all[i].ID < all[k].ID })
	sort.Slice(plan.Assignments, func(i, k int) bool { return plan.Assignments[i].ObligationID < plan.Assignments[k].ObligationID })
	return all, plan.Assignments, nil
}

func (j *ProjectionJournal) verifyWork(ctx context.Context, w retirementproof.OwnerWork, proof retirementproof.ProofRef) error {
	if j.Owner.Authority == nil || j.Owner.Verifier == nil {
		return retirementproof.ErrUnavailable
	}
	if err := j.Owner.Authority.CheckMaintenance(ctx, w.Cut); err != nil {
		return err
	}
	v, err := j.Owner.Verifier.VerifyForWork(ctx, proof, w.Ref)
	if err != nil {
		return err
	}
	return v.ValidateFor(w)
}
func workHash(w retirementproof.OwnerWork) retirementproof.Digest {
	raw, _ := json.Marshal(w)
	return retirementproof.Hash(retirementproof.WorkDomain, raw)
}
func (j *ProjectionJournal) ownerWork(ctx context.Context, ref retirementproof.OwnerWorkRef, legacy retirementproof.OwnerWork) (retirementproof.OwnerWork, error) {
	if ref == legacy.Ref {
		return legacy, nil
	}
	return j.Authority.ReadAuthoritativeOwnerWork(ctx, ref)
}

func (j *ProjectionJournal) verifyOwners(ctx context.Context, p Partition, legacy retirementproof.OwnerWork, requireAck bool) ([]OwnerPartitionAcceptance, error) {
	hash, _ := PartitionHash(p)
	pref := PartitionRef{p.ID, hash}
	owners := map[retirementproof.OwnerWorkRef]retirementproof.Digest{legacy.Ref: workHash(legacy)}
	for _, a := range p.Assignments {
		owners[a.Owner] = a.OwnerWorkHash
	}
	refs := make([]retirementproof.OwnerWorkRef, 0, len(owners))
	for ref := range owners {
		refs = append(refs, ref)
	}
	sort.Slice(refs, func(i, k int) bool { return refs[i].WorkID.String() < refs[k].WorkID.String() })
	result := []OwnerPartitionAcceptance{}
	for _, ref := range refs {
		w, err := j.ownerWork(ctx, ref, legacy)
		if err != nil {
			return nil, err
		}
		if w.Ref != ref || w.Cut != legacy.Cut || workHash(w) != owners[ref] {
			return nil, retirementproof.ErrConflict
		}
		if err = j.verifyWork(ctx, w, p.Proof); err != nil {
			return nil, err
		}
		if !requireAck {
			continue
		}
		var ack OwnerPartitionAcceptance
		if ref == legacy.Ref {
			var raw, digest []byte
			err = j.Owner.Pool.QueryRow(ctx, `SELECT acceptance_bytes,acceptance_sha256 FROM voice_legacy_owner_acceptances WHERE work_id=$1`, ref.WorkID).Scan(&raw, &digest)
			if err == nil {
				if json.Unmarshal(raw, &ack) != nil {
					return nil, retirementproof.ErrConflict
				}
				h := retirementproof.Hash("voice.legacy.partition-acceptance.v1", raw)
				if !bytes.Equal(h[:], digest) {
					return nil, retirementproof.ErrConflict
				}
				ack.CanonicalBytes = raw
			}
		}
		if ref != legacy.Ref {
			ack, err = j.Authority.ReadOwnerPartitionAcceptance(ctx, ref, pref)
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, retirementproof.ErrPending
		}
		if err != nil {
			return nil, err
		}
		if ack.OwnerWork != ref || ack.OwnerWorkHash != owners[ref] || ack.LegacyWork != legacy.Ref || ack.CaptureHash != p.CaptureHash || ack.Partition != pref || ack.Proof != p.Proof || len(ack.CanonicalBytes) == 0 {
			return nil, retirementproof.ErrConflict
		}
		body := ack
		body.CanonicalBytes = nil
		canonical, _ := json.Marshal(body)
		if !bytes.Equal(canonical, ack.CanonicalBytes) {
			return nil, retirementproof.ErrConflict
		}
		result = append(result, ack)
	}
	return result, nil
}

func (j *ProjectionJournal) readStep(ctx context.Context, ref store.LegacyProjectionStepRef) (store.LegacyProjectionStep, ProjectionLinkage, string, error) {
	var step store.LegacyProjectionStep
	var link ProjectionLinkage
	var resource string
	if j == nil || j.Owner == nil {
		return step, link, resource, retirementproof.ErrUnavailable
	}
	if err := j.Owner.checkSession(ctx); err != nil {
		return step, link, resource, err
	}
	var raw, digest, linkRaw []byte
	err := j.Owner.Pool.QueryRow(ctx, `SELECT step_bytes,step_sha256,linkage_bytes,resource_id FROM voice_legacy_projection_steps WHERE step_id=$1`, ref.ID).Scan(&raw, &digest, &linkRaw, &resource)
	if err != nil {
		return step, link, resource, err
	}
	if json.Unmarshal(raw, &step) != nil || json.Unmarshal(linkRaw, &link) != nil {
		return step, link, resource, retirementproof.ErrConflict
	}
	hash, err := store.LegacyProjectionStepHash(step)
	canonical, _ := json.Marshal(step)
	if err != nil || step.ID != ref.ID || hash != ref.Hash || !bytes.Equal(hash[:], digest) || !bytes.Equal(canonical, raw) {
		return step, link, resource, retirementproof.ErrConflict
	}
	return step, link, resource, nil
}

func findEndpoint(w retirementproof.OwnerWork, id string) (Endpoint, error) {
	targets, err := decodeTargets(w)
	if err != nil {
		return Endpoint{}, err
	}
	for _, e := range targets[0].Inventory.Endpoints {
		if e.ID == id {
			if e.StorageLineageHash == (retirementproof.Digest{}) || e.EvidencePrefix == "" || strings.HasPrefix(e.EvidencePrefix, e.Prefix) || strings.HasPrefix(e.Prefix, e.EvidencePrefix) {
				return Endpoint{}, retirementproof.ErrUnavailable
			}
			return e, nil
		}
	}
	return Endpoint{}, retirementproof.ErrConflict
}
func digestContains(values []retirementproof.Digest, target retirementproof.Digest) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}

// State is rederived from immutable C and PG-archived T. Prepared steps without
// a receipt block later ordinals; no latest Redis value becomes a new baseline.
func (j *ProjectionJournal) currentState(ctx context.Context, p Partition, w retirementproof.OwnerWork, resource string, beforeOrdinal *uint64) (ProjectionState, []byte, uint64, error) {
	targets, err := decodeTargets(w)
	if err != nil {
		return nil, nil, 0, err
	}
	state := ProjectionState{}
	for _, t := range targets {
		if t.Kind != "REDIS" || resource != "" && t.Endpoint.ID != resource {
			continue
		}
		for _, e := range t.Snapshot.Entries {
			state[t.Endpoint.ID+"\x00"+e.Key] = store.LegacyKeyImage{Kind: e.Kind, Value: e.Value, Members: e.Members, ExpiresAtUnixMilli: e.ExpiresAtUnixMilli}
		}
	}
	rows, err := j.Owner.Pool.Query(ctx, `SELECT resource_id,ordinal,step_bytes,step_sha256,receipt_bytes,receipt_sha256 FROM voice_legacy_projection_steps WHERE partition_id=$1 ORDER BY resource_id,ordinal`, p.ID)
	if err != nil {
		return nil, nil, 0, err
	}
	defer rows.Close()
	previous := []byte{}
	ordinal := uint64(1)
	for rows.Next() {
		var rid string
		var n int64
		var raw, digest, receipt, receiptHash []byte
		if err = rows.Scan(&rid, &n, &raw, &digest, &receipt, &receiptHash); err != nil {
			return nil, nil, 0, err
		}
		if resource != "" && rid != resource {
			continue
		}
		if beforeOrdinal != nil && uint64(n) >= *beforeOrdinal {
			continue
		}
		if receipt == nil {
			if resource == "" {
				continue // Routing sees only archived progress, independently per resource.
			}
			return nil, nil, 0, retirementproof.ErrPending
		}
		var step store.LegacyProjectionStep
		if json.Unmarshal(raw, &step) != nil {
			return nil, nil, 0, retirementproof.ErrConflict
		}
		hash, e := store.LegacyProjectionStepHash(step)
		rh := retirementproof.Hash("voice.legacy.projection-receipt.v1", receipt)
		if e != nil || !bytes.Equal(hash[:], digest) || !bytes.Equal(rh[:], receiptHash) {
			return nil, nil, 0, retirementproof.ErrConflict
		}
		if resource != "" && uint64(n) != ordinal {
			return nil, nil, 0, retirementproof.ErrConflict
		}
		for _, t := range step.Transitions {
			state[rid+"\x00"+t.Key] = t.After
		}
		if resource != "" {
			previous = receipt
			ordinal++
		}
	}
	return state, previous, ordinal, rows.Err()
}

func validateFootprint(p Partition, link ProjectionLinkage, c NativeProjectionCommand, state ProjectionState) error {
	assignments := map[string]OwnershipAssignment{}
	obligations := map[string]Obligation{}
	for _, a := range p.Assignments {
		assignments[a.ObligationID] = a
	}
	for _, o := range p.Obligations {
		obligations[o.ID] = o
	}
	for _, t := range c.Step.Transitions {
		current, exists := state[c.ResourceID+"\x00"+t.Key]
		if !exists {
			current = store.LegacyKeyImage{Kind: "none"}
		}
		if !sameCanonical(current, t.Before) {
			return retirementproof.ErrConflict
		}
		before, err := imageAtoms(strings.TrimPrefix(t.Key, c.Step.ScopePrefix), t.Before)
		if err != nil {
			return err
		}
		after, err := imageAtoms(strings.TrimPrefix(t.Key, c.Step.ScopePrefix), t.After)
		if err != nil {
			return err
		}
		paths := map[string]bool{}
		for path := range before {
			paths[path] = true
		}
		for path := range after {
			paths[path] = true
		}
		for path := range paths {
			if before[path] == after[path] {
				continue
			}
			id := c.ResourceID + "/" + hex.EncodeToString([]byte(t.Key)) + "/" + path
			a, ok := assignments[id]
			if !ok {
				return retirementproof.ErrConflict
			}
			if obligations[id].Retain || a.Owner != link.NativeOwnerWorkRef && a.Owner != link.LegacyWorkRef || !digestContains(c.TargetHashes, a.TargetHash) {
				return retirementproof.ErrConflict
			}
		}
	}
	return nil
}

func imageAtoms(key string, image store.LegacyKeyImage) (map[string]retirementproof.Digest, error) {
	result := map[string]retirementproof.Digest{}
	emit := func(path string, v any) {
		raw, _ := json.Marshal(v)
		result[path] = retirementproof.Hash("voice.legacy.obligation-value.v1", raw)
	}
	if image.Kind == "none" {
		return result, nil
	}
	expiry := "PERSISTENT"
	if image.ExpiresAtUnixMilli > 0 {
		expiry = "DEADLINE"
	}
	emit("@physical", struct {
		Kind, Expiry string
		Deadline     int64
	}{image.Kind, expiry, image.ExpiresAtUnixMilli})
	if image.Kind == "set" {
		for _, m := range image.Members {
			emit("member/"+hex.EncodeToString([]byte(m)), m)
		}
		return result, nil
	}
	if strings.HasPrefix(key, "call:") {
		var value any
		d := json.NewDecoder(strings.NewReader(image.Value))
		d.UseNumber()
		if d.Decode(&value) != nil {
			return nil, retirementproof.ErrConflict
		}
		if err := walkObligations(value, "", func(path string, v any) { emit("json"+path, v) }); err != nil {
			return nil, err
		}
	} else {
		emit("value", []byte(image.Value))
	}
	return result, nil
}

var _ GuardedProjection = (*ProjectionJournal)(nil)
var _ store.LegacyProjectionStepResolver = (*ProjectionJournal)(nil)
