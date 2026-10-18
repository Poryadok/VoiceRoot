package grpcsvc

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"sync/atomic"
	"testing"
	"time"
	moderationv1 "voice.app/voice/moderation/v1"
	"voice/backend/moderation/internal/automod"
	"voice/backend/moderation/internal/store"
	automodv1 "voice/backend/pkg/pb/voice/automod/v1"
	"voice/backend/pkg/principal"
)

// BE137 c11 deliberately adds requester privacy authority. Verified Gateway
// identity/Redis epoch alone cannot identify a terminal opaque subject after
// its raw account mapping was erased. Missing authority must never insert.
func TestReportIntakeRequiresCurrentSubjectAuthority(t *testing.T) {
	pool := reportIntakePool(t)
	svc := &ModerationGRPC{Reports: &store.ReportStore{Pool: pool}}
	request := &moderationv1.CreateReportRequest{TargetType: "user", TargetId: uuid.NewString(), Category: "spam", EvidenceJson: `{}`}
	ctx := verifiedReportContext(t, request, uuid.New())
	_, err := svc.CreateReport(ctx, request)
	require.Equal(t, codes.Unavailable, status.Code(err))
	var rows int
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM reports`).Scan(&rows))
	require.Zero(t, rows, "no receipt or raw identity link without requester subject authority")
}

type reportSubjectResolverFunc func(context.Context, uuid.UUID, uuid.UUID) (*automodv1.ResolveModerationSubjectResponse, error)

func (f reportSubjectResolverFunc) ResolveSubject(ctx context.Context, a, b uuid.UUID) (*automodv1.ResolveModerationSubjectResponse, error) {
	return f(ctx, a, b)
}

func reportSubjectResponse(account, subject uuid.UUID) *automodv1.ResolveModerationSubjectResponse {
	return &automodv1.ResolveModerationSubjectResponse{Subject: &automodv1.SubjectBinding{ProtocolVersion: 1, AccountId: account.String(), SubjectId: subject.String(), LifecycleRevision: 1, State: automodv1.SubjectState_SUBJECT_STATE_LIVE}, SessionEpoch: 1}
}
func reportOpenPrivacy(context.Context) (store.ReportPrivacyCheck, error) {
	return func(context.Context) error { return nil }, nil
}

func TestReportReporterTerminalAndGeneration(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	subjects := automod.NewStore(pool)
	reports := &store.ReportStore{Pool: pool, Subjects: subjects}
	subjects.BeforeTerminalSubjectPurge = func(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
		return reports.PurgeReportSubjectsTx(ctx, tx, []uuid.UUID{id})
	}
	request := &moderationv1.CreateReportRequest{TargetType: "user", TargetId: uuid.NewString(), Category: "spam"}
	verified := verifiedReportContext(t, request, uuid.New())
	p, _ := principal.FromContext(verified)
	account := uuid.MustParse(p.AccountID)
	subject := uuid.New()
	response := reportSubjectResponse(account, subject)
	svc := &ModerationGRPC{Reports: reports, ReportPrivacy: reportOpenPrivacy, ReportAuthority: reportSubjectResolverFunc(func(context.Context, uuid.UUID, uuid.UUID) (*automodv1.ResolveModerationSubjectResponse, error) {
		return response, nil
	})}
	receipt, err := svc.CreateReport(verified, request)
	require.NoError(t, err)
	var stored uuid.UUID
	var revision int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT reporter_subject_id,reporter_subject_revision FROM reports WHERE id=$1`, receipt.GetReport().GetId()).Scan(&stored, &revision))
	require.Equal(t, subject, stored)
	require.EqualValues(t, 1, revision)
	require.NoError(t, subjects.Purge(ctx, subject))
	_, err = svc.CreateReport(verified, request)
	require.Equal(t, codes.Unavailable, status.Code(err), "stale genuine LIVE cannot rebind after terminal purge")
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM reports`).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_subjects`).Scan(&count))
	require.Zero(t, count)

	// The subject-only restore branch must reject the first raw-account binding.
	absent := uuid.New()
	require.NoError(t, subjects.Purge(ctx, absent))
	response = reportSubjectResponse(account, absent)
	_, err = svc.CreateReport(verified, request)
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_subjects`).Scan(&count))
	require.Zero(t, count)

	// Closing and reopening a process gate during Auth does not revive an old check.
	var generation atomic.Int64
	generation.Store(1)
	svc.ReportPrivacy = func(context.Context) (store.ReportPrivacyCheck, error) {
		g := generation.Load()
		return func(context.Context) error {
			if generation.Load() != g {
				return errors.New("generation changed")
			}
			return nil
		}, nil
	}
	svc.ReportAuthority = reportSubjectResolverFunc(func(context.Context, uuid.UUID, uuid.UUID) (*automodv1.ResolveModerationSubjectResponse, error) {
		generation.Add(2)
		return reportSubjectResponse(account, uuid.New()), nil
	})
	_, err = svc.CreateReport(verified, request)
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM reports`).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_subjects`).Scan(&count))
	require.Zero(t, count, "binding and receipt must roll back together")
}

func TestReportRequesterAuthorityFailuresAreTargetIndependent(t *testing.T) {
	pool := reportIntakePool(t)
	subjects := automod.NewStore(pool)
	reports := &store.ReportStore{Pool: pool, Subjects: subjects}
	for _, failure := range []string{"unavailable", "not_found", "failed_precondition", "epoch", "account", "subject", "revision", "deleted"} {
		for _, target := range []string{"user", "message", "space", "story"} {
			t.Run(failure+"/"+target, func(t *testing.T) {
				request := &moderationv1.CreateReportRequest{TargetType: target, TargetId: uuid.NewString(), Category: "spam"}
				ctx := verifiedReportContext(t, request, uuid.New())
				p, _ := principal.FromContext(ctx)
				account := uuid.MustParse(p.AccountID)
				resolver := reportSubjectResolverFunc(func(context.Context, uuid.UUID, uuid.UUID) (*automodv1.ResolveModerationSubjectResponse, error) {
					r := reportSubjectResponse(account, uuid.New())
					switch failure {
					case "unavailable":
						return nil, status.Error(codes.Unavailable, "outage")
					case "not_found":
						return nil, status.Error(codes.NotFound, "unknown")
					case "failed_precondition":
						return nil, status.Error(codes.FailedPrecondition, "deleted or mismatch")
					case "epoch":
						r.SessionEpoch = 2
					case "account":
						r.Subject.AccountId = uuid.NewString()
					case "subject":
						r.Subject.SubjectId = "bad"
					case "revision":
						r.Subject.LifecycleRevision = 0
					case "deleted":
						r.Subject.State = automodv1.SubjectState_SUBJECT_STATE_DELETED
					}
					return r, nil
				})
				svc := &ModerationGRPC{Reports: reports, ReportAuthority: resolver, ReportPrivacy: reportOpenPrivacy}
				_, err := svc.CreateReport(ctx, request)
				expected := codes.Unavailable
				if failure == "epoch" {
					expected = codes.Unauthenticated
				}
				require.Equal(t, expected, status.Code(err))
			})
		}
	}
	var count int
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM reports`).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM automod_terminal_subjects`).Scan(&count))
	require.Zero(t, count, "Auth errors must never synthesize terminal fences")
}

func TestReportValidationTerminalNullTarget(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	subjects := automod.NewStore(pool)
	reports := &store.ReportStore{Pool: pool, Subjects: subjects, Privacy: reportOpenPrivacy}
	reporter := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Revision: 1, State: "LIVE"}
	target := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Revision: 1, State: "LIVE"}
	check, _ := reportOpenPrivacy(ctx)
	row, err := reports.InsertSubjectReport(ctx, uuid.New(), reporter, check, "user", uuid.New(), "spam", nil, `{"private":true}`)
	require.NoError(t, err)
	claim, err := reports.ClaimValidation(ctx)
	require.NoError(t, err)
	claim.ReporterSubject = &reporter
	claim.TargetSubject = &target
	claim.PrivacyCheck = check
	// Terminal arrives before the owner result can attach target account/subject.
	require.NoError(t, subjects.Purge(ctx, target.ID))
	called := false
	applied, err := reports.CompleteValidation(ctx, claim, "LIVE", target.AccountID, []store.IntakeEvent{{ID: uuid.New(), Kind: "domain.report_created", Subject: "domain", Payload: []byte("private")}, {ID: uuid.New(), Kind: "analytics.report_created", Subject: "analytics", Payload: []byte("private")}}, validationHookFunc(func(context.Context, pgx.Tx, store.ValidationFact) error { called = true; return nil }))
	require.NoError(t, err)
	require.False(t, applied)
	require.False(t, called)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM reports WHERE id=$1`, row.ID).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM report_intake_outbox`).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_subjects WHERE subject_id=$1`, target.ID).Scan(&count))
	require.Zero(t, count, "terminal subject must not be rebound by late callback")
}

func TestReportSubjectOnlyPurgeWithoutAccountMapping(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	subjects := automod.NewStore(pool)
	reports := &store.ReportStore{Pool: pool, Subjects: subjects}
	subject := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Revision: 1, State: "LIVE"}
	check, _ := reportOpenPrivacy(ctx)
	row, err := reports.InsertSubjectReport(ctx, uuid.New(), subject, check, "message", uuid.New(), "spam", nil, `{"private":true}`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM automod_subjects WHERE subject_id=$1`, subject.ID)
	require.NoError(t, err)
	injected := errors.New("participant failed")
	subjects.BeforeTerminalSubjectPurge = func(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
		if err := reports.PurgeReportSubjectsTx(ctx, tx, []uuid.UUID{id}); err != nil {
			return err
		}
		return injected
	}
	require.ErrorIs(t, subjects.Purge(ctx, subject.ID), injected)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM reports WHERE id=$1`, row.ID).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_terminal_subjects WHERE subject_id=$1`, subject.ID).Scan(&count))
	require.Zero(t, count, "participant failure must roll back opaque fence too")
	subjects.BeforeTerminalSubjectPurge = func(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
		return reports.PurgeReportSubjectsTx(ctx, tx, []uuid.UUID{id})
	}
	require.NoError(t, subjects.Purge(ctx, subject.ID))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM reports WHERE id=$1`, row.ID).Scan(&count))
	require.Zero(t, count)
}

func TestReportValidationAccountsPrecedeReportLock(t *testing.T) {
	pool := reportIntakePool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	subjects := automod.NewStore(pool)
	reports := &store.ReportStore{Pool: pool, Subjects: subjects}
	reporter := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Revision: 1, State: "LIVE"}
	target := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Revision: 1, State: "LIVE"}
	check, _ := reportOpenPrivacy(ctx)
	row, err := reports.InsertSubjectReport(ctx, uuid.New(), reporter, check, "user", uuid.New(), "spam", nil, `{}`)
	require.NoError(t, err)
	claim, err := reports.ClaimValidation(ctx)
	require.NoError(t, err)
	claim.ReporterSubject = &reporter
	claim.TargetSubject = &target
	claim.PrivacyCheck = check
	held, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = held.Rollback(context.Background()) }()
	_, err = held.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('moderation-account-sanction:' || $1::text,0))`, target.AccountID.String())
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() {
		_, err := reports.CompleteValidation(ctx, claim, "LIVE", target.AccountID, []store.IntakeEvent{{ID: uuid.New(), Kind: "domain.report_created", Subject: "domain", Payload: []byte("stable")}, {ID: uuid.New(), Kind: "analytics.report_created", Subject: "analytics", Payload: []byte("stable")}}, nil)
		done <- err
	}()
	deadline := time.Now().Add(4 * time.Second)
	for {
		var waiting bool
		require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event='advisory' AND query LIKE '%moderation-account-sanction%' AND pid<>pg_backend_pid())`).Scan(&waiting))
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("validation did not wait at account lock")
		}
		time.Sleep(5 * time.Millisecond)
	}
	probe, err := pool.Begin(ctx)
	require.NoError(t, err)
	_, err = probe.Exec(ctx, `SELECT id FROM reports WHERE id=$1 FOR UPDATE NOWAIT`, row.ID)
	require.NoError(t, err, "validation must not hold report row while waiting for any account")
	require.NoError(t, probe.Rollback(ctx))
	require.NoError(t, held.Commit(ctx))
	require.NoError(t, <-done)
}

func TestReportSubjectDispatchPrivacyAndDeadline(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	subjects := automod.NewStore(pool)
	reports := &store.ReportStore{Pool: pool, Subjects: subjects, Privacy: reportOpenPrivacy}
	reporter := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Revision: 1, State: "LIVE"}
	target := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Revision: 1, State: "LIVE"}
	check, _ := reportOpenPrivacy(ctx)
	row, err := reports.InsertSubjectReport(ctx, uuid.New(), reporter, check, "user", uuid.New(), "spam", nil, `{}`)
	require.NoError(t, err)
	claim, err := reports.ClaimValidation(ctx)
	require.NoError(t, err)
	claim.ReporterSubject = &reporter
	claim.TargetSubject = &target
	claim.PrivacyCheck = check
	events := []store.IntakeEvent{{ID: uuid.New(), Kind: "domain.report_created", Subject: "domain", Payload: []byte("stable")}, {ID: uuid.New(), Kind: "analytics.report_created", Subject: "analytics", Payload: []byte("stable")}}
	applied, err := reports.CompleteValidation(ctx, claim, "LIVE", target.AccountID, events, nil)
	require.NoError(t, err)
	require.True(t, applied)
	called := 0
	publish := func(context.Context, store.IntakeEvent) error { called++; return nil }
	reports.Privacy = nil
	_, err = reports.DeliverIntakeEvent(ctx, publish)
	require.Error(t, err)
	require.Zero(t, called)
	reports.Privacy = reportOpenPrivacy
	delivered, err := reports.DeliverIntakeEvent(ctx, publish)
	require.NoError(t, err)
	require.True(t, delivered)
	require.Equal(t, 1, called)
	// Local authoritative deadline closes the remaining dispatch even before the
	// background purge removes mapping/content. No fresh Auth RPC under SQL locks.
	_, err = pool.Exec(ctx, `UPDATE automod_subjects SET lifecycle_revision=2,lifecycle_state='DELETED',purge_at=clock_timestamp()-interval '1 second' WHERE subject_id=$1`, reporter.ID)
	require.NoError(t, err)
	_, err = reports.DeliverIntakeEvent(ctx, publish)
	require.Error(t, err)
	require.Equal(t, 1, called)
	subjects.BeforeTerminalSubjectPurge = func(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
		return reports.PurgeReportSubjectsTx(ctx, tx, []uuid.UUID{id})
	}
	require.NoError(t, subjects.Purge(ctx, reporter.ID))
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM report_intake_outbox WHERE report_id=$1`, row.ID).Scan(&count))
	require.Zero(t, count)
}

func TestReportSubjectInsertCommitGenerationRollback(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	subjects := automod.NewStore(pool)
	reports := &store.ReportStore{Pool: pool, Subjects: subjects}
	subject := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Revision: 1, State: "LIVE"}
	calls := 0
	check := store.ReportPrivacyCheck(func(context.Context) error {
		calls++
		if calls == 2 {
			return errors.New("closed before commit")
		}
		return nil
	})
	_, err := reports.InsertSubjectReport(ctx, uuid.New(), subject, check, "story", uuid.New(), "spam", nil, `{}`)
	require.Error(t, err)
	require.Equal(t, 2, calls)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM reports`).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_subjects`).Scan(&count))
	require.Zero(t, count, "no separately committed requester binding")
}

func TestReportDispatchDeadlineContext(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	subjects := automod.NewStore(pool)
	reports := &store.ReportStore{Pool: pool, Subjects: subjects, Privacy: reportOpenPrivacy}
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
	var purgeAt time.Time
	require.NoError(t, pool.QueryRow(ctx, `UPDATE automod_subjects SET lifecycle_revision=2,lifecycle_state='DELETED',purge_at=clock_timestamp()+interval '2 seconds' WHERE subject_id=$1 RETURNING purge_at`, reporter.ID).Scan(&purgeAt))
	attempted, err := reports.DeliverIntakeEvent(ctx, func(attempt context.Context, event store.IntakeEvent) error {
		deadline, ok := attempt.Deadline()
		require.True(t, ok)
		require.WithinDuration(t, purgeAt, deadline, time.Microsecond, "publication must not outlive authoritative purge deadline")
		<-attempt.Done()
		return attempt.Err()
	})
	require.True(t, attempted)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	var delivered int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM report_intake_outbox WHERE report_id=$1 AND delivered_at IS NOT NULL`, row.ID).Scan(&delivered))
	require.Zero(t, delivered)
}
