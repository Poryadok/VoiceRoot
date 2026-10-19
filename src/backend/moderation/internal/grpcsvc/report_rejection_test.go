package grpcsvc

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/moderation/internal/automod"
	"voice/backend/moderation/internal/reportrecovery"
	"voice/backend/moderation/internal/store"
	automodv1 "voice/backend/pkg/pb/voice/automod/v1"
)

func TestReportUserRejectionMinimizesOnlyCompletedIntake(t *testing.T) {
	for _, decision := range []string{"MISSING", "DELETED"} {
		t.Run(decision, func(t *testing.T) {
			pool := reportIntakePool(t)
			ctx := context.Background()
			reports := newReportFixtureStore(pool)
			target := uuid.New()
			description := "unnecessary rejected target body"
			row, err := insertReportFixture(t, reports, ctx, uuid.New(), "user", target, "spam", &description, `{"private":true}`)
			require.NoError(t, err)
			claim, err := reports.ClaimValidation(ctx)
			require.NoError(t, err)
			applied, err := completeReportFixture(t, reports, ctx, claim, decision, uuid.Nil, nil, nil)
			require.NoError(t, err)
			require.True(t, applied)
			var rawTarget *uuid.UUID
			require.NoError(t, pool.QueryRow(ctx, `SELECT target_id FROM reports WHERE id=$1`, row.ID).Scan(&rawTarget))
			require.Nil(t, rawTarget, "rejection must remove the requested User target UUID")
			stored, err := reports.GetReportByID(ctx, row.ID)
			require.NoError(t, err)
			require.Nil(t, stored.TargetID)
			require.Empty(t, reportRowToProto(stored).TargetId, "NULL is not a zero UUID surrogate")
			var outcome, provenance string
			require.NoError(t, pool.QueryRow(ctx, `SELECT decision,provenance FROM report_target_rejections WHERE report_id=$1`, row.ID).Scan(&outcome, &provenance))
			require.Equal(t, decision, outcome)
			require.Equal(t, "OWNER_VALIDATION", provenance)
			require.NoError(t, reports.RequirePrivacyRecovery(ctx))
			var count int
			for _, table := range []string{"report_intake_outbox", "automod_terminal_subjects"} {
				require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM `+table).Scan(&count))
				require.Zero(t, count)
			}
			applied, err = completeReportFixture(t, reports, ctx, claim, "LIVE", uuid.New(), retentionEvents(), nil)
			require.NoError(t, err)
			require.False(t, applied, "late LIVE cannot reopen this intake")
			_, err = insertReportFixture(t, reports, ctx, uuid.New(), "user", target, "spam", nil, `{}`)
			require.NoError(t, err, "a new report may still request the same real target")
			fresh, err := reports.ClaimValidation(ctx)
			require.NoError(t, err)
			applied, err = completeReportFixture(t, reports, ctx, fresh, "LIVE", uuid.New(), retentionEvents(), nil)
			require.NoError(t, err)
			require.True(t, applied, "only the new intake can become valid")
			_, err = pool.Exec(ctx, `DELETE FROM reports WHERE id=$1`, row.ID)
			require.NoError(t, err)
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM report_target_rejections WHERE report_id=$1`, row.ID).Scan(&count))
			require.Equal(t, 1, count, "receipt survives raw report deletion")
		})
	}
}

func TestReportUserRejectionLeaseExpiresBehindRowLock(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	reports := newReportFixtureStore(pool)
	row, err := insertReportFixture(t, reports, ctx, uuid.New(), "user", uuid.New(), "spam", nil, `{}`)
	require.NoError(t, err)
	claim, err := reports.ClaimValidation(ctx)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE reports SET validation_claim_until=clock_timestamp()+interval '3 seconds' WHERE id=$1`, row.ID)
	require.NoError(t, err)
	reporter, _, err := store.ResolveReportSubject(ctx, reportFixtureResolver(), *claim.ReporterAccountID, *claim.ReporterSubjectID)
	require.NoError(t, err)
	claim.ReporterSubject = &reporter
	claim.PrivacyCheck, err = reportOpenPrivacy(ctx)
	require.NoError(t, err)
	holder, err := pool.Begin(ctx)
	require.NoError(t, err)
	_, err = holder.Exec(ctx, `SELECT id FROM reports WHERE id=$1 FOR UPDATE`, row.ID)
	require.NoError(t, err)
	workCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	done := make(chan error, 1)
	received := false
	t.Cleanup(func() {
		_ = holder.Rollback(ctx)
		cancel()
		if !received {
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Error("completion did not stop")
			}
		}
	})
	go func() {
		_, e := reports.CompleteValidation(workCtx, claim, "MISSING", uuid.Nil, nil, nil)
		done <- e
	}()
	require.Eventually(t, func() bool {
		var waiting bool
		e := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%validation_claim_until>clock_timestamp()%')`).Scan(&waiting)
		return e == nil && waiting
	}, 10*time.Second, 25*time.Millisecond)
	require.Eventually(t, func() bool {
		var expired bool
		e := pool.QueryRow(ctx, `SELECT validation_claim_until<=clock_timestamp() FROM reports WHERE id=$1`, row.ID).Scan(&expired)
		return e == nil && expired
	}, 10*time.Second, 25*time.Millisecond)
	require.NoError(t, holder.Commit(ctx))
	select {
	case err = <-done:
		received = true
	case <-workCtx.Done():
		t.Fatal("completion exceeded bounded fixture")
	}
	require.ErrorIs(t, err, store.ErrReportPrivacy)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM report_target_rejections`).Scan(&count))
	require.Zero(t, count)
}

func TestReportUserRejectionSnapshotReplay(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	reports := newReportFixtureStore(pool)
	row, err := insertReportFixture(t, reports, ctx, uuid.New(), "user", uuid.New(), "spam", nil, `{}`)
	require.NoError(t, err)
	claim, err := reports.ClaimValidation(ctx)
	require.NoError(t, err)
	_, err = completeReportFixture(t, reports, ctx, claim, "MISSING", uuid.Nil, nil, nil)
	require.NoError(t, err)
	artifact, err := reports.ExportReportRecoverySnapshot(ctx, uuid.New(), uuid.New())
	require.NoError(t, err)
	require.Equal(t, 2, artifact.Cut.View.Protocol)
	require.Equal(t, 2, artifact.Cut.View.RecordCount, "subject and rejection inventory both counted")
	snapshot, err := reportrecovery.Parse(artifact.Manifest, artifact.Chunks, artifact.Cut)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM report_target_rejections WHERE report_id=$1`, row.ID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE reports SET validation_state='PENDING',status='pending',target_id=$2,description='restored raw body',evidence='{"backup":true}',resolution=NULL WHERE id=$1`, row.ID, *row.TargetID)
	require.NoError(t, err)
	checks := 0
	closing := func(context.Context, reportrecovery.Cut) (store.ReportPrivacyCheck, error) {
		return func(context.Context) error {
			checks++
			if checks > 1 {
				return store.ErrReportPrivacy
			}
			return nil
		}, nil
	}
	require.ErrorIs(t, reports.ImportReportRecoverySnapshot(ctx, snapshot, closing), store.ErrReportPrivacy)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM report_target_rejections`).Scan(&count))
	require.Zero(t, count, "new family rolls back with generation closure")
	allow := func(_ context.Context, cut reportrecovery.Cut) (store.ReportPrivacyCheck, error) {
		require.Equal(t, artifact.Cut, cut)
		return reportOpenPrivacy(ctx)
	}
	require.NoError(t, reports.ImportReportRecoverySnapshot(ctx, snapshot, allow))
	require.ErrorIs(t, reports.RequirePrivacyRecovery(ctx), store.ErrReportPrivacy, "journal-only import cannot itself repair raw backup")
	_, err = reports.RepairReportTargetRejections(ctx, reportOpenPrivacy, 10)
	require.NoError(t, err)
	require.NoError(t, reports.RequirePrivacyRecovery(ctx))
	require.NoError(t, reports.ImportReportRecoverySnapshot(ctx, snapshot, allow))
	_, err = pool.Exec(ctx, `DELETE FROM reports WHERE id=$1`, row.ID)
	require.NoError(t, err)
	require.NoError(t, reports.ImportReportRecoverySnapshot(ctx, snapshot, allow))
	_, err = reports.RepairReportTargetRejections(ctx, reportOpenPrivacy, 10)
	require.NoError(t, err)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM reports`).Scan(&count))
	require.Zero(t, count, "opaque union never resurrects raw report")
}

func TestReportUserRejectionCompletionRollsBackGeneration(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	reports := newReportFixtureStore(pool)
	row, err := insertReportFixture(t, reports, ctx, uuid.New(), "user", uuid.New(), "spam", nil, `{}`)
	require.NoError(t, err)
	claim, err := reports.ClaimValidation(ctx)
	require.NoError(t, err)
	reporter, _, err := store.ResolveReportSubject(ctx, reportFixtureResolver(), *claim.ReporterAccountID, *claim.ReporterSubjectID)
	require.NoError(t, err)
	claim.ReporterSubject = &reporter
	checks := 0
	claim.PrivacyCheck = func(context.Context) error {
		checks++
		if checks > 1 {
			return store.ErrReportPrivacy
		}
		return nil
	}
	_, err = reports.CompleteValidation(ctx, claim, "MISSING", uuid.Nil, nil, nil)
	require.ErrorIs(t, err, store.ErrReportPrivacy)
	var raw *uuid.UUID
	var state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT target_id,validation_state FROM reports WHERE id=$1`, row.ID).Scan(&raw, &state))
	require.Equal(t, row.TargetID, raw)
	require.Equal(t, "PENDING", state)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM report_target_rejections`).Scan(&count))
	require.Zero(t, count)
}

func TestReportUserRejectionKeepsIndependentSubjectAuthority(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	reports := newReportFixtureStore(pool)
	row, err := insertReportFixture(t, reports, ctx, uuid.New(), "user", uuid.New(), "spam", nil, `{}`)
	require.NoError(t, err)
	target := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Revision: 1, State: "LIVE"}
	require.NoError(t, reports.Subjects.BindSubject(ctx, target))
	_, err = pool.Exec(ctx, `INSERT INTO report_privacy_journal(report_id,slot,disposition,subject_id,evidence_kind,evidence_id,evidence_version) VALUES($1,'USER_TARGET','BOUND',$2,'AUTH_SUBJECT',$2,1)`, row.ID, target.ID)
	require.NoError(t, err)
	claim, err := reports.ClaimValidation(ctx)
	require.NoError(t, err)
	_, err = completeReportFixture(t, reports, ctx, claim, "DELETED", uuid.Nil, nil, nil)
	require.NoError(t, err)
	require.NoError(t, reports.RequirePrivacyRecovery(ctx))
	_, err = pool.Exec(ctx, `INSERT INTO automod_terminal_subjects(subject_id) VALUES($1)`, target.ID)
	require.NoError(t, err)
	require.ErrorIs(t, reports.RequirePrivacyRecovery(ctx), store.ErrReportPrivacy)
	_, err = reports.RepairReportTargetRejections(ctx, reportOpenPrivacy, 10)
	require.NoError(t, err) // Already minimized; no inferred cleanup.
	batch, err := reports.RepairReportTerminalReceipts(ctx, reportOpenPrivacy, 10)
	require.NoError(t, err)
	require.Equal(t, 1, batch.Removed)
	require.NoError(t, reports.RequirePrivacyRecovery(ctx))
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM report_target_rejections WHERE report_id=$1`, row.ID).Scan(&count))
	require.Equal(t, 1, count)
}

func TestReportUserRejectionBackfillNeverReattachesTarget(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	reports := newReportFixtureStore(pool)
	row, err := insertReportFixture(t, reports, ctx, uuid.New(), "user", uuid.New(), "spam", nil, `{}`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE reports SET validated_target_account_id=$2,validated_target_subject_id=$3,validated_target_subject_revision=1,validated_target_provenance='VERIFIED_AT_VALIDATION' WHERE id=$1`, row.ID, uuid.New(), uuid.New())
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO report_target_rejections(report_id,decision,provenance,schema_version) VALUES($1,'MISSING','OWNER_VALIDATION',1)`, row.ID)
	require.NoError(t, err)
	resolver := reportSubjectResolverFunc(func(context.Context, uuid.UUID, uuid.UUID) (*automodv1.ResolveModerationSubjectResponse, error) {
		t.Fatal("rejected target must never be rebound by modern backfill")
		return nil, nil
	})
	batch, err := reports.BackfillVerifiedReportJournal(ctx, resolver, reportOpenPrivacy, 10)
	require.NoError(t, err)
	require.Zero(t, batch.Journaled)
	_, err = reports.RepairReportTargetRejections(ctx, reportOpenPrivacy, 10)
	require.NoError(t, err)
	require.NoError(t, reports.RequirePrivacyRecovery(ctx))
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM report_privacy_journal WHERE report_id=$1 AND slot='USER_TARGET'`, row.ID).Scan(&count))
	require.Zero(t, count)
}

func TestReportUserRejectionPrivateRepair(t *testing.T) {
	for _, state := range []string{"MISSING", "DELETED", "PENDING"} {
		t.Run(state, func(t *testing.T) {
			pool := reportIntakePool(t)
			ctx := context.Background()
			reports := newReportFixtureStore(pool)
			body := "old backup body"
			row, err := insertReportFixture(t, reports, ctx, uuid.New(), "user", uuid.New(), "spam", &body, `{"old":true}`)
			require.NoError(t, err)
			if state != "PENDING" {
				// The pre-cutover old writer could persist a rejected raw target.
				_, err = pool.Exec(ctx, `ALTER TABLE reports DROP CONSTRAINT reports_target_representation`)
				require.NoError(t, err)
				_, err = pool.Exec(ctx, `UPDATE reports SET validation_state=$2,status='dismissed' WHERE id=$1`, row.ID, state)
				require.NoError(t, err)
			} else {
				_, err = pool.Exec(ctx, `INSERT INTO report_target_rejections(report_id,decision,provenance,schema_version) VALUES($1,'MISSING','OWNER_VALIDATION',1)`, row.ID)
				require.NoError(t, err)
			}
			require.ErrorIs(t, reports.RequirePrivacyRecovery(ctx), store.ErrReportPrivacy)
			batch, err := reports.RepairReportTargetRejections(ctx, reportOpenPrivacy, 1)
			require.NoError(t, err)
			require.Equal(t, 1, batch.Minimized)
			require.False(t, batch.More)
			require.NoError(t, reports.RequirePrivacyRecovery(ctx))
			var decision, provenance string
			require.NoError(t, pool.QueryRow(ctx, `SELECT decision,provenance FROM report_target_rejections WHERE report_id=$1`, row.ID).Scan(&decision, &provenance))
			if state == "PENDING" {
				require.Equal(t, "MISSING", decision)
				require.Equal(t, "OWNER_VALIDATION", provenance)
			} else {
				require.Equal(t, state, decision)
				require.Equal(t, "LEGACY_RECORDED_DECISION", provenance)
			}
			batch, err = reports.RepairReportTargetRejections(ctx, reportOpenPrivacy, 1)
			require.NoError(t, err)
			require.Zero(t, batch.Minimized)
			_, err = reports.ClaimValidation(ctx)
			require.Error(t, err, "old intake cannot reopen")
		})
	}
}

func TestReportUserRejectionRepairConflictsAndRollback(t *testing.T) {
	for _, state := range []string{"VALID", "DELETED", "OTHER_TYPE", "GENERATION"} {
		t.Run(state, func(t *testing.T) {
			pool := reportIntakePool(t)
			ctx := context.Background()
			reports := newReportFixtureStore(pool)
			kind := "user"
			if state == "OTHER_TYPE" {
				kind = "story"
			}
			row, err := insertReportFixture(t, reports, ctx, uuid.New(), kind, uuid.New(), "spam", nil, `{}`)
			require.NoError(t, err)
			if state == "VALID" {
				_, err = pool.Exec(ctx, `UPDATE reports SET validation_state='VALID' WHERE id=$1`, row.ID)
				require.NoError(t, err)
			}
			if state == "DELETED" {
				_, err = pool.Exec(ctx, `ALTER TABLE reports DROP CONSTRAINT reports_target_representation`)
				require.NoError(t, err)
				_, err = pool.Exec(ctx, `UPDATE reports SET validation_state='DELETED',status='dismissed' WHERE id=$1`, row.ID)
				require.NoError(t, err)
			}
			_, err = pool.Exec(ctx, `INSERT INTO report_target_rejections(report_id,decision,provenance,schema_version) VALUES($1,'MISSING','OWNER_VALIDATION',1)`, row.ID)
			require.NoError(t, err)
			acquire := store.ReportPrivacyAcquire(reportOpenPrivacy)
			if state == "GENERATION" {
				checks := 0
				acquire = func(context.Context) (store.ReportPrivacyCheck, error) {
					return func(context.Context) error {
						checks++
						if checks > 1 {
							return store.ErrReportPrivacy
						}
						return nil
					}, nil
				}
			}
			_, err = reports.RepairReportTargetRejections(ctx, acquire, 10)
			if state == "GENERATION" {
				require.ErrorIs(t, err, store.ErrReportPrivacy)
			} else {
				require.ErrorIs(t, err, store.ErrReportRecoveryDivergence)
			}
			var target *uuid.UUID
			require.NoError(t, pool.QueryRow(ctx, `SELECT target_id FROM reports WHERE id=$1`, row.ID).Scan(&target))
			require.Equal(t, row.TargetID, target, "failure rolls back minimization")
		})
	}
}

func TestReportUserRejectionCheckerRequiresReceipt(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	reports := newReportFixtureStore(pool)
	row, err := insertReportFixture(t, reports, ctx, uuid.New(), "user", uuid.New(), "spam", nil, `{}`)
	require.NoError(t, err)
	claim, err := reports.ClaimValidation(ctx)
	require.NoError(t, err)
	_, err = completeReportFixture(t, reports, ctx, claim, "MISSING", uuid.Nil, nil, nil)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM report_target_rejections WHERE report_id=$1`, row.ID)
	require.NoError(t, err)
	require.ErrorIs(t, reports.RequirePrivacyRecovery(ctx), store.ErrReportPrivacy)
}

func TestReportUserRejectionRestoredPendingCannotActivate(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	reports := newReportFixtureStore(pool)
	row, err := insertReportFixture(t, reports, ctx, uuid.New(), "user", uuid.New(), "spam", nil, `{}`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO report_target_rejections(report_id,decision,provenance,schema_version) VALUES($1,'MISSING','OWNER_VALIDATION',1)`, row.ID)
	require.NoError(t, err)
	claim, err := reports.ClaimValidation(ctx)
	require.NoError(t, err)
	applied, err := completeReportFixture(t, reports, ctx, claim, "LIVE", uuid.New(), retentionEvents(), nil)
	require.ErrorIs(t, err, store.ErrReportPrivacy)
	require.False(t, applied)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM report_intake_outbox`).Scan(&count))
	require.Zero(t, count)
}

func TestReportUserRejectionImportConflictingDecision(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	reports := newReportFixtureStore(pool)
	row, err := insertReportFixture(t, reports, ctx, uuid.New(), "user", uuid.New(), "spam", nil, `{}`)
	require.NoError(t, err)
	claim, err := reports.ClaimValidation(ctx)
	require.NoError(t, err)
	_, err = completeReportFixture(t, reports, ctx, claim, "MISSING", uuid.Nil, nil, nil)
	require.NoError(t, err)
	artifact, err := reports.ExportReportRecoverySnapshot(ctx, uuid.New(), uuid.New())
	require.NoError(t, err)
	snapshot, err := reportrecovery.Parse(artifact.Manifest, artifact.Chunks, artifact.Cut)
	require.NoError(t, err)
	for _, decision := range []string{"DELETED", "MISSING"} {
		records := snapshot.Records()
		for i := range records {
			if records[i].Rejection != nil {
				records[i].Rejection.Decision = decision
				records[i].Rejection.Provenance = "LEGACY_RECORDED_DECISION"
			}
		}
		view := artifact.Cut.View
		view.SnapshotID = uuid.New()
		manifest, chunks, cut, e := reportrecovery.Encode(view, records)
		require.NoError(t, e)
		incoming, e := reportrecovery.Parse(manifest, chunks, cut)
		require.NoError(t, e)
		allow := func(_ context.Context, actual reportrecovery.Cut) (store.ReportPrivacyCheck, error) {
			require.Equal(t, cut, actual)
			return reportOpenPrivacy(ctx)
		}
		e = reports.ImportReportRecoverySnapshot(ctx, incoming, allow)
		if decision == "DELETED" {
			require.ErrorIs(t, e, store.ErrReportRecoveryDivergence)
		} else {
			require.NoError(t, e)
		}
		var storedDecision, storedProvenance string
		require.NoError(t, pool.QueryRow(ctx, `SELECT decision,provenance FROM report_target_rejections WHERE report_id=$1`, row.ID).Scan(&storedDecision, &storedProvenance))
		require.Equal(t, "MISSING", storedDecision)
		require.Equal(t, "OWNER_VALIDATION", storedProvenance)
	}
	require.NoError(t, reports.RequirePrivacyRecovery(ctx))
}
