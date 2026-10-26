package authlifecycle

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	authv1 "voice.app/voice/auth/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/subscription/internal/currentauthority"
)

func ingressDB(t *testing.T) (*pgxpool.Pool, *pgxpool.Pool, string) {
	t.Helper()
	ctx := context.Background()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Join(filepath.Dir(file), "../../../migrations/subscription_db")
	admin := integrationtest.StartPostgres(t, ctx, "auth_lifecycle_ingress", "")
	for _, name := range []string{"000001_init", "000002_grace_reminders", "000003_space_lifecycle_provider_dedup", "000004_entitlement_outbox", "000005_provider_lifecycle", "000006_grace_reminder_outbox", "000007_authority_fence", "000008_kernel_writes", "000009_auth_lifecycle_ingress"} {
		body, err := os.ReadFile(filepath.Join(root, name+".up.sql"))
		require.NoError(t, err)
		_, err = admin.Exec(ctx, string(body))
		require.NoError(t, err)
	}
	_, err := admin.Exec(ctx, `CREATE ROLE old_runtime LOGIN PASSWORD 'fixture-only';CREATE ROLE kernel_writer LOGIN PASSWORD 'fixture-only';CREATE ROLE kernel_reader LOGIN PASSWORD 'fixture-only';
 GRANT USAGE ON SCHEMA public TO old_runtime;GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA public TO old_runtime`)
	require.NoError(t, err)
	generation := uuid.NewString()
	require.NoError(t, currentauthority.InstallFence(ctx, admin, currentauthority.FenceRequest{Generation: generation, SourceCommit: strings.Repeat("a", 40), WriterRole: "kernel_writer", ReaderRole: "kernel_reader", LegacyRoles: []string{"old_runtime"}}))
	cfg := admin.Config().Copy()
	cfg.ConnConfig.User = "kernel_writer"
	cfg.ConnConfig.Password = "fixture-only"
	writer, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(writer.Close)
	return admin, writer, generation
}

// These private fixtures cover local transaction semantics, never live Auth
// provenance. native_test.go supplies the actual Java/TLS/JWKS/Redis observation.
func observationFixture(t *testing.T, generation string, snapshot *authv1.ObserveSubscriptionAccountResponse) *Observation {
	t.Helper()
	snapshot = proto.Clone(snapshot).(*authv1.ObserveSubscriptionAccountResponse)
	snapshot.ObservedAt = timestamppb.Now()
	body, err := proto.MarshalOptions{Deterministic: true}.Marshal(snapshot)
	require.NoError(t, err)
	request := &authv1.ObserveSubscriptionAccountRequest{AccountId: snapshot.AccountId, MinimumLifecycleRevision: 1}
	requestBody, err := proto.MarshalOptions{Deterministic: true}.Marshal(request)
	require.NoError(t, err)
	return &Observation{generation: generation, account: snapshot.AccountId, requestID: uuid.NewString(), requestHash: sha256.Sum256(requestBody), snapshotHash: sha256.Sum256(body), snapshot: snapshot, payload: body, deadline: time.Now().Add(500 * time.Millisecond), minimum: 1}
}

func restoreFixture(snapshot *authv1.ObserveSubscriptionAccountResponse) *authv1.ObserveSubscriptionAccountResponse {
	s := proto.Clone(snapshot).(*authv1.ObserveSubscriptionAccountResponse)
	fact := proto.Clone(s.Head).(*authv1.SubscriptionAccountLifecycleFact)
	fact.SourceRevision = 2
	fact.EventId = uuid.NewString()
	fact.Kind = authv1.SubscriptionAccountLifecycleKind_SUBSCRIPTION_ACCOUNT_LIFECYCLE_KIND_RESTORE_REQUESTED
	fact.EffectiveAt = timestamppb.Now()
	fact.CapturedAt = fact.EffectiveAt
	s.ObservedAt = fact.EffectiveAt
	s.Facts = append(s.Facts, fact)
	s.Head = fact
	s.AccountStatus = "active"
	s.DeletedAt = nil
	s.SecurityRevision++
	return s
}

func TestInputIngestionIsOrderedReplaySafeAndNeverCurrent(t *testing.T) {
	admin, writer, generation := ingressDB(t)
	ctx := context.Background()
	original := captureFixture()
	first := observationFixture(t, generation, original)
	require.NoError(t, Ingest(ctx, writer, first))
	require.NoError(t, Ingest(ctx, writer, first))
	second := observationFixture(t, generation, restoreFixture(original))
	require.NoError(t, Ingest(ctx, writer, second))
	var revision, count int
	var state string
	require.NoError(t, admin.QueryRow(ctx, "SELECT auth_revision,state FROM subscription_auth_account_observations").Scan(&revision, &state))
	require.Equal(t, 2, revision)
	require.Equal(t, "OBSERVED", state)
	require.NoError(t, admin.QueryRow(ctx, "SELECT count(*) FROM subscription_auth_lifecycle_facts").Scan(&count))
	require.Equal(t, 2, count)
	require.NoError(t, admin.QueryRow(ctx, "SELECT count(*) FROM subscription_auth_observation_receipts").Scan(&count))
	require.Equal(t, 2, count)
	require.ErrorIs(t, Ingest(ctx, writer, observationFixture(t, generation, original)), ErrSourceDrift, "fresh transport does not authorize source revision rollback")
	_, err := writer.Exec(ctx, "UPDATE subscription_auth_account_observations SET state='CURRENT'")
	require.Error(t, err, "an observed input has no CURRENT state")
}

func TestInputReceiptFailureRollsBackHeadFactsAndReceipt(t *testing.T) {
	admin, writer, generation := ingressDB(t)
	ctx := context.Background()
	_, err := admin.Exec(ctx, `CREATE FUNCTION fixture_reject_input() RETURNS trigger LANGUAGE plpgsql AS $$BEGIN RAISE EXCEPTION 'fixture abort';END$$;
 CREATE TRIGGER fixture_reject_input BEFORE INSERT ON subscription_auth_observation_receipts FOR EACH ROW EXECUTE FUNCTION fixture_reject_input()`)
	require.NoError(t, err)
	require.Error(t, Ingest(ctx, writer, observationFixture(t, generation, captureFixture())))
	for _, table := range []string{"subscription_auth_account_observations", "subscription_auth_lifecycle_facts", "subscription_auth_observation_receipts"} {
		var count int
		require.NoError(t, admin.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&count))
		require.Zero(t, count, table)
	}
}

func TestInputRejectsSameRevisionDriftExpiredAndWrongGeneration(t *testing.T) {
	_, writer, generation := ingressDB(t)
	ctx := context.Background()
	original := captureFixture()
	require.NoError(t, Ingest(ctx, writer, observationFixture(t, generation, original)))
	changed := proto.Clone(original).(*authv1.ObserveSubscriptionAccountResponse)
	changed.RegularEmailVerificationPending = true
	require.ErrorIs(t, Ingest(ctx, writer, observationFixture(t, generation, changed)), ErrSourceDrift)
	expired := observationFixture(t, generation, original)
	expired.deadline = time.Now().Add(-time.Second)
	require.ErrorIs(t, Ingest(ctx, writer, expired), ErrUnavailable)
	require.ErrorIs(t, Ingest(ctx, writer, observationFixture(t, uuid.NewString(), original)), currentauthority.ErrFenceConflict)
}
