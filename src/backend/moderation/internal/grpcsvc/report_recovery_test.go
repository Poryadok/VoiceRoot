package grpcsvc

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"testing"
	"voice/backend/moderation/internal/automod"
	"voice/backend/moderation/internal/store"
)

func TestReportPrivacyRecoveryRejectsLegacy(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	reports := newReportFixtureStore(pool)
	require.NoError(t, reports.RequirePrivacyRecovery(ctx), "empty committed catalog has no unresolved report participant")
	_, err := reports.InsertReport(ctx, uuid.New(), "story", uuid.New(), "spam", nil, `{}`)
	require.NoError(t, err)
	require.ErrorIs(t, reports.RequirePrivacyRecovery(ctx), store.ErrReportPrivacy, "legacy NULL requires authoritative owner recovery, not READY")
	var rows, fences int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM reports`).Scan(&rows))
	require.Equal(t, 1, rows)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_terminal_subjects`).Scan(&fences))
	require.Zero(t, fences, "readiness does not invent terminal proof or erase unknown legacy")
}

func TestReportPrivacyRecoveryBindingAndTerminalChecks(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	reports := newReportFixtureStore(pool)
	require.ErrorIs(t, (*store.ReportStore)(nil).RequirePrivacyRecovery(ctx), store.ErrReportPrivacy)
	subject := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Revision: 1, State: "LIVE"}
	check, _ := reportOpenPrivacy(ctx)
	row, err := reports.InsertSubjectReport(ctx, uuid.New(), subject, check, "story", uuid.New(), "spam", nil, `{}`)
	require.NoError(t, err)
	require.NoError(t, reports.RequirePrivacyRecovery(ctx))
	_, err = pool.Exec(ctx, `UPDATE automod_subjects SET lifecycle_revision=2,lifecycle_state='DELETED',purge_at=clock_timestamp()+interval '1 hour' WHERE subject_id=$1`, subject.ID)
	require.NoError(t, err)
	require.NoError(t, reports.RequirePrivacyRecovery(ctx), "accepted reporter soft deletion remains valid before terminal deadline")
	_, err = pool.Exec(ctx, `UPDATE automod_subjects SET purge_at=clock_timestamp()-interval '1 second' WHERE subject_id=$1`, subject.ID)
	require.NoError(t, err)
	require.ErrorIs(t, reports.RequirePrivacyRecovery(ctx), store.ErrReportPrivacy)
	_, err = pool.Exec(ctx, `UPDATE automod_subjects SET lifecycle_revision=3,lifecycle_state='LIVE',purge_at=NULL WHERE subject_id=$1`, subject.ID)
	require.NoError(t, err)
	require.NoError(t, reports.RequirePrivacyRecovery(ctx), "fresh higher lifecycle revision can retain the same binding")
	_, err = pool.Exec(ctx, `UPDATE automod_subjects SET account_id=$2 WHERE subject_id=$1`, subject.ID, uuid.New())
	require.NoError(t, err)
	require.ErrorIs(t, reports.RequirePrivacyRecovery(ctx), store.ErrReportPrivacy, "changed account is not subject provenance")
	_, err = pool.Exec(ctx, `UPDATE automod_subjects SET account_id=$2 WHERE subject_id=$1`, subject.ID, subject.AccountID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE reports SET reporter_subject_revision=4 WHERE id=$1`, row.ID)
	require.NoError(t, err)
	require.ErrorIs(t, reports.RequirePrivacyRecovery(ctx), store.ErrReportPrivacy, "older mapping cannot verify stored newer provenance")
	_, err = pool.Exec(ctx, `UPDATE automod_subjects SET lifecycle_revision=4 WHERE subject_id=$1`, subject.ID)
	require.NoError(t, err)
	require.NoError(t, reports.RequirePrivacyRecovery(ctx))
	_, err = pool.Exec(ctx, `INSERT INTO automod_terminal_subjects(subject_id) VALUES($1)`, subject.ID)
	require.NoError(t, err)
	require.ErrorIs(t, reports.RequirePrivacyRecovery(ctx), store.ErrReportPrivacy, "terminal residue must be scrubbed before P5")
	reports.Subjects.BeforeTerminalSubjectPurge = func(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
		return reports.PurgeReportSubjectsTx(ctx, tx, []uuid.UUID{id})
	}
	require.NoError(t, reports.Subjects.Purge(ctx, subject.ID))
	require.NoError(t, reports.RequirePrivacyRecovery(ctx), "authoritative participant erasure resolves the blocker")
}

func TestReportPrivacyRecoveryValidatedTargetAndMissingMapping(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	reports := newReportFixtureStore(pool)
	check, _ := reportOpenPrivacy(ctx)
	reporter := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Revision: 1, State: "LIVE"}
	row, err := reports.InsertSubjectReport(ctx, uuid.New(), reporter, check, "user", uuid.New(), "spam", nil, `{}`)
	require.NoError(t, err)
	require.NoError(t, reports.RequirePrivacyRecovery(ctx), "pending unknown target is not an invalid reporter binding")
	target := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Revision: 1, State: "LIVE"}
	claim, err := reports.ClaimValidation(ctx)
	require.NoError(t, err)
	claim.ReporterSubject = &reporter
	claim.TargetSubject = &target
	claim.PrivacyCheck = check
	applied, err := reports.CompleteValidation(ctx, claim, "LIVE", target.AccountID, []store.IntakeEvent{{ID: uuid.New(), Kind: "domain.report_created", Subject: "domain", Payload: []byte("stable")}, {ID: uuid.New(), Kind: "analytics.report_created", Subject: "analytics", Payload: []byte("stable")}}, nil)
	require.NoError(t, err)
	require.True(t, applied)
	require.NoError(t, reports.RequirePrivacyRecovery(ctx), "both immutable subject bindings admit a fully validated User target")
	_, err = pool.Exec(ctx, `INSERT INTO automod_terminal_subjects(subject_id) VALUES($1)`, target.ID)
	require.NoError(t, err)
	require.ErrorIs(t, reports.RequirePrivacyRecovery(ctx), store.ErrReportPrivacy, "validated target terminal residue closes recovery")
	_, err = pool.Exec(ctx, `DELETE FROM automod_terminal_subjects WHERE subject_id=$1`, target.ID)
	require.NoError(t, err) // Isolated fixture reset, never a recovery operation.
	_, err = pool.Exec(ctx, `UPDATE reports SET validated_target_account_id=NULL,validated_target_subject_id=NULL,validated_target_subject_revision=NULL,validated_target_provenance='LEGACY_UNVERIFIED' WHERE id=$1`, row.ID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE reports SET validation_state='VALID' WHERE id=$1`, row.ID)
	require.NoError(t, err)
	require.ErrorIs(t, reports.RequirePrivacyRecovery(ctx), store.ErrReportPrivacy, "old VALID user target without subject proof cannot pass recovery")
	_, err = pool.Exec(ctx, `UPDATE reports SET validation_state='PENDING' WHERE id=$1`, row.ID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM automod_subjects WHERE subject_id=$1`, reporter.ID)
	require.NoError(t, err)
	require.ErrorIs(t, reports.RequirePrivacyRecovery(ctx), store.ErrReportPrivacy, "mapping loss is not permission to infer terminal or LIVE")
}
