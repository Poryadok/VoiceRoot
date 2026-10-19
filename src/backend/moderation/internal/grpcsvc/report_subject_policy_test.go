package grpcsvc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	moderationv1 "voice.app/voice/moderation/v1"
	"voice/backend/moderation/internal/automod"
	"voice/backend/moderation/internal/store"
	automodv1 "voice/backend/pkg/pb/voice/automod/v1"
	"voice/backend/pkg/principal"
)

func TestReportRequesterLifecycleAndRestrictionPolicy(t *testing.T) {
	pool := reportIntakePool(t)
	reports := newReportFixtureStore(pool)
	for _, kind := range []string{"user", "message", "space", "story"} {
		t.Run(kind, func(t *testing.T) {
			request := &moderationv1.CreateReportRequest{TargetType: kind, TargetId: uuid.NewString(), Category: "spam"}
			ctx := verifiedReportContext(t, request, uuid.New())
			p, _ := principal.FromContext(ctx)
			account := uuid.MustParse(p.AccountID)
			subject := uuid.New()
			response := reportSubjectResponse(account, subject)
			cycle := uuid.NewString()
			response.Subject.State = automodv1.SubjectState_SUBJECT_STATE_DELETED
			response.Subject.DeletionCycleId = &cycle
			response.Subject.PurgeAt = timestamppb.New(time.Now().Add(time.Hour))
			response.AuthRestricted = true
			svc := &ModerationGRPC{Reports: reports, ReportPrivacy: reportOpenPrivacy, ReportAuthority: reportSubjectResolverFunc(func(context.Context, uuid.UUID, uuid.UUID) (*automodv1.ResolveModerationSubjectResponse, error) {
				return response, nil
			})}
			_, err := svc.CreateReport(ctx, request)
			require.Equal(t, codes.Unauthenticated, status.Code(err), "valid deleted requester cannot submit")
			response = reportSubjectResponse(account, subject)
			response.AuthRestricted = true
			receipt, err := svc.CreateReport(ctx, request)
			require.NoError(t, err, "LIVE restriction does not invent a reporting ban")
			require.NotEmpty(t, receipt.GetReport().GetId())
			response = reportSubjectResponse(account, uuid.New())
			response.Subject.LifecycleRevision = 99
			_, err = svc.CreateReport(ctx, request)
			require.Equal(t, codes.Unavailable, status.Code(err), "current local subject cannot silently remap")
		})
	}
	var count int
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM reports`).Scan(&count))
	require.Equal(t, 4, count)
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM automod_subjects`).Scan(&count))
	require.Equal(t, 4, count)
}

func TestReportDispatchGenerationClosureAfterACK(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	reports := newReportFixtureStore(pool)
	reporter := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Revision: 1, State: "LIVE"}
	check, _ := reportOpenPrivacy(ctx)
	row, err := reports.InsertSubjectReport(ctx, uuid.New(), reporter, check, "story", uuid.New(), "spam", nil, `{}`)
	require.NoError(t, err)
	claim, err := reports.ClaimValidation(ctx)
	require.NoError(t, err)
	claim.ReporterSubject = &reporter
	claim.PrivacyCheck = check
	events := []store.IntakeEvent{{ID: uuid.New(), Kind: "domain.report_created", Subject: "domain", Payload: []byte("stable")}, {ID: uuid.New(), Kind: "analytics.report_created", Subject: "analytics", Payload: []byte("stable")}}
	applied, err := reports.CompleteValidation(ctx, claim, "LIVE", uuid.Nil, events, nil)
	require.NoError(t, err)
	require.True(t, applied)
	generation := int64(1)
	reports.Privacy = func(context.Context) (store.ReportPrivacyCheck, error) {
		captured := generation
		return func(context.Context) error {
			if captured != generation {
				return errors.New("privacy generation changed")
			}
			return nil
		}, nil
	}
	var published store.IntakeEvent
	committed, err := reports.DeliverIntakeEvent(ctx, func(_ context.Context, event store.IntakeEvent) error { published = event; generation++; return nil })
	require.Error(t, err)
	require.False(t, committed, "an old check cannot acknowledge delivery after gate closure")
	var delivered int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM report_intake_outbox WHERE report_id=$1 AND delivered_at IS NOT NULL`, row.ID).Scan(&delivered))
	require.Zero(t, delivered)
	// Already dispatched bytes cannot be retracted. A new admitted operation
	// retries the identical durable identity, rather than minting another effect.
	committed, err = reports.DeliverIntakeEvent(ctx, func(_ context.Context, event store.IntakeEvent) error {
		require.Equal(t, published.ID, event.ID)
		require.Equal(t, published.Payload, event.Payload)
		return nil
	})
	require.NoError(t, err)
	require.True(t, committed)
}

func TestReportReciprocalValidationAndTerminalPurge(t *testing.T) {
	pool := reportIntakePool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	reports := newReportFixtureStore(pool)
	subjects := reports.Subjects
	check, _ := reportOpenPrivacy(ctx)
	bindings := []automod.Subject{{ID: uuid.New(), AccountID: uuid.New(), Revision: 1, State: "LIVE"}, {ID: uuid.New(), AccountID: uuid.New(), Revision: 1, State: "LIVE"}}
	profiles := []uuid.UUID{uuid.New(), uuid.New()}
	for i := range bindings {
		_, err := reports.InsertSubjectReport(ctx, profiles[i], bindings[i], check, "user", profiles[1-i], "spam", nil, `{}`)
		require.NoError(t, err)
	}
	claims := make([]*store.ValidationClaim, 2)
	for i := range claims {
		claim, err := reports.ClaimValidation(ctx)
		require.NoError(t, err)
		index := 0
		if *claim.ReporterAccountID != bindings[0].AccountID {
			index = 1
		}
		claim.ReporterSubject = &bindings[index]
		claim.TargetSubject = &bindings[1-index]
		claim.PrivacyCheck = check
		claims[i] = claim
	}
	subjects.BeforeTerminalPurge = reports.PurgeReportAccountLinksTx
	subjects.BeforeTerminalSubjectPurge = func(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
		return reports.PurgeReportSubjectsTx(ctx, tx, []uuid.UUID{id})
	}
	threshold := &store.ReportThresholdStore{Pool: pool}
	start := make(chan struct{})
	done := make(chan error, 4)
	for _, claim := range claims {
		go func(claim *store.ValidationClaim) {
			<-start
			_, err := reports.CompleteValidation(ctx, claim, "LIVE", claim.TargetSubject.AccountID, []store.IntakeEvent{{ID: uuid.New(), Kind: "domain.report_created", Subject: "domain", Payload: []byte("stable")}, {ID: uuid.New(), Kind: "analytics.report_created", Subject: "analytics", Payload: []byte("stable")}}, threshold)
			done <- err
		}(claim)
	}
	for _, binding := range bindings {
		go func(id uuid.UUID) { <-start; done <- subjects.Purge(ctx, id) }(binding.ID)
	}
	close(start)
	for range 4 {
		select {
		case err := <-done:
			require.NoError(t, err)
		case <-ctx.Done():
			t.Fatal("reciprocal validation, real threshold hook and terminal purge did not quiesce")
		}
	}
	for _, table := range []string{"reports", "report_intake_outbox", "report_threshold_evaluations", "automod_subjects"} {
		var count int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM `+table).Scan(&count))
		require.Zero(t, count, table+" must not retain late activation")
	}
	var fences int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_terminal_subjects`).Scan(&fences))
	require.Equal(t, 2, fences)
}
