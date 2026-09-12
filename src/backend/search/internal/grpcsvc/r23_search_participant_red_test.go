package grpcsvc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	searchv1 "voice.app/voice/search/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
	"voice/backend/search/internal/store"
)

// These are intentionally RED contract tests for Search participant 7. The
// production roots do not exist on the R23 contract base: migration 000003,
// durable store, trusted principal boundary, lifecycle RPC implementation and
// startup recovery. Do not weaken the assertions while adding those roots.
func TestR23SearchParticipantREDManifest(t *testing.T) {
	want := []string{
		"trusted Space-only lifecycle boundary",
		"durable FROZEN/LIVE fence with same-generation replay and conflict",
		"database-time schedule generation and lower-generation stale no-op",
		"purge cleanup of Space/chat/message projections in every active index",
		"restart replay, 30-day full evidence retention and permanent PURGED fence",
		"guarded DOWN refuses durable lifecycle evidence and empty DOWN succeeds",
	}
	require.Len(t, want, 6)
	require.Equal(t, commonv1.ParticipantId_PARTICIPANT_ID_SEARCH, searchParticipantID)
}

func TestR23SearchParticipant_MigrationRootsAndGuardedDown(t *testing.T) {
	root := r23SearchRepoRoot(t)
	upPath := filepath.Join(root, "src", "backend", "migrations", "search_db", "000003_space_lifecycle.up.sql")
	downPath := filepath.Join(root, "src", "backend", "migrations", "search_db", "000003_space_lifecycle.down.sql")

	up, err := os.ReadFile(upPath)
	require.NoError(t, err, "R23 requires the additive Search lifecycle migration")
	for _, invariant := range []string{
		"FROZEN",
		"PURGE_DECIDED",
		"PURGED",
		"request_bytes",
		"receipt_bytes",
		"30 days",
	} {
		require.Contains(t, string(up), invariant, "migration must persist %s", invariant)
	}
	for _, projection := range []string{
		"message_search_documents",
		"chat_search_documents",
		"space_search_documents",
	} {
		require.Contains(t, string(up), projection, "purge must durably clean %s", projection)
	}

	down, err := os.ReadFile(downPath)
	require.NoError(t, err, "R23 requires a reversible-only-when-empty guarded DOWN")
	require.Contains(t, string(down), "RAISE EXCEPTION", "DOWN must refuse durable lifecycle evidence")
	require.Contains(t, string(down), "lifecycle", "DOWN guard must name its durable lifecycle evidence")
	for _, evidence := range []string{
		"search_space_lifecycle_operations",
		"search_space_lifecycle_receipts",
		"search_space_purge_receipts",
		"search_space_chat_manifest_pages",
	} {
		require.Contains(t, string(down), evidence, "DOWN must independently inspect %s evidence", evidence)
	}
}

func TestR23SearchParticipant_TrustedBoundaryRejectsBeforeLifecycleMutation(t *testing.T) {
	fixture := startR23SearchFixture(t)
	svc := fixture.first
	request := r23SearchFenceRequest(t, 7, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN)
	before := r23SearchMutationSnapshot(t, fixture)

	// The only accepted caller is the authenticated Space workload. A raw,
	// missing, wrong-service, wrong-method, wrong-audience or changed-body
	// caller must be rejected before reaching the lifecycle handler. The exact
	// principal transport is shared R23 infrastructure; this participant test
	// freezes the Search-side boundary rather than accepting generated stubs.
	contexts := map[string]context.Context{
		"missing":            context.Background(),
		"raw":                metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-service-id", "space")),
		"wrong-issuer":       r23SearchPrincipalContext(t, request, searchv1.SearchService_ApplySpaceLifecycleFence_FullMethodName, "chat", "service:chat", "search", ""),
		"wrong-subject":      r23SearchPrincipalContext(t, request, searchv1.SearchService_ApplySpaceLifecycleFence_FullMethodName, "space", "service:chat", "search", ""),
		"wrong-audience":     r23SearchPrincipalContext(t, request, searchv1.SearchService_ApplySpaceLifecycleFence_FullMethodName, "space", "service:space", "chat", ""),
		"wrong-rpc":          r23SearchPrincipalContext(t, request, searchv1.SearchService_PurgeSpace_FullMethodName, "space", "service:space", "search", ""),
		"wrong-request-hash": r23SearchPrincipalContext(t, request, searchv1.SearchService_ApplySpaceLifecycleFence_FullMethodName, "space", "service:space", "search", "sha256:"+strings.Repeat("0", 64)),
	}
	for name, callCtx := range contexts {
		t.Run(name, func(t *testing.T) {
			_, err := svc.ApplySpaceLifecycleFence(callCtx, request)
			require.NotEqual(t, codes.Unimplemented, status.Code(err), "%s must not fall through to the generated lifecycle stub", name)
			require.Contains(t, []codes.Code{codes.PermissionDenied, codes.Unauthenticated}, status.Code(err), "%s must be denied at the trusted participant boundary", name)
		})
	}
	t.Run("service-token-with-user-claims", func(t *testing.T) {
		callCtx := principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "space", Subject: "service:space", Audience: "search", RPC: searchv1.SearchService_ApplySpaceLifecycleFence_FullMethodName, RequestID: uuid.NewString(), RequestHash: r23SearchRequestHash(t, request), AccountID: uuid.NewString(), ProfileID: uuid.NewString(), SessionEpoch: 1})
		_, err := svc.ApplySpaceLifecycleFence(callCtx, request)
		require.Equal(t, codes.PermissionDenied, status.Code(err))
	})
	t.Run("changed-body", func(t *testing.T) {
		changed := proto.Clone(request).(*searchv1.ApplySpaceLifecycleFenceRequest)
		changed.Fence.Manifest.ManifestId = "tampered-root"
		_, err := svc.ApplySpaceLifecycleFence(r23SearchTrustedSpaceContext(t, request, searchv1.SearchService_ApplySpaceLifecycleFence_FullMethodName), changed)
		require.Contains(t, []codes.Code{codes.PermissionDenied, codes.Unauthenticated}, status.Code(err), "a principal bound to the original bytes cannot authorize a changed body")
	})
	require.Equal(t, before, r23SearchMutationSnapshot(t, fixture), "rejected Apply callers cannot mutate a lifecycle or projection row")
}

func TestR23SearchParticipant_PurgeTrustedBoundaryRejectsBeforeLifecycleMutation(t *testing.T) {
	fixture := startR23SearchFixture(t)
	svc := fixture.first
	request := r23SearchPurgeRequest()
	before := r23SearchMutationSnapshot(t, fixture)
	contexts := map[string]context.Context{
		"missing":            context.Background(),
		"raw":                metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-service-id", "space")),
		"wrong-issuer":       r23SearchPrincipalContext(t, request, searchv1.SearchService_PurgeSpace_FullMethodName, "chat", "service:chat", "search", ""),
		"wrong-subject":      r23SearchPrincipalContext(t, request, searchv1.SearchService_PurgeSpace_FullMethodName, "space", "service:chat", "search", ""),
		"wrong-audience":     r23SearchPrincipalContext(t, request, searchv1.SearchService_PurgeSpace_FullMethodName, "space", "service:space", "chat", ""),
		"wrong-rpc":          r23SearchPrincipalContext(t, request, searchv1.SearchService_ApplySpaceLifecycleFence_FullMethodName, "space", "service:space", "search", ""),
		"wrong-request-hash": r23SearchPrincipalContext(t, request, searchv1.SearchService_PurgeSpace_FullMethodName, "space", "service:space", "search", "sha256:"+strings.Repeat("0", 64)),
	}
	for name, callCtx := range contexts {
		t.Run(name, func(t *testing.T) {
			_, err := svc.PurgeSpace(callCtx, request)
			require.NotEqual(t, codes.Unimplemented, status.Code(err), "%s must not reach the generated lifecycle stub", name)
			require.Contains(t, []codes.Code{codes.PermissionDenied, codes.Unauthenticated}, status.Code(err), "%s must be denied before mutation", name)
		})
	}
	t.Run("changed-body", func(t *testing.T) {
		changed := proto.Clone(request).(*searchv1.PurgeSpaceRequest)
		changed.Purge.Manifest.ManifestId = "tampered-root"
		_, err := svc.PurgeSpace(r23SearchTrustedSpaceContext(t, request, searchv1.SearchService_PurgeSpace_FullMethodName), changed)
		require.Contains(t, []codes.Code{codes.PermissionDenied, codes.Unauthenticated}, status.Code(err), "a principal bound to the original bytes cannot authorize a changed body")
	})
	require.Equal(t, before, r23SearchMutationSnapshot(t, fixture), "rejected Purge callers cannot mutate a lifecycle or projection row")
}

func TestR23SearchParticipant_MigrationUpDownEvidenceRetentionAndTOCTOU(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	root := r23SearchRepoRoot(t)
	base := filepath.Join(root, "src", "backend", "migrations", "search_db", "000001_init.up.sql")
	pool := integrationtest.StartPostgres(t, ctx, "search_r23", base)
	integrationtest.ApplySQLFile(t, ctx, pool, root, filepath.Join("src", "backend", "migrations", "search_db", "000002_verification_type.up.sql"))

	// The real fixture deliberately loads the absent R23 root. Once GREEN adds
	// it, this test executes UP, verifies empty DOWN, then reapplies UP to prove
	// the guard is safe under migration retry. The following named evidence is
	// required so later subtests can exercise retention and lock-before-check.
	integrationtest.ApplySQLFile(t, ctx, pool, root, filepath.Join("src", "backend", "migrations", "search_db", "000003_space_lifecycle.up.sql"))
	for _, table := range []string{"search_space_lifecycle_fences", "search_space_lifecycle_operations", "search_space_lifecycle_receipts", "search_space_purge_receipts"} {
		var found *string
		require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass($1)::text`, table).Scan(&found))
		require.NotNil(t, found, "UP must create durable %s", table)
	}

	// Empty DOWN succeeds, but an evidence row makes DOWN wait on the same
	// durable lock and refuse. The permanent PURGED row is the proof that a
	// rollback never reopens identifier reuse after terminal cleanup.
	integrationtest.ApplySQLFile(t, ctx, pool, root, filepath.Join("src", "backend", "migrations", "search_db", "000003_space_lifecycle.down.sql"))
	integrationtest.ApplySQLFile(t, ctx, pool, root, filepath.Join("src", "backend", "migrations", "search_db", "000003_space_lifecycle.up.sql"))
	_, err := pool.Exec(ctx, `INSERT INTO search_space_lifecycle_fences(space_id,generation,state,deletion_operation_id,updated_at)
VALUES($1,8,'PURGED',$2,clock_timestamp())`, r23SearchSpaceID, r23SearchDeletionID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, r23SearchMigrationSQL(t, "000003_space_lifecycle.down.sql"))
	require.Error(t, err, "guarded DOWN must refuse a permanent terminal fence")
	var state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM search_space_lifecycle_fences WHERE space_id=$1`, r23SearchSpaceID).Scan(&state))
	require.Equal(t, "PURGED", state)
	require.NoError(t, func() error {
		_, deleteErr := pool.Exec(ctx, `DELETE FROM search_space_lifecycle_fences WHERE space_id=$1`, r23SearchSpaceID)
		return deleteErr
	}())

	writer, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = writer.Rollback(context.Background()) }()
	toctouSpace, toctouOperation := uuid.New(), uuid.New()
	_, err = writer.Exec(ctx, `INSERT INTO search_space_lifecycle_fences(space_id,generation,state,deletion_operation_id,updated_at)
VALUES($1,8,'PURGED',$2,clock_timestamp())`, toctouSpace, toctouOperation)
	require.NoError(t, err)
	downConn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer downConn.Release()
	downPID := downConn.Conn().PgConn().PID()
	downSQL := r23SearchMigrationSQL(t, "000003_space_lifecycle.down.sql")
	downDone := make(chan error, 1)
	go func() { _, downErr := downConn.Exec(ctx, downSQL); downDone <- downErr }()
	require.Eventually(t, func() bool {
		var waiting bool
		err := pool.QueryRow(ctx, `SELECT wait_event_type='Lock' FROM pg_stat_activity WHERE pid=$1`, downPID).Scan(&waiting)
		return err == nil && waiting
	}, 5*time.Second, 20*time.Millisecond, "DOWN must lock before it decides that no durable evidence exists")
	require.NoError(t, writer.Commit(ctx))
	require.Error(t, <-downDone, "DOWN must re-check after waiting and refuse the committed terminal evidence")
	var retained int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM search_space_lifecycle_fences WHERE space_id=$1 AND state='PURGED'`, toctouSpace).Scan(&retained))
	require.Equal(t, 1, retained)

	for _, evidence := range []struct {
		name   string
		table  string
		insert string
	}{
		{"operations", "search_space_lifecycle_operations", `INSERT INTO search_space_lifecycle_operations(space_id,deletion_operation_id,generation,request_bytes,request_sha256,state,created_at,retain_until) VALUES($1,$2,7,'r',decode(repeat('68',32),'hex'),'FROZEN',clock_timestamp(),clock_timestamp()+interval '30 days')`},
		{"lifecycle_receipts", "search_space_lifecycle_receipts", `INSERT INTO search_space_lifecycle_receipts(space_id,deletion_operation_id,generation,receipt_bytes,request_sha256,applied_at,retain_until) VALUES($1,$2,7,'r',decode(repeat('68',32),'hex'),clock_timestamp(),clock_timestamp()+interval '30 days')`},
		{"purge_receipts", "search_space_purge_receipts", `INSERT INTO search_space_purge_receipts(space_id,deletion_operation_id,generation,request_bytes,request_sha256,receipt_bytes,completed_at,retain_until) VALUES($1,$2,8,'q',decode(repeat('68',32),'hex'),'r',clock_timestamp(),clock_timestamp()+interval '30 days')`},
		{"manifest_pages", "search_space_chat_manifest_pages", `INSERT INTO search_space_chat_manifest_pages(space_id,deletion_operation_id,generation,page_index,page_bytes,page_sha256,item_count,next_page_token,created_at) VALUES($1,$2,7,0,'p',decode(repeat('68',32),'hex'),1,'',clock_timestamp())`},
	} {
		t.Run("down-refuses-"+evidence.name, func(t *testing.T) {
			casePool := r23SearchMigrationPool(t, ctx, root, evidence.name)
			args := []any{uuid.New(), uuid.New()}
			_, insertErr := casePool.Exec(ctx, evidence.insert, args...)
			require.NoError(t, insertErr)
			_, downErr := casePool.Exec(ctx, r23SearchMigrationSQL(t, "000003_space_lifecycle.down.sql"))
			require.Error(t, downErr, "%s evidence independently blocks DOWN", evidence.name)
			var rows int
			require.NoError(t, casePool.QueryRow(ctx, `SELECT count(*) FROM `+evidence.table).Scan(&rows))
			require.Positive(t, rows, "%s evidence remains after refused DOWN", evidence.name)
			var relation *string
			require.NoError(t, casePool.QueryRow(ctx, `SELECT to_regclass($1)::text`, evidence.table).Scan(&relation))
			require.NotNil(t, relation, "%s schema remains after refused DOWN", evidence.name)
		})
	}
}

func TestR23SearchParticipant_FenceReplayConflictScheduleAndRestart(t *testing.T) {
	// GREEN must wire a durable PostgreSQL lifecycle store into each new Search
	// server. A new server instance is the response-loss/restart oracle: exact
	// bytes replay, changed same-generation bytes conflict, lower generation is
	// stale/no-op, and only a higher LIVE generation releases FROZEN.
	fixture := startR23SearchFixture(t)
	first := fixture.first
	r23AssertSearchFixtureVisible(t, fixture)
	frozenRequest := r23SearchFenceRequest(t, 1, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN)
	frozen, err := first.ApplySpaceLifecycleFence(r23SearchTrustedSpaceContext(t, frozenRequest, searchv1.SearchService_ApplySpaceLifecycleFence_FullMethodName), frozenRequest)
	require.NoError(t, err, "FROZEN must be durably accepted from the trusted Space caller")
	require.Equal(t, commonv1.ParticipantId_PARTICIPANT_ID_SEARCH, frozen.GetReceipt().GetParticipantId())
	require.Equal(t, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, frozen.GetReceipt().GetAppliedState())
	require.NotEmpty(t, frozen.GetReceipt().GetReceiptId())
	require.Equal(t, uint64(1), frozen.GetReceipt().GetGeneration())
	require.NotEmpty(t, frozen.GetReceipt().GetRequestSha256())
	wire, hashErr := proto.MarshalOptions{Deterministic: true}.Marshal(frozenRequest)
	require.NoError(t, hashErr)
	require.Equal(t, domainSeparatedSHA("voice.search.v1.ApplySpaceLifecycleFenceRequest", wire), frozen.GetReceipt().GetRequestSha256(), "receipt hash matches the Space coordinator wrapper-FQN contract")
	require.NotEmpty(t, frozen.GetReceipt().GetManifestSha256())
	require.NotNil(t, frozen.GetReceipt().GetAppliedAt())
	r23AssertSearchFixtureFrozen(t, fixture)
	freshBeforeReplay := fixture.newServer()
	freshReadCtx := metadata.NewIncomingContext(fixture.ctx, metadata.Pairs("x-voice-profile-id", uuid.NewString()))
	_, err = freshBeforeReplay.SearchInChat(freshReadCtx, &searchv1.SearchInChatRequest{Chat: &chatv1.ChatRef{Id: fixture.targetChat.String()}, Query: "needle"})
	require.Equal(t, codes.Unavailable, status.Code(err), "a fresh Search server observes the durable FROZEN fence before replay")

	replayRequest := proto.Clone(frozenRequest).(*searchv1.ApplySpaceLifecycleFenceRequest)
	replayed, err := first.ApplySpaceLifecycleFence(r23SearchTrustedSpaceContext(t, replayRequest, searchv1.SearchService_ApplySpaceLifecycleFence_FullMethodName), replayRequest)
	require.NoError(t, err)
	require.True(t, proto.Equal(frozen, replayed), "exact same-generation replay must return saved bytes")

	changed := proto.Clone(frozenRequest).(*searchv1.ApplySpaceLifecycleFenceRequest)
	changed.Fence.Manifest.ManifestId = "changed-root"
	_, err = first.ApplySpaceLifecycleFence(r23SearchTrustedSpaceContext(t, changed, searchv1.SearchService_ApplySpaceLifecycleFence_FullMethodName), changed)
	require.Equal(t, codes.AlreadyExists, status.Code(err), "changed bytes for the same operation/generation conflict")

	recoverRequest := proto.Clone(frozenRequest).(*searchv1.ApplySpaceLifecycleFenceRequest)
	restarted := fixture.newServer()
	recovered, err := restarted.ApplySpaceLifecycleFence(r23SearchTrustedSpaceContext(t, recoverRequest, searchv1.SearchService_ApplySpaceLifecycleFence_FullMethodName), recoverRequest)
	require.NoError(t, err, "restart must recover exact durable receipt")
	require.True(t, proto.Equal(frozen, recovered))

	gap := r23SearchFenceRequest(t, 3, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE)
	_, err = restarted.ApplySpaceLifecycleFence(r23SearchTrustedSpaceContext(t, gap, searchv1.SearchService_ApplySpaceLifecycleFence_FullMethodName), gap)
	require.Equal(t, codes.Unavailable, status.Code(err), "generation gaps require coordinator reconciliation")

	live := r23SearchFenceRequest(t, 2, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE)
	restored, err := restarted.ApplySpaceLifecycleFence(r23SearchTrustedSpaceContext(t, live, searchv1.SearchService_ApplySpaceLifecycleFence_FullMethodName), live)
	require.NoError(t, err, "only the higher restore generation may release Search")
	require.Equal(t, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE, restored.GetReceipt().GetAppliedState())
	lower, err := restarted.ApplySpaceLifecycleFence(r23SearchTrustedSpaceContext(t, frozenRequest, searchv1.SearchService_ApplySpaceLifecycleFence_FullMethodName), frozenRequest)
	require.NoError(t, err)
	require.Equal(t, uint64(2), lower.GetReceipt().GetGeneration(), "lower generation returns the durable current receipt, never stale applied-state evidence")
}

func TestR23SearchParticipant_ExactReplayWaitsForQueuedHigherGeneration(t *testing.T) {
	fixture := startR23SearchFixture(t)
	frozen := r23SearchFenceRequest(t, 1, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN)
	_, err := fixture.first.ApplySpaceLifecycleFence(r23SearchTrustedSpaceContext(t, frozen, searchv1.SearchService_ApplySpaceLifecycleFence_FullMethodName), frozen)
	require.NoError(t, err)

	blocker, err := fixture.pool.Begin(fixture.ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = blocker.Rollback(context.Background()) })
	_, err = blocker.Exec(fixture.ctx, `SELECT pg_advisory_xact_lock($1)`, searchLifecycleLock)
	require.NoError(t, err)

	type result struct {
		response *searchv1.ApplySpaceLifecycleFenceResponse
		err      error
	}
	live := r23SearchFenceRequest(t, 2, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE)
	liveResult := make(chan result, 1)
	go func() {
		response, callErr := fixture.newServer().ApplySpaceLifecycleFence(r23SearchTrustedSpaceContext(t, live, searchv1.SearchService_ApplySpaceLifecycleFence_FullMethodName), live)
		liveResult <- result{response: response, err: callErr}
	}()
	r23WaitForAdvisoryWaiters(t, fixture, 1)

	replayResult := make(chan result, 1)
	go func() {
		replay := proto.Clone(frozen).(*searchv1.ApplySpaceLifecycleFenceRequest)
		response, callErr := fixture.newServer().ApplySpaceLifecycleFence(r23SearchTrustedSpaceContext(t, replay, searchv1.SearchService_ApplySpaceLifecycleFence_FullMethodName), replay)
		replayResult <- result{response: response, err: callErr}
	}()
	select {
	case stale := <-replayResult:
		require.Failf(t, "unlocked lifecycle replay", "exact generation-1 replay returned generation %d while generation 2 was already queued: %v", stale.response.GetReceipt().GetGeneration(), stale.err)
	case <-time.After(200 * time.Millisecond):
	}

	require.NoError(t, blocker.Commit(fixture.ctx))
	committed := <-liveResult
	require.NoError(t, committed.err)
	require.Equal(t, uint64(2), committed.response.GetReceipt().GetGeneration())
	replayed := <-replayResult
	require.NoError(t, replayed.err)
	require.Equal(t, uint64(2), replayed.response.GetReceipt().GetGeneration(), "a replay serialized after a higher commit returns the current receipt")
}

func TestR23SearchParticipant_RejectsChatPagesOutsideExactSpaceRoot(t *testing.T) {
	fixture := startR23SearchFixture(t)
	ids := []uuid.UUID{fixture.targetChat, fixture.controlChat}
	sort.Slice(ids, func(i, j int) bool { return strings.Compare(string(ids[i][:]), string(ids[j][:])) < 0 })
	fixture.first.ChatManifest = newR23ManifestClientForIDs(t, ids)

	before := r23SearchMutationSnapshot(t, fixture)
	frozen := r23SearchFenceRequest(t, 1, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN)
	_, err := fixture.first.ApplySpaceLifecycleFence(r23SearchTrustedSpaceContext(t, frozen, searchv1.SearchService_ApplySpaceLifecycleFence_FullMethodName), frozen)
	require.Equal(t, codes.Unavailable, status.Code(err), "same manifest_id with different hash, count, and page items cannot bind Search to a wider deletion scope")
	require.Equal(t, before, r23SearchMutationSnapshot(t, fixture), "rejected Chat pages create no fence or receipt")
	r23AssertSearchFixtureVisible(t, fixture)

	decide := r23SearchFenceRequest(t, 2, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED)
	_, err = fixture.first.ApplySpaceLifecycleFence(r23SearchTrustedSpaceContext(t, decide, searchv1.SearchService_ApplySpaceLifecycleFence_FullMethodName), decide)
	require.Equal(t, codes.Unavailable, status.Code(err))
	purge := r23SearchPurgeRequest()
	_, err = fixture.first.PurgeSpace(r23SearchTrustedSpaceContext(t, purge, searchv1.SearchService_PurgeSpace_FullMethodName), purge)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	require.Equal(t, before, r23SearchMutationSnapshot(t, fixture), "mismatched imported scope yields neither receipt nor deletion")
}

func TestR23SearchParticipant_PersistedBindingMustMatchSpaceRootBeforeDecisionAndPurge(t *testing.T) {
	fixture := startR23SearchFixture(t)
	frozen := r23SearchFenceRequest(t, 1, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN)
	_, err := fixture.first.ApplySpaceLifecycleFence(r23SearchTrustedSpaceContext(t, frozen, searchv1.SearchService_ApplySpaceLifecycleFence_FullMethodName), frozen)
	require.NoError(t, err)

	changed := r23SearchManifest()
	changed.ManifestSha256 = bytesOf('x', 32)
	changed.ItemCount++
	decide := r23SearchFenceRequest(t, 2, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED)
	decide.Fence.Manifest = changed
	_, err = fixture.newServer().ApplySpaceLifecycleFence(r23SearchTrustedSpaceContext(t, decide, searchv1.SearchService_ApplySpaceLifecycleFence_FullMethodName), decide)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "persisted Chat evidence must match Space hash and count before PURGE_DECIDED")
	r23AssertSearchFixtureStillPhysical(t, fixture)

	legitimateDecision := r23SearchFenceRequest(t, 2, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED)
	_, err = fixture.newServer().ApplySpaceLifecycleFence(r23SearchTrustedSpaceContext(t, legitimateDecision, searchv1.SearchService_ApplySpaceLifecycleFence_FullMethodName), legitimateDecision)
	require.NoError(t, err)
	purge := r23SearchPurgeRequest()
	purge.Purge.Manifest = changed
	_, err = fixture.newServer().PurgeSpace(r23SearchTrustedSpaceContext(t, purge, searchv1.SearchService_PurgeSpace_FullMethodName), purge)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "Purge must reject any root that differs from the persisted Space decision")
	r23AssertSearchFixtureStillPhysical(t, fixture)
	var purgeReceipts int
	require.NoError(t, fixture.pool.QueryRow(fixture.ctx, `SELECT count(*) FROM search_space_purge_receipts WHERE space_id=$1`, r23SearchSpaceID).Scan(&purgeReceipts))
	require.Zero(t, purgeReceipts)
}

func TestR23SearchParticipant_PurgeCleanupPermanentFenceAndRetainedReceipt(t *testing.T) {
	// The request's schedule generation is seven and its purge generation is
	// eight. PURGE_DECIDED is irreversible; completion removes all Search
	// projections for the Space's exact imported Chat pages, retains full
	// request/receipt bytes for 30 days from participant completion, and keeps
	// the compact PURGED identity-reuse fence permanently.
	fixture := startR23SearchFixture(t)
	svc := fixture.first
	frozen := r23SearchFenceRequest(t, 1, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN)
	_, err := svc.ApplySpaceLifecycleFence(r23SearchTrustedSpaceContext(t, frozen, searchv1.SearchService_ApplySpaceLifecycleFence_FullMethodName), frozen)
	require.NoError(t, err, "Search must durably accept FROZEN generation 7 before purge decision")
	r23AssertSearchFixtureFrozen(t, fixture)
	directPurge := r23SearchPurgeRequest()
	_, err = fixture.newServer().PurgeSpace(r23SearchTrustedSpaceContext(t, directPurge, searchv1.SearchService_PurgeSpace_FullMethodName), directPurge)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "Purge without durable PURGE_DECIDED generation 8 is forbidden")
	purgeFence := r23SearchFenceRequest(t, 2, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED)
	decided, err := svc.ApplySpaceLifecycleFence(r23SearchTrustedSpaceContext(t, purgeFence, searchv1.SearchService_ApplySpaceLifecycleFence_FullMethodName), purgeFence)
	require.NoError(t, err, "Space commits irreversible PURGE_DECIDED before Search cleanup")
	decidedReplay, err := fixture.newServer().ApplySpaceLifecycleFence(r23SearchTrustedSpaceContext(t, purgeFence, searchv1.SearchService_ApplySpaceLifecycleFence_FullMethodName), proto.Clone(purgeFence).(*searchv1.ApplySpaceLifecycleFenceRequest))
	require.NoError(t, err)
	require.True(t, proto.Equal(decided, decidedReplay), "same-generation PURGE_DECIDED replay returns durable receipt")
	purge := r23SearchPurgeRequest()

	failedCtx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = svc.PurgeSpace(principal.WithVerified(failedCtx, principal.Principal{Kind: "service", Issuer: "space", Subject: "service:space", Audience: "search", RPC: searchv1.SearchService_PurgeSpace_FullMethodName, RequestID: uuid.NewString(), RequestHash: r23SearchRequestHash(t, purge)}), purge)
	require.Error(t, err, "an injected cancelled cleanup attempt never commits a partial purge")
	r23AssertSearchFixtureFrozen(t, fixture)
	r23AssertSearchFixtureStillPhysical(t, fixture)
	r23AssertCancelledPurgeDurableState(t, fixture)

	completed, err := svc.PurgeSpace(r23SearchTrustedSpaceContext(t, purge, searchv1.SearchService_PurgeSpace_FullMethodName), purge)
	require.NoError(t, err)
	require.Equal(t, commonv1.PurgeReceiptState_PURGE_RECEIPT_STATE_COMPLETED, completed.GetReceipt().GetState())
	require.Equal(t, searchParticipantID, completed.GetReceipt().GetParticipantId())
	require.NotEmpty(t, completed.GetReceipt().GetReceiptId())
	require.NotEmpty(t, completed.GetReceipt().GetRequestSha256())
	require.NotNil(t, completed.GetReceipt().GetCompletedAt())
	var databaseNow time.Time
	require.NoError(t, fixture.pool.QueryRow(fixture.ctx, `SELECT clock_timestamp()`).Scan(&databaseNow))
	require.WithinDuration(t, databaseNow, completed.GetReceipt().GetCompletedAt().AsTime(), 5*time.Second, "completed_at comes from database time")
	r23AssertSearchFixturePurged(t, fixture)
	r23AssertSearchPurgeEvidence(t, fixture, purge, completed)

	replayPurge := proto.Clone(purge).(*searchv1.PurgeSpaceRequest)
	replayed, err := fixture.newServer().PurgeSpace(r23SearchTrustedSpaceContext(t, replayPurge, searchv1.SearchService_PurgeSpace_FullMethodName), replayPurge)
	require.NoError(t, err)
	require.True(t, proto.Equal(completed, replayed), "purge response loss/restart must replay saved receipt")

	changed := proto.Clone(purge).(*searchv1.PurgeSpaceRequest)
	changed.Purge.Manifest.ManifestId = "changed-root"
	_, err = svc.PurgeSpace(r23SearchTrustedSpaceContext(t, changed, searchv1.SearchService_PurgeSpace_FullMethodName), changed)
	require.Equal(t, codes.AlreadyExists, status.Code(err))

	postPurgeLive := r23SearchFenceRequest(t, 3, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE)
	_, err = svc.ApplySpaceLifecycleFence(r23SearchTrustedSpaceContext(t, postPurgeLive, searchv1.SearchService_ApplySpaceLifecycleFence_FullMethodName), postPurgeLive)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "permanent PURGED fence prevents identifier reuse")
}

func TestR23SearchParticipant_PurgeDecisionRejectsCorruptDurableManifestPage(t *testing.T) {
	fixture := startR23SearchFixture(t)
	frozen := r23SearchFenceRequest(t, 1, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN)
	_, err := fixture.first.ApplySpaceLifecycleFence(r23SearchTrustedSpaceContext(t, frozen, searchv1.SearchService_ApplySpaceLifecycleFence_FullMethodName), frozen)
	require.NoError(t, err)
	_, err = fixture.pool.Exec(fixture.ctx, `UPDATE search_space_chat_manifest_pages SET page_bytes=decode('00','hex') WHERE space_id=$1 AND deletion_operation_id=$2`, r23SearchSpaceID, r23SearchDeletionID)
	require.NoError(t, err)
	decide := r23SearchFenceRequest(t, 2, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED)
	_, err = fixture.newServer().ApplySpaceLifecycleFence(r23SearchTrustedSpaceContext(t, decide, searchv1.SearchService_ApplySpaceLifecycleFence_FullMethodName), decide)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "corrupt durable Chat page must prevent irreversible purge decision")
	var generation uint64
	var state string
	require.NoError(t, fixture.pool.QueryRow(fixture.ctx, `SELECT generation,state FROM search_space_lifecycle_fences WHERE space_id=$1`, r23SearchSpaceID).Scan(&generation, &state))
	require.Equal(t, uint64(1), generation)
	require.Equal(t, "FROZEN", state)
}

func TestR23SearchParticipant_RetentionWaitsForSpaceLockAndKeepsCompactFences(t *testing.T) {
	fixture := startR23PurgedSearchFixture(t)
	r23ExpireSearchEvidence(t, fixture)

	blocker, err := fixture.pool.Begin(fixture.ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = blocker.Rollback(context.Background()) })
	_, err = blocker.Exec(fixture.ctx, `SELECT pg_advisory_xact_lock($1::integer,hashtext($2::text))`, r23SearchSpaceLockNamespace, r23SearchSpaceID.String())
	require.NoError(t, err)

	pruned := make(chan error, 1)
	go func() { pruned <- store.PruneExpiredLifecycleEvidence(fixture.ctx, fixture.pool) }()
	select {
	case pruneErr := <-pruned:
		require.Failf(t, "retention bypassed lifecycle lock", "retention completed before the Space lock was released: %v", pruneErr)
	case <-time.After(200 * time.Millisecond):
	}
	require.NoError(t, blocker.Commit(fixture.ctx))
	require.NoError(t, <-pruned)
	r23AssertExpiredEvidenceCompacted(t, fixture)
}

func TestR23SearchParticipant_RetentionDeletesEvidenceAtomically(t *testing.T) {
	fixture := startR23PurgedSearchFixture(t)
	r23ExpireSearchEvidence(t, fixture)

	const pauseGate int64 = 0x5345415243482301
	blocker, err := fixture.pool.Begin(fixture.ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = blocker.Rollback(context.Background()) })
	_, err = blocker.Exec(fixture.ctx, `SELECT pg_advisory_xact_lock($1)`, pauseGate)
	require.NoError(t, err)
	_, err = fixture.pool.Exec(fixture.ctx, fmt.Sprintf(`CREATE FUNCTION r23_pause_search_retention() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_advisory_xact_lock(%d); RETURN NULL; END $$; CREATE TRIGGER r23_pause_search_retention BEFORE DELETE ON search_space_lifecycle_operations FOR EACH STATEMENT EXECUTE FUNCTION r23_pause_search_retention()`, pauseGate))
	require.NoError(t, err)

	pruned := make(chan error, 1)
	go func() { pruned <- store.PruneExpiredLifecycleEvidence(fixture.ctx, fixture.pool) }()
	r23WaitForAdvisoryWaiters(t, fixture, 1)
	var visibleReceipts int
	require.NoError(t, fixture.pool.QueryRow(fixture.ctx, `SELECT count(*) FROM search_space_lifecycle_receipts WHERE space_id=$1`, r23SearchSpaceID).Scan(&visibleReceipts))
	require.Positive(t, visibleReceipts, "other transactions cannot observe a partial retention deletion")
	require.NoError(t, blocker.Commit(fixture.ctx))
	require.NoError(t, <-pruned)
	r23AssertExpiredEvidenceCompacted(t, fixture)
}

var (
	searchParticipantID = commonv1.ParticipantId_PARTICIPANT_ID_SEARCH
	r23SearchSpaceID    = uuid.MustParse("00000000-0000-0000-0000-000000000701")
	r23SearchDeletionID = uuid.MustParse("00000000-0000-0000-0000-000000000702")
	r23SearchTargetChat = uuid.MustParse("00000000-0000-0000-0000-000000000703")
)

const r23SearchSpaceLockNamespace int32 = 0x53454152

func r23SearchManifest() *commonv1.ManifestBinding {
	return &commonv1.ManifestBinding{ManifestId: "r23-search-root", ManifestSha256: chatManifestSHA(r23SearchSpaceID, r23SearchDeletionID, 1, []uuid.UUID{r23SearchTargetChat}), ItemCount: 1}
}

func bytesOf(value byte, count int) []byte {
	out := make([]byte, count)
	for i := range out {
		out[i] = value
	}
	return out
}

func r23SearchFenceRequest(t *testing.T, generation uint64, state commonv1.LifecycleFenceState) *searchv1.ApplySpaceLifecycleFenceRequest {
	t.Helper()
	return &searchv1.ApplySpaceLifecycleFenceRequest{Fence: &commonv1.SpaceLifecycleFenceRequest{
		ProtocolVersion: 1, SpaceId: r23SearchSpaceID.String(), DeletionOperationId: r23SearchDeletionID.String(),
		Generation: generation, DesiredState: state, Manifest: r23SearchManifest(),
	}}
}

func r23SearchTrustedSpaceContext(t *testing.T, request proto.Message, rpc string) context.Context {
	t.Helper()
	return r23SearchPrincipalContext(t, request, rpc, "space", "service:space", "search", "")
}

func r23SearchPrincipalContext(t *testing.T, request proto.Message, rpc, issuer, subject, audience, hashOverride string) context.Context {
	t.Helper()
	hash := r23SearchRequestHash(t, request)
	if hashOverride != "" {
		hash = hashOverride
	}
	return principal.WithVerified(context.Background(), principal.Principal{
		Kind: "service", Issuer: issuer, Subject: subject, Audience: audience,
		RPC: rpc, RequestID: uuid.NewString(), RequestHash: hash,
	})
}

func r23SearchRequestHash(t *testing.T, request proto.Message) string {
	t.Helper()
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	return hash
}

func r23SearchPurgeRequest() *searchv1.PurgeSpaceRequest {
	return &searchv1.PurgeSpaceRequest{Purge: &commonv1.SpacePurgeRequest{
		ProtocolVersion: 1, SpaceId: r23SearchSpaceID.String(), DeletionOperationId: r23SearchDeletionID.String(),
		Generation: 2, PurgeDecidedAt: timestamppb.New(time.Now().UTC().Truncate(time.Microsecond)),
		ParticipantId: searchParticipantID, Manifest: r23SearchManifest(),
	}}
}

type r23ManifestClient struct {
	chatv1.ChatServiceClient
	page *chatv1.SpacePurgeManifestPage
}

func (c *r23ManifestClient) GetSpacePurgeManifestPage(_ context.Context, _ *chatv1.GetSpacePurgeManifestPageRequest, _ ...grpc.CallOption) (*chatv1.GetSpacePurgeManifestPageResponse, error) {
	return &chatv1.GetSpacePurgeManifestPageResponse{Page: proto.Clone(c.page).(*chatv1.SpacePurgeManifestPage)}, nil
}

func newR23ManifestClient(t *testing.T, chatID uuid.UUID) *r23ManifestClient {
	return newR23ManifestClientForIDs(t, []uuid.UUID{chatID})
}

func newR23ManifestClientForIDs(t *testing.T, ids []uuid.UUID) *r23ManifestClient {
	t.Helper()
	binding := &commonv1.ManifestBinding{ManifestId: r23SearchManifest().GetManifestId(), ItemCount: uint64(len(ids))}
	binding.ManifestSha256 = chatManifestSHA(r23SearchSpaceID, r23SearchDeletionID, 1, ids)
	raw := make([]string, len(ids))
	for i, id := range ids {
		raw[i] = id.String()
	}
	page := &chatv1.SpacePurgeManifestPage{ProtocolVersion: 1, Manifest: binding, PageIndex: 0, ItemIds: raw}
	clone := proto.Clone(page).(*chatv1.SpacePurgeManifestPage)
	wire, err := proto.MarshalOptions{Deterministic: true}.Marshal(clone)
	require.NoError(t, err)
	page.PageSha256 = domainSeparatedSHA(string(page.ProtoReflect().Descriptor().FullName()), wire)
	return &r23ManifestClient{page: page}
}

func r23SearchMigrationSQL(t *testing.T, filename string) string {
	t.Helper()
	path := filepath.Join(r23SearchRepoRoot(t), "src", "backend", "migrations", "search_db", filename)
	contents, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(contents)
}

type r23SearchFixture struct {
	ctx            context.Context
	pool           *pgxpool.Pool
	messages       *store.MessageSearchStore
	projections    *store.ProfileSpaceSearchStore
	first          *SearchGRPC
	newServer      func() *SearchGRPC
	targetChat     uuid.UUID
	controlChat    uuid.UUID
	controlSpace   uuid.UUID
	targetMessage  uuid.UUID
	controlMessage uuid.UUID
}

func startR23SearchFixture(t *testing.T) r23SearchFixture {
	t.Helper()
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	t.Cleanup(cancel)
	root := r23SearchRepoRoot(t)
	pool := integrationtest.StartPostgres(t, ctx, "search_r23_lifecycle", filepath.Join(root, "src", "backend", "migrations", "search_db", "000001_init.up.sql"))
	integrationtest.ApplySQLFile(t, ctx, pool, root, filepath.Join("src", "backend", "migrations", "search_db", "000002_verification_type.up.sql"))
	integrationtest.ApplySQLFile(t, ctx, pool, root, filepath.Join("src", "backend", "migrations", "search_db", "000003_space_lifecycle.up.sql"))

	messages := store.NewMessageSearchStore(pool)
	projections := store.NewProfileSpaceSearchStore(pool)
	targetChat, controlChat, controlSpace := r23SearchTargetChat, uuid.New(), uuid.New()
	require.NoError(t, projections.UpsertSpace(ctx, store.SpaceDocument{SpaceID: r23SearchSpaceID, Name: "Target R23 Space", Visibility: "public"}))
	require.NoError(t, projections.UpsertSpace(ctx, store.SpaceDocument{SpaceID: controlSpace, Name: "Control R23 Space", Visibility: "public"}))
	require.NoError(t, projections.UpsertChat(ctx, targetChat, "Target R23 Chat"))
	require.NoError(t, projections.UpsertChat(ctx, controlChat, "Control R23 Chat"))
	viewer := uuid.New()
	targetMessage, controlMessage := uuid.New(), uuid.New()
	require.NoError(t, messages.Upsert(ctx, store.MessageDocument{MessageID: targetMessage, ChatID: targetChat, SenderProfileID: viewer, Body: "target lifecycle needle", CreatedAt: time.Now().UTC()}))
	require.NoError(t, messages.Upsert(ctx, store.MessageDocument{MessageID: controlMessage, ChatID: controlChat, SenderProfileID: viewer, Body: "control lifecycle needle", CreatedAt: time.Now().UTC()}))
	manifestClient := newR23ManifestClient(t, targetChat)

	newServer := func() *SearchGRPC {
		freshMessages := store.NewMessageSearchStore(pool)
		freshProjections := store.NewProfileSpaceSearchStore(pool)
		return &SearchGRPC{
			Messages:     &MessageStoreAdapter{MessageSearchStore: freshMessages},
			Spaces:       &SpaceStoreAdapter{ProfileSpaceSearchStore: freshProjections},
			ChatManifest: manifestClient,
			Chats: &ProjectionChatAccess{Store: freshProjections, Accessible: func(context.Context, uuid.UUID) ([]uuid.UUID, error) {
				return []uuid.UUID{targetChat, controlChat}, nil
			}},
		}
	}
	return r23SearchFixture{ctx: ctx, pool: pool, messages: messages, projections: projections, first: newServer(), newServer: newServer, targetChat: targetChat, controlChat: controlChat, controlSpace: controlSpace, targetMessage: targetMessage, controlMessage: controlMessage}
}

func startR23PurgedSearchFixture(t *testing.T) r23SearchFixture {
	t.Helper()
	fixture := startR23SearchFixture(t)
	frozen := r23SearchFenceRequest(t, 1, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN)
	_, err := fixture.first.ApplySpaceLifecycleFence(r23SearchTrustedSpaceContext(t, frozen, searchv1.SearchService_ApplySpaceLifecycleFence_FullMethodName), frozen)
	require.NoError(t, err)
	decide := r23SearchFenceRequest(t, 2, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED)
	_, err = fixture.first.ApplySpaceLifecycleFence(r23SearchTrustedSpaceContext(t, decide, searchv1.SearchService_ApplySpaceLifecycleFence_FullMethodName), decide)
	require.NoError(t, err)
	purge := r23SearchPurgeRequest()
	_, err = fixture.first.PurgeSpace(r23SearchTrustedSpaceContext(t, purge, searchv1.SearchService_PurgeSpace_FullMethodName), purge)
	require.NoError(t, err)
	return fixture
}

func r23ExpireSearchEvidence(t *testing.T, fixture r23SearchFixture) {
	t.Helper()
	for _, table := range []string{"search_space_lifecycle_receipts", "search_space_lifecycle_operations", "search_space_purge_receipts"} {
		_, err := fixture.pool.Exec(fixture.ctx, `UPDATE `+table+` SET retain_until=clock_timestamp()-interval '1 second' WHERE space_id=$1`, r23SearchSpaceID)
		require.NoError(t, err)
	}
}

func r23WaitForAdvisoryWaiters(t *testing.T, fixture r23SearchFixture, minimum int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var waiters int
		require.NoError(t, fixture.pool.QueryRow(fixture.ctx, `SELECT count(*) FROM pg_locks WHERE locktype='advisory' AND NOT granted`).Scan(&waiters))
		if waiters >= minimum {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	require.Fail(t, "timed out waiting for PostgreSQL advisory-lock waiter")
}

func r23AssertExpiredEvidenceCompacted(t *testing.T, fixture r23SearchFixture) {
	t.Helper()
	for _, table := range []string{
		"search_space_lifecycle_receipts", "search_space_lifecycle_operations", "search_space_purge_receipts",
		"search_space_chat_manifests", "search_space_chat_manifest_pages", "search_space_chat_manifest_items",
	} {
		var count int
		require.NoError(t, fixture.pool.QueryRow(fixture.ctx, `SELECT count(*) FROM `+table+` WHERE space_id=$1`, r23SearchSpaceID).Scan(&count))
		require.Zero(t, count, "expired full evidence must be deleted atomically from %s", table)
	}
	var fences, chats int
	require.NoError(t, fixture.pool.QueryRow(fixture.ctx, `SELECT count(*) FROM search_space_lifecycle_fences WHERE space_id=$1 AND state='PURGED'`, r23SearchSpaceID).Scan(&fences))
	require.Equal(t, 1, fences, "compact PURGED fence is permanent")
	require.NoError(t, fixture.pool.QueryRow(fixture.ctx, `SELECT count(*) FROM search_space_purged_chat_fences WHERE space_id=$1 AND chat_id=$2`, r23SearchSpaceID, fixture.targetChat).Scan(&chats))
	require.Equal(t, 1, chats, "compact purged-chat fence survives full-evidence deletion")
}

func r23AssertSearchFixtureVisible(t *testing.T, fixture r23SearchFixture) {
	t.Helper()
	ctx := metadata.NewIncomingContext(fixture.ctx, metadata.Pairs("x-voice-profile-id", uuid.NewString()))
	target, err := fixture.first.SearchInChat(ctx, &searchv1.SearchInChatRequest{Chat: &chatv1.ChatRef{Id: fixture.targetChat.String()}, Query: "needle"})
	require.NoError(t, err)
	require.Len(t, target.GetSearchResults().GetHits(), 1)
	control, err := fixture.first.SearchInChat(ctx, &searchv1.SearchInChatRequest{Chat: &chatv1.ChatRef{Id: fixture.controlChat.String()}, Query: "needle"})
	require.NoError(t, err)
	require.Len(t, control.GetSearchResults().GetHits(), 1)
}

func r23AssertSearchFixtureFrozen(t *testing.T, fixture r23SearchFixture) {
	t.Helper()
	ctx := metadata.NewIncomingContext(fixture.ctx, metadata.Pairs("x-voice-profile-id", uuid.NewString()))
	_, err := fixture.first.SearchInChat(ctx, &searchv1.SearchInChatRequest{Chat: &chatv1.ChatRef{Id: fixture.targetChat.String()}, Query: "needle"})
	require.Equal(t, codes.Unavailable, status.Code(err), "FROZEN denies target Space/chat/message projection reads")
	control, err := fixture.first.SearchInChat(ctx, &searchv1.SearchInChatRequest{Chat: &chatv1.ChatRef{Id: fixture.controlChat.String()}, Query: "needle"})
	require.NoError(t, err)
	require.Len(t, control.GetSearchResults().GetHits(), 1, "control Space remains available")
	_, err = fixture.first.SearchSpaces(ctx, &searchv1.SearchSpacesRequest{Query: "Target"})
	require.Equal(t, codes.Unavailable, status.Code(err), "FROZEN denies target Space catalog reads")
	_, err = fixture.first.SearchGlobal(ctx, &searchv1.SearchGlobalRequest{Query: "needle"})
	require.Equal(t, codes.Unavailable, status.Code(err), "FROZEN denies global target Space/chat/message projection reads")
	_, _, err = fixture.messages.SearchInChat(fixture.ctx, fixture.targetChat, "needle", nil, 20)
	require.Error(t, err, "FROZEN denies raw message projection reads")
	_, _, err = fixture.messages.SearchGlobalMessages(fixture.ctx, "needle", nil, 20, []uuid.UUID{fixture.targetChat})
	require.Error(t, err, "FROZEN denies raw global-message reads without routing through SearchSpaces")
	projections := store.NewProfileSpaceSearchStore(fixture.pool)
	_, _, err = projections.SearchSpaces(fixture.ctx, "Target", nil, 20)
	require.Error(t, err, "FROZEN denies raw Space projection reads")
	_, err = projections.SearchChats(fixture.ctx, "Target", 20)
	require.Error(t, err, "FROZEN denies raw chat projection reads")
	err = fixture.messages.Upsert(fixture.ctx, store.MessageDocument{MessageID: uuid.New(), ChatID: fixture.targetChat, SenderProfileID: uuid.New(), Body: "frozen write", CreatedAt: time.Now().UTC()})
	require.Error(t, err, "FROZEN denies target projection writes in the same durable fence transaction")
	require.Error(t, projections.UpsertSpace(fixture.ctx, store.SpaceDocument{SpaceID: r23SearchSpaceID, Name: "frozen space write", Visibility: "public"}), "FROZEN denies Space projection writes")
	require.Error(t, projections.UpsertChat(fixture.ctx, fixture.targetChat, "frozen chat write"), "FROZEN denies chat projection writes")
	require.Error(t, fixture.messages.Delete(fixture.ctx, fixture.targetMessage), "FROZEN denies message projection deletes")
	require.Error(t, fixture.projections.DeleteSpace(fixture.ctx, r23SearchSpaceID), "FROZEN denies Space projection deletes")
}

func r23AssertSearchFixturePurged(t *testing.T, fixture r23SearchFixture) {
	t.Helper()
	for _, table := range []string{"message_search_documents", "chat_search_documents", "space_search_documents"} {
		var count int
		query := `SELECT count(*) FROM ` + table + ` WHERE ` + map[string]string{
			"message_search_documents": "chat_id=$1",
			"chat_search_documents":    "chat_id=$1",
			"space_search_documents":   "space_id=$1",
		}[table]
		argument := any(r23SearchSpaceID)
		if table == "message_search_documents" || table == "chat_search_documents" {
			argument = fixture.targetChat
		}
		require.NoError(t, fixture.pool.QueryRow(fixture.ctx, query, argument).Scan(&count))
		require.Zero(t, count, "purge must delete target %s rows before its completion receipt", table)
	}
	for _, table := range []string{"message_search_documents", "chat_search_documents", "space_search_documents"} {
		var count int
		query := `SELECT count(*) FROM ` + table + ` WHERE ` + map[string]string{
			"message_search_documents": "chat_id=$1",
			"chat_search_documents":    "chat_id=$1",
			"space_search_documents":   "space_id=$1",
		}[table]
		argument := any(fixture.controlSpace)
		if table == "message_search_documents" || table == "chat_search_documents" {
			argument = fixture.controlChat
		}
		require.NoError(t, fixture.pool.QueryRow(fixture.ctx, query, argument).Scan(&count))
		require.Equal(t, 1, count, "purge must preserve control %s rows", table)
	}
	for _, table := range []string{"search_space_chat_manifests", "search_space_chat_manifest_pages", "search_space_chat_manifest_items"} {
		var count int
		require.NoError(t, fixture.pool.QueryRow(fixture.ctx, `SELECT count(*) FROM `+table+` WHERE space_id=$1`, r23SearchSpaceID).Scan(&count))
		require.Positive(t, count, "exact imported manifest association remains durable after purge")
	}
}

func r23AssertSearchFixtureStillPhysical(t *testing.T, fixture r23SearchFixture) {
	t.Helper()
	for _, check := range []struct {
		table string
		where string
		arg   any
	}{
		{"message_search_documents", "chat_id=$1", fixture.targetChat},
		{"chat_search_documents", "chat_id=$1", fixture.targetChat},
		{"space_search_documents", "space_id=$1", r23SearchSpaceID},
		{"search_space_chat_manifest_pages", "space_id=$1", r23SearchSpaceID},
	} {
		var count int
		require.NoError(t, fixture.pool.QueryRow(fixture.ctx, `SELECT count(*) FROM `+check.table+` WHERE `+check.where, check.arg).Scan(&count))
		require.Equal(t, 1, count, "failed cleanup must preserve target %s rows for retry", check.table)
	}
}

func r23AssertCancelledPurgeDurableState(t *testing.T, fixture r23SearchFixture) {
	t.Helper()
	var state string
	require.NoError(t, fixture.pool.QueryRow(fixture.ctx, `SELECT state FROM search_space_lifecycle_fences WHERE space_id=$1`, r23SearchSpaceID).Scan(&state))
	require.Equal(t, "PURGE_DECIDED", state, "a cancelled cleanup leaves the durable irreversible decision for retry")
	for _, table := range []string{"search_space_lifecycle_operations", "search_space_lifecycle_receipts"} {
		var count int
		require.NoError(t, fixture.pool.QueryRow(fixture.ctx, `SELECT count(*) FROM `+table+` WHERE space_id=$1`, r23SearchSpaceID).Scan(&count))
		require.Positive(t, count, "cancelled cleanup retains durable %s evidence", table)
	}
	var purgeReceipts int
	require.NoError(t, fixture.pool.QueryRow(fixture.ctx, `SELECT count(*) FROM search_space_purge_receipts WHERE space_id=$1`, r23SearchSpaceID).Scan(&purgeReceipts))
	require.Zero(t, purgeReceipts, "cancelled cleanup cannot manufacture a purge receipt")
}

func r23SearchMutationSnapshot(t *testing.T, fixture r23SearchFixture) map[string]int {
	t.Helper()
	tables := []string{
		"message_search_documents", "chat_search_documents", "space_search_documents", "search_space_chat_manifest_pages",
		"search_space_chat_manifests", "search_space_chat_manifest_items",
		"search_space_lifecycle_fences", "search_space_lifecycle_operations", "search_space_lifecycle_receipts", "search_space_purge_receipts",
	}
	snapshot := make(map[string]int, len(tables))
	for _, table := range tables {
		var count int
		require.NoError(t, fixture.pool.QueryRow(fixture.ctx, `SELECT count(*) FROM `+table).Scan(&count))
		snapshot[table] = count
	}
	return snapshot
}

func r23SearchMigrationPool(t *testing.T, ctx context.Context, root, name string) *pgxpool.Pool {
	t.Helper()
	pool := integrationtest.StartPostgres(t, ctx, "search_r23_down_"+name, filepath.Join(root, "src", "backend", "migrations", "search_db", "000001_init.up.sql"))
	integrationtest.ApplySQLFile(t, ctx, pool, root, filepath.Join("src", "backend", "migrations", "search_db", "000002_verification_type.up.sql"))
	integrationtest.ApplySQLFile(t, ctx, pool, root, filepath.Join("src", "backend", "migrations", "search_db", "000003_space_lifecycle.up.sql"))
	return pool
}

func r23AssertSearchPurgeEvidence(t *testing.T, fixture r23SearchFixture, request *searchv1.PurgeSpaceRequest, response *searchv1.PurgeSpaceResponse) {
	t.Helper()
	var requestBytes, requestHash, receiptBytes []byte
	var completedAt, retainUntil time.Time
	require.NoError(t, fixture.pool.QueryRow(fixture.ctx, `SELECT request_bytes,request_sha256,receipt_bytes,completed_at,retain_until
FROM search_space_purge_receipts WHERE space_id=$1 AND deletion_operation_id=$2 AND generation=$3`,
		r23SearchSpaceID, r23SearchDeletionID, 2).Scan(&requestBytes, &requestHash, &receiptBytes, &completedAt, &retainUntil))
	wantRequest, err := (proto.MarshalOptions{Deterministic: true}).Marshal(request)
	require.NoError(t, err)
	wantReceipt, err := (proto.MarshalOptions{Deterministic: true}).Marshal(response)
	require.NoError(t, err)
	require.Equal(t, wantRequest, requestBytes, "durable request bytes are exact deterministic wire bytes")
	require.Equal(t, domainSeparatedSHA("voice.search.v1.PurgeSpaceRequest", wantRequest), requestHash)
	require.Equal(t, requestHash, response.GetReceipt().GetRequestSha256())
	require.Equal(t, wantReceipt, receiptBytes, "durable receipt bytes are exact deterministic wire bytes")
	require.WithinDuration(t, completedAt, response.GetReceipt().GetCompletedAt().AsTime(), time.Second, "receipt completion timestamp is database-generated")
	require.Equal(t, completedAt.AddDate(0, 0, 30), retainUntil, "full evidence retention ends exactly 30 days after participant completion")
}

func r23SearchRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
}
