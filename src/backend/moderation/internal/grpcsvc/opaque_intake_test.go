package grpcsvc

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	moderationv1 "voice.app/voice/moderation/v1"
	"voice/backend/moderation/internal/store"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
)

func reportIntakePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testing.Short() {
		t.Skip("PostgreSQL report intake integration")
	}
	ctx := context.Background()
	var pool *pgxpool.Pool
	if dsn := os.Getenv("REPORT_INTAKE_TEST_DSN"); dsn != "" {
		var err error
		pool, err = pgxpool.New(ctx, dsn)
		require.NoError(t, err)
		t.Cleanup(pool.Close)
		// This DSN is an explicitly isolated disposable test database.
		_, err = pool.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public")
		require.NoError(t, err)
	} else {
		pool = integrationtest.StartPostgres(t, ctx, "reportintake", "")
	}
	files, err := filepath.Glob(filepath.Join(repoRoot(t), "src/backend/migrations/moderation_db/*.up.sql"))
	require.NoError(t, err)
	for _, path := range files {
		integrationtestApplySQL(t, ctx, pool, path)
	}
	return pool
}

func verifiedReportContext(t *testing.T, request proto.Message, profile uuid.UUID) context.Context {
	t.Helper()
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	// The verified context is the domain seam. The legacy metadata also makes
	// the regression execute the pre-cutover implementation for meaningful RED;
	// transport tests separately require strict rejection of these raw headers.
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-profile-id", profile.String()))
	account := uuid.NewString()
	return principal.WithVerified(ctx, principal.Principal{Kind: "delegated_user", Issuer: "gateway", Subject: account, Audience: "moderation", RPC: moderationv1.ModerationService_CreateReport_FullMethodName,
		RequestID: uuid.NewString(), RequestHash: hash, AccountID: account, ProfileID: profile.String(), SessionEpoch: 1, AccountType: "regular"})
}

func TestReportOpaqueReceipt(t *testing.T) {
	pool := reportIntakePool(t)
	svc := &ModerationGRPC{Reports: &store.ReportStore{Pool: pool}}
	for _, targetType := range []string{"user", "message", "space", "story"} {
		t.Run(targetType, func(t *testing.T) {
			description := "Private complaint"
			request := &moderationv1.CreateReportRequest{TargetType: targetType, TargetId: uuid.NewString(), Category: "spam", Description: &description, EvidenceJson: `{}`}
			response, err := svc.CreateReport(verifiedReportContext(t, request, uuid.New()), request)
			require.NoError(t, err, "unknown IDs must be durably accepted without owner configuration")
			require.NotNil(t, response.GetReport())
			receipt, err := uuid.Parse(response.GetReport().GetId())
			require.NoError(t, err)
			require.NotEqual(t, uuid.Nil, receipt)
			require.True(t, proto.Equal(response, &moderationv1.CreateReportResponse{Report: &moderationv1.Report{Id: receipt.String()}}), "intake must return only a receipt, never report status/content")
			var exists bool
			require.NoError(t, pool.QueryRow(context.Background(), "SELECT EXISTS(SELECT 1 FROM reports WHERE id=$1)", receipt).Scan(&exists))
			require.True(t, exists, "receipt must acknowledge an actual committed row")
		})
	}
}

func TestReportPendingCannotEnterQueueOrActions(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	reports := &store.ReportStore{Pool: pool}
	svc := &ModerationGRPC{Reports: reports}
	target := uuid.New()
	request := &moderationv1.CreateReportRequest{TargetType: "user", TargetId: target.String(), Category: "spam"}
	response, err := svc.CreateReport(verifiedReportContext(t, request, uuid.New()), request)
	require.NoError(t, err)
	receipt := uuid.MustParse(response.GetReport().GetId())
	t.Run("count", func(t *testing.T) {
		count, err := reports.CountReports24h(ctx, target)
		require.NoError(t, err)
		require.Zero(t, count, "unverified intake must not contribute to a sanction threshold")
	})
	t.Run("queue", func(t *testing.T) {
		rows, err := reports.ListReports(ctx, "", 50)
		require.NoError(t, err)
		require.Empty(t, rows, "unverified intake must not become actionable")
	})
	t.Run("paginated_queue", func(t *testing.T) {
		page, err := reports.ListReportsFilteredPage(ctx, "pending", "content", "", 50)
		require.NoError(t, err)
		require.Empty(t, page.Rows)
	})
	t.Run("resolve", func(t *testing.T) {
		staff := metadata.NewIncomingContext(ctx, metadata.Pairs("x-voice-internal", "true", "x-voice-profile-id", uuid.NewString()))
		_, err := svc.ResolveReport(staff, &moderationv1.ResolveReportRequest{ReportId: receipt.String(), NewStatus: "resolved"})
		require.Equal(t, codes.NotFound, status.Code(err), "an explicit report ID must not bypass validation")
	})
	t.Run("sanction", func(t *testing.T) {
		sanctions := &store.SanctionStore{Pool: pool}
		_, err := sanctions.InsertSanction(ctx, uuid.New(), "warning", "Unverified report", &receipt, uuid.New(), nil)
		require.Error(t, err, "a report relation must not bypass validation")
		var count int
		require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM sanctions").Scan(&count))
		require.Zero(t, count)
	})
	t.Run("staff_ban_has_no_auth_or_recovery_effect", func(t *testing.T) {
		auth := &intakeAuthCounter{}
		svc.Sanctions = &store.SanctionStore{Pool: pool}
		svc.Auth = auth
		staff := metadata.NewIncomingContext(ctx, metadata.Pairs("x-voice-internal", "true", "x-voice-profile-id", uuid.NewString()))
		id := receipt.String()
		_, err := svc.ApplySanction(staff, &moderationv1.ApplySanctionRequest{TargetAccountId: uuid.NewString(), Type: "perm_ban", Reason: "Unverified report", ReportId: &id})
		require.Equal(t, codes.FailedPrecondition, status.Code(err))
		require.Zero(t, auth.calls, "validation denial must not suspend or activate an account")
		var count int
		require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM moderation_account_status_sync").Scan(&count))
		require.Zero(t, count, "validation denial is not ambiguous mutation recovery")
	})
}

type intakeAuthCounter struct{ calls int }

func TestReportRawIdentityCannotWrite(t *testing.T) {
	pool := reportIntakePool(t)
	svc := &ModerationGRPC{Reports: &store.ReportStore{Pool: pool}}
	req := &moderationv1.CreateReportRequest{TargetType: "user", TargetId: uuid.NewString(), Category: "spam"}
	raw := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-profile-id", uuid.NewString(), "x-voice-user-id", uuid.NewString()))
	_, err := svc.CreateReport(raw, req)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	valid := verifiedReportContext(t, req, uuid.New())
	changed := proto.Clone(req).(*moderationv1.CreateReportRequest)
	changed.TargetId = uuid.NewString()
	_, err = svc.CreateReport(valid, changed)
	require.Equal(t, codes.Unauthenticated, status.Code(err), "verified identity must be bound to this exact request")
	var count int
	require.NoError(t, pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM reports").Scan(&count))
	require.Zero(t, count)
}

func (a *intakeAuthCounter) SetAccountStatus(context.Context, uuid.UUID, string, string) error {
	a.calls++
	return nil
}
