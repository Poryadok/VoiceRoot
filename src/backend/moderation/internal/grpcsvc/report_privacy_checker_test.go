package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/moderation/internal/automod"
	"voice/backend/moderation/internal/store"
)

func TestReportPrivacyCheckerRequiresMatchingProvenanceJournal(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	reports := newReportFixtureStore(pool)
	check, _ := reportOpenPrivacy(ctx)
	reporter := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Revision: 1, State: "LIVE"}
	row, err := reports.InsertSubjectReport(ctx, uuid.New(), reporter, check, "user", uuid.New(), "spam", nil, `{}`)
	require.NoError(t, err)
	require.NoError(t, reports.RequirePrivacyRecovery(ctx), "opaque pending target has no invented ownership or eligibility")
	_, err = pool.Exec(ctx, `DELETE FROM report_privacy_journal WHERE report_id=$1`, row.ID)
	require.NoError(t, err)
	require.ErrorIs(t, reports.RequirePrivacyRecovery(ctx), store.ErrReportPrivacy, "raw links alone do not replace the atomic journal")
	_, err = pool.Exec(ctx, `INSERT INTO report_privacy_journal(report_id,slot,disposition,subject_id,evidence_kind,evidence_id,evidence_version)
 VALUES($1,'REPORTER','BOUND',$2,'AUTH_SUBJECT',$2,1)`, row.ID, reporter.ID)
	require.NoError(t, err)
	require.NoError(t, reports.RequirePrivacyRecovery(ctx))
	_, err = pool.Exec(ctx, `UPDATE report_privacy_journal SET subject_id=$2 WHERE report_id=$1`, row.ID, uuid.New())
	require.NoError(t, err)
	require.ErrorIs(t, reports.RequirePrivacyRecovery(ctx), store.ErrReportPrivacy)
	_, err = pool.Exec(ctx, `UPDATE report_privacy_journal SET subject_id=$2 WHERE report_id=$1`, row.ID, reporter.ID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE reports SET reporter_provenance='LEGACY_UNVERIFIED' WHERE id=$1`, row.ID)
	require.NoError(t, err)
	require.ErrorIs(t, reports.RequirePrivacyRecovery(ctx), store.ErrReportPrivacy, "coherent raw UUIDs do not manufacture submission authority")
	_, err = pool.Exec(ctx, `UPDATE reports SET reporter_provenance='OWNER_RECOVERED_LEGACY' WHERE id=$1`, row.ID)
	require.NoError(t, err)
	require.ErrorIs(t, reports.RequirePrivacyRecovery(ctx), store.ErrReportPrivacy, "legacy privacy recovery needs owning User fact")
	_, err = pool.Exec(ctx, `UPDATE report_privacy_journal SET evidence_kind='USER_OWNER_FACT',evidence_id=$2 WHERE report_id=$1`, row.ID, uuid.New())
	require.NoError(t, err)
	require.NoError(t, reports.RequirePrivacyRecovery(ctx))
	_, err = pool.Exec(ctx, `UPDATE report_privacy_journal SET disposition='REDACTED',evidence_kind='USER_ERASURE_FACT' WHERE report_id=$1`, row.ID)
	require.NoError(t, err)
	require.ErrorIs(t, reports.RequirePrivacyRecovery(ctx), store.ErrReportPrivacy, "terminal receipt still requires raw participant cleanup")
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM reports WHERE id=$1`, row.ID).Scan(&count))
	require.Equal(t, 1, count, "checker is read-only")
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM report_intake_outbox`).Scan(&count))
	require.Zero(t, count, "checker creates no validation or moderation effects")
}

func TestReportPrivacyCheckerRequiresOwnerUnionAndGeneration(t *testing.T) {
	pool := reportIntakePool(t)
	ctx := context.Background()
	reports := newReportFixtureStore(pool)
	subject := uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO report_privacy_journal(report_id,slot,disposition,subject_id,evidence_kind,evidence_id,evidence_version)
 VALUES($1,'REPORTER','REDACTED',$2,'AUTH_TERMINAL',$2,1)`, uuid.New(), subject)
	require.NoError(t, err)
	require.ErrorIs(t, reports.RequirePrivacyRecovery(ctx), store.ErrReportPrivacy, "report receipt alone cannot substitute owning terminal union")
	_, err = pool.Exec(ctx, `INSERT INTO automod_terminal_subjects(subject_id) VALUES($1)`, subject)
	require.NoError(t, err)
	require.NoError(t, reports.RequirePrivacyRecovery(ctx))
	_, err = pool.Exec(ctx, `DELETE FROM report_privacy_journal_metadata`)
	require.NoError(t, err)
	require.ErrorIs(t, reports.RequirePrivacyRecovery(ctx), store.ErrReportPrivacy, "missing durable journal generation is not first provision proof")
}

func TestReportPrivacyCheckerRequiresTargetReceiptCoherence(t *testing.T) {
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
	require.NoError(t, reports.RequirePrivacyRecovery(ctx))
	_, err = pool.Exec(ctx, `DELETE FROM report_privacy_journal WHERE report_id=$1 AND slot='USER_TARGET'`, row.ID)
	require.NoError(t, err)
	require.ErrorIs(t, reports.RequirePrivacyRecovery(ctx), store.ErrReportPrivacy)
	_, err = pool.Exec(ctx, `INSERT INTO report_privacy_journal(report_id,slot,disposition,subject_id,evidence_kind,evidence_id,evidence_version)
 VALUES($1,'USER_TARGET','BOUND',$2,'AUTH_SUBJECT',$2,1)`, row.ID, target.ID)
	require.NoError(t, err)
	require.NoError(t, reports.RequirePrivacyRecovery(ctx))
	_, err = pool.Exec(ctx, `UPDATE report_privacy_journal SET subject_id=$2 WHERE report_id=$1 AND slot='USER_TARGET'`, row.ID, uuid.New())
	require.NoError(t, err)
	require.ErrorIs(t, reports.RequirePrivacyRecovery(ctx), store.ErrReportPrivacy)
	_, err = pool.Exec(ctx, `UPDATE report_privacy_journal SET subject_id=$2 WHERE report_id=$1 AND slot='USER_TARGET'`, row.ID, target.ID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE reports SET validated_target_provenance='OWNER_RECOVERED_LEGACY' WHERE id=$1`, row.ID)
	require.NoError(t, err)
	require.ErrorIs(t, reports.RequirePrivacyRecovery(ctx), store.ErrReportPrivacy)
	_, err = pool.Exec(ctx, `UPDATE report_privacy_journal SET evidence_kind='USER_OWNER_FACT',evidence_id=$2 WHERE report_id=$1 AND slot='USER_TARGET'`, row.ID, uuid.New())
	require.NoError(t, err)
	require.NoError(t, reports.RequirePrivacyRecovery(ctx), "privacy-only target recovery is distinct from fresh validation authority")
	_, err = pool.Exec(ctx, `UPDATE reports SET validation_state='PENDING',validated_target_account_id=NULL,
 validated_target_subject_id=NULL,validated_target_subject_revision=NULL,validated_target_provenance='LEGACY_UNVERIFIED' WHERE id=$1`, row.ID)
	require.NoError(t, err) // Old raw backup plus independently current receipt fixture.
	require.ErrorIs(t, reports.RequirePrivacyRecovery(ctx), store.ErrReportPrivacy, "known target receipt cannot be ignored because raw backup predates validation")
}
