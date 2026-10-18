package retirementproof

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"github.com/google/uuid"
)

const P3FenceRequestDomain = "voice.calls.v1.ApplySpaceLifecycleFenceRequest"

var sourceKinds = []string{"GAME_PROVISION_AND_CONVERSION", "LEGACY_COMPATIBILITY", "MEDIA_AND_GRANT_PRODUCERS", "NATIVE_ORDINARY", "NATIVE_SPACE", "OWNED_ROOM", "SHARED_IDENTITY_FENCES"}
var componentKinds = []string{"ADMISSION_BARRIER", "CREDENTIAL_MANIFEST", "DOCKER_FENCE", "ISSUER_MANIFEST", "POSTGRES_FENCE", "PROVIDER_MANIFEST", "REDIS_FENCE", "SOURCE_CATALOG"}

type TrustedVerifier struct {
	Journal      ProtectedJournal
	Owner        OwnerWorkReader
	Installation InstallationReader
	Resources    ResourceObserver
}

// ValidateResourceRecord checks canonical shape/bindings only. It authenticates
// nothing and returns no VerifiedRetirement; protected journal provenance,
// authoritative owner extraction and current actual observations are mandatory.
func ValidateResourceRecord(r ResourceRecord) error { return validateResource(r, r.Ref) }

func (v TrustedVerifier) VerifyForWork(ctx context.Context, proof ProofRef, work OwnerWorkRef) (VerifiedRetirement, error) {
	if v.Journal == nil || v.Owner == nil || v.Installation == nil || v.Resources == nil {
		return VerifiedRetirement{}, ErrUnavailable
	}
	r, err := v.Journal.ReadResourceProof(ctx, proof)
	if err != nil {
		return VerifiedRetirement{}, err
	}
	if err = validateResource(r, proof); err != nil {
		return VerifiedRetirement{}, err
	}
	w, err := v.Owner.ReadOwnerWork(ctx, work)
	if err != nil {
		return VerifiedRetirement{}, err
	}
	if w.Ref != work {
		return VerifiedRetirement{}, conflict("owner work reference")
	}
	if err = validateWork(w); err != nil {
		return VerifiedRetirement{}, err
	}
	if w.Cut != r.Document.Cut {
		return VerifiedRetirement{}, conflict("cut binding")
	}
	found := false
	for _, source := range r.Document.SourceBindings {
		if source.SourceKind == w.SourceKind && source.OwnerProtocol == w.Ref.OwnerProtocol && source.ContractHash == w.SourceManifestHash {
			found = true
		}
	}
	if !found {
		return VerifiedRetirement{}, conflict("source contract")
	}
	requiredDomains, resolutionEvidence, err := resolveIssuance(r, w)
	if err != nil {
		return VerifiedRetirement{}, err
	}
	for _, required := range requiredDomains {
		found = false
		for _, retired := range r.Document.RetiredDomains {
			if required == retired {
				found = true
			}
		}
		if !found {
			return VerifiedRetirement{}, conflict("unretired issuance domain")
		}
	}
	installation, err := v.Installation.CurrentInstallation(ctx)
	if err != nil {
		return VerifiedRetirement{}, err
	}
	cut := r.Document.Cut
	if installation.EnvironmentID != cut.EnvironmentID || installation.AuthorityIncarnation != cut.AuthorityIncarnation || installation.IssuerImplementationManifestHash != cut.IssuerImplementationManifestHash {
		return VerifiedRetirement{}, conflict("external installation")
	}
	// The observer rechecks actual closed admission/resource custody on every
	// acceptance and recovery. A matching stored hash is not a resource fence.
	if err = v.Resources.CheckResourceRetirement(ctx, r); err != nil {
		return VerifiedRetirement{}, err
	}
	b, err := json.Marshal(w)
	if err != nil {
		return VerifiedRetirement{}, ErrUnavailable
	}
	workHash := Hash(WorkDomain, b)
	resolutionBytes := mustJSON(DomainResolution{ProtocolV1, proof, work, workHash, w.SealedTargetManifestHash, w.Cut, requiredDomains, resolutionEvidence})
	return VerifiedRetirement{&verifiedBinding{Proof: proof, Work: work, Cut: w.Cut, WorkHash: workHash, TargetHash: w.SealedTargetManifestHash, ResolutionBytes: resolutionBytes, ResolutionHash: Hash(ResolutionDomain, resolutionBytes)}}, nil
}

// ValidateFor is the owner's final same-transaction binding check. It does not
// replace the owner's restrictive-head/target/progress revalidation or journal
// provenance. A zero/caller-constructed value cannot authorize completion.
func (v VerifiedRetirement) ValidateFor(work OwnerWork) error {
	if v.binding == nil {
		return ErrUnavailable
	}
	if err := validateWork(work); err != nil {
		return err
	}
	b, err := json.Marshal(work)
	if err != nil {
		return ErrUnavailable
	}
	if v.binding.Work != work.Ref || v.binding.Cut != work.Cut || v.binding.WorkHash != Hash(WorkDomain, b) || v.binding.TargetHash != work.SealedTargetManifestHash {
		return conflict("verified owner work changed")
	}
	return nil
}

func conflict(what string) error { return fmt.Errorf("%w: %s", ErrConflict, what) }
func validCut(c CutBinding) bool {
	return c.EnvironmentID != uuid.Nil && c.AuthorityIncarnation != uuid.Nil && c.CutoverOperationID != uuid.Nil && c.CoverageEpoch > 0 && c.CoverageEpoch <= math.MaxInt64 && c.PlanHash != (Digest{}) && c.DatabaseIdentity != (Digest{}) && c.SourceCatalogHash != (Digest{}) && c.SchemaContractHash != (Digest{}) && c.WriterManifestHash != (Digest{}) && c.ProviderManifestHash != (Digest{}) && c.CredentialManifestHash != (Digest{}) && c.ClosedAdmissionBarrierID != uuid.Nil && c.ClosedAdmissionBarrierHash != (Digest{}) && c.IssuerImplementationManifestHash != (Digest{})
}
func validDomain(d DomainBinding) bool {
	return d.RealmID != "" && d.CredentialCohortID != uuid.Nil && d.RouteIdentity != "" && d.ImplementationHash != (Digest{})
}

func validateResource(r ResourceRecord, ref ProofRef) error {
	d := r.Document
	if ref.Protocol != ProtocolV1 || d.Protocol != ProtocolV1 || d.Kind != WholeRealmRetired {
		return ErrUnsupported
	}
	if ref != r.Ref || ref.ProofID == uuid.Nil || d.ProofID != ref.ProofID || !validCut(d.Cut) || d.PredecessorExecutionCohortID == uuid.Nil {
		return conflict("resource identity")
	}
	b, err := json.Marshal(d)
	if err != nil || !bytes.Equal(b, r.CanonicalBytes) || Hash(ResourceDomain, b) != ref.ReceiptHash {
		return conflict("resource canonical bytes")
	}
	if len(d.SourceBindings) != len(sourceKinds) {
		return conflict("closed source catalogue")
	}
	for i, s := range d.SourceBindings {
		if s.SourceKind != sourceKinds[i] || s.OwnerProtocol == "" || s.ContractHash == (Digest{}) {
			return conflict("closed source catalogue")
		}
	}
	if !sortedStrings(d.PredecessorRealmIDs) || !sortedStrings(d.ReplacementRealmIDs) || len(d.ReplacementCredentialCohortIDs) == 0 || len(d.RetiredDomains) == 0 || len(d.ReplacementDomains) == 0 {
		return conflict("realm inventory")
	}
	oldRealms, newRealms := map[string]bool{}, map[string]bool{}
	oldCredentials, newCredentials := map[uuid.UUID]bool{}, map[uuid.UUID]bool{}
	for _, s := range d.PredecessorRealmIDs {
		oldRealms[s] = true
	}
	for _, s := range d.ReplacementRealmIDs {
		if oldRealms[s] {
			return conflict("retired realm reused")
		}
		newRealms[s] = true
	}
	for i, id := range d.ReplacementCredentialCohortIDs {
		if id == uuid.Nil || newCredentials[id] || (i > 0 && bytes.Compare(d.ReplacementCredentialCohortIDs[i-1][:], id[:]) >= 0) {
			return conflict("replacement credential inventory")
		}
		newCredentials[id] = true
	}
	seenOld, seenNew := map[string]bool{}, map[string]bool{}
	for i, x := range d.RetiredDomains {
		if !validDomain(x) || !oldRealms[x.RealmID] || (i > 0 && !domainLess(d.RetiredDomains[i-1], x)) {
			return conflict("retired domain inventory")
		}
		oldCredentials[x.CredentialCohortID] = true
		seenOld[x.RealmID] = true
	}
	for i, x := range d.ReplacementDomains {
		if !validDomain(x) || !newRealms[x.RealmID] || !newCredentials[x.CredentialCohortID] || oldCredentials[x.CredentialCohortID] || (i > 0 && !domainLess(d.ReplacementDomains[i-1], x)) {
			return conflict("replacement domain inventory")
		}
		seenNew[x.RealmID] = true
	}
	if len(seenOld) != len(oldRealms) || len(seenNew) != len(newRealms) {
		return conflict("unmapped realm")
	}
	if len(d.Components) != len(componentKinds) || len(r.Components) != len(componentKinds) {
		return conflict("resource components")
	}
	for i, kind := range componentKinds {
		c, record := d.Components[i], r.Components[i]
		if c.Kind != kind || c.Protocol != ProtocolV1 || c.JournalRef == uuid.Nil || record.Ref != c || len(record.CanonicalBytes) == 0 || sha256.Sum256(record.CanonicalBytes) != c.CanonicalBytesHash {
			return conflict("component provenance/bytes")
		}
		var expected Digest
		var domain string
		switch kind {
		case "SOURCE_CATALOG":
			expected, domain = d.Cut.SourceCatalogHash, CatalogDomain
			if !bytes.Equal(record.CanonicalBytes, mustJSON(d.SourceBindings)) {
				return conflict("source manifest bytes")
			}
		case "PROVIDER_MANIFEST":
			expected, domain = d.Cut.ProviderManifestHash, ProviderDomain
			if !bytes.Equal(record.CanonicalBytes, mustJSON(struct{ Retired, Replacement []DomainBinding }{d.RetiredDomains, d.ReplacementDomains})) {
				return conflict("provider manifest bytes")
			}
		case "CREDENTIAL_MANIFEST":
			expected, domain = d.Cut.CredentialManifestHash, CredentialDomain
		case "ADMISSION_BARRIER":
			expected, domain = d.Cut.ClosedAdmissionBarrierHash, BarrierDomain
		case "ISSUER_MANIFEST":
			expected, domain = d.Cut.IssuerImplementationManifestHash, IssuerDomain
		}
		if domain != "" && Hash(domain, record.CanonicalBytes) != expected {
			return conflict("cut manifest digest")
		}
	}
	return nil
}
func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }
func sortedStrings(ss []string) bool {
	if len(ss) == 0 || !sort.StringsAreSorted(ss) {
		return false
	}
	for i, s := range ss {
		if s == "" || (i > 0 && ss[i-1] == s) {
			return false
		}
	}
	return true
}
func domainLess(a, b DomainBinding) bool {
	if a.RealmID != b.RealmID {
		return a.RealmID < b.RealmID
	}
	if a.CredentialCohortID != b.CredentialCohortID {
		return bytes.Compare(a.CredentialCohortID[:], b.CredentialCohortID[:]) < 0
	}
	return a.RouteIdentity < b.RouteIdentity
}
func validateWork(w OwnerWork) error {
	if w.Ref.Protocol != ProtocolV1 {
		return ErrUnsupported
	}
	if w.Ref.WorkID == uuid.Nil || w.Ref.OwnerProtocol == "" || !validCut(w.Cut) || w.SourceKind == "" || w.SourceManifestHash == (Digest{}) || len(w.OwnerRequestBytes) == 0 || len(w.SealedTargetManifestBytes) == 0 || w.SealedTargetCount > math.MaxInt64 {
		return conflict("owner work identity")
	}
	if Hash(w.OwnerRequestDomain, w.OwnerRequestBytes) != w.Ref.RequestHash || Hash(TargetDomain, w.SealedTargetManifestBytes) != w.SealedTargetManifestHash {
		return conflict("owner request/targets")
	}
	var targets []json.RawMessage
	if json.Unmarshal(w.SealedTargetManifestBytes, &targets) != nil || uint64(len(targets)) != w.SealedTargetCount {
		return conflict("sealed target manifest")
	}
	if w.SourceKind == "NATIVE_SPACE" && w.SpaceID == uuid.Nil {
		return conflict("Space identity")
	}
	switch w.CapturedHead.State {
	case HeadAbsent:
		if len(w.CapturedHead.CanonicalBytes) != 0 || w.CapturedHead.Hash != Hash(HeadDomain, []byte(HeadAbsent)) {
			return conflict("absent lifecycle head")
		}
	case HeadPresent:
		if len(w.CapturedHead.CanonicalBytes) == 0 || w.CapturedHead.Hash != Hash(HeadDomain, w.CapturedHead.CanonicalBytes) {
			return conflict("captured lifecycle head")
		}
	default:
		return conflict("lifecycle head state")
	}
	for i, d := range w.CapturedProviderBindings {
		if d == (Digest{}) || (i > 0 && bytes.Compare(w.CapturedProviderBindings[i-1][:], d[:]) >= 0) {
			return conflict("captured provider bindings")
		}
	}
	for i, d := range w.UnknownIssuanceMarkers {
		if d == (Digest{}) || (i > 0 && bytes.Compare(w.UnknownIssuanceMarkers[i-1][:], d[:]) >= 0) {
			return conflict("unknown issuance markers")
		}
	}
	// Until an owner proves a local-only empty issuance domain, nonempty work
	// cannot silently claim that its unknown credential history is covered.
	if w.SealedTargetCount > 0 && len(w.CapturedProviderBindings) == 0 && len(w.UnknownIssuanceMarkers) == 0 {
		return conflict("unknown issuance domain")
	}
	switch w.Purpose {
	case ColdCutDrain:
		if w.ExistingP3 != nil || w.OwnerRequestDomain != OwnerRequestDomain {
			return conflict("cold cut request")
		}
	case P3FrozenCompletion:
		p := w.ExistingP3
		if w.SourceKind != "NATIVE_SPACE" || p == nil || p.DeletionOperationID == uuid.Nil || p.Generation == 0 || p.Generation > math.MaxInt64 || p.ManifestID == uuid.Nil || p.ManifestHash == (Digest{}) || p.ItemCount > math.MaxInt64 || p.RequestDomain != P3FenceRequestDomain || w.OwnerRequestDomain != p.RequestDomain || !bytes.Equal(w.OwnerRequestBytes, p.CanonicalRequestBytes) || p.RequestHash != w.Ref.RequestHash {
			return conflict("original P3 binding")
		}
	default:
		return ErrUnsupported
	}
	return nil
}

func resolveIssuance(r ResourceRecord, w OwnerWork) ([]DomainBinding, []DomainResolutionEvidence, error) {
	var manifest CredentialManifest
	var manifestRef ComponentRef
	for _, c := range r.Components {
		if c.Ref.Kind == "CREDENTIAL_MANIFEST" {
			manifestRef = c.Ref
			if json.Unmarshal(c.CanonicalBytes, &manifest) != nil || !bytes.Equal(mustJSON(manifest), c.CanonicalBytes) {
				return nil, nil, conflict("credential manifest bytes")
			}
		}
	}
	if manifest.Protocol != ProtocolV1 {
		return nil, nil, ErrUnsupported
	}
	validateDomains := func(ds []DomainBinding) bool {
		if len(ds) == 0 {
			return false
		}
		for i, d := range ds {
			if !validDomain(d) || (i > 0 && !domainLess(ds[i-1], d)) {
				return false
			}
			found := false
			for _, retired := range r.Document.RetiredDomains {
				if d == retired {
					found = true
				}
			}
			if !found {
				return false
			}
		}
		return true
	}
	providers := map[Digest]ProviderIssuanceBinding{}
	for i, p := range manifest.ProviderBindings {
		if p.ProviderBinding == (Digest{}) || (i > 0 && bytes.Compare(manifest.ProviderBindings[i-1].ProviderBinding[:], p.ProviderBinding[:]) >= 0) || !validateDomains(p.Domains) || !json.Valid(p.EvidenceBytes) {
			return nil, nil, conflict("provider issuance manifest")
		}
		providers[p.ProviderBinding] = p
	}
	history := map[string]HistoricalIssuanceBinding{}
	for i, h := range manifest.HistoricalSources {
		key := h.SourceKind + "\x00" + h.OwnerProtocol
		if h.SourceKind == "" || h.OwnerProtocol == "" || !validateDomains(h.Domains) || !json.Valid(h.EvidenceBytes) || (i > 0 && manifest.HistoricalSources[i-1].SourceKind+"\x00"+manifest.HistoricalSources[i-1].OwnerProtocol >= key) {
			return nil, nil, conflict("historical issuance manifest")
		}
		known := false
		for _, s := range r.Document.SourceBindings {
			if s.SourceKind == h.SourceKind && s.OwnerProtocol == h.OwnerProtocol {
				known = true
			}
		}
		if !known {
			return nil, nil, conflict("historical source contract")
		}
		history[key] = h
	}
	required := map[DomainBinding]bool{}
	evidence := []DomainResolutionEvidence{}
	for _, binding := range w.CapturedProviderBindings {
		p, ok := providers[binding]
		if !ok {
			return nil, nil, ErrUnsupported
		}
		for _, d := range p.Domains {
			required[d] = true
		}
		evidence = append(evidence, DomainResolutionEvidence{ProviderBinding: binding, CredentialManifestComponent: manifestRef, EntryHash: Hash(IssuanceEvidenceDomain, mustJSON(p))})
	}
	for _, marker := range w.UnknownIssuanceMarkers {
		h, ok := history[w.SourceKind+"\x00"+w.Ref.OwnerProtocol]
		if !ok {
			return nil, nil, ErrUnsupported
		}
		for _, d := range h.Domains {
			required[d] = true
		}
		evidence = append(evidence, DomainResolutionEvidence{UnknownMarker: marker, CredentialManifestComponent: manifestRef, EntryHash: Hash(IssuanceEvidenceDomain, mustJSON(h))})
	}
	result := make([]DomainBinding, 0, len(required))
	for d := range required {
		result = append(result, d)
	}
	sort.Slice(result, func(i, j int) bool { return domainLess(result[i], result[j]) })
	return result, evidence, nil
}
