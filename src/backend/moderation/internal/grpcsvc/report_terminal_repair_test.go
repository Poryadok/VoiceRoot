package grpcsvc

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/moderation/internal/automod"
	"voice/backend/moderation/internal/store"
)

func TestReportTerminalReceiptRepairOldNullSlots(t *testing.T) {
	for _, slot := range []string{"REPORTER", "USER_TARGET"} {
		t.Run(slot, func(t *testing.T) {
			pool := reportIntakePool(t)
			ctx := context.Background()
			reports := newReportFixtureStore(pool)
			row, err := reports.InsertReport(ctx, uuid.New(), "user", uuid.New(), "spam", nil, `{}`)
			require.NoError(t, err)
			subject := uuid.New()
			_, err = pool.Exec(ctx, `INSERT INTO report_privacy_journal(report_id,slot,disposition,subject_id,evidence_kind,evidence_id,evidence_version)
 VALUES($1,$2,'BOUND',$3,'USER_OWNER_FACT',$4,1)`, row.ID, slot, subject, uuid.New())
			require.NoError(t, err)
			_, err = pool.Exec(ctx, `INSERT INTO automod_terminal_subjects(subject_id) VALUES($1)`, subject)
			require.NoError(t, err)
			require.ErrorIs(t, reports.RequirePrivacyRecovery(ctx), store.ErrReportPrivacy)
			batch, err := reports.RepairReportTerminalReceipts(ctx, reportOpenPrivacy, 10)
			require.NoError(t, err)
			require.Equal(t, 1, batch.Removed)
			require.False(t, batch.More)
			var count int
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM reports`).Scan(&count))
			require.Zero(t, count, "exact report-slot receipt cleans old NULL raw binding without guessing account")
			var disposition, kind string
			var evidence uuid.UUID
			require.NoError(t, pool.QueryRow(ctx, `SELECT disposition,evidence_kind,evidence_id FROM report_privacy_journal WHERE report_id=$1 AND slot=$2`, row.ID, slot).Scan(&disposition, &kind, &evidence))
			require.Equal(t, "REDACTED", disposition)
			require.Equal(t, "AUTH_TERMINAL", kind)
			require.Equal(t, subject, evidence)
			require.NoError(t, reports.RequirePrivacyRecovery(ctx))
			batch, err = reports.RepairReportTerminalReceipts(ctx, reportOpenPrivacy, 10)
			require.NoError(t, err)
			require.Zero(t, batch.Removed, "crash/restart replay is idempotent")
		})
	}
}

func TestReportTerminalReceiptRepairDoesNotInferAuthority(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	reports := newReportFixtureStore(pool)
	row, err := reports.InsertReport(ctx, uuid.New(), "story", uuid.New(), "spam", nil, `{}`)
	require.NoError(t, err)
	batch, err := reports.RepairReportTerminalReceipts(ctx, reportOpenPrivacy, 10)
	require.NoError(t, err)
	require.Zero(t, batch.Removed)
	subject := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Revision: 1, State: "LIVE"}
	require.NoError(t, reports.Subjects.BindSubject(ctx, subject))
	_, err = pool.Exec(ctx, `UPDATE automod_subjects SET lifecycle_state='DELETED',purge_at=clock_timestamp()-interval '1 second' WHERE subject_id=$1`, subject.ID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO report_privacy_journal(report_id,slot,disposition,subject_id,evidence_kind,evidence_id,evidence_version)
 VALUES($1,'REPORTER','BOUND',$2,'USER_OWNER_FACT',$3,1)`, row.ID, subject.ID, uuid.New())
	require.NoError(t, err)
	batch, err = reports.RepairReportTerminalReceipts(ctx, reportOpenPrivacy, 10)
	require.NoError(t, err)
	require.Zero(t, batch.Removed, "expired local DELETED is not positive terminal authority")
	_, err = pool.Exec(ctx, `UPDATE report_privacy_journal SET disposition='REDACTED',evidence_kind='AUTH_TERMINAL',evidence_id=$2 WHERE report_id=$1`, row.ID, subject.ID)
	require.NoError(t, err)
	batch, err = reports.RepairReportTerminalReceipts(ctx, reportOpenPrivacy, 10)
	require.NoError(t, err)
	require.Zero(t, batch.Removed, "receipt without actual terminal union cannot fabricate owning fence")
	_, err = reports.RepairReportTerminalReceipts(ctx, nil, 10)
	require.ErrorIs(t, err, store.ErrReportPrivacy)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM reports WHERE id=$1`, row.ID).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_terminal_subjects`).Scan(&count))
	require.Zero(t, count)
}

func TestReportTerminalReceiptRepairGenerationAndBound(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	reports := newReportFixtureStore(pool)
	for range 2 {
		row, err := reports.InsertReport(ctx, uuid.New(), "story", uuid.New(), "spam", nil, `{}`)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `INSERT INTO report_privacy_journal(report_id,slot,disposition,evidence_kind,evidence_id,evidence_version)
 VALUES($1,'REPORTER','REDACTED','USER_ERASURE_FACT',$2,1)`, row.ID, uuid.New())
		require.NoError(t, err) // Explicit positive canonical erasure fact fixture.
	}
	checks := 0
	closing := func(context.Context) (store.ReportPrivacyCheck, error) {
		return func(context.Context) error {
			checks++
			if checks > 1 {
				return store.ErrReportPrivacy
			}
			return nil
		}, nil
	}
	_, err := reports.RepairReportTerminalReceipts(ctx, closing, 1)
	require.ErrorIs(t, err, store.ErrReportPrivacy)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM reports`).Scan(&count))
	require.Equal(t, 2, count, "generation closure rolls back raw deletion")
	batch, err := reports.RepairReportTerminalReceipts(ctx, reportOpenPrivacy, 1)
	require.NoError(t, err)
	require.Equal(t, 1, batch.Removed)
	require.True(t, batch.More)
	batch, err = reports.RepairReportTerminalReceipts(ctx, reportOpenPrivacy, 1)
	require.NoError(t, err)
	require.Equal(t, 1, batch.Removed)
	require.False(t, batch.More)
	require.NoError(t, reports.RequirePrivacyRecovery(ctx))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_terminal_subjects`).Scan(&count))
	require.Zero(t, count, "profile erasure receipt does not manufacture account terminal authority")
}

func TestReportTerminalReceiptRepairConflictRollsBackBatch(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	reports := newReportFixtureStore(pool)
	var ids []uuid.UUID
	for range 2 {
		row, err := reports.InsertReport(ctx, uuid.New(), "story", uuid.New(), "spam", nil, `{}`)
		require.NoError(t, err)
		ids = append(ids, row.ID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	for _, id := range ids {
		subject := uuid.New()
		_, err := pool.Exec(ctx, `INSERT INTO report_privacy_journal(report_id,slot,disposition,subject_id,evidence_kind,evidence_id,evidence_version)
 VALUES($1,'REPORTER','BOUND',$2,'USER_OWNER_FACT',$3,1)`, id, subject, uuid.New())
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `INSERT INTO automod_terminal_subjects(subject_id) VALUES($1)`, subject)
		require.NoError(t, err)
	}
	_, err := pool.Exec(ctx, `UPDATE reports SET reporter_account_id=$2,reporter_subject_id=$3,
 reporter_subject_revision=1,reporter_provenance='OWNER_RECOVERED_LEGACY' WHERE id=$1`, ids[1], uuid.New(), uuid.New())
	require.NoError(t, err) // Deliberately conflicting restored row, not an owner inference.
	_, err = reports.RepairReportTerminalReceipts(ctx, reportOpenPrivacy, 10)
	require.ErrorIs(t, err, store.ErrReportRecoveryDivergence)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM reports`).Scan(&count))
	require.Equal(t, 2, count)
	var disposition string
	require.NoError(t, pool.QueryRow(ctx, `SELECT disposition FROM report_privacy_journal WHERE report_id=$1`, ids[0]).Scan(&disposition))
	require.Equal(t, "BOUND", disposition, "earlier journal terminalization must roll back with later conflicting row")
}

func TestReportTerminalReceiptRepairErasureIsExactReport(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	reports := newReportFixtureStore(pool)
	profile := uuid.New()
	first, err := reports.InsertReport(ctx, profile, "story", uuid.New(), "spam", nil, `{}`)
	require.NoError(t, err)
	second, err := reports.InsertReport(ctx, profile, "story", uuid.New(), "spam", nil, `{}`)
	require.NoError(t, err)
	fact := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO report_privacy_journal(report_id,slot,disposition,evidence_kind,evidence_id,evidence_version)
 VALUES($1,'REPORTER','REDACTED','USER_ERASURE_FACT',$2,9)`, first.ID, fact)
	require.NoError(t, err)
	batch, err := reports.RepairReportTerminalReceipts(ctx, reportOpenPrivacy, 10)
	require.NoError(t, err)
	require.Equal(t, 1, batch.Removed)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM reports WHERE id=$1`, second.ID).Scan(&count))
	require.Equal(t, 1, count, "a receipt for one report UUID cannot authorize a profile-wide guessed sweep")
	var gotFact uuid.UUID
	var version int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT evidence_id,evidence_version FROM report_privacy_journal WHERE report_id=$1`, first.ID).Scan(&gotFact, &version))
	require.Equal(t, fact, gotFact)
	require.EqualValues(t, 9, version)
	require.ErrorIs(t, reports.RequirePrivacyRecovery(ctx), store.ErrReportPrivacy, "remaining unknown history stays unresolved")
}

func TestReportTerminalReceiptRepairRetriesChangedObservation(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	reports := newReportFixtureStore(pool)
	row, err := reports.InsertReport(ctx, uuid.New(), "story", uuid.New(), "spam", nil, `{}`)
	require.NoError(t, err)
	account, subject := uuid.New(), uuid.New()
	_, err = pool.Exec(ctx, `UPDATE reports SET reporter_account_id=$2,reporter_subject_id=$3,
 reporter_subject_revision=1,reporter_provenance='OWNER_RECOVERED_LEGACY' WHERE id=$1`, row.ID, account, subject)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO report_privacy_journal(report_id,slot,disposition,evidence_kind,evidence_id,evidence_version)
 VALUES($1,'REPORTER','REDACTED','USER_ERASURE_FACT',$2,1)`, row.ID, uuid.New())
	require.NoError(t, err)
	holder, err := pool.Begin(ctx)
	require.NoError(t, err)
	_, err = holder.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('moderation-account-sanction:' || $1::text,0))`, account.String())
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
				t.Error("repair did not stop after cancellation")
			}
		}
	})
	go func() {
		_, repairErr := reports.RepairReportTerminalReceipts(workCtx, reportOpenPrivacy, 10)
		done <- repairErr
	}()
	require.Eventually(t, func() bool {
		var waiting bool
		queryErr := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database()
 AND wait_event_type='Lock' AND wait_event='advisory' AND query LIKE '%moderation-account-sanction:%')`).Scan(&waiting)
		return queryErr == nil && waiting
	}, 10*time.Second, 50*time.Millisecond, "repair must finish its optimistic read then wait for the original account lock")
	// Controlled stale-observation injection while holding the original account
	// lock; repair must not erase or expand its lock set to this newly seen tuple.
	_, err = holder.Exec(ctx, `UPDATE reports SET reporter_account_id=$2,reporter_subject_id=$3 WHERE id=$1`, row.ID, uuid.New(), uuid.New())
	require.NoError(t, err)
	require.NoError(t, holder.Commit(ctx))
	select {
	case err = <-done:
		gotResult = true
	case <-workCtx.Done():
		t.Fatal("repair failed to finish bounded retry")
	}
	require.ErrorIs(t, err, store.ErrReportPrivacy)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM reports WHERE id=$1`, row.ID).Scan(&count))
	require.Equal(t, 1, count, "changed snapshot must retry with a new complete account/subject lock set")
}
