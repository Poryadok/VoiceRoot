package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ReportTargetRejection is finality of one User intake, never identity evidence.
// It deliberately contains no target UUID, subject, account or payload digest.
type ReportTargetRejection struct {
	ReportID      uuid.UUID
	Decision      string
	Provenance    string
	SchemaVersion int
}

func validReportTargetRejection(r ReportTargetRejection) bool {
	return r.ReportID != uuid.Nil && r.SchemaVersion == 1 &&
		(r.Decision == "MISSING" || r.Decision == "DELETED") &&
		(r.Provenance == "OWNER_VALIDATION" || r.Provenance == "LEGACY_RECORDED_DECISION")
}

// Caller owns all account, subject, report and subject-journal locks first.
// Same-decision replay preserves the original decision provenance.
func recordReportTargetRejectionTx(ctx context.Context, tx pgx.Tx, r ReportTargetRejection) error {
	if !validReportTargetRejection(r) {
		return ErrReportRecoveryDivergence
	}
	if err := lockReportTargetRejectionTx(ctx, tx, r.ReportID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO report_target_rejections(report_id,decision,provenance,schema_version)
 VALUES($1,$2,$3,$4) ON CONFLICT(report_id) DO NOTHING`, r.ReportID, r.Decision, r.Provenance, r.SchemaVersion); err != nil {
		return err
	}
	var decision string
	if err := tx.QueryRow(ctx, `SELECT decision FROM report_target_rejections WHERE report_id=$1 FOR UPDATE`, r.ReportID).Scan(&decision); err != nil {
		return err
	}
	if decision != r.Decision {
		return ErrReportRecoveryDivergence
	}
	return nil
}

// A row lock cannot protect an absent receipt. The same final lock namespace
// serializes importer/repair insertion with ordinary completion's absence guard.
func lockReportTargetRejectionTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('moderation-report-rejection:' || $1::text,0))`, id.String())
	return err
}

func requireNoReportTargetRejectionTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	if err := lockReportTargetRejectionTx(ctx, tx, id); err != nil {
		return err
	}
	var decision string
	err := tx.QueryRow(ctx, `SELECT decision FROM report_target_rejections WHERE report_id=$1 FOR UPDATE`, id).Scan(&decision)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return ErrReportPrivacy
}

func minimizeReportUserTargetTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, decision string) error {
	_, err := tx.Exec(ctx, `UPDATE reports SET target_id=NULL,validation_state=$2,status='dismissed',description=NULL,evidence='{}'::jsonb,
 validated_target_account_id=NULL,validated_target_subject_id=NULL,validated_target_subject_revision=NULL,
 validated_target_provenance='LEGACY_UNVERIFIED',resolution=jsonb_build_object('code',$3::text),
 resolved_at=COALESCE(resolved_at,now()),updated_at=now(),validation_claim=NULL,validation_claim_until=NULL,
 validation_failure_code=$3 WHERE id=$1 AND target_type='user'`, id, decision, map[string]string{"MISSING": "TARGET_NOT_FOUND", "DELETED": "TARGET_DELETED"}[decision])
	if err != nil {
		return err
	}
	// These tables carry pending work for this intake. A threshold case or
	// effective sanction is not an ordinary rejection participant.
	for _, query := range []string{
		`DELETE FROM report_intake_outbox WHERE report_id=$1`,
		`DELETE FROM report_threshold_evaluations WHERE report_id=$1`,
	} {
		if _, err := tx.Exec(ctx, query, id); err != nil {
			return err
		}
	}
	return nil
}
