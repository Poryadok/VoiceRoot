package partycoverage

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"voice/backend/voice/internal/retirementproof"
)

// This builds storage-contract fixture material, never actual deployment,
// historical issuance or resource retirement evidence.
func resourceStorageFixture(t *testing.T, plan cutPlan) retirementproof.ResourceRecord {
	t.Helper()
	h := func(s string) [32]byte { return sha256.Sum256([]byte(s)) }
	encode := func(v any) []byte { raw, err := json.Marshal(v); require.NoError(t, err); return raw }
	old := retirementproof.DomainBinding{RealmID: "fixture-old", CredentialCohortID: uuid.New(), RouteIdentity: "fixture-old-route", ImplementationHash: h("fixture-provider")}
	newDomain := retirementproof.DomainBinding{RealmID: "fixture-new", CredentialCohortID: uuid.New(), RouteIdentity: "fixture-new-route", ImplementationHash: h("fixture-provider")}
	b := plan.Binding
	d := retirementproof.ResourceDocument{Protocol: 1, ProofID: resourceProofID(plan.OperationID), Kind: retirementproof.WholeRealmRetired, Cut: retirementproof.CutBinding{EnvironmentID: b.EnvironmentID, AuthorityIncarnation: b.AuthorityIncarnation, CutoverOperationID: plan.OperationID, CoverageEpoch: b.CoverageEpoch, DatabaseIdentity: b.DatabaseIdentity, SchemaContractHash: b.SchemaContractHash, WriterManifestHash: b.WriterManifestHash, ClosedAdmissionBarrierID: uuid.New()}, PredecessorExecutionCohortID: uuid.New(), PredecessorRealmIDs: []string{old.RealmID}, ReplacementRealmIDs: []string{newDomain.RealmID}, ReplacementCredentialCohortIDs: []uuid.UUID{newDomain.CredentialCohortID}, RetiredDomains: []retirementproof.DomainBinding{old}, ReplacementDomains: []retirementproof.DomainBinding{newDomain}}
	for _, kind := range []string{"GAME_PROVISION_AND_CONVERSION", "LEGACY_COMPATIBILITY", "MEDIA_AND_GRANT_PRODUCERS", "NATIVE_ORDINARY", "NATIVE_SPACE", "OWNED_ROOM", "SHARED_IDENTITY_FENCES"} {
		d.SourceBindings = append(d.SourceBindings, retirementproof.SourceBinding{SourceKind: kind, OwnerProtocol: "fixture-only", ContractHash: h(kind)})
	}
	r := retirementproof.ResourceRecord{}
	for _, kind := range []string{"ADMISSION_BARRIER", "CREDENTIAL_MANIFEST", "DOCKER_FENCE", "ISSUER_MANIFEST", "POSTGRES_FENCE", "PROVIDER_MANIFEST", "REDIS_FENCE", "SOURCE_CATALOG"} {
		raw := []byte(`{"fixture":"not real retirement"}`)
		switch kind {
		case "SOURCE_CATALOG":
			raw = encode(d.SourceBindings)
			d.Cut.SourceCatalogHash = retirementproof.Hash(retirementproof.CatalogDomain, raw)
		case "PROVIDER_MANIFEST":
			raw = encode(struct {
				Retired, Replacement []retirementproof.DomainBinding
			}{d.RetiredDomains, d.ReplacementDomains})
			d.Cut.ProviderManifestHash = retirementproof.Hash(retirementproof.ProviderDomain, raw)
		case "CREDENTIAL_MANIFEST":
			raw = encode(retirementproof.CredentialManifest{Protocol: 1})
			d.Cut.CredentialManifestHash = retirementproof.Hash(retirementproof.CredentialDomain, raw)
		case "ADMISSION_BARRIER":
			d.Cut.ClosedAdmissionBarrierHash = retirementproof.Hash(retirementproof.BarrierDomain, raw)
		case "ISSUER_MANIFEST":
			d.Cut.IssuerImplementationManifestHash = retirementproof.Hash(retirementproof.IssuerDomain, raw)
		}
		ref := retirementproof.ComponentRef{Kind: kind, Protocol: 1, CanonicalBytesHash: sha256.Sum256(raw), JournalRef: uuid.NewSHA1(d.ProofID, []byte(kind))}
		d.Components = append(d.Components, ref)
		r.Components = append(r.Components, retirementproof.ComponentRecord{Ref: ref, CanonicalBytes: raw})
	}
	r.Document = d
	return r
}

func TestResourceJournalConcurrentIdenticalAppendReadsWinner(t *testing.T) {
	ctx, c, schema := coverageDB(t)
	up, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "voice_db", "000016_party_resource_retirement.up.sql"))
	require.NoError(t, err)
	_, err = c.Exec(ctx, string(up))
	require.NoError(t, err)
	plan := cutPlan{OperationID: uuid.New(), Binding: fixtureBinding(), Scope: []byte(`{"fixture":"concurrency only"}`)}
	tx, err := c.Begin(ctx)
	require.NoError(t, err)
	plan.Binding.DatabaseIdentity, err = ObserveDatabaseIdentity(ctx, tx)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback(ctx))
	r := resourceStorageFixture(t, plan)
	plan.Binding.SourceCatalogHash = r.Document.Cut.SourceCatalogHash
	raw, err := json.Marshal(plan)
	require.NoError(t, err)
	r.Document.Cut.PlanHash = sha256.Sum256(raw)
	sealResourceFixture(t, &r)
	state, err := (journal{c}).prepare(ctx, plan)
	require.NoError(t, err)
	_, err = (journal{c}).advance(ctx, state, "FROZEN", []byte("fixture-only"))
	require.NoError(t, err)
	hold, err := c.Begin(ctx)
	require.NoError(t, err)
	defer hold.Rollback(ctx)
	_, err = hold.Exec(ctx, `SELECT 1 FROM voice_party_coverage_head WHERE singleton FOR UPDATE`)
	require.NoError(t, err)
	names := []string{"resource-append-a-" + uuid.NewString(), "resource-append-b-" + uuid.NewString()}
	results := make(chan error, 2)
	for _, name := range names {
		cfg := c.Config().Copy()
		cfg.RuntimeParams["application_name"] = name
		cfg.RuntimeParams["default_transaction_isolation"] = "repeatable read"
		conn, err := pgx.ConnectConfig(ctx, cfg)
		require.NoError(t, err)
		defer conn.Close(context.Background())
		go func() { results <- (resourceJournal{conn, schema}).append(ctx, r) }()
	}
	// Both requests have completed their initial absence read and are now behind
	// the exact head lock. No scheduling/timing guess can hide the conflict.
	require.Eventually(t, func() bool {
		_, err := hold.Exec(ctx, `SELECT pg_stat_clear_snapshot()`)
		if err != nil {
			return false
		}
		var n int
		err = hold.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE application_name=ANY($1::text[]) AND wait_event_type='Lock'`, names).Scan(&n)
		return err == nil && n == 2
	}, 5*time.Second, 10*time.Millisecond)
	require.NoError(t, hold.Commit(ctx))
	for range 2 {
		require.NoError(t, <-results, "same immutable request must read winner")
	}
	var n int
	require.NoError(t, c.QueryRow(ctx, `SELECT count(*) FROM voice_party_resource_retirements`).Scan(&n))
	require.Equal(t, 1, n)
}
func sealResourceFixture(t *testing.T, r *retirementproof.ResourceRecord) {
	raw, err := json.Marshal(r.Document)
	require.NoError(t, err)
	r.CanonicalBytes = raw
	r.Ref = retirementproof.ProofRef{Protocol: 1, ProofID: r.Document.ProofID, ReceiptHash: retirementproof.Hash(retirementproof.ResourceDomain, raw)}
}

func TestResourceJournalBindsFrozenPlanAndLostAcknowledgement(t *testing.T) {
	ctx, c, schema := coverageDB(t)
	up, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "voice_db", "000016_party_resource_retirement.up.sql"))
	require.NoError(t, err)
	_, err = c.Exec(ctx, string(up))
	require.NoError(t, err)
	plan := cutPlan{OperationID: uuid.New(), Binding: fixtureBinding(), Scope: []byte(`{"fixture":"journal only"}`)}
	tx, err := c.Begin(ctx)
	require.NoError(t, err)
	plan.Binding.DatabaseIdentity, err = ObserveDatabaseIdentity(ctx, tx)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback(ctx))
	r := resourceStorageFixture(t, plan)
	plan.Binding.SourceCatalogHash = r.Document.Cut.SourceCatalogHash
	r.Document.Cut.PlanHash = sha256.Sum256(func() []byte { b, e := json.Marshal(plan); require.NoError(t, e); return b }())
	sealResourceFixture(t, &r)
	coverage := journal{c}
	state, err := coverage.prepare(ctx, plan)
	require.NoError(t, err)
	store := resourceJournal{c, schema}
	require.Error(t, store.append(ctx, r), "prepared is not an enforced frozen cut")
	_, err = coverage.advance(ctx, state, "FROZEN", []byte("storage fixture only"))
	require.NoError(t, err)
	require.NoError(t, store.append(ctx, r))
	require.NoError(t, store.append(ctx, r), "lost ack must reuse exact proof/components")
	var count int
	require.NoError(t, c.QueryRow(ctx, `SELECT count(*) FROM voice_party_resource_retirements`).Scan(&count))
	require.Equal(t, 1, count)
	changed := r
	changed.Document.PredecessorExecutionCohortID = uuid.New()
	sealResourceFixture(t, &changed)
	require.ErrorIs(t, store.append(ctx, changed), retirementproof.ErrConflict)
	wrong := r
	wrong.Document.Cut.DatabaseIdentity[0]++
	sealResourceFixture(t, &wrong)
	require.Error(t, store.append(ctx, wrong))
	var phase string
	require.NoError(t, c.QueryRow(ctx, `SELECT phase FROM voice_party_coverage_head WHERE singleton`).Scan(&phase))
	require.Equal(t, "FROZEN", phase, "persisting proof never activates coverage")
}
