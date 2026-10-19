package legacycut

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/voice/internal/retirementproof"
	"voice/backend/voice/internal/store"
)

type inventoryFixture struct{ inventory Inventory }

func (f *inventoryFixture) ReadClosedLegacyInventory(context.Context, retirementproof.CutBinding) (Inventory, error) {
	return f.inventory, nil
}
func (*inventoryFixture) CheckMaintenance(context.Context, retirementproof.CutBinding) error {
	return nil
}

type endpointsFixture struct {
	snapshot   store.LegacyRedisSnapshot
	cleaned    int
	cleanupErr error
}

func (f *endpointsFixture) Capture(context.Context, Endpoint) (store.LegacyRedisSnapshot, error) {
	return f.snapshot, nil
}
func (f *endpointsFixture) CompareCleanup(context.Context, Endpoint, store.LegacyRedisSnapshot) error {
	f.cleaned++
	return f.cleanupErr
}

// This suite uses real PG persistence. The authority/endpoint fixtures are UNIT
// inputs, never installed retirement evidence or a deployment qualification.
func TestLegacyOwnerPreparePersistsAndRetainsUnknownIssuance(t *testing.T) {
	o, f, e, cut := newOwnerFixture(t)
	pool := o.Pool
	ctx := context.Background()
	id := uuid.New()
	ref, err := o.PrepareCutoverDrain(ctx, cut, id)
	require.NoError(t, err)
	require.Equal(t, id, ref.WorkID)
	var raw []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT work_bytes FROM voice_legacy_owner_works WHERE work_id=$1`, id).Scan(&raw))
	var work retirementproof.OwnerWork
	require.NoError(t, json.Unmarshal(raw, &work))
	require.Equal(t, SourceKind, work.SourceKind)
	require.NotEmpty(t, work.UnknownIssuanceMarkers, "empty Redis is not empty historical credential issuance")
	retry, err := o.PrepareCutoverDrain(ctx, cut, id)
	require.NoError(t, err)
	require.Equal(t, ref, retry)
	// Protected inventory changes never silently replace accepted request bytes.
	f.inventory.Endpoints[0].Prefix = "another:"
	_, err = o.PrepareCutoverDrain(ctx, cut, id)
	require.ErrorIs(t, err, retirementproof.ErrConflict)
	require.Zero(t, e.cleaned)
}

func newOwnerFixture(t *testing.T) (*Owner, *inventoryFixture, *endpointsFixture, retirementproof.CutBinding) {
	t.Helper()
	dsn := os.Getenv("VOICE_LEGACY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("owned isolated PostgreSQL required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	cut := retirementproof.CutBinding{EnvironmentID: uuid.New(), AuthorityIncarnation: uuid.New(), CutoverOperationID: uuid.New(), CoverageEpoch: 1, ClosedAdmissionBarrierID: uuid.New()}
	d := retirementproof.Hash("fixture", []byte("unit only"))
	cut.PlanHash = d
	cut.DatabaseIdentity = d
	cut.SourceCatalogHash = d
	cut.SchemaContractHash = d
	cut.WriterManifestHash = d
	cut.ProviderManifestHash = d
	cut.CredentialManifestHash = d
	cut.ClosedAdmissionBarrierHash = d
	cut.IssuerImplementationManifestHash = d
	f := &inventoryFixture{Inventory{Cut: cut, Endpoints: []Endpoint{{ID: "historical", ResourceIdentity: d, Database: 0, Prefix: "legacy:"}}, ExecutionCohorts: []ExecutionCohort{}, CanonicalDeploymentBytes: json.RawMessage(`{"closed":"unit inventory"}`)}}
	e := &endpointsFixture{snapshot: store.LegacyRedisSnapshot{Prefix: "legacy:", Entries: []store.LegacyRedisEntry{}}}
	o := &Owner{Pool: pool, Authority: f, Inventory: f, Endpoints: e}
	return o, f, e, cut
}

func TestLegacyOwnerRederivesUnknownEvidenceOnEveryRead(t *testing.T) {
	o, inventory, _, cut := newOwnerFixture(t)
	ctx := context.Background()
	ref, err := o.PrepareCutoverDrain(ctx, cut, uuid.New())
	require.NoError(t, err)
	w, err := o.ReadOwnerWork(ctx, ref)
	require.NoError(t, err)
	for _, mutate := range []func(*retirementproof.OwnerWork){
		func(w *retirementproof.OwnerWork) { w.UnknownIssuanceMarkers = w.UnknownIssuanceMarkers[1:] },
		func(w *retirementproof.OwnerWork) {
			w.CapturedProviderBindings = []retirementproof.Digest{retirementproof.Hash("fake", []byte("current endpoint is not historical authority"))}
		},
		func(w *retirementproof.OwnerWork) { w.Cut.CoverageEpoch++ },
	} {
		changed := w
		mutate(&changed)
		_, err := decodeTargets(changed)
		require.ErrorIs(t, err, retirementproof.ErrConflict)
	}
	// Retained caller-like arrays cannot replace current protected source closure.
	inventory.inventory.Endpoints = []Endpoint{}
	_, err = o.ReadOwnerWork(ctx, ref)
	require.ErrorIs(t, err, retirementproof.ErrConflict)
}

func TestLegacyOwnerProofFailureNeverCleansAndAcceptedFailureResumes(t *testing.T) {
	o, inventory, endpoints, cut := newOwnerFixture(t)
	proof := newProofFixture(t, &cut)
	inventory.inventory.Cut = cut
	o.Verifier = retirementproof.TrustedVerifier{Journal: proof, Owner: o, Installation: proof, Resources: proof}
	ctx := context.Background()
	ref, err := o.PrepareCutoverDrain(ctx, cut, uuid.New())
	require.NoError(t, err)
	// A reference is not proof provenance.
	forged := proof.record.Ref
	forged.ProofID = uuid.New()
	_, err = o.ResumeCutoverDrain(ctx, ref, forged)
	require.ErrorIs(t, err, retirementproof.ErrConflict)
	require.Zero(t, endpoints.cleaned)
	endpoints.cleanupErr = retirementproof.ErrPending
	_, err = o.ResumeCutoverDrain(ctx, ref, proof.record.Ref)
	require.ErrorIs(t, err, retirementproof.ErrPending)
	var state string
	require.NoError(t, o.Pool.QueryRow(ctx, `SELECT state FROM voice_legacy_owner_works WHERE work_id=$1`, ref.WorkID).Scan(&state))
	require.Equal(t, "ACCEPTED", state)
	_, err = o.ReadCutoverDrain(ctx, ref)
	require.ErrorIs(t, err, retirementproof.ErrPending)
	endpoints.cleanupErr = nil
	receipt, err := o.ResumeCutoverDrain(ctx, ref, proof.record.Ref)
	require.NoError(t, err)
	cleaned := endpoints.cleaned
	replayed, err := o.ResumeCutoverDrain(ctx, ref, proof.record.Ref)
	require.NoError(t, err)
	require.Equal(t, receipt, replayed)
	require.Equal(t, cleaned, endpoints.cleaned)
	read, err := o.ReadCutoverDrain(ctx, ref)
	require.NoError(t, err)
	require.Equal(t, receipt, read)
	// Restored database bytes cannot override external incarnation/fence loss.
	proof.observationErr = retirementproof.ErrUnavailable
	_, err = o.ResumeCutoverDrain(ctx, ref, proof.record.Ref)
	require.ErrorIs(t, err, retirementproof.ErrUnavailable)
	proof.observationErr = nil
	proof.installation.AuthorityIncarnation = uuid.New()
	_, err = o.ResumeCutoverDrain(ctx, ref, proof.record.Ref)
	require.ErrorIs(t, err, retirementproof.ErrConflict)
	// Database guards preserve historical bytes even for the cleanup role.
	_, err = o.Pool.Exec(ctx, `UPDATE voice_legacy_owner_acceptances SET acceptance_bytes='forged' WHERE work_id=$1`, ref.WorkID)
	require.Error(t, err)
	_, err = o.Pool.Exec(ctx, `UPDATE voice_legacy_owner_works SET work_bytes='forged' WHERE work_id=$1`, ref.WorkID)
	require.Error(t, err)
}

type proofFixture struct {
	record         retirementproof.ResourceRecord
	installation   retirementproof.Installation
	observationErr error
}

func (f *proofFixture) ReadResourceProof(_ context.Context, r retirementproof.ProofRef) (retirementproof.ResourceRecord, error) {
	if r != f.record.Ref {
		return retirementproof.ResourceRecord{}, retirementproof.ErrConflict
	}
	return f.record, nil
}
func (f *proofFixture) CurrentInstallation(context.Context) (retirementproof.Installation, error) {
	return f.installation, nil
}
func (f *proofFixture) CheckResourceRetirement(context.Context, retirementproof.ResourceRecord) error {
	return f.observationErr
}

// Fixture issuer evidence is UNIT-only; it is deliberately not an installed
// ProtectedInventoryReader/fencer or a claim that a media realm was retired.
func newProofFixture(t *testing.T, cut *retirementproof.CutBinding) *proofFixture {
	t.Helper()
	d := retirementproof.Hash("fixture", []byte("provider implementation"))
	old := retirementproof.DomainBinding{RealmID: "unit-old", CredentialCohortID: uuid.New(), RouteIdentity: "unit-old-route", ImplementationHash: d}
	newDomain := retirementproof.DomainBinding{RealmID: "unit-new", CredentialCohortID: uuid.New(), RouteIdentity: "unit-new-route", ImplementationHash: d}
	sources := []retirementproof.SourceBinding{}
	for _, s := range []string{"GAME_PROVISION_AND_CONVERSION", SourceKind, "MEDIA_AND_GRANT_PRODUCERS", "NATIVE_ORDINARY", "NATIVE_SPACE", "OWNED_ROOM", "SHARED_IDENTITY_FENCES"} {
		h := d
		protocol := "unit-other-owner"
		if s == SourceKind {
			h = ContractHash()
			protocol = Protocol
		}
		sources = append(sources, retirementproof.SourceBinding{SourceKind: s, OwnerProtocol: protocol, ContractHash: h})
	}
	f := &proofFixture{record: retirementproof.ResourceRecord{Document: retirementproof.ResourceDocument{Protocol: 1, ProofID: uuid.New(), Kind: retirementproof.WholeRealmRetired, PredecessorExecutionCohortID: uuid.New(), PredecessorRealmIDs: []string{old.RealmID}, ReplacementRealmIDs: []string{newDomain.RealmID}, ReplacementCredentialCohortIDs: []uuid.UUID{newDomain.CredentialCohortID}, RetiredDomains: []retirementproof.DomainBinding{old}, ReplacementDomains: []retirementproof.DomainBinding{newDomain}, SourceBindings: sources}}}
	for _, kind := range []string{"ADMISSION_BARRIER", "CREDENTIAL_MANIFEST", "DOCKER_FENCE", "ISSUER_MANIFEST", "POSTGRES_FENCE", "PROVIDER_MANIFEST", "REDIS_FENCE", "SOURCE_CATALOG"} {
		raw := []byte(`{"unit":"fixture only"}`)
		switch kind {
		case "SOURCE_CATALOG":
			raw = mustJSON(t, sources)
			cut.SourceCatalogHash = retirementproof.Hash(retirementproof.CatalogDomain, raw)
		case "PROVIDER_MANIFEST":
			raw = mustJSON(t, struct {
				Retired, Replacement []retirementproof.DomainBinding
			}{[]retirementproof.DomainBinding{old}, []retirementproof.DomainBinding{newDomain}})
			cut.ProviderManifestHash = retirementproof.Hash(retirementproof.ProviderDomain, raw)
		case "CREDENTIAL_MANIFEST":
			raw = mustJSON(t, retirementproof.CredentialManifest{Protocol: 1, HistoricalSources: []retirementproof.HistoricalIssuanceBinding{{SourceKind: SourceKind, OwnerProtocol: Protocol, Domains: []retirementproof.DomainBinding{old}, EvidenceBytes: []byte(`{"unit":"complete historical source confinement fixture"}`)}}})
			cut.CredentialManifestHash = retirementproof.Hash(retirementproof.CredentialDomain, raw)
		case "ADMISSION_BARRIER":
			cut.ClosedAdmissionBarrierHash = retirementproof.Hash(retirementproof.BarrierDomain, raw)
		case "ISSUER_MANIFEST":
			cut.IssuerImplementationManifestHash = retirementproof.Hash(retirementproof.IssuerDomain, raw)
		}
		ref := retirementproof.ComponentRef{Kind: kind, Protocol: 1, CanonicalBytesHash: sha256.Sum256(raw), JournalRef: uuid.New()}
		f.record.Document.Components = append(f.record.Document.Components, ref)
		f.record.Components = append(f.record.Components, retirementproof.ComponentRecord{Ref: ref, CanonicalBytes: raw})
	}
	f.record.Document.Cut = *cut
	f.record.CanonicalBytes = mustJSON(t, f.record.Document)
	f.record.Ref = retirementproof.ProofRef{Protocol: 1, ProofID: f.record.Document.ProofID, ReceiptHash: retirementproof.Hash(retirementproof.ResourceDomain, f.record.CanonicalBytes)}
	f.installation = retirementproof.Installation{EnvironmentID: cut.EnvironmentID, AuthorityIncarnation: cut.AuthorityIncarnation, IssuerImplementationManifestHash: cut.IssuerImplementationManifestHash}
	return f
}
func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, e := json.Marshal(v)
	require.NoError(t, e)
	return b
}
