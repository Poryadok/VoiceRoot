package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/moderation/internal/store"
)

func TestReportPrivacyJournalAtomicMonotonic(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	reports := newReportFixtureStore(pool)
	row, err := reports.InsertReport(ctx, uuid.New(), "story", uuid.New(), "spam", nil, `{}`)
	require.NoError(t, err)
	subject := uuid.New()
	record := store.ReportRecoveryRecord{ReportID: row.ID, Slot: "REPORTER", Disposition: "BOUND", SubjectID: &subject, EvidenceKind: "USER_OWNER_FACT", EvidenceID: uuid.New(), EvidenceVersion: 1}
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, reports.RecordReportRecoveryTx(ctx, tx, record))
	require.NoError(t, tx.Rollback(ctx))
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM report_privacy_journal`).Scan(&count))
	require.Zero(t, count, "journal cannot outlive a rolled-back report mutation")
	tx, err = pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, reports.RecordReportRecoveryTx(ctx, tx, record))
	require.NoError(t, tx.Commit(ctx))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM report_privacy_journal`).Scan(&count))
	require.Equal(t, 1, count, "committed binding must survive restart")
	// A different subject is divergence, never last-writer-wins ownership.
	other := uuid.New()
	conflict := record
	conflict.SubjectID = &other
	tx, err = pool.Begin(ctx)
	require.NoError(t, err)
	require.ErrorIs(t, reports.RecordReportRecoveryTx(ctx, tx, conflict), store.ErrReportRecoveryDivergence)
	require.NoError(t, tx.Rollback(ctx))
	terminal := record
	terminal.Disposition = "REDACTED"
	terminal.EvidenceKind = "AUTH_TERMINAL"
	terminal.EvidenceID = subject
	tx, err = pool.Begin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `INSERT INTO automod_terminal_subjects(subject_id) VALUES($1)`, subject)
	require.NoError(t, err)
	require.NoError(t, reports.RecordReportRecoveryTx(ctx, tx, terminal))
	_, err = tx.Exec(ctx, `DELETE FROM reports WHERE id=$1`, row.ID)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	tx, err = pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, reports.RecordReportRecoveryTx(ctx, tx, record), "older binding cannot resurrect a redacted slot")
	require.NoError(t, tx.Commit(ctx))
	var disposition string
	require.NoError(t, pool.QueryRow(ctx, `SELECT disposition FROM report_privacy_journal WHERE report_id=$1 AND slot='REPORTER'`, row.ID).Scan(&disposition))
	require.Equal(t, "REDACTED", disposition)
}
