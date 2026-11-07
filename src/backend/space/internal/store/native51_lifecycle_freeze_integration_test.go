//go:build spacebanintegration

package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/memberadmission"
)

// The controller provisions and qualifies native51 before this test. The
// source, native catalog, guard functions and DB-issued frames are real. The
// typed Auth receipt is a controlled store-bound fixture, not a network proof.
func TestNative51FreezeRequiresBoundProofAndPreservesMembership(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("SPACE_BAN_TEST_DATABASE_URL")
	require.NotEmpty(t, dsn)
	admin, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(admin.Close)
	facts, err := (&SpaceStore{Pool: admin}).InspectNativeInstallerSource(ctx, 51)
	require.NoError(t, err)
	require.EqualValues(t, 51, facts.Version)
	st := &SpaceStore{Pool: projectionServingLogin(t, ctx, admin)}
	create, account, actor := signedProjectionCreator(t, st, &spacev1.CreateSpaceRequest{Name: "R23 durable lifecycle", Visibility: "public"})
	space, err := create()
	require.NoError(t, err)
	member := uuid.New()
	intent, err := st.PrepareMemberAdmission(ctx, MemberAdmissionInput{OperationID: uuid.New(), SpaceID: space.ID, ProfileID: member, AccountID: uuid.New()})
	require.NoError(t, err)
	role := uuid.New()
	receipt, err := memberadmission.EnsureReceiptFingerprint(intent.EnsureBinding(), role.String())
	require.NoError(t, err)
	_, err = st.PublishMemberAdmission(ctx, intent.Input.OperationID, role, receipt[:], nil)
	require.NoError(t, err, "real native publication with controlled Role receipt")
	operation := uuid.New()
	request := lifecycleScheduleRequest(space.ID, operation)
	_, err = st.ReserveLifecycleSchedule(ctx, account, actor, 7, request)
	require.NoError(t, err)
	snapshot := func() string {
		var value string
		require.NoError(t, admin.QueryRow(ctx, `SELECT jsonb_build_object(
 'aggregate',(SELECT to_jsonb(a) FROM space_lifecycle_aggregates a WHERE space_id=$1),
 'operation',(SELECT to_jsonb(o) FROM space_lifecycle_operations o WHERE operation_id=$2),
 'heads',(SELECT jsonb_agg(to_jsonb(h) ORDER BY profile_id) FROM space_effective_member_heads h WHERE space_id=$1),
 'ordinary_heads',(SELECT jsonb_agg(to_jsonb(h) ORDER BY profile_id) FROM space_ordinary_member_heads h WHERE space_id=$1),
 'members',(SELECT jsonb_agg(to_jsonb(m) ORDER BY profile_id) FROM space_members m WHERE space_id=$1),
 'participants',(SELECT jsonb_agg(to_jsonb(p) ORDER BY generation,participant_id,request_kind) FROM space_lifecycle_participants p WHERE space_id=$1),
 'member_outbox',(SELECT jsonb_agg(to_jsonb(o) ORDER BY event_id) FROM space_member_admission_outbox o WHERE space_id=$1),
 'lifecycle_outbox',(SELECT jsonb_agg(to_jsonb(o) ORDER BY event_id) FROM space_lifecycle_outbox o WHERE space_id=$1),
 'source_revision',(SELECT to_jsonb(e) FROM space_voice_access_epochs e WHERE space_id=$1),
 'registry',(SELECT to_jsonb(g) FROM native_membership_guard.spaces g WHERE space_id=$1),
 'frames',(SELECT count(*) FROM native_membership_guard.frames WHERE space_id=$1))::text`, space.ID, operation).Scan(&value))
		return value
	}
	before := snapshot()
	var generation, revision int64
	var episode uuid.UUID
	require.NoError(t, admin.QueryRow(ctx, `SELECT generation,ledger_revision,episode_id FROM space_effective_member_heads WHERE space_id=$1 AND profile_id=$2`, space.ID, actor).Scan(&generation, &revision, &episode))
	lineage := func() string {
		var value string
		require.NoError(t, admin.QueryRow(ctx, `SELECT jsonb_agg(jsonb_build_object('profile',profile_id,'generation',generation,'episode',episode_id,'state',state,'ordinary',ordinary_witness) ORDER BY profile_id)::text FROM space_effective_member_heads WHERE space_id=$1`, space.ID).Scan(&value))
		return value
	}
	beforeLineage := lineage()
	var count int
	require.NoError(t, admin.QueryRow(ctx, `SELECT count(*) FROM space_effective_member_heads WHERE space_id=$1`, space.ID).Scan(&count))
	require.Equal(t, 2, count)
	_, err = st.BeginLifecycleFreeze(ctx, space.ID, lifecycleManifestFixture())
	require.ErrorIs(t, err, ErrLifecycleConflict, "freeze cannot precede its bound proof")
	require.Equal(t, before, snapshot())
	_, err = st.Pool.Exec(ctx, `UPDATE space_lifecycle_aggregates SET phase='FREEZE_PENDING' WHERE space_id=$1`, space.ID)
	var pgerr *pgconn.PgError
	require.ErrorAs(t, err, &pgerr, "direct SQL cannot acquire lifecycle transition authority")
	require.Equal(t, "55000", pgerr.Code)
	require.Equal(t, before, snapshot())
	var phase string
	require.NoError(t, admin.QueryRow(ctx, `SELECT phase FROM space_lifecycle_aggregates WHERE space_id=$1`, space.ID).Scan(&phase))
	require.Equal(t, "SCHEDULE_PENDING", phase)
	_, confirmation, proofDigest := lifecycleScheduleRequestEvidence(t, request)
	wrongProof, wrongHash := lifecycleAuthReceiptEvidence(t, uuid.New(), actor, space.ID, operation, 7, confirmation, proofDigest)
	_, err = st.RecordLifecycleDeletionProofReceipt(ctx, actor, operation, wrongProof, wrongHash)
	require.Error(t, err, "another account's bound proof cannot authorize this operation")
	require.Equal(t, before, snapshot())
	proof, hash := lifecycleAuthReceiptEvidence(t, account, actor, space.ID, operation, 7, confirmation, proofDigest)
	_, err = st.RecordLifecycleDeletionProofReceipt(ctx, actor, operation, proof, hash)
	require.NoError(t, err)
	bound := snapshot()
	altered := append([]byte(nil), proof...)
	altered[len(altered)-1] ^= 1
	_, err = st.RecordLifecycleDeletionProofReceipt(ctx, actor, operation, altered, lifecycleDomainHash("voice.auth.v1.ConsumeSpaceDeletionProofResponse", altered))
	require.Error(t, err, "altered receipt replay cannot replace the bound proof")
	require.Equal(t, bound, snapshot())
	// Permit reads but block the final head UPDATE. Once PostgreSQL reports
	// that wait, the guarded aggregate update already passed its frame check.
	blocker, err := admin.Begin(ctx)
	require.NoError(t, err)
	defer blocker.Rollback(context.Background())
	_, err = blocker.Exec(ctx, `LOCK TABLE space_effective_member_heads IN SHARE MODE`)
	require.NoError(t, err)
	cfg := st.Pool.Config().Copy()
	app := "freeze-rollback-" + uuid.NewString()
	cfg.ConnConfig.RuntimeParams["application_name"] = app
	worker, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(worker.Close)
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, callErr := (&SpaceStore{Pool: worker}).BeginLifecycleFreeze(workerCtx, space.ID, lifecycleManifestFixture())
		done <- callErr
	}()
	deadline := time.Now().Add(10 * time.Second)
	blocked := false
	for time.Now().Before(deadline) {
		select {
		case callErr := <-done:
			t.Fatalf("freeze returned before head write could be blocked: %v", callErr)
		default:
		}
		require.NoError(t, admin.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE application_name=$1 AND wait_event_type='Lock' AND query LIKE '%UPDATE space_effective_member_heads%')`, app).Scan(&blocked))
		if blocked {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	require.True(t, blocked, "freeze must reach the blocked head write after framing")
	cancel()
	select {
	case callErr := <-done:
		require.Error(t, callErr)
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled freeze did not return")
	}
	require.NoError(t, blocker.Rollback(ctx))
	require.Equal(t, bound, snapshot(), "cancelled framed transition must roll back all durable effects")
	_, err = st.BeginLifecycleFreeze(ctx, space.ID, lifecycleManifestFixture())
	require.NoError(t, err)
	require.NoError(t, admin.QueryRow(ctx, `SELECT phase FROM space_lifecycle_aggregates WHERE space_id=$1`, space.ID).Scan(&phase))
	require.Equal(t, "FREEZE_PENDING", phase)
	var afterGeneration, afterRevision int64
	var afterEpisode uuid.UUID
	require.NoError(t, admin.QueryRow(ctx, `SELECT generation,ledger_revision,episode_id FROM space_effective_member_heads WHERE space_id=$1 AND profile_id=$2`, space.ID, actor).Scan(&afterGeneration, &afterRevision, &afterEpisode))
	require.Equal(t, generation, afterGeneration)
	require.Equal(t, episode, afterEpisode)
	require.GreaterOrEqual(t, afterRevision, revision)
	require.Equal(t, beforeLineage, lineage(), "freeze must preserve the full member cohort's effective lineages")
	frozen := snapshot()
	_, err = st.BeginLifecycleFreeze(ctx, space.ID, lifecycleManifestFixture())
	require.NoError(t, err, "exact replay must remain read-only")
	require.Equal(t, frozen, snapshot())
	badManifest := lifecycleManifestFixture()
	badManifest.ManifestId = uuid.NewString()
	_, err = st.BeginLifecycleFreeze(ctx, space.ID, badManifest)
	require.Error(t, err, "a changed manifest cannot replace the saved one")
	require.Equal(t, frozen, snapshot())
	var frames int
	require.NoError(t, admin.QueryRow(ctx, `SELECT count(*) FROM native_membership_guard.frames`).Scan(&frames))
	require.Zero(t, frames, "every committed or rejected attempt leaves no frame")
}
