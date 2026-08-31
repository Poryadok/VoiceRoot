package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	commonv1 "voice.app/voice/common/v1"
	moderationv1 "voice.app/voice/moderation/v1"
)

func TestListAppeals_statusFilterAndPagination(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startModerationPostgresPlatform(t, ctx)
	client, cleanup := startModerationGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	modProfile := uuid.New()
	modCtx := withInternalModCtx(ctx, modProfile)

	createAppeal := func(targetAccount uuid.UUID, reason string) string {
		applied, err := client.ApplySanction(modCtx, &moderationv1.ApplySanctionRequest{
			TargetAccountId: targetAccount.String(),
			Type:            "warning",
			Reason:          "list appeals test",
		})
		require.NoError(t, err)
		submitted, err := client.SubmitAppeal(withAccountCtx(ctx, targetAccount), &moderationv1.SubmitAppealRequest{
			SanctionId: applied.GetSanction().GetId(),
			Reason:     reason,
		})
		require.NoError(t, err)
		return submitted.GetAppeal().GetId()
	}

	firstID := createAppeal(uuid.New(), "first appeal")
	secondID := createAppeal(uuid.New(), "second appeal")

	page1, err := client.ListAppeals(modCtx, &moderationv1.ListAppealsRequest{
		StatusFilter: "pending",
		Page:         &commonv1.CursorPageRequest{PageSize: 1},
	})
	require.NoError(t, err)
	require.Len(t, page1.GetAppealList().GetAppeals(), 1)
	require.NotEmpty(t, page1.GetAppealList().GetNextCursor())

	page2, err := client.ListAppeals(modCtx, &moderationv1.ListAppealsRequest{
		StatusFilter: "pending",
		Page: &commonv1.CursorPageRequest{
			Cursor:   page1.GetAppealList().GetNextCursor(),
			PageSize: 10,
		},
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(page2.GetAppealList().GetAppeals()), 1)

	ids := []string{
		page1.GetAppealList().GetAppeals()[0].GetId(),
	}
	for _, appeal := range page2.GetAppealList().GetAppeals() {
		ids = append(ids, appeal.GetId())
	}
	require.Contains(t, ids, firstID)
	require.Contains(t, ids, secondID)

	_, err = client.ReviewAppeal(modCtx, &moderationv1.ReviewAppealRequest{
		AppealId: firstID,
		Status:   "approved",
	})
	require.NoError(t, err)

	pendingOnly, err := client.ListAppeals(modCtx, &moderationv1.ListAppealsRequest{
		StatusFilter: "pending",
	})
	require.NoError(t, err)
	for _, appeal := range pendingOnly.GetAppealList().GetAppeals() {
		require.Equal(t, "pending", appeal.GetStatus())
		require.NotEqual(t, firstID, appeal.GetId())
	}
}

func TestListAppeals_InternalOnly(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startModerationPostgresPlatform(t, ctx)
	client, cleanup := startModerationGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	_, err := client.ListAppeals(withReporterProfile(ctx, uuid.New()), &moderationv1.ListAppealsRequest{})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
