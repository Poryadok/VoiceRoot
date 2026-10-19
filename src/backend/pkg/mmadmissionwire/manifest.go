package mmadmissionwire

// ResourceV1 contains identifiers/contracts only. Secret material is never a wire input.
type ResourceV1 struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	Target       string `json:"target"`
	ContractHash Hash   `json:"contract_sha256"`
}
type MountV1 struct {
	ID          string `json:"id"`
	Destination string `json:"destination"`
	ReadOnly    bool   `json:"read_only"`
	ContentHash Hash   `json:"content_sha256"`
}
type ReplicaV1 struct {
	ReplicaID      ID        `json:"replica_id"`
	LaunchID       ID        `json:"launch_id"`
	RuntimeUID     uint32    `json:"runtime_uid"`
	ExecutableHash Hash      `json:"executable_sha256"`
	Argv           []string  `json:"argv"`
	Mounts         []MountV1 `json:"mounts"`
}
type LaunchSpecV1 struct {
	Version                      uint32       `json:"version"`
	AuthorityScope               Scope        `json:"authority_scope"`
	ScopeID                      ID           `json:"scope_id"`
	ImageDigest                  string       `json:"image_digest"`
	ConfigContractHash           Hash         `json:"config_contract_sha256"`
	CredentialRefsManifestHash   Hash         `json:"credential_refs_manifest_sha256"`
	UpstreamContractManifestHash Hash         `json:"upstream_contract_manifest_sha256"`
	BrokerContractManifestHash   Hash         `json:"broker_contract_manifest_sha256"`
	FreshRoutes                  []string     `json:"fresh_routes"`
	Factories                    []string     `json:"factories"`
	Resources                    []ResourceV1 `json:"resources"`
	Replicas                     []ReplicaV1  `json:"replicas"`
}

func (s LaunchSpecV1) Validate() error {
	if s.Version != 1 || !scope(s.AuthorityScope) || s.ScopeID == (ID{}) || len(s.ImageDigest) != 71 || s.ImageDigest[:7] != "sha256:" || !registry(s.FreshRoutes) || !registry(s.Factories) || len(s.Resources) < 3 || len(s.Resources) > 128 || len(s.Replicas) == 0 || len(s.Replicas) > 64 {
		return ErrInvalid
	}
	if _, e := ParseHash(s.ImageDigest[7:]); e != nil {
		return e
	}
	for _, h := range []Hash{s.ConfigContractHash, s.CredentialRefsManifestHash, s.UpstreamContractManifestHash, s.BrokerContractManifestHash} {
		if h == (Hash{}) {
			return ErrInvalid
		}
	}
	kinds := map[string]bool{}
	for i, r := range s.Resources {
		if !text(r.ID, 128) || !text(r.Target, 4096) || !text(r.Kind, 32) || r.ContractHash == (Hash{}) || (i > 0 && s.Resources[i-1].ID >= r.ID) {
			return ErrInvalid
		}
		kinds[r.Kind] = true
	}
	for _, k := range []string{"POSTGRES", "REDIS", "NATS"} {
		if !kinds[k] {
			return ErrInvalid
		}
	}
	for i, r := range s.Replicas {
		if r.ReplicaID == (ID{}) || r.LaunchID == (ID{}) || r.ExecutableHash == (Hash{}) || r.RuntimeUID == 0 || len(r.Argv) == 0 || len(r.Argv) > 64 || len(r.Mounts) < 2 || len(r.Mounts) > 32 || (i > 0 && string(s.Replicas[i-1].ReplicaID[:]) >= string(r.ReplicaID[:])) {
			return ErrInvalid
		}
		for _, a := range r.Argv {
			if !text(a, 4096) {
				return ErrInvalid
			}
		}
		paths := map[string]bool{}
		for j, m := range r.Mounts {
			if !text(m.ID, 128) || !text(m.Destination, 4096) || !m.ReadOnly || m.ContentHash == (Hash{}) || (j > 0 && r.Mounts[j-1].ID >= m.ID) || paths[m.Destination] {
				return ErrInvalid
			}
			paths[m.Destination] = true
		}
		if !paths[DescriptorPath] || !paths[ReadbackSocket] {
			return ErrInvalid
		}
	}
	return nil
}
func LaunchHash(s LaunchSpecV1) (Hash, error) {
	if s.Validate() != nil {
		return Hash{}, ErrInvalid
	}
	e := newEncoder("voice.mm.admission.launch.v1")
	e.u32(s.Version)
	e.str(string(s.AuthorityScope))
	e.id(s.ScopeID)
	e.str(s.ImageDigest)
	for _, h := range []Hash{s.ConfigContractHash, s.CredentialRefsManifestHash, s.UpstreamContractManifestHash, s.BrokerContractManifestHash} {
		e.hash(h)
	}
	e.strings(s.FreshRoutes)
	e.strings(s.Factories)
	e.u32(uint32(len(s.Resources)))
	for _, r := range s.Resources {
		e.str(r.ID)
		e.str(r.Kind)
		e.str(r.Target)
		e.hash(r.ContractHash)
	}
	e.u32(uint32(len(s.Replicas)))
	for _, r := range s.Replicas {
		e.id(r.ReplicaID)
		e.id(r.LaunchID)
		e.u32(r.RuntimeUID)
		e.hash(r.ExecutableHash)
		e.strings(r.Argv)
		e.u32(uint32(len(r.Mounts)))
		for _, m := range r.Mounts {
			e.str(m.ID)
			e.str(m.Destination)
			if m.ReadOnly {
				e.WriteByte(1)
			} else {
				e.WriteByte(0)
			}
			e.hash(m.ContentHash)
		}
	}
	return e.sum()
}

type ArmedReceiptV1 struct {
	Version                 uint32  `json:"version"`
	AuthorityScope          Scope   `json:"authority_scope"`
	ScopeID                 ID      `json:"scope_id"`
	ControllerID            ID      `json:"controller_id"`
	TransitionID            ID      `json:"transition_id"`
	PredecessorReceiptHash  *Hash   `json:"predecessor_receipt_sha256"`
	DatabaseLineageID       ID      `json:"database_lineage_id"`
	Binding                 Binding `json:"binding"`
	LaunchSpecHash          Hash    `json:"launch_spec_sha256"`
	ClosureEvidenceHash     Hash    `json:"closure_evidence_sha256"`
	PreparationEvidenceHash Hash    `json:"preparation_evidence_sha256"`
	PreviousReceiptHash     *Hash   `json:"previous_receipt_sha256"`
}

func ArmedHash(r ArmedReceiptV1) (Hash, error) {
	if r.Binding.AdmissionReceiptHash != (Hash{}) {
		return Hash{}, ErrInvalid
	}
	b := r.Binding
	b.AdmissionReceiptHash = Hash{1}
	if r.Version != 1 || !scope(r.AuthorityScope) || r.ScopeID == (ID{}) || r.ControllerID == (ID{}) || r.TransitionID != b.TransitionID || r.DatabaseLineageID == (ID{}) || b.Validate() != nil || r.LaunchSpecHash != b.LaunchManifestHash || r.ClosureEvidenceHash == (Hash{}) || r.PreparationEvidenceHash == (Hash{}) {
		return Hash{}, ErrInvalid
	}
	e := newEncoder("voice.mm.admission.armed.v1")
	e.u32(r.Version)
	e.str(string(r.AuthorityScope))
	e.id(r.ScopeID)
	e.id(r.ControllerID)
	e.id(r.TransitionID)
	optionalHash(e, r.PredecessorReceiptHash)
	e.id(r.DatabaseLineageID)
	bindingBytes(e, b, false)
	e.hash(r.LaunchSpecHash)
	e.hash(r.ClosureEvidenceHash)
	e.hash(r.PreparationEvidenceHash)
	optionalHash(e, r.PreviousReceiptHash)
	return e.sum()
}
func (r ArmedReceiptV1) Validate() error { _, e := ArmedHash(r); return e }
func (r OpenReceiptV1) Validate() error  { _, e := OpenHash(r); return e }

// ReceiptBinding constructs the final binding; receipts themselves always carry
// an explicit zero placeholder, so no unbound JSON field can vary its meaning.
func ReceiptBinding(r ArmedReceiptV1) (Binding, error) {
	h, e := ArmedHash(r)
	b := r.Binding
	b.AdmissionReceiptHash = h
	return b, e
}

type OpenReceiptV1 struct {
	Version                      uint32 `json:"version"`
	ArmedReceiptHash             Hash   `json:"armed_receipt_sha256"`
	ObservedLaunchesHash         Hash   `json:"observed_launches_sha256"`
	PreparedAcknowledgementsHash Hash   `json:"prepared_acknowledgements_sha256"`
	StateRevision                uint64 `json:"state_revision,string"`
}

func OpenHash(r OpenReceiptV1) (Hash, error) {
	if r.Version != 1 || r.ArmedReceiptHash == (Hash{}) || r.ObservedLaunchesHash == (Hash{}) || r.PreparedAcknowledgementsHash == (Hash{}) || r.StateRevision == 0 {
		return Hash{}, ErrInvalid
	}
	e := newEncoder("voice.mm.admission.open.v1")
	e.u32(r.Version)
	e.hash(r.ArmedReceiptHash)
	e.hash(r.ObservedLaunchesHash)
	e.hash(r.PreparedAcknowledgementsHash)
	e.u64(r.StateRevision)
	return e.sum()
}
func optionalHash(e *encoder, h *Hash) {
	if h == nil {
		e.WriteByte(0)
		return
	}
	e.WriteByte(1)
	e.hash(*h)
}
