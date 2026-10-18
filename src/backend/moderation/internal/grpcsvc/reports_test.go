package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	moderationv1 "voice.app/voice/moderation/v1"
)

// TestCreateReport_StoryTarget_Accepted documents stories (docs/features/stories.md) story reports (stories.md → Moderation).
func TestCreateReport_StoryTarget_Accepted(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := reportIntakePool(t)
	client, cleanup := startModerationGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	reporter := uuid.New()
	storyID := uuid.New()
	resp, err := client.CreateReport(withReporterProfile(ctx, reporter), &moderationv1.CreateReportRequest{
		TargetType:   "story",
		TargetId:     storyID.String(),
		Category:     "offensive",
		EvidenceJson: `{}`,
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.GetReport().GetId())
	require.Empty(t, resp.GetReport().GetTargetType())
	require.Empty(t, resp.GetReport().GetTargetId())
	require.Empty(t, resp.GetReport().GetStatus())
}
