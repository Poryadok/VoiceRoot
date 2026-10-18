package retirementproof

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
)

// These fixtures model trusted readers for unit tests only. They do not issue
// resource receipts or establish actual retirement/activation capability.
type fixture struct {
	record                               ResourceRecord
	work                                 OwnerWork
	installation                         Installation
	journalErr, ownerErr, observationErr error
	observations                         int
}

func (f *fixture) ReadResourceProof(context.Context, ProofRef) (ResourceRecord, error) {
	return f.record, f.journalErr
}
func (f *fixture) ReadOwnerWork(context.Context, OwnerWorkRef) (OwnerWork, error) {
	return f.work, f.ownerErr
}
func (f *fixture) CurrentInstallation(context.Context) (Installation, error) {
	return f.installation, nil
}
func (f *fixture) CheckResourceRetirement(context.Context, ResourceRecord) error {
	f.observations++
	return f.observationErr
}
func (f *fixture) verifier() TrustedVerifier {
	return TrustedVerifier{Journal: f, Owner: f, Installation: f, Resources: f}
}
func jsonBytes(v any) []byte {
	b, e := json.Marshal(v)
	if e != nil {
		panic(e)
	}
	return b
}
func digest(s string) Digest { return sha256.Sum256([]byte(s)) }
func (f *fixture) seal() {
	f.record.CanonicalBytes = jsonBytes(f.record.Document)
	f.record.Ref = ProofRef{ProtocolV1, f.record.Document.ProofID, Hash(ResourceDomain, f.record.CanonicalBytes)}
}
func newFixture() *fixture {
	domain := DomainBinding{"retired-realm", uuid.New(), "closed-old-route", digest("provider")}
	replacement := DomainBinding{"replacement-realm", uuid.New(), "new-route", digest("provider")}
	sources := []SourceBinding{}
	for _, s := range []string{"GAME_PROVISION_AND_CONVERSION", "LEGACY_COMPATIBILITY", "MEDIA_AND_GRANT_PRODUCERS", "NATIVE_ORDINARY", "NATIVE_SPACE", "OWNED_ROOM", "SHARED_IDENTITY_FENCES"} {
		sources = append(sources, SourceBinding{s, "test-owner-v1", digest(s)})
	}
	cut := CutBinding{EnvironmentID: uuid.New(), AuthorityIncarnation: uuid.New(), CutoverOperationID: uuid.New(), CoverageEpoch: 1, PlanHash: digest("plan"), DatabaseIdentity: digest("db"), SchemaContractHash: digest("schema"), WriterManifestHash: digest("writers"), ClosedAdmissionBarrierID: uuid.New()}
	f := &fixture{record: ResourceRecord{Document: ResourceDocument{Protocol: ProtocolV1, ProofID: uuid.New(), Kind: WholeRealmRetired, Cut: cut, PredecessorExecutionCohortID: uuid.New(), PredecessorRealmIDs: []string{domain.RealmID}, ReplacementRealmIDs: []string{replacement.RealmID}, ReplacementCredentialCohortIDs: []uuid.UUID{replacement.CredentialCohortID}, RetiredDomains: []DomainBinding{domain}, ReplacementDomains: []DomainBinding{replacement}, SourceBindings: sources}}}
	for _, kind := range []string{"ADMISSION_BARRIER", "CREDENTIAL_MANIFEST", "DOCKER_FENCE", "ISSUER_MANIFEST", "POSTGRES_FENCE", "PROVIDER_MANIFEST", "REDIS_FENCE", "SOURCE_CATALOG"} {
		b := []byte(`{"fixture":"not production evidence"}`)
		switch kind {
		case "SOURCE_CATALOG":
			b = jsonBytes(sources)
			f.record.Document.Cut.SourceCatalogHash = Hash(CatalogDomain, b)
		case "PROVIDER_MANIFEST":
			b = jsonBytes(struct{ Retired, Replacement []DomainBinding }{[]DomainBinding{domain}, []DomainBinding{replacement}})
			f.record.Document.Cut.ProviderManifestHash = Hash(ProviderDomain, b)
		case "CREDENTIAL_MANIFEST":
			f.record.Document.Cut.CredentialManifestHash = Hash(CredentialDomain, b)
		case "ADMISSION_BARRIER":
			f.record.Document.Cut.ClosedAdmissionBarrierHash = Hash(BarrierDomain, b)
		case "ISSUER_MANIFEST":
			f.record.Document.Cut.IssuerImplementationManifestHash = Hash(IssuerDomain, b)
		}
		ref := ComponentRef{kind, ProtocolV1, sha256.Sum256(b), uuid.New()}
		f.record.Document.Components = append(f.record.Document.Components, ref)
		f.record.Components = append(f.record.Components, ComponentRecord{ref, b})
	}
	cut = f.record.Document.Cut
	request := []byte(`{"command":"cold-cut"}`)
	targets := []byte(`[{"native":"exact-owner-target"}]`)
	f.work = OwnerWork{Ref: OwnerWorkRef{ProtocolV1, "test-owner-v1", uuid.New(), Hash(OwnerRequestDomain, request)}, Purpose: ColdCutDrain, SourceKind: "NATIVE_SPACE", Cut: cut, SpaceID: uuid.New(), CapturedHead: OwnerHeadBinding{State: HeadAbsent, Hash: Hash(HeadDomain, []byte(HeadAbsent))}, OwnerRequestDomain: OwnerRequestDomain, OwnerRequestBytes: request, SourceManifestHash: digest("NATIVE_SPACE"), SealedTargetManifestBytes: targets, SealedTargetManifestHash: Hash(TargetDomain, targets), SealedTargetCount: 1, RequiredDomains: []DomainBinding{domain}}
	f.installation = Installation{cut.EnvironmentID, cut.AuthorityIncarnation, cut.IssuerImplementationManifestHash}
	f.seal()
	return f
}

func TestVerifierResolvesBothJournalsAndReobserves(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	v, err := f.verifier().VerifyForWork(ctx, f.record.Ref, f.work.Ref)
	if err != nil {
		t.Fatal(err)
	}
	if err = v.ValidateFor(f.work); err != nil {
		t.Fatal(err)
	}
	if v.ProofRef() != f.record.Ref || v.WorkRef() != f.work.Ref || v.WorkHash() != Hash(WorkDomain, jsonBytes(f.work)) || v.TargetManifestHash() != f.work.SealedTargetManifestHash {
		t.Fatal("incomplete verified binding")
	}
	f.observationErr = ErrPending
	if _, err = f.verifier().VerifyForWork(ctx, f.record.Ref, f.work.Ref); !errors.Is(err, ErrPending) {
		t.Fatalf("recovery must reobserve: %v", err)
	}
	if f.observations != 2 {
		t.Fatal("resource observations skipped")
	}
	changed := f.work
	changed.SealedTargetManifestBytes = []byte(`[]`)
	if v.ValidateFor(changed) == nil {
		t.Fatal("changed target accepted")
	}
	if (VerifiedRetirement{}).ValidateFor(f.work) == nil {
		t.Fatal("zero value authenticated work")
	}
}

func TestVerifierRejectsCopiedOrIncompleteEvidence(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*fixture)
	}{
		{"uploaded-only", func(f *fixture) { f.journalErr = ErrUnavailable }},
		{"unknown-work", func(f *fixture) { f.ownerErr = ErrUnavailable }},
		{"restored-external-incarnation", func(f *fixture) { f.installation.AuthorityIncarnation = uuid.New() }},
		{"wrong-cut", func(f *fixture) { f.work.Cut.CutoverOperationID = uuid.New() }},
		{"wrong-source-contract", func(f *fixture) { f.work.SourceManifestHash = digest("other") }},
		{"changed-target-bytes", func(f *fixture) { f.work.SealedTargetManifestBytes = []byte(`[]`) }},
		{"wrong-target-count", func(f *fixture) { f.work.SealedTargetCount = 2 }},
		{"unmapped-credential-domain", func(f *fixture) { f.work.RequiredDomains[0].CredentialCohortID = uuid.New() }},
		{"missing-resource-component", func(f *fixture) { f.record.Components = f.record.Components[1:] }},
		{"changed-component", func(f *fixture) { f.record.Components[0].CanonicalBytes = []byte(`{}`) }},
		{"noncanonical-receipt", func(f *fixture) { f.record.CanonicalBytes = append(f.record.CanonicalBytes, ' ') }},
		{"reused-old-realm", func(f *fixture) {
			f.record.Document.ReplacementRealmIDs = f.record.Document.PredecessorRealmIDs
			f.seal()
		}},
		{"incomplete-catalog", func(f *fixture) { f.record.Document.SourceBindings = f.record.Document.SourceBindings[1:]; f.seal() }},
		{"fabricated-p3", func(f *fixture) { f.work.Purpose = P3FrozenCompletion }},
		{"coldcut-with-synthetic-deletion", func(f *fixture) { f.work.ExistingP3 = &P3Binding{DeletionOperationID: uuid.New()} }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			tt.mutate(f)
			if _, err := f.verifier().VerifyForWork(context.Background(), f.record.Ref, f.work.Ref); err == nil {
				t.Fatal("unsafe proof accepted")
			}
		})
	}
}

func TestP3PreservesNativeWrapperMethodHash(t *testing.T) {
	f := newFixture()
	// Opaque deterministic protobuf wrapper bytes, not JSON and not the inner
	// SpaceLifecycleFenceRequest. Native owner validates its protobuf semantics.
	wrapper := []byte{0x0a, 0x02, 0x08, 0x01}
	f.work.Purpose = P3FrozenCompletion
	f.work.OwnerRequestDomain = P3FenceRequestDomain
	f.work.OwnerRequestBytes = wrapper
	f.work.Ref.RequestHash = Hash(P3FenceRequestDomain, wrapper)
	f.work.ExistingP3 = &P3Binding{DeletionOperationID: uuid.New(), Generation: 3, RequestDomain: P3FenceRequestDomain, CanonicalRequestBytes: wrapper, RequestHash: f.work.Ref.RequestHash, ManifestID: uuid.New(), ManifestHash: digest("original manifest"), ItemCount: 7}
	v, err := f.verifier().VerifyForWork(context.Background(), f.record.Ref, f.work.Ref)
	if err != nil {
		t.Fatal(err)
	}
	if err = v.ValidateFor(f.work); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*OwnerWork){
		func(w *OwnerWork) { w.ExistingP3.Generation++ },
		func(w *OwnerWork) { w.ExistingP3.ManifestID = uuid.New() },
		func(w *OwnerWork) { w.ExistingP3.ItemCount++ },
	} {
		w := f.work
		p := *w.ExistingP3
		w.ExistingP3 = &p
		change(&w)
		if v.ValidateFor(w) == nil {
			t.Fatal("changed original P3 accepted")
		}
	}
	f.work.OwnerRequestDomain = OwnerRequestDomain
	f.work.Ref.RequestHash = Hash(OwnerRequestDomain, wrapper)
	if _, err = f.verifier().VerifyForWork(context.Background(), f.record.Ref, f.work.Ref); err == nil {
		t.Fatal("generic digest replaced protobuf method domain")
	}
}
