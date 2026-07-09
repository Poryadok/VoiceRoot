package grpcsvc

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/pkg/integrationtest"

	moderationv1 "voice.app/voice/moderation/v1"
)

// TestCreateReport_StoryTarget_Accepted documents stories (docs/features/stories.md) story reports (stories.md → Moderation).
func TestCreateReport_StoryTarget_Accepted(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "moderationdb", filepath.Join(repoRoot(t), "src", "backend", "migrations", "moderation_db", "000001_reports.up.sql"))
	integrationtestApplySQL(t, ctx, pool, filepath.Join(repoRoot(t), "src", "backend", "migrations", "moderation_db", "000003_story_reports.up.sql"))
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
	require.Equal(t, "story", resp.GetReport().GetTargetType())
	require.Equal(t, storyID.String(), resp.GetReport().GetTargetId())
	require.Equal(t, "pending", resp.GetReport().GetStatus())
}
