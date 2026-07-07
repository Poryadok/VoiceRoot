package grpcsvc

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/pkg/integrationtest"

	moderationv1 "voice.app/voice/moderation/v1"
)

var validReportCategories = []string{
	"spam", "harassment", "offensive", "fake", "cheating", "other",
}

// TestCreateReport_ValidCategories documents PLAN app stack1 report categories.
func TestCreateReport_ValidCategories(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	migrationPath := filepath.Join(repoRoot(t), "src", "backend", "migrations", "moderation_db", "000001_reports.up.sql")
	pool := integrationtest.StartPostgres(t, ctx, "moderationdb", migrationPath)

	reporter := uuid.New()
	target := uuid.New()
	client, cleanup := startModerationGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	for _, cat := range validReportCategories {
		cat := cat
		t.Run(cat, func(t *testing.T) {
			req := &moderationv1.CreateReportRequest{
				TargetType:   "user",
				TargetId:     target.String(),
				Category:     cat,
				EvidenceJson: `{}`,
			}
			if cat == "other" {
				req.Description = strPtr("needs human review")
			}
			resp, err := client.CreateReport(withReporterProfile(ctx, reporter), req)
			require.NoError(t, err)
			require.NotEmpty(t, resp.GetReport().GetId())
			require.Equal(t, cat, resp.GetReport().GetCategory())
			require.Equal(t, reporter.String(), resp.GetReport().GetReporterProfileId())
			require.Equal(t, "pending", resp.GetReport().GetStatus())

			var count int
			err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM reports WHERE id = $1`, resp.GetReport().GetId()).Scan(&count)
			require.NoError(t, err)
			require.Equal(t, 1, count)
		})
	}
}

// TestCreateReport_OtherWithoutDescription_InvalidArgument documents other category requires description.
func TestCreateReport_OtherWithoutDescription_InvalidArgument(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "moderationdb", filepath.Join(repoRoot(t), "src", "backend", "migrations", "moderation_db", "000001_reports.up.sql"))
	client, cleanup := startModerationGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	_, err := client.CreateReport(withReporterProfile(ctx, uuid.New()), &moderationv1.CreateReportRequest{
		TargetType:   "user",
		TargetId:     uuid.New().String(),
		Category:     "other",
		EvidenceJson: `{}`,
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

// TestCreateReport_InvalidCategory_InvalidArgument documents unknown categories are rejected.
func TestCreateReport_InvalidCategory_InvalidArgument(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "moderationdb", filepath.Join(repoRoot(t), "src", "backend", "migrations", "moderation_db", "000001_reports.up.sql"))
	client, cleanup := startModerationGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	_, err := client.CreateReport(withReporterProfile(ctx, uuid.New()), &moderationv1.CreateReportRequest{
		TargetType:   "user",
		TargetId:     uuid.New().String(),
		Category:     "not_a_category",
		EvidenceJson: `{}`,
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

// TestCreateReport_MMToxicAliasMapsToCheating documents mm_toxic accepted at gRPC (stored as cheating).
func TestCreateReport_MMToxicAliasMapsToCheating(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "moderationdb", filepath.Join(repoRoot(t), "src", "backend", "migrations", "moderation_db", "000001_reports.up.sql"))
	client, cleanup := startModerationGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	resp, err := client.CreateReport(withReporterProfile(ctx, uuid.New()), &moderationv1.CreateReportRequest{
		TargetType:   "user",
		TargetId:     uuid.New().String(),
		Category:     "mm_toxic",
		EvidenceJson: `{}`,
	})
	require.NoError(t, err)
	require.Equal(t, "cheating", resp.GetReport().GetCategory())
}

// TestCreateReport_DescriptionTooLong_InvalidArgument documents server-side 500 char cap.
func TestCreateReport_DescriptionTooLong_InvalidArgument(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "moderationdb", filepath.Join(repoRoot(t), "src", "backend", "migrations", "moderation_db", "000001_reports.up.sql"))
	client, cleanup := startModerationGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	longDesc := strings.Repeat("x", 501)
	_, err := client.CreateReport(withReporterProfile(ctx, uuid.New()), &moderationv1.CreateReportRequest{
		TargetType:    "user",
		TargetId:      uuid.New().String(),
		Category:      "spam",
		Description:   &longDesc,
		EvidenceJson:  `{}`,
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

// TestCreateReport_SelfTarget_Denied documents reporters cannot file against themselves.
func TestCreateReport_SelfTarget_Denied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "moderationdb", filepath.Join(repoRoot(t), "src", "backend", "migrations", "moderation_db", "000001_reports.up.sql"))
	client, cleanup := startModerationGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	self := uuid.New()
	_, err := client.CreateReport(withReporterProfile(ctx, self), &moderationv1.CreateReportRequest{
		TargetType:   "user",
		TargetId:     self.String(),
		Category:     "spam",
		EvidenceJson: `{}`,
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

// TestCreateReport_ResponseOmitsStatusToReporter documents reporter sees acceptance without resolution status updates.
func TestCreateReport_ResponseOmitsStatusToReporter(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "moderationdb", filepath.Join(repoRoot(t), "src", "backend", "migrations", "moderation_db", "000001_reports.up.sql"))
	client, cleanup := startModerationGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	reporter := uuid.New()
	resp, err := client.CreateReport(withReporterProfile(ctx, reporter), &moderationv1.CreateReportRequest{
		TargetType:   "message",
		TargetId:     uuid.New().String(),
		Category:     "harassment",
		EvidenceJson: `{"message_ids":["` + uuid.New().String() + `"]}`,
	})
	require.NoError(t, err)
	report := resp.GetReport()
	require.Equal(t, "pending", report.GetStatus())
	require.Empty(t, report.GetAssignedToProfileId())
	require.Nil(t, report.ResolvedAt)
	require.Empty(t, report.GetResolutionJson())
}

// TestCreateReport_Threshold_WritesAutoModLog documents auto-mod log on report threshold (min 10 / 24h).
func TestCreateReport_Threshold_WritesAutoModLog(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	root := repoRoot(t)
	pool := integrationtest.StartPostgres(t, ctx, "moderationdb", filepath.Join(root, "src", "backend", "migrations", "moderation_db", "000001_reports.up.sql"))
	integrationtestApplySQL(t, ctx, pool, filepath.Join(root, "src", "backend", "migrations", "moderation_db", "000002_sanctions.up.sql"))
	client, cleanup := startModerationGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	targetProfile := uuid.New()
	for i := 0; i < 10; i++ {
		_, err := client.CreateReport(withReporterProfile(ctx, uuid.New()), &moderationv1.CreateReportRequest{
			TargetType:   "user",
			TargetId:     targetProfile.String(),
			Category:     "spam",
			EvidenceJson: `{}`,
		})
		require.NoError(t, err)
	}

	_, err := client.CreateReport(withReporterProfile(ctx, uuid.New()), &moderationv1.CreateReportRequest{
		TargetType:   "user",
		TargetId:     targetProfile.String(),
		Category:     "spam",
		EvidenceJson: `{}`,
	})
	require.NoError(t, err)

	var logCount int
	err = pool.QueryRow(ctx, `
SELECT COUNT(*) FROM auto_mod_log
WHERE target_profile_id = $1 AND trigger = 'report_threshold' AND action = 'shadow_ban'
  AND created_at > $2`,
		targetProfile, time.Now().UTC().Add(-24*time.Hour),
	).Scan(&logCount)
	require.NoError(t, err)
	require.GreaterOrEqual(t, logCount, 1, "report threshold must write auto_mod_log")

	var sanctionCount int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM sanctions WHERE target_account_id = $1`, targetProfile).Scan(&sanctionCount)
	require.NoError(t, err)
	require.Equal(t, 0, sanctionCount, "app stack1 must not insert sanctions on report threshold")
}

// TestListReports_InternalOnly documents queue listing is not a public reporter API.
func TestListReports_InternalOnly(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "moderationdb", filepath.Join(repoRoot(t), "src", "backend", "migrations", "moderation_db", "000001_reports.up.sql"))
	client, cleanup := startModerationGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	_, err := client.ListReports(withReporterProfile(ctx, uuid.New()), &moderationv1.ListReportsRequest{
		StatusFilter: "pending",
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func strPtr(s string) *string { return &s }
