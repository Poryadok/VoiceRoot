package partysearch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/matchmaking/internal/mmadmission"
	"voice/backend/matchmaking/internal/partysource"
	"voice/backend/matchmaking/internal/store"
)

func TestNativeAdmissionLegacyCreatesRefuseBegunHead(t *testing.T) {
	ctx := context.Background()
	pool := partyDB(t)
	p := partyParams(t, pool)
	legacyParty := func() error {
		binding := admissionFixture()
		_, err := (&store.PartyStore{Pool: pool, NativeAdmission: &binding}).Create(ctx, store.CreatePartyParams{LeaderProfileID: uuid.New(), GameID: p.GameID, Mode: p.Mode, Criteria: p.Criteria})
		return err
	}
	legacySession := func() error {
		_, err := (&store.SessionStore{Pool: pool}).Create(ctx, store.CreateSessionParams{ProfileID: uuid.New(), GameID: p.GameID, Mode: p.Mode, Criteria: p.Criteria, TimeoutAt: p.TimeoutAt})
		return err
	}
	require.NoError(t, legacyParty())
	require.NoError(t, legacySession())
	for _, phase := range []string{"CLOSED", "PREPARED", "ADMITTED"} {
		t.Run(phase, func(t *testing.T) {
			_, err := pool.Exec(ctx, `DELETE FROM public.mm_native_admission_head`)
			require.NoError(t, err)
			if phase == "ADMITTED" {
				insertAdmissionFixture(t, pool, admissionFixture())
			} else {
				_, err = pool.Exec(ctx, `INSERT INTO public.mm_native_admission_head(protocol_version,state) VALUES(1,$1)`, phase)
				require.NoError(t, err)
			}
			require.ErrorIs(t, legacyParty(), mmadmission.ErrUnavailable)
			require.ErrorIs(t, legacySession(), mmadmission.ErrUnavailable)
			var parties, sessions int
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM parties`).Scan(&parties))
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM search_sessions`).Scan(&sessions))
			require.Equal(t, 1, parties)
			require.Equal(t, 1, sessions)
		})
	}
}

// This deliberately synthetic local-head fixture tests only transaction
// serialization/equality. It is NOT an installed MM admission/custody receipt.
func admissionFixture() mmadmission.Binding {
	return mmadmission.Binding{ProtocolVersion: 1, EnvironmentID: uuid.New(), InstallationID: uuid.New(), AuthorityIncarnation: uuid.New(), TransitionID: uuid.New(), AdmissionGeneration: 1, CredentialCohortID: uuid.New(), DatabaseBindingHash: [32]byte{1}, SchemaContractHash: [32]byte{2}, SourceFactoryManifestHash: [32]byte{3}, FreshRouteManifestHash: [32]byte{4}, AdmissionReceiptHash: [32]byte{5}, LaunchManifestHash: [32]byte{6}, ExpectedLoginRoleOID: 42, ExpectedLoginRoleName: "fixture-not-installed"}
}

func insertAdmissionFixture(t *testing.T, pool *pgxpool.Pool, b mmadmission.Binding) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `INSERT INTO public.mm_native_admission_head(protocol_version,state,environment_id,installation_id,authority_incarnation,transition_id,admission_generation,credential_cohort_id,database_binding_sha256,schema_contract_sha256,source_factory_manifest_sha256,fresh_route_manifest_sha256,admission_receipt_sha256,launch_manifest_sha256,expected_login_role_oid,expected_login_role_name) VALUES(1,'ADMITTED',$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`, b.EnvironmentID, b.InstallationID, b.AuthorityIncarnation, b.TransitionID, b.AdmissionGeneration, b.CredentialCohortID, b.DatabaseBindingHash[:], b.SchemaContractHash[:], b.SourceFactoryManifestHash[:], b.FreshRouteManifestHash[:], b.AdmissionReceiptHash[:], b.LaunchManifestHash[:], b.ExpectedLoginRoleOID, b.ExpectedLoginRoleName)
	require.NoError(t, err)
}

func TestNativeAdmissionGuardRequiresExactHeadAndCloseSerialization(t *testing.T) {
	ctx := context.Background()
	pool := partyDB(t)
	binding := admissionFixture()
	acquire := func(expected *mmadmission.Binding) error {
		tx, err := pool.Begin(ctx)
		require.NoError(t, err)
		defer tx.Rollback(ctx)
		err = mmadmission.Acquire(ctx, tx, expected)
		if err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	require.NoError(t, acquire(nil), "never-begun pre-cut table alone is not admission")
	require.ErrorIs(t, acquire(&binding), mmadmission.ErrUnavailable)
	_, err := pool.Exec(ctx, `INSERT INTO public.mm_native_admission_head(protocol_version,state) VALUES(1,'CLOSED')`)
	require.NoError(t, err)
	require.ErrorIs(t, acquire(nil), mmadmission.ErrUnavailable)
	require.ErrorIs(t, acquire(&binding), mmadmission.ErrUnavailable)
	_, err = pool.Exec(ctx, `UPDATE public.mm_native_admission_head SET state='PREPARED'`)
	require.NoError(t, err)
	require.ErrorIs(t, acquire(nil), mmadmission.ErrUnavailable)
	_, err = pool.Exec(ctx, `DELETE FROM public.mm_native_admission_head`)
	require.NoError(t, err)
	insertAdmissionFixture(t, pool, binding)
	require.ErrorIs(t, acquire(nil), mmadmission.ErrUnavailable)
	require.NoError(t, acquire(&binding))
	wrong := binding
	wrong.FreshRouteManifestHash[0]++
	require.ErrorIs(t, acquire(&wrong), mmadmission.ErrUnavailable)
	wrong = binding
	wrong.AdmissionGeneration++
	require.ErrorIs(t, acquire(&wrong), mmadmission.ErrUnavailable)
	wrong = binding
	wrong.CredentialCohortID = uuid.New()
	require.ErrorIs(t, acquire(&wrong), mmadmission.ErrUnavailable)
	wrong = binding
	wrong.ExpectedLoginRoleOID++
	require.ErrorIs(t, acquire(&wrong), mmadmission.ErrUnavailable)
	rr, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	require.NoError(t, err)
	require.ErrorIs(t, mmadmission.Acquire(ctx, rr, &binding), mmadmission.ErrUnavailable)
	require.NoError(t, rr.Rollback(ctx))

	active, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer active.Rollback(ctx)
	require.NoError(t, mmadmission.Acquire(ctx, active, &binding))
	closeStarted := make(chan struct{})
	closed := make(chan error, 1)
	go func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		tx, err := pool.Begin(closeCtx)
		if err != nil {
			closed <- err
			return
		}
		defer tx.Rollback(context.Background())
		close(closeStarted)
		if err := mmadmission.AcquireCloseGate(closeCtx, tx); err != nil {
			closed <- err
			return
		}
		if _, err := tx.Exec(closeCtx, `UPDATE public.mm_native_admission_head SET state='CLOSED'`); err != nil {
			closed <- err
			return
		}
		closed <- tx.Commit(closeCtx)
	}()
	<-closeStarted
	require.Eventually(t, func() bool {
		select {
		case err := <-closed:
			t.Fatalf("close bypassed active admission: %v", err)
		default:
		}
		var waiting bool
		err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%pg_advisory_xact_lock(%')`).Scan(&waiting)
		return err == nil && waiting
	}, 3*time.Second, 10*time.Millisecond)
	require.NoError(t, active.Commit(ctx))
	require.NoError(t, <-closed)
	require.ErrorIs(t, acquire(&binding), mmadmission.ErrUnavailable)
	down, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "matchmaking_db", "000024_mm_native_admission.down.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(down))
	require.Error(t, err, "begun marker cannot be erased by DOWN")
}

func TestNativeAdmissionBegunHeadRefusesUnboundPositiveWriters(t *testing.T) {
	pool := partyDB(t)
	ctx := context.Background()
	f := prepareNativeProposal(t, pool)
	_, err := pool.Exec(ctx, `INSERT INTO public.mm_native_admission_head(protocol_version,state) VALUES(1,'CLOSED')`)
	require.NoError(t, err)
	t.Run("source enqueue", func(t *testing.T) {
		_, err := (&store.PartyStore{Pool: pool}).CreateSourceParty(ctx, partyParams(t, pool))
		require.ErrorIs(t, err, mmadmission.ErrUnavailable)
	})
	t.Run("proposal reservation", func(t *testing.T) {
		_, err := (&store.MatchStore{Pool: pool}).CreateValidatedProposal(ctx, f.candidate, f.proof)
		require.ErrorIs(t, err, mmadmission.ErrUnavailable)
	})
	requireProposalStillSearching(t, pool, f.search)
}

func TestNativeAdmissionBoundWriterRechecksAfterActualCloseWait(t *testing.T) {
	ctx := context.Background()
	pool := partyDB(t)
	binding := admissionFixture()
	insertAdmissionFixture(t, pool, binding)
	parties := &store.PartyStore{Pool: pool, NativeAdmission: &binding}
	matches := &store.MatchStore{Pool: pool, NativeAdmission: &binding}
	create := func() (store.ProposalCandidate, *partysource.ValidatedProposalAttempt, store.SourcePartySearch) {
		p := partyParams(t, pool)
		created, err := parties.CreateSourceParty(ctx, p)
		require.NoError(t, err)
		candidate, err := matches.CaptureProposalCandidate(ctx, partyProposal(p, created.Sessions))
		require.NoError(t, err)
		return candidate, validateNativeProposal(t, pool, candidate), created
	}
	positive, proof, _ := create()
	_, err := matches.CreateValidatedProposal(ctx, positive, proof)
	require.NoError(t, err, "synthetic local binding only: this does not qualify installed serving")
	candidate, immediate, searching := create()
	closer, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer closer.Rollback(ctx)
	require.NoError(t, mmadmission.AcquireCloseGate(ctx, closer))
	_, err = closer.Exec(ctx, `UPDATE public.mm_native_admission_head SET state='CLOSED'`)
	require.NoError(t, err)
	finished := make(chan error, 1)
	go func() {
		bounded, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, err := matches.CreateValidatedProposal(bounded, candidate, immediate)
		finished <- err
	}()
	require.Eventually(t, func() bool {
		select {
		case err := <-finished:
			t.Fatalf("writer bypassed actual close gate: %v", err)
		default:
		}
		var waiting bool
		err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%pg_advisory_xact_lock_shared(%')`).Scan(&waiting)
		return err == nil && waiting
	}, 3*time.Second, 10*time.Millisecond)
	require.NoError(t, closer.Commit(ctx))
	require.ErrorIs(t, <-finished, mmadmission.ErrUnavailable)
	requireProposalStillSearching(t, pool, searching)
	_, err = parties.CreateSourceParty(ctx, partyParams(t, pool))
	require.ErrorIs(t, err, mmadmission.ErrUnavailable)
}
