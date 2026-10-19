package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"voice/backend/moderation/internal/automod"
	"voice/backend/moderation/internal/store"
)

func TestReportProvenanceSeparatesPrivacyRecovery(t *testing.T) {
	t.Run("new_submission_and_validation", func(t *testing.T) {
		pool := reportIntakePool(t)
		ctx := context.Background()
		reports := newReportFixtureStore(pool)
		check, _ := reportOpenPrivacy(ctx)
		reporter := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Revision: 1, State: "LIVE"}
		target := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Revision: 1, State: "LIVE"}
		row, err := reports.InsertSubjectReport(ctx, uuid.New(), reporter, check, "user", uuid.New(), "spam", nil, `{}`)
		require.NoError(t, err)
		var provenance string
		require.NoError(t, pool.QueryRow(ctx, `SELECT reporter_provenance FROM reports WHERE id=$1`, row.ID).Scan(&provenance))
		require.Equal(t, "VERIFIED_AT_SUBMISSION", provenance)
		claim, err := reports.ClaimValidation(ctx)
		require.NoError(t, err)
		claim.ReporterSubject, claim.TargetSubject, claim.PrivacyCheck = &reporter, &target, check
		applied, err := reports.CompleteValidation(ctx, claim, "LIVE", target.AccountID, retentionEvents(), nil)
		require.NoError(t, err)
		require.True(t, applied)
		require.NoError(t, pool.QueryRow(ctx, `SELECT validated_target_provenance FROM reports WHERE id=$1`, row.ID).Scan(&provenance))
		require.Equal(t, "VERIFIED_AT_VALIDATION", provenance)
	})
	t.Run("recovered_legacy_is_not_submission", func(t *testing.T) {
		pool := reportIntakePool(t)
		ctx := context.Background()
		reports := newReportFixtureStore(pool)
		row, err := reports.InsertReport(ctx, uuid.New(), "story", uuid.New(), "spam", nil, `{}`)
		require.NoError(t, err)
		subject := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Revision: 1, State: "LIVE"}
		require.NoError(t, reports.Subjects.BindSubject(ctx, subject))
		_, err = pool.Exec(ctx, `UPDATE reports SET reporter_account_id=$2,reporter_subject_id=$3,reporter_subject_revision=1,reporter_provenance='OWNER_RECOVERED_LEGACY' WHERE id=$1`, row.ID, subject.AccountID, subject.ID)
		require.NoError(t, err) // Explicit verified privacy-authority fixture, not a production adapter.
		require.NoError(t, reports.RequirePrivacyRecovery(ctx))
		_, err = reports.ClaimValidation(ctx)
		require.ErrorIs(t, err, pgx.ErrNoRows, "privacy repair cannot promote historical pending intake")
		// A lease restored from an older process must not override the current
		// row's recovery-only provenance, even if its cached fact says verified.
		claimID := uuid.New()
		_, err = pool.Exec(ctx, `UPDATE reports SET validation_claim=$2,validation_claim_until=now()+interval '30 seconds' WHERE id=$1`, row.ID, claimID)
		require.NoError(t, err)
		revision := int64(1)
		claim := &store.ValidationClaim{ValidationFact: store.ValidationFact{ReportID: row.ID, ReporterProfileID: row.ReporterProfileID, ReporterAccountID: &subject.AccountID, ReporterSubjectID: &subject.ID, ReporterSubjectRevision: &revision, ReporterProvenance: "VERIFIED_AT_SUBMISSION", TargetType: row.TargetType, TargetID: row.TargetID, CreatedAt: row.CreatedAt}, ClaimID: claimID, ReporterSubject: &subject}
		claim.PrivacyCheck, _ = reportOpenPrivacy(ctx)
		applied, err := reports.CompleteValidation(ctx, claim, "LIVE", uuid.Nil, retentionEvents(), nil)
		require.ErrorIs(t, err, store.ErrReportPrivacy)
		require.False(t, applied)
		var events int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM report_intake_outbox`).Scan(&events))
		require.Zero(t, events)
	})
}
