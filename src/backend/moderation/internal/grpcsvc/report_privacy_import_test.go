package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/moderation/internal/reportrecovery"
	"voice/backend/moderation/internal/store"
)

func TestReportJournalImportAtomicMonotonicAndGeneration(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	reports := newReportFixtureStore(pool)
	first := uuid.MustParse("00000000-0000-4000-8000-000000000005")
	second := uuid.MustParse("00000000-0000-4000-8000-000000000006")
	subject := uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO report_privacy_journal(report_id,slot,disposition,subject_id,evidence_kind,evidence_id,evidence_version)
 VALUES($1,'REPORTER','BOUND',$3,'USER_OWNER_FACT',$4,1),($2,'REPORTER','BOUND',$3,'USER_OWNER_FACT',$4,1)`, first, second, subject, uuid.New())
	require.NoError(t, err)
	export, err := reports.ExportReportRecoverySnapshot(ctx, uuid.New(), uuid.New())
	require.NoError(t, err)
	snapshot, err := reportrecovery.Parse(export.Manifest, export.Chunks, export.Cut)
	require.NoError(t, err)
	authorize := func(_ context.Context, cut reportrecovery.Cut) (store.ReportPrivacyCheck, error) {
		require.Equal(t, export.Cut, cut, "coordinator must authorize the exact parsed bytes and identity")
		return func(context.Context) error { return nil }, nil
	}
	_, err = pool.Exec(ctx, `TRUNCATE report_privacy_journal`)
	require.NoError(t, err)
	require.ErrorIs(t, reports.ImportReportRecoverySnapshot(ctx, snapshot, nil), store.ErrReportPrivacy)
	require.ErrorIs(t, reports.ImportReportRecoverySnapshot(ctx, &reportrecovery.Snapshot{}, authorize), store.ErrReportPrivacy)
	require.NoError(t, reports.ImportReportRecoverySnapshot(ctx, snapshot, authorize))
	require.NoError(t, reports.ImportReportRecoverySnapshot(ctx, snapshot, authorize), "restart replay is idempotent")
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM report_privacy_journal`).Scan(&count))
	require.Equal(t, 2, count)
	_, err = pool.Exec(ctx, `UPDATE report_privacy_journal SET disposition='REDACTED',evidence_kind='USER_ERASURE_FACT' WHERE report_id=$1`, first)
	require.NoError(t, err)
	require.NoError(t, reports.ImportReportRecoverySnapshot(ctx, snapshot, authorize))
	var disposition string
	require.NoError(t, pool.QueryRow(ctx, `SELECT disposition FROM report_privacy_journal WHERE report_id=$1`, first).Scan(&disposition))
	require.Equal(t, "REDACTED", disposition, "older snapshot cannot regress terminal receipt")
	_, err = pool.Exec(ctx, `DELETE FROM report_privacy_journal WHERE report_id=$1`, first)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE report_privacy_journal SET subject_id=$1 WHERE report_id=$2`, uuid.New(), second)
	require.NoError(t, err)
	require.ErrorIs(t, reports.ImportReportRecoverySnapshot(ctx, snapshot, authorize), store.ErrReportRecoveryDivergence)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM report_privacy_journal WHERE report_id=$1`, first).Scan(&count))
	require.Zero(t, count, "later conflicting slot rolls back earlier insert")
	_, err = pool.Exec(ctx, `TRUNCATE report_privacy_journal`)
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
	require.Equal(t, 2, checks, "captured generation rechecked immediately before commit")
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM report_privacy_journal`).Scan(&count))
	require.Zero(t, count, "closed generation rolls back the complete union")
	require.NoError(t, reports.ImportReportRecoverySnapshot(ctx, snapshot, authorize))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM reports`).Scan(&count))
	require.Zero(t, count, "opaque import cannot resurrect profile/account/report payload")
}

func TestReportJournalImportTerminalReceiptIsNotOwningAuthority(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	reports := newReportFixtureStore(pool)
	report, subject := uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO report_privacy_journal(report_id,slot,disposition,subject_id,evidence_kind,evidence_id,evidence_version)
 VALUES($1,'REPORTER','REDACTED',$2,'AUTH_TERMINAL',$2,1)`, report, subject)
	require.NoError(t, err)
	export, err := reports.ExportReportRecoverySnapshot(ctx, uuid.New(), uuid.New())
	require.NoError(t, err)
	snapshot, err := reportrecovery.Parse(export.Manifest, export.Chunks, export.Cut)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE report_privacy_journal SET disposition='BOUND',evidence_kind='AUTH_SUBJECT' WHERE report_id=$1`, report)
	require.NoError(t, err)
	authorize := func(context.Context, reportrecovery.Cut) (store.ReportPrivacyCheck, error) {
		return func(context.Context) error { return nil }, nil
	}
	require.NoError(t, reports.ImportReportRecoverySnapshot(ctx, snapshot, authorize))
	var disposition string
	require.NoError(t, pool.QueryRow(ctx, `SELECT disposition FROM report_privacy_journal WHERE report_id=$1`, report).Scan(&disposition))
	require.Equal(t, "REDACTED", disposition, "authorized incoming terminal receipt dominates current binding")
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_terminal_subjects`).Scan(&count))
	require.Zero(t, count, "report receipt cannot manufacture owning Auth terminal facts")
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM reports`).Scan(&count))
	require.Zero(t, count, "journal-only import cannot restore raw payload")
}
