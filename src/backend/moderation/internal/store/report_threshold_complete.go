package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type ReportThresholdCase struct {
	ID, SanctionID, AccountID, ProfileID uuid.UUID
	Audience, Threshold, Reporters       int64
	CreatedAt                            time.Time
}
type ReportThresholdEvent struct {
	ID            uuid.UUID
	Kind, Subject string
	Payload       []byte
}
type ReportThresholdEvents func(ReportThresholdCase) ([]ReportThresholdEvent, error)

func (s *ReportThresholdStore) SettleExpired(ctx context.Context, claim *ReportThresholdClaim) (bool, error) {
	if s == nil || s.Pool == nil || claim == nil {
		return false, errStoreNotConfigured
	}
	tag, err := s.Pool.Exec(ctx, `UPDATE report_threshold_evaluations e SET completed_at=clock_timestamp(),
outcome='EXPIRED_EVIDENCE',claim_id=NULL,claim_until=NULL FROM reports r
WHERE e.report_id=r.id AND e.report_id=$1 AND e.claim_id=$2 AND e.completed_at IS NULL
AND r.created_at<=clock_timestamp()-interval '24 hours'`, claim.ReportID, claim.ClaimID)
	return tag.RowsAffected() > 0, err
}

// Complete has no network effects. The pure serializer runs inside the same
// account transaction; retries dispatch the immutable original outbox bytes.
func (s *ReportThresholdStore) Complete(ctx context.Context, claim *ReportThresholdClaim, observation ReportThresholdObservation, events ReportThresholdEvents) error {
	if s == nil || s.Pool == nil || s.Sanctions == nil || s.BindLiveSubjectTx == nil || claim == nil || events == nil {
		return errStoreNotConfigured
	}
	return s.Sanctions.WithAccountLock(ctx, claim.AccountID, func(tx pgx.Tx) error {
		var now time.Time
		if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
			return err
		}
		if !observation.valid(claim, now) {
			return errReportThresholdStale
		}
		var current bool
		err := tx.QueryRow(ctx, `SELECT true FROM report_threshold_evaluations WHERE report_id=$1 AND claim_id=$2
AND completed_at IS NULL AND claim_until>$3 FOR UPDATE`, claim.ReportID, claim.ClaimID, now).Scan(&current)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		settle := func(outcome string) error {
			_, err := tx.Exec(ctx, `UPDATE report_threshold_evaluations SET completed_at=$3,outcome=$4,failure_code=NULL,
claim_id=NULL,claim_until=NULL WHERE report_id=$1 AND claim_id=$2`, claim.ReportID, claim.ClaimID, now, outcome)
			return err
		}
		var eligible bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM reports WHERE id=$1 AND target_type='user' AND target_id=$2
AND validation_state='VALID' AND status IN ('pending','reviewing') AND validated_target_account_id=$3
AND reporter_account_id IS NOT NULL AND reporter_account_id<>$3 AND created_at>$4::timestamptz-interval '24 hours' AND created_at<=$4::timestamptz)`, claim.ReportID, claim.ProfileID, claim.AccountID, now).Scan(&eligible); err != nil {
			return err
		}
		if !eligible {
			return settle("INELIGIBLE_EVIDENCE")
		}
		if err := s.BindLiveSubjectTx(ctx, tx, claim.AccountID, observation.SubjectID, observation.LifecycleRevision); err != nil {
			return err
		}
		if observation.Audience == 0 {
			return settle("EMPTY_AUDIENCE")
		}
		threshold, err := ReportThreshold(observation.Audience)
		if err != nil {
			return err
		}
		// Shared account serialization protects the cutoff. Row locks make staff
		// resolution linearize with this exact distinct-account count.
		rows, err := tx.Query(ctx, `SELECT id,reporter_account_id FROM reports WHERE target_type='user' AND target_id=$1
AND validated_target_account_id=$2 AND validation_state='VALID' AND status IN ('pending','reviewing')
AND reporter_account_id IS NOT NULL AND reporter_account_id<>$2
AND created_at>$3::timestamptz-interval '24 hours' AND created_at<=$3::timestamptz
AND created_at>COALESCE((SELECT closed_at FROM report_threshold_account_cutoffs WHERE account_id=$2),'-infinity'::timestamptz)
ORDER BY id FOR UPDATE`, claim.ProfileID, claim.AccountID, now)
		if err != nil {
			return err
		}
		var reportIDs []uuid.UUID
		reporters := make(map[uuid.UUID]struct{})
		for rows.Next() {
			var report, reporter uuid.UUID
			if err := rows.Scan(&report, &reporter); err != nil {
				rows.Close()
				return err
			}
			reportIDs = append(reportIDs, report)
			reporters[reporter] = struct{}{}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		if int64(len(reporters)) < threshold {
			return settle("BELOW_THRESHOLD")
		}
		var suppressed bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM report_threshold_cases WHERE account_id=$1 AND closed_at IS NULL)
OR EXISTS(SELECT 1 FROM sanctions WHERE target_account_id=$1 AND type='shadow_ban' AND revoked_at IS NULL
AND (expires_at IS NULL OR expires_at>$2))`, claim.AccountID, now).Scan(&suppressed); err != nil {
			return err
		}
		if suppressed {
			return settle("EXISTING_SHADOW_BAN")
		}
		created := ReportThresholdCase{ID: uuid.New(), SanctionID: uuid.New(), AccountID: claim.AccountID, ProfileID: claim.ProfileID, Audience: observation.Audience, Threshold: threshold, Reporters: int64(len(reporters)), CreatedAt: now}
		_, err = tx.Exec(ctx, `INSERT INTO report_threshold_cases(id,account_id,subject_id,lifecycle_revision,target_profile_id,
metric_version,audience_accounts,threshold_accounts,reporter_accounts,snapshot_id,observed_at,created_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, created.ID, claim.AccountID, observation.SubjectID, int64(observation.LifecycleRevision), claim.ProfileID,
			observation.Metric, observation.Audience, threshold, created.Reporters, observation.SnapshotID, observation.StartedAt, now)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO sanctions(id,target_account_id,type,reason,issued_by,issuer_service,machine_case_id,created_at,updated_at)
VALUES($1,$2,'shadow_ban','Distinct verified account report threshold; pending review',NULL,'service:moderation/automod',$3,$4,$4)`, created.SanctionID, claim.AccountID, created.ID, now)
		if err != nil {
			return err
		}
		for _, report := range reportIDs {
			if _, err := tx.Exec(ctx, `INSERT INTO report_threshold_case_reports(case_id,report_id) VALUES($1,$2)`, created.ID, report); err != nil {
				return err
			}
		}
		details, err := json.Marshal(map[string]any{"case_id": created.ID.String(), "sanction_id": created.SanctionID.String(), "metric_version": observation.Metric, "audience_accounts": observation.Audience, "threshold_accounts": threshold, "reporter_accounts": created.Reporters})
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO auto_mod_log(target_profile_id,trigger,action,details,created_at) VALUES($1,'report_threshold','shadow_ban',$2,$3)`, claim.ProfileID, details, now); err != nil {
			return err
		}
		serialized, err := events(created)
		if err != nil {
			return err
		}
		if len(serialized) == 0 {
			return errors.New("report threshold events required")
		}
		for _, event := range serialized {
			if event.ID == uuid.Nil || event.Kind == "" || event.Subject == "" || len(event.Payload) == 0 {
				return errors.New("invalid report threshold event")
			}
			if _, err := tx.Exec(ctx, `INSERT INTO report_threshold_outbox(id,case_id,kind,subject,payload) VALUES($1,$2,$3,$4,$5)`, event.ID, created.ID, event.Kind, event.Subject, event.Payload); err != nil {
				return err
			}
		}
		if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
			return err
		}
		if !observation.valid(claim, now) {
			return errReportThresholdStale
		}
		return settle("SHADOW_BAN_CREATED")
	})
}
