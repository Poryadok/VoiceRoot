package ordinarycall

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/voice/internal/retirementproof"
)

// Component fixture adapted from native owner564; not installed issuer,
// external retirement evidence or maintenance-credential qualification.
type ordinaryResourceFixture struct {
	record         retirementproof.ResourceRecord
	installation   retirementproof.Installation
	observationErr error
	observations   int
}

func (f *ordinaryResourceFixture) ReadResourceProof(_ context.Context, ref retirementproof.ProofRef) (retirementproof.ResourceRecord, error) {
	if ref != f.record.Ref {
		return retirementproof.ResourceRecord{}, retirementproof.ErrConflict
	}
	return f.record, nil
}
func (f *ordinaryResourceFixture) CurrentInstallation(context.Context) (retirementproof.Installation, error) {
	return f.installation, nil
}
func (f *ordinaryResourceFixture) CheckResourceRetirement(context.Context, retirementproof.ResourceRecord) error {
	f.observations++
	return f.observationErr
}

func ordinaryFixtureJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	require.NoError(t, err)
	return raw
}

func newOrdinaryResourceFixture(t *testing.T) *ordinaryResourceFixture {
	t.Helper()
	hash := func(text string) retirementproof.Digest { return sha256.Sum256([]byte(text)) }
	old := retirementproof.DomainBinding{RealmID: "fixture-retired-realm", CredentialCohortID: uuid.New(), RouteIdentity: "fixture-retired-route", ImplementationHash: hash("fixture-provider")}
	newRealm := retirementproof.DomainBinding{RealmID: "fixture-distinct-realm", CredentialCohortID: uuid.New(), RouteIdentity: "fixture-new-route", ImplementationHash: hash("fixture-provider")}
	var sources []retirementproof.SourceBinding
	for _, kind := range []string{"GAME_PROVISION_AND_CONVERSION", "LEGACY_COMPATIBILITY", "MEDIA_AND_GRANT_PRODUCERS", "NATIVE_ORDINARY", "NATIVE_SPACE", "OWNED_ROOM", "SHARED_IDENTITY_FENCES"} {
		binding := retirementproof.SourceBinding{SourceKind: kind, OwnerProtocol: "fixture-other-owner", ContractHash: hash(kind)}
		if kind == "NATIVE_ORDINARY" {
			binding.OwnerProtocol = OrdinaryCutoverOwnerProtocol
			binding.ContractHash = OrdinaryCutoverContractHash()
		}
		sources = append(sources, binding)
	}
	cut := ordinaryCutFixture()
	doc := retirementproof.ResourceDocument{Protocol: retirementproof.ProtocolV1, ProofID: uuid.New(), Kind: retirementproof.WholeRealmRetired, Cut: cut,
		PredecessorExecutionCohortID: uuid.New(), PredecessorRealmIDs: []string{old.RealmID}, ReplacementRealmIDs: []string{newRealm.RealmID}, ReplacementCredentialCohortIDs: []uuid.UUID{newRealm.CredentialCohortID}, RetiredDomains: []retirementproof.DomainBinding{old}, ReplacementDomains: []retirementproof.DomainBinding{newRealm}, SourceBindings: sources}
	f := &ordinaryResourceFixture{}
	for _, kind := range []string{"ADMISSION_BARRIER", "CREDENTIAL_MANIFEST", "DOCKER_FENCE", "ISSUER_MANIFEST", "POSTGRES_FENCE", "PROVIDER_MANIFEST", "REDIS_FENCE", "SOURCE_CATALOG"} {
		raw := []byte(`{"fixture":"owner component only, not resource acceptance"}`)
		switch kind {
		case "SOURCE_CATALOG":
			raw = ordinaryFixtureJSON(t, sources)
			doc.Cut.SourceCatalogHash = retirementproof.Hash(retirementproof.CatalogDomain, raw)
		case "PROVIDER_MANIFEST":
			raw = ordinaryFixtureJSON(t, struct {
				Retired, Replacement []retirementproof.DomainBinding
			}{[]retirementproof.DomainBinding{old}, []retirementproof.DomainBinding{newRealm}})
			doc.Cut.ProviderManifestHash = retirementproof.Hash(retirementproof.ProviderDomain, raw)
		case "CREDENTIAL_MANIFEST":
			raw = ordinaryFixtureJSON(t, retirementproof.CredentialManifest{Protocol: retirementproof.ProtocolV1, HistoricalSources: []retirementproof.HistoricalIssuanceBinding{{SourceKind: "NATIVE_ORDINARY", OwnerProtocol: OrdinaryCutoverOwnerProtocol, Domains: []retirementproof.DomainBinding{old}, EvidenceBytes: []byte(`{"fixture":"source-specific complete historical confinement"}`)}}})
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
	f.record.CanonicalBytes = ordinaryFixtureJSON(t, doc)
	f.record.Ref = retirementproof.ProofRef{Protocol: retirementproof.ProtocolV1, ProofID: doc.ProofID, ReceiptHash: retirementproof.Hash(retirementproof.ResourceDomain, f.record.CanonicalBytes)}
	f.installation = retirementproof.Installation{EnvironmentID: doc.Cut.EnvironmentID, AuthorityIncarnation: doc.Cut.AuthorityIncarnation, IssuerImplementationManifestHash: doc.Cut.IssuerImplementationManifestHash}
	return f
}

type ordinaryVerifierFunc func(context.Context, retirementproof.ProofRef, retirementproof.OwnerWorkRef) (retirementproof.VerifiedRetirement, error)

func (f ordinaryVerifierFunc) VerifyForWork(ctx context.Context, p retirementproof.ProofRef, w retirementproof.OwnerWorkRef) (retirementproof.VerifiedRetirement, error) {
	return f(ctx, p, w)
}

func TestOrdinaryCutoverProofAcceptanceReobservesAndBindsNative(t *testing.T) {
	ctx := context.Background()
	pool := ordinaryPostgres(t)
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	ddl, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations", "voice_db", "000025_ordinary_cutover_drain.up.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(ddl))
	require.NoError(t, err)
	store := NewPostgresStore(pool, storageTestEncoder)
	c := &Coordinator{Journal: store, Media: &mediaFake{}, Projection: &projectionFake{}, Now: func() time.Time { return testNow }}
	pending, _, _ := pendingGroup(t)
	active, err := c.Start(ctx, pending.Intent, nil)
	require.NoError(t, err)
	resource := newOrdinaryResourceFixture(t)
	owner := &CutoverDrainOwner{Store: store, Authority: cutoverAuthorityFixture{}}
	ref, err := owner.PrepareCutoverDrain(ctx, resource.record.Document.Cut, CutoverExpectedRoom{WorkID: uuid.New(), RoomID: active.Intent.RoomID, Incarnation: active.Intent.OperationID, Version: active.Version})
	require.NoError(t, err)
	proof := resource.record.Ref
	count := func() int {
		var n int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_ordinary_cutover_acceptances WHERE work_id=$1`, ref.WorkID).Scan(&n))
		return n
	}
	require.ErrorIs(t, owner.AcceptCutoverProof(ctx, ref, proof), ErrUnavailable)
	owner.Verifier = ordinaryVerifierFunc(func(context.Context, retirementproof.ProofRef, retirementproof.OwnerWorkRef) (retirementproof.VerifiedRetirement, error) {
		return retirementproof.VerifiedRetirement{}, nil
	})
	require.Error(t, owner.AcceptCutoverProof(ctx, ref, proof), "a zero verified value cannot mint acceptance")
	require.Zero(t, count())
	trusted := retirementproof.TrustedVerifier{Journal: resource, Owner: owner, Installation: resource, Resources: resource}
	owner.Verifier = trusted
	wrong := proof
	wrong.ReceiptHash[0] ^= 1
	require.Error(t, owner.AcceptCutoverProof(ctx, ref, wrong))
	require.Zero(t, count())
	resource.observationErr = retirementproof.ErrPending
	require.ErrorIs(t, owner.AcceptCutoverProof(ctx, ref, proof), retirementproof.ErrPending)
	require.Zero(t, count())
	resource.observationErr = nil
	require.NoError(t, owner.AcceptCutoverProof(ctx, ref, proof))
	require.Equal(t, 1, count())
	work, err := owner.ReadOwnerWork(ctx, ref)
	require.NoError(t, err)
	verified, err := trusted.VerifyForWork(ctx, proof, ref)
	require.NoError(t, err)
	var resolution, digest []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT resolution_bytes,resolution_sha256 FROM voice_ordinary_cutover_acceptances WHERE work_id=$1 AND proof_id=$2`, ref.WorkID, proof.ProofID).Scan(&resolution, &digest))
	require.Equal(t, verified.DomainResolutionBytes(), resolution)
	hash := verified.DomainResolutionHash()
	require.Equal(t, hash[:], digest)
	require.NoError(t, verified.ValidateFor(work))
	require.NoError(t, owner.AcceptCutoverProof(ctx, ref, proof))
	require.Equal(t, 1, count(), "lost ACK reuses retained acceptance")
	resource.observationErr = retirementproof.ErrPending
	require.ErrorIs(t, owner.AcceptCutoverProof(ctx, ref, proof), retirementproof.ErrPending, "persisted acceptance does not bypass fresh external observation")
	resource.observationErr = nil
	// A successful external verification cannot authorize a recycled native room.
	// Mutate the document between verification and the owner's final native lock.
	before, err := store.Load(ctx, active.Intent.RoomID)
	require.NoError(t, err)
	owner.Verifier = ordinaryVerifierFunc(func(ctx context.Context, p retirementproof.ProofRef, w retirementproof.OwnerWorkRef) (retirementproof.VerifiedRetirement, error) {
		v, e := trusted.VerifyForWork(ctx, p, w)
		if e != nil {
			return v, e
		}
		intent := before.Intent
		intent.OperationID = uuid.New()
		changed, e := NewPendingRoom(intent, testNow)
		if e != nil {
			return v, e
		}
		raw, e := json.Marshal(changed)
		if e != nil {
			return v, e
		}
		_, e = pool.Exec(ctx, `UPDATE voice_ordinary_rooms SET document=$2,operation_id=$3,state=$4,version=$5,roster_version=$6,projection_version=0 WHERE room_id=$1`, active.Intent.RoomID, raw, changed.Intent.OperationID, changed.Stage, changed.Version, changed.RosterVersion)
		return v, e
	})
	require.ErrorIs(t, owner.AcceptCutoverProof(ctx, ref, proof), ErrConflict)
	require.Equal(t, 1, count())
	var state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM voice_ordinary_cutover_works WHERE work_id=$1`, ref.WorkID).Scan(&state))
	require.Equal(t, "PREPARED", state, "proof acceptance is never native cleanup or a terminal receipt")
}
