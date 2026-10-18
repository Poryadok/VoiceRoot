package spacemedia

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	callsv1 "voice.app/voice/calls/v1"
	commonv1 "voice.app/voice/common/v1"
	"voice/backend/voice/internal/retirementproof"
	"voice/backend/voice/internal/spacelifecycle"
)

// This models protected reader/resource observations for owner component tests.
// It is NOT an issuer, actual realm fence or resource/activation acceptance.
type ownerResourceFixture struct {
	record         retirementproof.ResourceRecord
	installation   retirementproof.Installation
	observationErr error
	observations   int
}

type unavailableOwnerMediaFixture struct{}

func (unavailableOwnerMediaFixture) RetirementCapability() string { return "EJECT_ONLY" }
func (unavailableOwnerMediaFixture) RemoveParticipant(context.Context, string, string) error {
	return spacelifecycle.ErrUnavailable
}
func (unavailableOwnerMediaFixture) CloseRoom(context.Context, string) error {
	return spacelifecycle.ErrUnavailable
}

func TestNativeOwnerQualifiedP3CompletesOriginalPendingReceiptOnly(t *testing.T) {
	ctx := t.Context()
	pool := startSpaceMediaAdmissionPostgres(t, ctx)
	applyOwnerMigration(t, ctx, pool)
	admissions := NewPostgresAdmissionStore(pool)
	a := admissionFixture()
	require.NoError(t, prepareAdmission(ctx, admissions, &a))
	require.NoError(t, admissions.Fence(ctx, a))
	require.NoError(t, admissions.MarkCommitted(ctx, a.OperationID, a.Generation))
	require.NoError(t, admissions.MarkProjectionApplied(ctx, a.OperationID, a.Generation))
	calls, _ := newP3Redis(t)
	media := &p3Media{}
	coordinator := &Coordinator{ProjectionGates: calls, Calls: calls, Store: calls, Admissions: admissions, Media: media}
	store := spacelifecycle.NewPostgresStore(pool, BeginPartyRoomClose)
	effects := spacelifecycle.NewEffects(pool, unavailableOwnerMediaFixture{})
	effects.ProjectionFence = calls
	// Deliberately omit native drain: the restrictive capture commits while
	// response/recovery remains pending. No external effect is invented.
	service := &spacelifecycle.Service{Store: store, Effects: effects}
	manifest := retirementproof.Hash("fixture-manifest", []byte("original-pending-P3"))
	frozen := &commonv1.SpaceLifecycleFenceRequest{ProtocolVersion: 1, SpaceId: a.SpaceID.String(), DeletionOperationId: uuid.NewString(), Generation: 1,
		DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN,
		Manifest:     &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: manifest[:], ItemCount: 1}}
	_, err := service.ApplySpaceLifecycleFence(ctx, frozen)
	require.Error(t, err)
	var originalBytes, originalHash []byte
	var reservedID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT request_bytes,request_sha256,receipt_id FROM voice_space_lifecycle_fence_receipts WHERE space_id=$1 AND generation=1`, a.SpaceID).Scan(&originalBytes, &originalHash, &reservedID))
	wrapped, err := (proto.MarshalOptions{Deterministic: true}).Marshal(&callsv1.ApplySpaceLifecycleFenceRequest{Fence: frozen})
	require.NoError(t, err)
	require.Equal(t, wrapped, originalBytes)
	resource := newOwnerResourceFixture(t)
	owner := &spacelifecycle.NativeRetirementOwner{Store: store, Authority: ownerMaintenanceFixture{resource.record.Document.Cut}, NativeDrain: coordinator, ProjectionFence: calls}
	owner.Verifier = retirementproof.TrustedVerifier{Journal: resource, Owner: owner, Installation: resource, Resources: resource}
	head, err := owner.ObserveCutoverHead(ctx, a.SpaceID)
	require.NoError(t, err)
	var requestHash retirementproof.Digest
	copy(requestHash[:], originalHash)
	p3 := &retirementproof.P3Binding{DeletionOperationID: uuid.MustParse(frozen.DeletionOperationId), Generation: 1, RequestDomain: retirementproof.P3FenceRequestDomain, CanonicalRequestBytes: originalBytes, RequestHash: requestHash, ManifestID: uuid.MustParse(frozen.Manifest.ManifestId), ManifestHash: manifest, ItemCount: 1}
	ref, err := owner.PrepareCutoverDrain(ctx, resource.record.Document.Cut, spacelifecycle.ExpectedOwnerBinding{WorkID: uuid.New(), SpaceID: a.SpaceID, Purpose: retirementproof.P3FrozenCompletion, Head: head, ExistingP3: p3})
	require.NoError(t, err)
	receipt, err := owner.ResumeCutoverDrain(ctx, ref, resource.record.Ref)
	require.NoError(t, err)
	var saved []byte
	var proven bool
	var ejected int
	require.NoError(t, pool.QueryRow(ctx, `SELECT receipt_bytes,retirement_proven FROM voice_space_lifecycle_fence_receipts WHERE space_id=$1 AND generation=1`, a.SpaceID).Scan(&saved, &proven))
	require.True(t, proven)
	var public commonv1.SpaceLifecycleFenceReceipt
	require.NoError(t, proto.Unmarshal(saved, &public))
	require.Equal(t, reservedID.String(), public.ReceiptId)
	require.Equal(t, frozen.DeletionOperationId, public.DeletionOperationId)
	require.Equal(t, originalHash, public.RequestSha256)
	require.Equal(t, manifest[:], public.ManifestSha256)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_space_lifecycle_targets WHERE space_id=$1 AND ejected_at IS NOT NULL`, a.SpaceID).Scan(&ejected))
	require.Zero(t, ejected, "REALM_RETIRED is not a fabricated ejection")
	require.Empty(t, media.removed)
	require.Equal(t, "EJECT_ONLY", service.RetirementCapability())
	replay, err := owner.ResumeCutoverDrain(ctx, ref, resource.record.Ref)
	require.NoError(t, err)
	require.Equal(t, receipt.CanonicalBytes, replay.CanonicalBytes)
}

func (f *ownerResourceFixture) ReadResourceProof(_ context.Context, ref retirementproof.ProofRef) (retirementproof.ResourceRecord, error) {
	if ref != f.record.Ref {
		return retirementproof.ResourceRecord{}, retirementproof.ErrConflict
	}
	return f.record, nil
}
func (f *ownerResourceFixture) CurrentInstallation(context.Context) (retirementproof.Installation, error) {
	return f.installation, nil
}
func (f *ownerResourceFixture) CheckResourceRetirement(context.Context, retirementproof.ResourceRecord) error {
	f.observations++
	return f.observationErr
}

func ownerFixtureJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	require.NoError(t, err)
	return raw
}

func newOwnerResourceFixture(t *testing.T) *ownerResourceFixture {
	t.Helper()
	hash := func(text string) retirementproof.Digest { return sha256.Sum256([]byte(text)) }
	old := retirementproof.DomainBinding{RealmID: "fixture-retired-realm", CredentialCohortID: uuid.New(), RouteIdentity: "fixture-retired-route", ImplementationHash: hash("fixture-provider")}
	newRealm := retirementproof.DomainBinding{RealmID: "fixture-distinct-realm", CredentialCohortID: uuid.New(), RouteIdentity: "fixture-new-route", ImplementationHash: hash("fixture-provider")}
	var sources []retirementproof.SourceBinding
	for _, kind := range []string{"GAME_PROVISION_AND_CONVERSION", "LEGACY_COMPATIBILITY", "MEDIA_AND_GRANT_PRODUCERS", "NATIVE_ORDINARY", "NATIVE_SPACE", "OWNED_ROOM", "SHARED_IDENTITY_FENCES"} {
		binding := retirementproof.SourceBinding{SourceKind: kind, OwnerProtocol: "fixture-other-owner", ContractHash: hash(kind)}
		if kind == "NATIVE_SPACE" {
			binding.OwnerProtocol = spacelifecycle.NativeOwnerProtocol
			binding.ContractHash = spacelifecycle.NativeOwnerContractHash()
		}
		sources = append(sources, binding)
	}
	cut := ownerCutFixture()
	doc := retirementproof.ResourceDocument{Protocol: retirementproof.ProtocolV1, ProofID: uuid.New(), Kind: retirementproof.WholeRealmRetired, Cut: cut,
		PredecessorExecutionCohortID: uuid.New(), PredecessorRealmIDs: []string{old.RealmID}, ReplacementRealmIDs: []string{newRealm.RealmID}, ReplacementCredentialCohortIDs: []uuid.UUID{newRealm.CredentialCohortID}, RetiredDomains: []retirementproof.DomainBinding{old}, ReplacementDomains: []retirementproof.DomainBinding{newRealm}, SourceBindings: sources}
	f := &ownerResourceFixture{}
	for _, kind := range []string{"ADMISSION_BARRIER", "CREDENTIAL_MANIFEST", "DOCKER_FENCE", "ISSUER_MANIFEST", "POSTGRES_FENCE", "PROVIDER_MANIFEST", "REDIS_FENCE", "SOURCE_CATALOG"} {
		raw := []byte(`{"fixture":"owner component only, not resource acceptance"}`)
		switch kind {
		case "SOURCE_CATALOG":
			raw = ownerFixtureJSON(t, sources)
			doc.Cut.SourceCatalogHash = retirementproof.Hash(retirementproof.CatalogDomain, raw)
		case "PROVIDER_MANIFEST":
			raw = ownerFixtureJSON(t, struct {
				Retired, Replacement []retirementproof.DomainBinding
			}{[]retirementproof.DomainBinding{old}, []retirementproof.DomainBinding{newRealm}})
			doc.Cut.ProviderManifestHash = retirementproof.Hash(retirementproof.ProviderDomain, raw)
		case "CREDENTIAL_MANIFEST":
			raw = ownerFixtureJSON(t, retirementproof.CredentialManifest{Protocol: retirementproof.ProtocolV1, HistoricalSources: []retirementproof.HistoricalIssuanceBinding{{SourceKind: "NATIVE_SPACE", OwnerProtocol: spacelifecycle.NativeOwnerProtocol, Domains: []retirementproof.DomainBinding{old}, EvidenceBytes: []byte(`{"fixture":"source-specific complete historical confinement"}`)}}})
			doc.Cut.CredentialManifestHash = retirementproof.Hash(retirementproof.CredentialDomain, raw)
		case "ADMISSION_BARRIER":
			doc.Cut.ClosedAdmissionBarrierHash = retirementproof.Hash(retirementproof.BarrierDomain, raw)
		case "ISSUER_MANIFEST":
			doc.Cut.IssuerImplementationManifestHash = retirementproof.Hash(retirementproof.IssuerDomain, raw)
		}
		ref := retirementproof.ComponentRef{Kind: kind, Protocol: retirementproof.ProtocolV1, CanonicalBytesHash: sha256.Sum256(raw), JournalRef: uuid.New()}
		doc.Components = append(doc.Components, ref)
		f.record.Components = append(f.record.Components, retirementproof.ComponentRecord{Ref: ref, CanonicalBytes: raw})
	}
	f.record.Document = doc
	f.record.CanonicalBytes = ownerFixtureJSON(t, doc)
	f.record.Ref = retirementproof.ProofRef{Protocol: retirementproof.ProtocolV1, ProofID: doc.ProofID, ReceiptHash: retirementproof.Hash(retirementproof.ResourceDomain, f.record.CanonicalBytes)}
	f.installation = retirementproof.Installation{EnvironmentID: doc.Cut.EnvironmentID, AuthorityIncarnation: doc.Cut.AuthorityIncarnation, IssuerImplementationManifestHash: doc.Cut.IssuerImplementationManifestHash}
	return f
}

func TestNativeOwnerQualifiedRefCleansExactNativeWithoutCallingReplacementRealm(t *testing.T) {
	ctx := t.Context()
	pool := startSpaceMediaAdmissionPostgres(t, ctx)
	applyOwnerMigration(t, ctx, pool)
	admissions := NewPostgresAdmissionStore(pool)
	a := admissionFixture()
	require.NoError(t, prepareAdmission(ctx, admissions, &a))
	require.NoError(t, admissions.Fence(ctx, a))
	require.NoError(t, admissions.MarkCommitted(ctx, a.OperationID, a.Generation))
	require.NoError(t, admissions.MarkProjectionApplied(ctx, a.OperationID, a.Generation))
	calls, _ := newP3Redis(t)
	media := &p3Media{}
	coordinator := &Coordinator{ProjectionGates: calls, Calls: calls, Store: calls, Admissions: admissions, Media: media}
	resource := newOwnerResourceFixture(t)
	owner := &spacelifecycle.NativeRetirementOwner{Store: spacelifecycle.NewPostgresStore(pool, BeginPartyRoomClose), Authority: ownerMaintenanceFixture{resource.record.Document.Cut}, NativeDrain: coordinator, ProjectionFence: calls}
	head, err := owner.ObserveCutoverHead(ctx, a.SpaceID)
	require.NoError(t, err)
	ref, err := owner.PrepareCutoverDrain(ctx, resource.record.Document.Cut, spacelifecycle.ExpectedOwnerBinding{WorkID: uuid.New(), SpaceID: a.SpaceID, Purpose: retirementproof.ColdCutDrain, Head: head})
	require.NoError(t, err)
	_, err = owner.ResumeCutoverDrain(ctx, ref, resource.record.Ref)
	require.Error(t, err, "no verifier means no qualified completion")
	owner.Verifier = retirementproof.TrustedVerifier{Journal: resource, Owner: owner, Installation: resource, Resources: resource}
	work, err := owner.ReadOwnerWork(ctx, ref)
	require.NoError(t, err)
	target := spacelifecycle.CapturedTarget{OperationID: a.OperationID.String(), Generation: a.Generation, AccountID: a.AccountID.String(), ProfileID: a.ProfileID.String(), SpaceID: a.SpaceID.String(), RoomID: a.RoomID, VoiceRoomID: a.VoiceRoomID, RoomGeneration: a.RoomGeneration, Identity: a.Identity}
	require.Error(t, admissions.CompleteRetiredCapturedDrain(ctx, target, retirementproof.VerifiedRetirement{}, work), "zero proof cannot authorize owner cleanup")
	verified, err := owner.Verifier.VerifyForWork(ctx, resource.record.Ref, ref)
	require.NoError(t, err)
	require.Error(t, admissions.CompleteRetiredCapturedDrain(ctx, target, verified, work), "trusted resource proof still requires actual owner acceptance under the barrier")
	badProof := resource.record.Ref
	badProof.ReceiptHash[0] ^= 1
	_, err = owner.ResumeCutoverDrain(ctx, ref, badProof)
	require.Error(t, err)
	resource.observationErr = retirementproof.ErrPending
	_, err = owner.ResumeCutoverDrain(ctx, ref, resource.record.Ref)
	require.Error(t, err)
	var accepted int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_native_owner_acceptances WHERE work_id=$1`, ref.WorkID).Scan(&accepted))
	require.Zero(t, accepted, "unobserved retirement cannot authorize cleanup")
	resource.observationErr = nil
	receipt, err := owner.ResumeCutoverDrain(ctx, ref, resource.record.Ref)
	require.NoError(t, err)
	require.NotNil(t, receipt)
	require.Equal(t, ref, receipt.Work)
	require.Empty(t, media.removed, "retired realm is satisfied by proof; never eject replacement by recycled room name")
	var state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT participant_state FROM voice_space_media_admissions WHERE operation_id=$1`, a.OperationID).Scan(&state))
	require.Equal(t, "ENDED", state)
	var fences, p3Heads int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_account_voice_fences WHERE admission_operation_id=$1`, a.OperationID).Scan(&fences))
	require.Zero(t, fences)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_space_lifecycle_fence_heads WHERE space_id=$1`, a.SpaceID).Scan(&p3Heads))
	require.Zero(t, p3Heads)
	replay, err := owner.ResumeCutoverDrain(ctx, ref, resource.record.Ref)
	require.NoError(t, err)
	require.Equal(t, receipt.CanonicalBytes, replay.CanonicalBytes)
	observed, err := owner.ReadCutoverDrain(ctx, ref)
	require.NoError(t, err)
	require.Equal(t, receipt.CanonicalBytes, observed.CanonicalBytes)
}
