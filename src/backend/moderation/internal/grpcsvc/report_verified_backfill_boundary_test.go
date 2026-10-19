package grpcsvc

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"voice/backend/moderation/internal/automod"
	"voice/backend/moderation/internal/store"
	automodv1 "voice/backend/pkg/pb/voice/automod/v1"
)

func missingVerifiedReporter(t *testing.T) (*pgxpool.Pool, *store.ReportStore, *store.ReportRow, automod.Subject) {
	t.Helper()
	pool := reportIntakePool(t)
	ctx := context.Background()
	reports := newReportFixtureStore(pool)
	check, _ := reportOpenPrivacy(ctx)
	subject := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Revision: 1, State: "LIVE"}
	row, err := reports.InsertSubjectReport(ctx, uuid.New(), subject, check, "story", uuid.New(), "spam", nil, `{}`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM report_privacy_journal WHERE report_id=$1`, row.ID)
	require.NoError(t, err)
	return pool, reports, row, subject
}

func TestReportVerifiedJournalBackfillAuthorityFailures(t *testing.T) {
	for _, outcome := range []string{"unavailable", "failed_precondition_text", "wrong_subject", "stale_revision", "missing_cycle", "expired_deleted", "newer_local_live"} {
		t.Run(outcome, func(t *testing.T) {
			pool, reports, row, subject := missingVerifiedReporter(t)
			ctx := context.Background()
			if outcome == "stale_revision" {
				_, err := pool.Exec(ctx, `UPDATE reports SET reporter_subject_revision=3 WHERE id=$1`, row.ID)
				require.NoError(t, err)
			}
			resolver := reportSubjectResolverFunc(func(_ context.Context, account, expected uuid.UUID) (*automodv1.ResolveModerationSubjectResponse, error) {
				require.Equal(t, subject.AccountID, account)
				require.Equal(t, subject.ID, expected)
				if outcome == "unavailable" {
					return nil, status.Error(codes.Unavailable, "authority unavailable")
				}
				if outcome == "failed_precondition_text" {
					return nil, status.Error(codes.FailedPrecondition, "terminal purged")
				}
				response := reportSubjectResponse(account, expected)
				if outcome == "wrong_subject" {
					response.Subject.SubjectId = uuid.NewString()
				}
				if outcome == "missing_cycle" || outcome == "expired_deleted" || outcome == "newer_local_live" {
					cycle := uuid.NewString()
					response.Subject.DeletionCycleId = &cycle
					response.Subject.State = automodv1.SubjectState_SUBJECT_STATE_DELETED
					response.Subject.LifecycleRevision = 2
					response.AuthRestricted = true
					deadline := time.Now().Add(time.Hour).Truncate(time.Microsecond)
					if outcome == "expired_deleted" {
						deadline = time.Now().Add(-time.Second).Truncate(time.Microsecond)
					}
					response.Subject.PurgeAt = timestamppb.New(deadline)
					if outcome == "missing_cycle" {
						response.Subject.DeletionCycleId = nil
					}
					if outcome == "newer_local_live" {
						_, err := pool.Exec(ctx, `UPDATE automod_subjects SET lifecycle_revision=3 WHERE subject_id=$1`, subject.ID)
						require.NoError(t, err)
					}
				}
				return response, nil
			})
			_, err := reports.BackfillVerifiedReportJournal(ctx, resolver, reportOpenPrivacy, 10)
			require.Error(t, err)
			for _, query := range []string{`SELECT count(*) FROM report_privacy_journal`, `SELECT count(*) FROM automod_terminal_subjects`, `SELECT count(*) FROM report_intake_outbox`} {
				var count int
				require.NoError(t, pool.QueryRow(ctx, query).Scan(&count))
				require.Zero(t, count)
			}
			var count int
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM reports WHERE id=$1`, row.ID).Scan(&count))
			require.Equal(t, 1, count)
			var state string
			var revision int64
			require.NoError(t, pool.QueryRow(ctx, `SELECT lifecycle_state,lifecycle_revision FROM automod_subjects WHERE subject_id=$1`, subject.ID).Scan(&state, &revision))
			require.Equal(t, "LIVE", state)
			if outcome == "newer_local_live" {
				require.EqualValues(t, 3, revision)
			} else {
				require.EqualValues(t, 1, revision)
			}
		})
	}
}

func TestReportVerifiedJournalBackfillCapturedGenerationRollback(t *testing.T) {
	for _, failedCheck := range []int{2, 3} {
		t.Run(map[int]string{2: "before_transaction", 3: "before_commit"}[failedCheck], func(t *testing.T) {
			pool, reports, _, subject := missingVerifiedReporter(t)
			ctx := context.Background()
			_, err := pool.Exec(ctx, `DELETE FROM automod_subjects WHERE subject_id=$1`, subject.ID)
			require.NoError(t, err)
			checks := 0
			acquire := func(context.Context) (store.ReportPrivacyCheck, error) {
				return func(context.Context) error {
					checks++
					if checks >= failedCheck {
						return store.ErrReportPrivacy
					}
					return nil
				}, nil
			}
			resolver := reportSubjectResolverFunc(func(_ context.Context, account, expected uuid.UUID) (*automodv1.ResolveModerationSubjectResponse, error) {
				return reportSubjectResponse(account, expected), nil
			})
			_, err = reports.BackfillVerifiedReportJournal(ctx, resolver, acquire, 10)
			require.ErrorIs(t, err, store.ErrReportPrivacy)
			require.Equal(t, failedCheck, checks)
			for _, query := range []string{`SELECT count(*) FROM report_privacy_journal`, `SELECT count(*) FROM automod_subjects`, `SELECT count(*) FROM report_intake_outbox`} {
				var count int
				require.NoError(t, pool.QueryRow(ctx, query).Scan(&count))
				require.Zero(t, count)
			}
		})
	}
}

func TestReportVerifiedJournalBackfillDeadlineCrossingRollsBack(t *testing.T) {
	pool, reports, row, subject := missingVerifiedReporter(t)
	ctx := context.Background()
	holder, err := pool.Begin(ctx)
	require.NoError(t, err)
	_, err = holder.Exec(ctx, `SELECT id FROM reports WHERE id=$1 FOR UPDATE`, row.ID)
	require.NoError(t, err)
	workCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	done := make(chan error, 1)
	gotResult := false
	t.Cleanup(func() {
		_ = holder.Rollback(ctx)
		cancel()
		if !gotResult {
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Error("backfill did not stop after cancellation")
			}
		}
	})
	deadlineReady := make(chan time.Time, 1)
	resolver := reportSubjectResolverFunc(func(callCtx context.Context, account, expected uuid.UUID) (*automodv1.ResolveModerationSubjectResponse, error) {
		var deadline time.Time
		if err := pool.QueryRow(callCtx, `SELECT clock_timestamp()+interval '5 seconds'`).Scan(&deadline); err != nil {
			return nil, err
		}
		deadlineReady <- deadline
		cycle := uuid.NewString()
		response := reportSubjectResponse(account, expected)
		response.Subject.LifecycleRevision = 2
		response.Subject.State = automodv1.SubjectState_SUBJECT_STATE_DELETED
		response.Subject.DeletionCycleId = &cycle
		response.Subject.PurgeAt = timestamppb.New(deadline)
		response.AuthRestricted = true
		return response, nil
	})
	go func() {
		_, workErr := reports.BackfillVerifiedReportJournal(workCtx, resolver, reportOpenPrivacy, 10)
		done <- workErr
	}()
	require.Eventually(t, func() bool {
		var waiting bool
		queryErr := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock'
 AND query LIKE '%reporter_subject_revision%FOR UPDATE%')`).Scan(&waiting)
		return queryErr == nil && waiting
	}, 10*time.Second, 50*time.Millisecond, "retained binding must reach report lock before deadline")
	var deadline time.Time
	select {
	case deadline = <-deadlineReady:
	case <-workCtx.Done():
		t.Fatal("authority did not return its captured deadline")
	}
	require.Eventually(t, func() bool {
		var expired bool
		queryErr := pool.QueryRow(ctx, `SELECT clock_timestamp()>$1::timestamptz`, deadline).Scan(&expired)
		return queryErr == nil && expired
	}, 10*time.Second, 25*time.Millisecond)
	require.NoError(t, holder.Commit(ctx))
	select {
	case err = <-done:
		gotResult = true
	case <-workCtx.Done():
		t.Fatal("deadline recheck did not finish")
	}
	require.ErrorIs(t, err, store.ErrReportPrivacy)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM report_privacy_journal`).Scan(&count))
	require.Zero(t, count)
	var state string
	var revision int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT lifecycle_state,lifecycle_revision FROM automod_subjects WHERE subject_id=$1`, subject.ID).Scan(&state, &revision))
	require.Equal(t, "LIVE", state)
	require.EqualValues(t, 1, revision, "expired captured DELETED cannot replace retained local history")
}

func TestReportVerifiedJournalBackfillRespectsNewOwnerFact(t *testing.T) {
	pool, reports, row, subject := missingVerifiedReporter(t)
	ctx := context.Background()
	fact := uuid.New()
	resolver := reportSubjectResolverFunc(func(callCtx context.Context, account, expected uuid.UUID) (*automodv1.ResolveModerationSubjectResponse, error) {
		// A concurrent, explicitly authorized privacy operation commits its
		// recovery-only class+owner fact while this worker has no SQL locks.
		tx, err := pool.Begin(callCtx)
		require.NoError(t, err)
		_, err = tx.Exec(callCtx, `SELECT pg_advisory_xact_lock(hashtextextended('moderation-account-sanction:' || $1::text,0))`, account.String())
		require.NoError(t, err)
		_, err = tx.Exec(callCtx, `SELECT pg_advisory_xact_lock(hashtextextended('moderation-subject-lifecycle:' || $1::text,0))`, expected.String())
		require.NoError(t, err)
		_, err = tx.Exec(callCtx, `UPDATE reports SET reporter_provenance='OWNER_RECOVERED_LEGACY' WHERE id=$1`, row.ID)
		require.NoError(t, err)
		require.NoError(t, reports.RecordReportRecoveryTx(callCtx, tx, store.ReportRecoveryRecord{ReportID: row.ID, Slot: "REPORTER", Disposition: "BOUND", SubjectID: &subject.ID, EvidenceKind: "USER_OWNER_FACT", EvidenceID: fact, EvidenceVersion: 1}))
		require.NoError(t, tx.Commit(callCtx))
		return reportSubjectResponse(account, expected), nil
	})
	_, err := reports.BackfillVerifiedReportJournal(ctx, resolver, reportOpenPrivacy, 10)
	require.ErrorIs(t, err, store.ErrReportPrivacy)
	var provenance, kind string
	var evidence uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT r.reporter_provenance,j.evidence_kind,j.evidence_id FROM reports r JOIN report_privacy_journal j ON j.report_id=r.id WHERE r.id=$1`, row.ID).Scan(&provenance, &kind, &evidence))
	require.Equal(t, "OWNER_RECOVERED_LEGACY", provenance)
	require.Equal(t, "USER_OWNER_FACT", kind)
	require.Equal(t, fact, evidence)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM report_intake_outbox`).Scan(&count))
	require.Zero(t, count)
}

func TestReportVerifiedJournalBackfillBoundedDeduplicatedAuthority(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	reports := newReportFixtureStore(pool)
	check, _ := reportOpenPrivacy(ctx)
	subject := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Revision: 1, State: "LIVE"}
	for range 3 {
		_, err := reports.InsertSubjectReport(ctx, uuid.New(), subject, check, "story", uuid.New(), "spam", nil, `{}`)
		require.NoError(t, err)
	}
	_, err := pool.Exec(ctx, `DELETE FROM report_privacy_journal`)
	require.NoError(t, err)
	calls := 0
	resolver := reportSubjectResolverFunc(func(_ context.Context, account, expected uuid.UUID) (*automodv1.ResolveModerationSubjectResponse, error) {
		calls++
		require.Equal(t, subject.AccountID, account)
		require.Equal(t, subject.ID, expected)
		return reportSubjectResponse(account, expected), nil
	})
	batch, err := reports.BackfillVerifiedReportJournal(ctx, resolver, reportOpenPrivacy, 2)
	require.NoError(t, err)
	require.Equal(t, 2, batch.Journaled)
	require.True(t, batch.More)
	require.Equal(t, 1, calls)
	batch, err = reports.BackfillVerifiedReportJournal(ctx, resolver, reportOpenPrivacy, 2)
	require.NoError(t, err)
	require.Equal(t, 1, batch.Journaled)
	require.False(t, batch.More)
	require.Equal(t, 2, calls)
	batch, err = reports.BackfillVerifiedReportJournal(ctx, resolver, reportOpenPrivacy, 2)
	require.NoError(t, err)
	require.Zero(t, batch.Journaled)
	require.Equal(t, 2, calls)
	require.NoError(t, reports.RequirePrivacyRecovery(ctx))
}
