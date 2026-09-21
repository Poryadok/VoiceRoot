package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	userv1 "voice.app/voice/user/v1"
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

func TestActivatedGenerationNeverDowngradesToLegacyAuthority(t *testing.T) {
	require.Error(t, requireProtectedProjectionAuthority(true, false, false))
	require.Error(t, requireProtectedProjectionAuthority(false, true, false))
	require.NoError(t, requireProtectedProjectionAuthority(false, false, false))
	require.NoError(t, requireProtectedProjectionAuthority(true, true, true))
}
