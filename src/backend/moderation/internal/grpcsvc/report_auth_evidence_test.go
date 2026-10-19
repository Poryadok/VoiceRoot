package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/moderation/internal/automod"
	"voice/backend/moderation/internal/store"
)

func TestReportJournalRejectsMismatchedAuthEvidence(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	reports := newReportFixtureStore(pool)
	row, err := reports.InsertReport(ctx, uuid.New(), "story", uuid.New(), "spam", nil, `{}`)
	require.NoError(t, err)
	subject := uuid.New()
	record := store.ReportRecoveryRecord{ReportID: row.ID, Slot: "REPORTER", Disposition: "BOUND", SubjectID: &subject, EvidenceKind: "AUTH_SUBJECT", EvidenceID: uuid.New(), EvidenceVersion: 1}
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback(ctx) })
	require.ErrorIs(t, reports.RecordReportRecoveryTx(ctx, tx, record), store.ErrReportPrivacy)
	require.NoError(t, tx.Rollback(ctx))
	record.EvidenceKind = "USER_OWNER_FACT"
	tx, err = pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, reports.RecordReportRecoveryTx(ctx, tx, record), "User owner authority has its own distinct immutable fact UUID")
	require.NoError(t, tx.Commit(ctx))
}

func TestReportPrivacyCheckerRejectsMismatchedAuthEvidence(t *testing.T) {
	for _, slot := range []string{"REPORTER", "USER_TARGET"} {
		t.Run(slot, func(t *testing.T) {
			pool := reportIntakePool(t)
			ctx := context.Background()
			reports := newReportFixtureStore(pool)
			check, _ := reportOpenPrivacy(ctx)
			reporter := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Revision: 1, State: "LIVE"}
			target := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Revision: 1, State: "LIVE"}
			row, err := reports.InsertSubjectReport(ctx, uuid.New(), reporter, check, "user", uuid.New(), "spam", nil, `{}`)
			require.NoError(t, err)
			claim, err := reports.ClaimValidation(ctx)
			require.NoError(t, err)
			claim.ReporterSubject, claim.TargetSubject, claim.PrivacyCheck = &reporter, &target, check
			applied, err := reports.CompleteValidation(ctx, claim, "LIVE", target.AccountID, retentionEvents(), nil)
			require.NoError(t, err)
			require.True(t, applied)
			_, err = pool.Exec(ctx, `UPDATE report_privacy_journal SET evidence_id=$3 WHERE report_id=$1 AND slot=$2`, row.ID, slot, uuid.New())
			require.NoError(t, err)
			require.ErrorIs(t, reports.RequirePrivacyRecovery(ctx), store.ErrReportPrivacy, "unrelated Auth evidence UUID cannot validate raw report")
			_, err = pool.Exec(ctx, `DELETE FROM reports WHERE id=$1`, row.ID)
			require.NoError(t, err)
			require.ErrorIs(t, reports.RequirePrivacyRecovery(ctx), store.ErrReportPrivacy, "orphan journal must also retain valid Auth evidence identity")
		})
	}
}
