// Package retirementproof defines the leaf contract between the protected
// resource issuer and native owners. Hashes/Go structs are not authentication:
// production resolution must read the protected issuer/owner journals and
// independently reobserve resource fences and external installation binding.
package retirementproof

import (
	"context"
	"crypto/sha256"
	"errors"
	"github.com/google/uuid"
)

type Digest = [32]byte

var (
	ErrUnavailable = errors.New("retirement proof unavailable")
	ErrConflict    = errors.New("retirement proof conflict")
	ErrPending     = errors.New("retirement work pending")
	ErrUnsupported = errors.New("retirement proof unsupported")
)

const (
	ProtocolV1             uint32 = 1
	WholeRealmRetired             = "WHOLE_REALM_RETIRED"
	ColdCutDrain                  = "COLD_CUT_DRAIN"
	P3FrozenCompletion            = "P3_FROZEN_COMPLETION"
	HeadAbsent                    = "ABSENT"
	HeadPresent                   = "PRESENT"
	ResourceDomain                = "voice.party.retirement.resource.v1"
	WorkDomain                    = "voice.party.retirement.owner-work.v1"
	TargetDomain                  = "voice.party.retirement.targets.v1"
	HeadDomain                    = "voice.party.retirement.owner-head.v1"
	OwnerRequestDomain            = "voice.party.retirement.owner-request.v1"
	CatalogDomain                 = "voice.party.retirement.source-catalog.v1"
	ProviderDomain                = "voice.party.retirement.provider-manifest.v1"
	CredentialDomain              = "voice.party.retirement.credential-manifest.v1"
	BarrierDomain                 = "voice.party.retirement.admission-barrier.v1"
	IssuerDomain                  = "voice.party.retirement.issuer-manifest.v1"
	ResolutionDomain              = "voice.party.retirement.domain-resolution.v1"
	IssuanceEvidenceDomain        = "voice.party.retirement.issuance-evidence.v1"
)

func Hash(domain string, canonical []byte) Digest {
	return sha256.Sum256(append(append([]byte(domain), 0), canonical...))
}

type ProofRef struct {
	Protocol    uint32
	ProofID     uuid.UUID
	ReceiptHash Digest
}
type OwnerWorkRef struct {
	Protocol      uint32
	OwnerProtocol string
	WorkID        uuid.UUID
	RequestHash   Digest
}

// CutBinding is shared by the resource receipt and every authoritative native
// owner work. It contains no passwords, bearer tokens or private signing keys.
type CutBinding struct {
	EnvironmentID, AuthorityIncarnation, CutoverOperationID                               uuid.UUID
	CoverageEpoch                                                                         uint64
	PlanHash, DatabaseIdentity, SourceCatalogHash, SchemaContractHash, WriterManifestHash Digest
	ProviderManifestHash, CredentialManifestHash                                          Digest
	ClosedAdmissionBarrierID                                                              uuid.UUID
	ClosedAdmissionBarrierHash, IssuerImplementationManifestHash                          Digest
}

type DomainBinding struct {
	RealmID            string
	CredentialCohortID uuid.UUID
	RouteIdentity      string
	ImplementationHash Digest
}
type SourceBinding struct {
	SourceKind, OwnerProtocol string
	ContractHash              Digest
}
type ComponentRef struct {
	Kind               string
	Protocol           uint32
	CanonicalBytesHash Digest
	JournalRef         uuid.UUID
}
type ComponentRecord struct {
	Ref            ComponentRef
	CanonicalBytes []byte
}

// CredentialManifest is protected issuer evidence, never a caller-supplied
// owner mapping. Opaque native provider bindings are preserved exactly. Unknown
// historical issuance requires a complete closed source-domain declaration.
type CredentialManifest struct {
	Protocol          uint32
	ProviderBindings  []ProviderIssuanceBinding
	HistoricalSources []HistoricalIssuanceBinding
}
type ProviderIssuanceBinding struct {
	ProviderBinding Digest
	Domains         []DomainBinding
	EvidenceBytes   []byte
}
type HistoricalIssuanceBinding struct {
	SourceKind, OwnerProtocol string
	Domains                   []DomainBinding
	EvidenceBytes             []byte
}

type DomainResolutionEvidence struct {
	ProviderBinding, UnknownMarker Digest
	CredentialManifestComponent    ComponentRef
	EntryHash                      Digest
}
type DomainResolution struct {
	Protocol                     uint32
	Proof                        ProofRef
	Work                         OwnerWorkRef
	WorkHash, TargetManifestHash Digest
	Cut                          CutBinding
	Domains                      []DomainBinding
	Evidence                     []DomainResolutionEvidence
}

type ResourceDocument struct {
	Protocol                                 uint32
	ProofID                                  uuid.UUID
	Kind                                     string
	Cut                                      CutBinding
	PredecessorExecutionCohortID             uuid.UUID
	PredecessorRealmIDs, ReplacementRealmIDs []string
	ReplacementCredentialCohortIDs           []uuid.UUID
	RetiredDomains, ReplacementDomains       []DomainBinding
	SourceBindings                           []SourceBinding
	Components                               []ComponentRef
}

// ResourceRecord comes only from the protected journal reader. Canonical bytes
// and complete component bytes must be recoverable, never just stored hashes.
type ResourceRecord struct {
	Ref            ProofRef
	Document       ResourceDocument
	CanonicalBytes []byte
	Components     []ComponentRecord
}

type OwnerHeadBinding struct {
	State          string
	CanonicalBytes []byte
	Hash           Digest
}
type P3Binding struct {
	DeletionOperationID   uuid.UUID
	Generation            uint64
	RequestDomain         string
	CanonicalRequestBytes []byte
	RequestHash           Digest
	ManifestID            uuid.UUID
	ManifestHash          Digest
	ItemCount             uint64
}
type OwnerWork struct {
	Ref                       OwnerWorkRef
	Purpose, SourceKind       string
	Cut                       CutBinding
	SpaceID                   uuid.UUID
	CapturedHead              OwnerHeadBinding
	OwnerRequestDomain        string
	OwnerRequestBytes         []byte
	SourceManifestHash        Digest
	SealedTargetManifestBytes []byte
	SealedTargetManifestHash  Digest
	SealedTargetCount         uint64
	CapturedProviderBindings  []Digest
	UnknownIssuanceMarkers    []Digest
	ExistingP3                *P3Binding
}

type Installation struct {
	EnvironmentID, AuthorityIncarnation uuid.UUID
	IssuerImplementationManifestHash    Digest
}
type ProtectedJournal interface {
	ReadResourceProof(context.Context, ProofRef) (ResourceRecord, error)
}
type OwnerWorkReader interface {
	ReadOwnerWork(context.Context, OwnerWorkRef) (OwnerWork, error)
}
type InstallationReader interface {
	CurrentInstallation(context.Context) (Installation, error)
}
type ResourceObserver interface {
	CheckResourceRetirement(context.Context, ResourceRecord) error
}
type Verifier interface {
	VerifyForWork(context.Context, ProofRef, OwnerWorkRef) (VerifiedRetirement, error)
}

type verifiedBinding struct {
	Proof                ProofRef
	Work                 OwnerWorkRef
	Cut                  CutBinding
	WorkHash, TargetHash Digest
	ResolutionBytes      []byte
	ResolutionHash       Digest
}
type VerifiedRetirement struct{ binding *verifiedBinding }

func (v VerifiedRetirement) ProofRef() ProofRef {
	if v.binding == nil {
		return ProofRef{}
	}
	return v.binding.Proof
}
func (v VerifiedRetirement) WorkRef() OwnerWorkRef {
	if v.binding == nil {
		return OwnerWorkRef{}
	}
	return v.binding.Work
}
func (v VerifiedRetirement) CutBinding() CutBinding {
	if v.binding == nil {
		return CutBinding{}
	}
	return v.binding.Cut
}
func (v VerifiedRetirement) WorkHash() Digest {
	if v.binding == nil {
		return Digest{}
	}
	return v.binding.WorkHash
}
func (v VerifiedRetirement) TargetManifestHash() Digest {
	if v.binding == nil {
		return Digest{}
	}
	return v.binding.TargetHash
}

func (v VerifiedRetirement) DomainResolutionBytes() []byte {
	if v.binding == nil {
		return nil
	}
	return append([]byte(nil), v.binding.ResolutionBytes...)
}
func (v VerifiedRetirement) DomainResolutionHash() Digest {
	if v.binding == nil {
		return Digest{}
	}
	return v.binding.ResolutionHash
}
