package grpcsvc

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	chatv1 "voice.app/voice/chat/v1"

	moderationv1 "voice.app/voice/moderation/v1"
)

func TestModerationPlatform_ApplySanction_allTypes(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startModerationPostgresPlatform(t, ctx)
	client, cleanup := startModerationGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	modProfile := uuid.New()
	targetAccount := uuid.New()
	modCtx := withInternalModCtx(ctx, modProfile)

	for _, sanctionType := range []string{"warning", "temp_ban", "perm_ban", "shadow_ban"} {
		sanctionType := sanctionType
		t.Run(sanctionType, func(t *testing.T) {
			req := &moderationv1.ApplySanctionRequest{
				TargetAccountId: targetAccount.String(),
				Type:            sanctionType,
				Reason:          "phase14 red test",
			}
			if sanctionType == "temp_ban" {
				req.ExpiresAt = timestamppb.New(time.Now().UTC().Add(24 * time.Hour))
			}
			resp, err := client.ApplySanction(modCtx, req)
			require.NoError(t, err)
			require.NotNil(t, resp.GetSanction())
			require.Equal(t, sanctionType, resp.GetSanction().GetType())
			require.Equal(t, targetAccount.String(), resp.GetSanction().GetTargetAccountId())
			require.Equal(t, modProfile.String(), resp.GetSanction().GetIssuedByProfileId())
		})
	}
}

func TestModerationPlatform_RevokeSanction(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startModerationPostgresPlatform(t, ctx)
	client, cleanup := startModerationGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	modProfile := uuid.New()
	targetAccount := uuid.New()
	modCtx := withInternalModCtx(ctx, modProfile)

	applied, err := client.ApplySanction(modCtx, &moderationv1.ApplySanctionRequest{
		TargetAccountId: targetAccount.String(),
		Type:            "temp_ban",
		Reason:          "revoke me",
		ExpiresAt:       timestamppb.New(time.Now().UTC().Add(48 * time.Hour)),
	})
	require.NoError(t, err)
	require.NotEmpty(t, applied.GetSanction().GetId())

	_, err = client.RevokeSanction(modCtx, &moderationv1.RevokeSanctionRequest{
		SanctionId: applied.GetSanction().GetId(),
	})
	require.NoError(t, err)

	active, err := client.GetActiveSanction(modCtx, &moderationv1.GetActiveSanctionRequest{
		AccountId: targetAccount.String(),
	})
	require.Error(t, err)
	require.Equal(t, codes.NotFound, status.Code(err), "revoked sanction must not be active")
	_ = active
}

func TestModerationPlatform_GetReport_ResolveReport_AssignReport(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startModerationPostgresPlatform(t, ctx)
	client, cleanup := startModerationGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	reporter := uuid.New()
	target := uuid.New()
	modProfile := uuid.New()
	modCtx := withInternalModCtx(ctx, modProfile)

	created, err := client.CreateReport(withReporterProfile(ctx, reporter), &moderationv1.CreateReportRequest{
		TargetType:   "user",
		TargetId:     target.String(),
		Category:     "harassment",
		EvidenceJson: `{}`,
	})
	require.NoError(t, err)
	reportID := created.GetReport().GetId()

	got, err := client.GetReport(modCtx, &moderationv1.GetReportRequest{ReportId: reportID})
	require.NoError(t, err)
	require.Equal(t, reportID, got.GetReport().GetId())
	require.Equal(t, "pending", got.GetReport().GetStatus())

	assigned, err := client.ResolveReport(modCtx, &moderationv1.ResolveReportRequest{
		ReportId:              reportID,
		NewStatus:             "reviewing",
		ResolutionJson:        `{"note":"assigned for review"}`,
		AssignedToProfileId:   strPtr(modProfile.String()),
	})
	require.NoError(t, err)
	require.Equal(t, "reviewing", assigned.GetReport().GetStatus())
	require.Equal(t, modProfile.String(), assigned.GetReport().GetAssignedToProfileId())

	resolved, err := client.ResolveReport(modCtx, &moderationv1.ResolveReportRequest{
		ReportId:       reportID,
		NewStatus:      "resolved",
		ResolutionJson: `{"action":"warned"}`,
	})
	require.NoError(t, err)
	require.Equal(t, "resolved", resolved.GetReport().GetStatus())
	require.NotNil(t, resolved.GetReport().ResolvedAt)
}

func TestModerationPlatform_ListReports_queueFilter_content_vs_spaces(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startModerationPostgresPlatform(t, ctx)
	client, cleanup := startModerationGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	modCtx := withInternalModCtx(ctx, uuid.New())

	for _, spec := range []struct {
		targetType string
		targetID   uuid.UUID
	}{
		{"user", uuid.New()},
		{"message", uuid.New()},
		{"space", uuid.New()},
	} {
		_, err := client.CreateReport(withReporterProfile(ctx, uuid.New()), &moderationv1.CreateReportRequest{
			TargetType:   spec.targetType,
			TargetId:     spec.targetID.String(),
			Category:     "spam",
			EvidenceJson: `{}`,
		})
		require.NoError(t, err)
	}

	contentList, err := client.ListReports(modCtx, &moderationv1.ListReportsRequest{
		StatusFilter: "pending",
		QueueFilter:  "content",
	})
	require.NoError(t, err)
	for _, r := range contentList.GetReportList().GetReports() {
		require.Contains(t, []string{"user", "message"}, r.GetTargetType())
	}
	require.GreaterOrEqual(t, len(contentList.GetReportList().GetReports()), 2)

	spacesList, err := client.ListReports(modCtx, &moderationv1.ListReportsRequest{
		StatusFilter: "pending",
		QueueFilter:  "spaces",
	})
	require.NoError(t, err)
	for _, r := range spacesList.GetReportList().GetReports() {
		require.Equal(t, "space", r.GetTargetType())
	}
	require.Len(t, spacesList.GetReportList().GetReports(), 1)
}

func TestModerationPlatform_IsShadowBanned(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startModerationPostgresPlatform(t, ctx)
	client, cleanup := startModerationGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	modProfile := uuid.New()
	targetAccount := uuid.New()
	modCtx := withInternalModCtx(ctx, modProfile)

	before, err := client.IsShadowBanned(modCtx, &moderationv1.IsShadowBannedRequest{
		AccountId: targetAccount.String(),
	})
	require.NoError(t, err)
	require.False(t, before.GetShadowBanned())

	_, err = client.ApplySanction(modCtx, &moderationv1.ApplySanctionRequest{
		TargetAccountId: targetAccount.String(),
		Type:            "shadow_ban",
		Reason:          "automod threshold",
	})
	require.NoError(t, err)

	after, err := client.IsShadowBanned(modCtx, &moderationv1.IsShadowBannedRequest{
		AccountId: targetAccount.String(),
	})
	require.NoError(t, err)
	require.True(t, after.GetShadowBanned())
}

func TestModerationPlatform_CheckMessage_spamMuteFirstSecondOffense(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startModerationPostgresPlatform(t, ctx)
	client, cleanup := startModerationGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	senderProfile := uuid.New()
	chatID := uuid.New()
	dm := chatv1.ChatType_CHAT_TYPE_DM
	spamBody := "http://spam.example/deal http://spam.example/deal http://spam.example/deal"

	first, err := client.CheckMessage(withInternalModCtx(ctx, senderProfile), &moderationv1.CheckMessageRequest{
		Chat:             &chatv1.ChatRef{Id: chatID.String(), Type: &dm},
		Content:          spamBody,
		SenderProfileId:  senderProfile.String(),
	})
	require.NoError(t, err)
	require.False(t, first.GetCheckResult().GetAllowed())
	require.Equal(t, "spam_mute", first.GetCheckResult().GetBlockReason())

	second, err := client.CheckMessage(withInternalModCtx(ctx, senderProfile), &moderationv1.CheckMessageRequest{
		Chat:            &chatv1.ChatRef{Id: chatID.String(), Type: &dm},
		Content:         spamBody,
		SenderProfileId: senderProfile.String(),
	})
	require.NoError(t, err)
	require.False(t, second.GetCheckResult().GetAllowed())
	require.Equal(t, "spam_mute_permanent", second.GetCheckResult().GetBlockReason())
}

func TestModerationPlatform_SubmitAppeal_ReviewAppeal(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startModerationPostgresPlatform(t, ctx)
	client, cleanup := startModerationGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	modProfile := uuid.New()
	targetAccount := uuid.New()
	modCtx := withInternalModCtx(ctx, modProfile)

	sanction, err := client.ApplySanction(modCtx, &moderationv1.ApplySanctionRequest{
		TargetAccountId: targetAccount.String(),
		Type:            "temp_ban",
		Reason:          "appeal target",
		ExpiresAt:       timestamppb.New(time.Now().UTC().Add(7 * 24 * time.Hour)),
	})
	require.NoError(t, err)

	appealCtx := metadataWithAccount(ctx, targetAccount)
	submitted, err := client.SubmitAppeal(appealCtx, &moderationv1.SubmitAppealRequest{
		SanctionId: sanction.GetSanction().GetId(),
		Reason:     "mistaken ban",
	})
	require.NoError(t, err)
	require.Equal(t, "pending", submitted.GetAppeal().GetStatus())

	reviewed, err := client.ReviewAppeal(modCtx, &moderationv1.ReviewAppealRequest{
		AppealId:      submitted.GetAppeal().GetId(),
		Status:        "approved",
		ModeratorNote: strPtr("reversed on review"),
	})
	require.NoError(t, err)
	require.Equal(t, "approved", reviewed.GetAppeal().GetStatus())
	require.Equal(t, modProfile.String(), reviewed.GetAppeal().GetReviewedByProfileId())
}

func TestModerationPlatform_ApplySanction_writesAuditLog(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startModerationPostgresPlatform(t, ctx)
	client, cleanup := startModerationGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	modProfile := uuid.New()
	targetAccount := uuid.New()
	modCtx := withInternalModCtx(ctx, modProfile)

	resp, err := client.ApplySanction(modCtx, &moderationv1.ApplySanctionRequest{
		TargetAccountId: targetAccount.String(),
		Type:            "warning",
		Reason:          "audit trail",
	})
	require.NoError(t, err)

	var auditCount int
	err = pool.QueryRow(ctx, `
SELECT COUNT(*) FROM moderation_audit_log
WHERE actor_profile_id = $1
  AND action = 'sanction_applied'
  AND target_type = 'account'
  AND target_id = $2
  AND details->>'sanction_id' = $3`,
		modProfile, targetAccount, resp.GetSanction().GetId(),
	).Scan(&auditCount)
	require.NoError(t, err)
	require.Equal(t, 1, auditCount, "sanction must write moderation_audit_log row with actor id")
}

func metadataWithAccount(ctx context.Context, accountID uuid.UUID) context.Context {
	return withReporterProfile(ctx, accountID)
}
