package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/moderation/internal/reportrecovery"
	"voice/backend/moderation/internal/store"
)

func TestReportJournalExportFiniteCommittedObservation(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	reports := newReportFixtureStore(pool)
	incarnation, operation := uuid.New(), uuid.New()
	empty, err := reports.ExportReportRecoverySnapshot(ctx, incarnation, operation)
	require.NoError(t, err)
	parsed, err := reportrecovery.Parse(empty.Manifest, empty.Chunks, empty.Cut)
	require.NoError(t, err)
	require.Empty(t, parsed.Records())
	require.Equal(t, incarnation, empty.Cut.View.StoreIncarnation)
	require.Equal(t, operation, empty.Cut.View.OperationID)
	require.NotEqual(t, uuid.Nil, empty.Cut.View.JournalGeneration)
	row, err := reports.InsertReport(ctx, uuid.New(), "story", uuid.New(), "spam", nil, `{}`)
	require.NoError(t, err)
	subject := uuid.New()
	record := store.ReportRecoveryRecord{ReportID: row.ID, Slot: "REPORTER", Disposition: "BOUND", SubjectID: &subject, EvidenceKind: "USER_OWNER_FACT", EvidenceID: uuid.New(), EvidenceVersion: 3}
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback(ctx) })
	require.NoError(t, reports.RecordReportRecoveryTx(ctx, tx, record))
	uncommitted, err := reports.ExportReportRecoverySnapshot(ctx, incarnation, operation)
	require.NoError(t, err)
	require.Zero(t, uncommitted.Cut.View.RecordCount, "uncommitted report mutation cannot escape its transaction")
	require.NoError(t, tx.Commit(ctx))
	committed, err := reports.ExportReportRecoverySnapshot(ctx, incarnation, operation)
	require.NoError(t, err)
	parsed, err = reportrecovery.Parse(committed.Manifest, committed.Chunks, committed.Cut)
	require.NoError(t, err)
	require.Len(t, parsed.Records(), 1)
	require.Equal(t, row.ID, parsed.Records()[0].ReportID)
	require.Equal(t, subject, *parsed.Records()[0].SubjectID)
	require.Equal(t, empty.Cut.View.JournalGeneration, committed.Cut.View.JournalGeneration)
	require.NotEqual(t, empty.Cut.View.SnapshotID, committed.Cut.View.SnapshotID)
	require.NotEqual(t, uncommitted.Cut.View.ViewID, committed.Cut.View.ViewID)
	require.Positive(t, committed.Cut.View.DiagnosticHighWater)
	_, err = reports.ExportReportRecoverySnapshot(ctx, uuid.Nil, operation)
	require.Error(t, err)
	_, err = reports.ExportReportRecoverySnapshot(ctx, incarnation, uuid.Nil)
	require.Error(t, err)
}

func TestReportJournalExportRejectsCorruptAuthorityTuple(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	reports := newReportFixtureStore(pool)
	_, err := pool.Exec(ctx, `INSERT INTO report_privacy_journal(report_id,slot,disposition,evidence_kind,evidence_id,evidence_version)
 VALUES($1,'REPORTER','REDACTED','AUTH_TERMINAL',$2,1)`, uuid.New(), uuid.New())
	require.NoError(t, err)
	artifact, err := reports.ExportReportRecoverySnapshot(ctx, uuid.New(), uuid.New())
	require.Error(t, err, "terminal subject must be explicit and agree with its evidence")
	require.Nil(t, artifact, "corruption cannot publish a partial snapshot")
}
