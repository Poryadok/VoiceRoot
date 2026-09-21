package main

import (
	"context"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/search/internal/profileprojection"
)

func TestDesiredProjectionGenerationIsOptionalAndFailClosed(t *testing.T) {
	t.Setenv("SEARCH_USER_PROJECTION_DESIRED_GENERATION", "")
	_, set, err := desiredProjectionGeneration()
	require.NoError(t, err)
	require.False(t, set)

	t.Setenv("SEARCH_USER_PROJECTION_DESIRED_GENERATION", "2")
	generation, set, err := desiredProjectionGeneration()
	require.NoError(t, err)
	require.True(t, set)
	require.Equal(t, uint64(2), generation)

	t.Setenv("SEARCH_USER_PROJECTION_DESIRED_GENERATION", "0")
	_, _, err = desiredProjectionGeneration()
	require.Error(t, err)
}

type cutoffReplayClient struct{ userv1.UserServiceClient }

func (cutoffReplayClient) ListSearchProfileSnapshot(context.Context, *userv1.ListSearchProfileSnapshotRequest, ...grpc.CallOption) (*userv1.ListSearchProfileSnapshotResponse, error) {
	return &userv1.ListSearchProfileSnapshotResponse{Events: []*userv1.SearchProfileProjectionEvent{{JournalOffset: 1}}}, nil
}
func (cutoffReplayClient) ListSearchProfileJournal(context.Context, *userv1.ListSearchProfileJournalRequest, ...grpc.CallOption) (*userv1.ListSearchProfileJournalResponse, error) {
	return &userv1.ListSearchProfileJournalResponse{Events: []*userv1.SearchProfileProjectionEvent{{JournalOffset: 2}, {JournalOffset: 3}}}, nil
}

type requestAwareRecoveryClient struct {
	userv1.UserServiceClient
	calls int
}

// The controller accepts the generated User client interface, so this fixture
// deliberately embeds it and overrides only the protected snapshot/journal
// calls exercised by recovery; no legacy hydrator is involved.
func (c *requestAwareRecoveryClient) BeginSearchProfileSnapshot(context.Context, *userv1.BeginSearchProfileSnapshotRequest, ...grpc.CallOption) (*userv1.BeginSearchProfileSnapshotResponse, error) {
	return &userv1.BeginSearchProfileSnapshotResponse{HighWatermark: 1}, nil
}

func (c *requestAwareRecoveryClient) ListSearchProfileSnapshot(context.Context, *userv1.ListSearchProfileSnapshotRequest, ...grpc.CallOption) (*userv1.ListSearchProfileSnapshotResponse, error) {
	return &userv1.ListSearchProfileSnapshotResponse{Events: []*userv1.SearchProfileProjectionEvent{{JournalOffset: 1}}}, nil
}
func (c *requestAwareRecoveryClient) ListSearchProfileJournal(_ context.Context, request *userv1.ListSearchProfileJournalRequest, _ ...grpc.CallOption) (*userv1.ListSearchProfileJournalResponse, error) {
	c.calls++
	if request.GetAfterOffset() == 2 && c.calls == 1 {
		return &userv1.ListSearchProfileJournalResponse{Events: []*userv1.SearchProfileProjectionEvent{{JournalOffset: 3}}}, nil
	}
	if request.GetAfterOffset() == 2 {
		return &userv1.ListSearchProfileJournalResponse{Events: []*userv1.SearchProfileProjectionEvent{{JournalOffset: 3}}}, nil
	}
	return &userv1.ListSearchProfileJournalResponse{}, nil
}

func TestReplayProjectionEvidenceStopsAtPersistedCutoffWhenPageAdvanced(t *testing.T) {
	evidence, err := replayProjectionEvidenceAt(context.Background(), cutoffReplayClient{}, 1, 1, 2)
	require.NoError(t, err)
	require.Equal(t, uint64(2), evidence.Cutoff)
	require.Error(t, requireAuthoritativeCutoff(context.Background(), cutoffReplayClient{}, 2))
}

func TestRequestAwareProtectedClientExposesCatchUpOffsetAfterStaleCutoff(t *testing.T) {
	client := &requestAwareRecoveryClient{}
	require.Error(t, requireAuthoritativeCutoff(context.Background(), client, 2))
	page, err := client.ListSearchProfileJournal(context.Background(), &userv1.ListSearchProfileJournalRequest{AfterOffset: 2})
	require.NoError(t, err)
	require.Len(t, page.GetEvents(), 1)
	require.Equal(t, uint64(3), page.GetEvents()[0].GetJournalOffset())
}

type controllerRecoveryClient struct {
	userv1.UserServiceClient
	mu     sync.Mutex
	events []*userv1.SearchProfileProjectionEvent
	after  []uint64
	begins int
}

func (c *controllerRecoveryClient) BeginSearchProfileSnapshot(context.Context, *userv1.BeginSearchProfileSnapshotRequest, ...grpc.CallOption) (*userv1.BeginSearchProfileSnapshotResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.begins++
	return &userv1.BeginSearchProfileSnapshotResponse{HighWatermark: 1}, nil
}
func (c *controllerRecoveryClient) ListSearchProfileSnapshot(_ context.Context, r *userv1.ListSearchProfileSnapshotRequest, _ ...grpc.CallOption) (*userv1.ListSearchProfileSnapshotResponse, error) {
	if r.GetHighWatermark() != 1 {
		return nil, context.Canceled
	}
	return &userv1.ListSearchProfileSnapshotResponse{Events: c.events[:1]}, nil
}
func (c *controllerRecoveryClient) ListSearchProfileJournal(_ context.Context, r *userv1.ListSearchProfileJournalRequest, _ ...grpc.CallOption) (*userv1.ListSearchProfileJournalResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.after = append(c.after, r.GetAfterOffset())
	if r.GetAfterOffset() == 1 {
		return &userv1.ListSearchProfileJournalResponse{Events: c.events[1:]}, nil
	}
	if r.GetAfterOffset() == 2 {
		return &userv1.ListSearchProfileJournalResponse{Events: c.events[2:]}, nil
	}
	return &userv1.ListSearchProfileJournalResponse{}, nil
}

func TestRunDesiredProjectionGeneration_RecoversReadyCutoff(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	root := searchMainRoot(t)
	pool := integrationtest.StartPostgres(t, ctx, "search_controller_recovery", filepath.Join(root, "src", "backend", "migrations", "search_db", "000001_init.up.sql"))
	for _, m := range []string{"000003_space_lifecycle.up.sql", "000002_verification_type.up.sql", "000004_user_profile_projection.up.sql", "000005_user_profile_projection_snapshot.up.sql", "000006_user_profile_projection_quarantine.up.sql", "000007_user_profile_projection_fence.up.sql", "000008_user_profile_projection_generations.up.sql"} {
		integrationtest.ApplySQLFile(t, ctx, pool, root, filepath.Join("src", "backend", "migrations", "search_db", m))
	}
	pid, aid := uuid.NewString(), uuid.NewString()
	makeEvent := func(offset, rev uint64) *userv1.SearchProfileProjectionEvent {
		return &userv1.SearchProfileProjectionEvent{ProtocolVersion: 1, EventId: uuid.NewString(), ProfileId: pid, SourceRevision: rev, JournalOffset: offset, Payload: &userv1.SearchProfileProjectionEvent_Upsert{Upsert: &userv1.SearchProfileUpsert{AccountId: aid, Username: "recover", Discriminator: "0001", DisplayName: "Recover", UsernameSearchKey: "recover", DisplayNameSearchKey: "recover", NormalizationVersion: 1}}}
	}
	e1, e2, e3 := makeEvent(1, 1), makeEvent(2, 2), makeEvent(3, 3)
	client := &controllerRecoveryClient{events: []*userv1.SearchProfileProjectionEvent{e1, e2, e3}}
	lease, ok, err := profileprojection.TryStartGeneration(ctx, pool, 2)
	require.NoError(t, err)
	require.True(t, ok)
	a := &profileprojection.StoreAdapter{Pool: pool, Generation: 2}
	require.NoError(t, a.StartSnapshot(ctx, 1))
	_, err = a.ApplyAndCheckpoint(ctx, e1, 1)
	require.NoError(t, err)
	require.NoError(t, a.FinishSnapshot(ctx))
	_, err = a.ApplyAndCheckpoint(ctx, e2, 2)
	require.NoError(t, err)
	require.NoError(t, profileprojection.MarkGenerationReady(ctx, pool, 2))
	profileprojection.FinishGenerationRebuild(ctx, lease)
	done := make(chan struct{})
	go func() { runDesiredProjectionGeneration(ctx, nil, client, pool, 2); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("controller recovery timed out")
	}
	route, err := profileprojection.LoadGenerationRoute(ctx, pool)
	require.NoError(t, err)
	require.Equal(t, uint64(2), route.Active)
}

func searchMainRoot(t *testing.T) string {
	t.Helper()
	_, f, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(f), "..", "..", "..", ".."))
}

func TestActivatedGenerationNeverDowngradesToLegacyAuthority(t *testing.T) {
	require.Error(t, requireProtectedProjectionAuthority(true, false, false))
	require.Error(t, requireProtectedProjectionAuthority(false, true, false))
	require.NoError(t, requireProtectedProjectionAuthority(false, false, false))
	require.NoError(t, requireProtectedProjectionAuthority(true, true, true))
}
