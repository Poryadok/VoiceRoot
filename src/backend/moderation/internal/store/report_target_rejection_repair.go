package store

import (
	"context"
	"errors"
	"reflect"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ReportRejectionRepairBatch is bounded local progress, never boot readiness.
type ReportRejectionRepairBatch struct {
	Minimized int
	More      bool
}

// RepairReportTargetRejections applies recorded final intake decisions, not
// inferred owner deletion. No reciprocal RPC or effects admission is used.
func (s *ReportStore) RepairReportTargetRejections(ctx context.Context, acquire ReportPrivacyAcquire, limit int) (ReportRejectionRepairBatch, error) {
	if s == nil || s.Pool == nil || limit < 1 || limit > 1000 {
		return ReportRejectionRepairBatch{}, ErrReportPrivacy
	}
	check, err := AcquireReportPrivacy(ctx, acquire)
	if err != nil {
		return ReportRejectionRepairBatch{}, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return ReportRejectionRepairBatch{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var generation uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT journal_generation FROM report_privacy_journal_metadata WHERE singleton`).Scan(&generation); err != nil {
		return ReportRejectionRepairBatch{}, err
	}
	if generation == uuid.Nil {
		return ReportRejectionRepairBatch{}, ErrReportPrivacy
	}
	rows, err := tx.Query(ctx, `SELECT r.id FROM reports r LEFT JOIN report_target_rejections q ON q.report_id=r.id
 WHERE (r.target_type='user' AND r.validation_state IN ('MISSING','DELETED') AND
  (q.report_id IS NULL OR r.target_id IS NOT NULL OR r.status<>'dismissed' OR r.description IS NOT NULL OR r.evidence<>'{}'::jsonb
   OR r.validated_target_account_id IS NOT NULL OR r.validated_target_subject_id IS NOT NULL OR r.validated_target_subject_revision IS NOT NULL
   OR r.validated_target_provenance<>'LEGACY_UNVERIFIED' OR r.validation_claim IS NOT NULL OR r.validation_claim_until IS NOT NULL
   OR r.resolution IS DISTINCT FROM jsonb_build_object('code',CASE r.validation_state WHEN 'MISSING' THEN 'TARGET_NOT_FOUND' ELSE 'TARGET_DELETED' END)
   OR EXISTS(SELECT 1 FROM report_intake_outbox o WHERE o.report_id=r.id)
   OR EXISTS(SELECT 1 FROM report_threshold_evaluations e WHERE e.report_id=r.id)))
 OR (q.report_id IS NOT NULL AND (r.target_type<>'user' OR r.validation_state NOT IN ('MISSING','DELETED') OR q.decision<>r.validation_state))
 ORDER BY r.id LIMIT $1`, limit+1)
	if err != nil {
		return ReportRejectionRepairBatch{}, err
	}
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return ReportRejectionRepairBatch{}, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return ReportRejectionRepairBatch{}, err
	}
	more := len(ids) > limit
	if more {
		ids = ids[:limit]
	}
	type observation struct {
		raw     reportRecoveryRaw
		journal []ReportRecoveryRecord
	}
	observations := make([]observation, 0, len(ids))
	var accounts []uuid.UUID
	var subjects []reportDispatchBinding
	for _, id := range ids {
		raw, e := readReportRecoveryRaw(ctx, tx, id, false)
		if e != nil {
			return ReportRejectionRepairBatch{}, e
		}
		journal, e := readReportRecoveryJournal(ctx, tx, id, false)
		if e != nil {
			return ReportRejectionRepairBatch{}, e
		}
		observations = append(observations, observation{raw, journal})
		for _, account := range []*uuid.UUID{raw.ReporterAccount, raw.TargetAccount} {
			if account != nil {
				accounts = append(accounts, *account)
			}
		}
		for _, subject := range []*uuid.UUID{raw.ReporterSubject, raw.TargetSubject} {
			if subject != nil {
				subjects = append(subjects, reportDispatchBinding{subject: *subject})
			}
		}
		for _, record := range journal {
			if record.SubjectID != nil {
				subjects = append(subjects, reportDispatchBinding{subject: *record.SubjectID})
			}
		}
	}
	if err = lockReportAccounts(ctx, tx, accounts...); err != nil {
		return ReportRejectionRepairBatch{}, err
	}
	if err = lockDispatchSubjects(ctx, tx, subjects); err != nil {
		return ReportRejectionRepairBatch{}, err
	}
	for _, o := range observations {
		raw, e := readReportRecoveryRaw(ctx, tx, o.raw.ID, true)
		if e != nil {
			return ReportRejectionRepairBatch{}, e
		}
		if !reflect.DeepEqual(raw, o.raw) {
			return ReportRejectionRepairBatch{}, ErrReportPrivacy
		}
	}
	// All subject-journal locks precede every rejection-record lock.
	for _, o := range observations {
		journal, e := readReportRecoveryJournal(ctx, tx, o.raw.ID, true)
		if e != nil {
			return ReportRejectionRepairBatch{}, e
		}
		if !reflect.DeepEqual(journal, o.journal) {
			return ReportRejectionRepairBatch{}, ErrReportPrivacy
		}
		for _, record := range journal {
			if !validReportRecoveryRecord(record) || record.Disposition == "REDACTED" {
				return ReportRejectionRepairBatch{}, ErrReportPrivacy
			}
			if record.SubjectID != nil {
				var terminal bool
				if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM automod_terminal_subjects WHERE subject_id=$1)`, *record.SubjectID).Scan(&terminal); e != nil {
					return ReportRejectionRepairBatch{}, e
				}
				if terminal {
					return ReportRejectionRepairBatch{}, ErrReportPrivacy
				}
			}
		}
	}
	for _, o := range observations {
		var decision string
		if err = lockReportTargetRejectionTx(ctx, tx, o.raw.ID); err != nil {
			return ReportRejectionRepairBatch{}, err
		}
		e := tx.QueryRow(ctx, `SELECT decision FROM report_target_rejections WHERE report_id=$1 FOR UPDATE`, o.raw.ID).Scan(&decision)
		if e != nil && !errors.Is(e, pgx.ErrNoRows) {
			return ReportRejectionRepairBatch{}, e
		}
		if o.raw.TargetType != "user" || o.raw.ValidationState == "VALID" {
			return ReportRejectionRepairBatch{}, ErrReportRecoveryDivergence
		}
		if decision != "" && o.raw.ValidationState != "PENDING" && decision != o.raw.ValidationState {
			return ReportRejectionRepairBatch{}, ErrReportRecoveryDivergence
		}
		if decision == "" {
			decision = o.raw.ValidationState
			if decision != "MISSING" && decision != "DELETED" {
				return ReportRejectionRepairBatch{}, ErrReportRecoveryDivergence
			}
		}
		if e = recordReportTargetRejectionTx(ctx, tx, ReportTargetRejection{o.raw.ID, decision, "LEGACY_RECORDED_DECISION", 1}); e != nil {
			return ReportRejectionRepairBatch{}, e
		}
		if e = minimizeReportUserTargetTx(ctx, tx, o.raw.ID, decision); e != nil {
			return ReportRejectionRepairBatch{}, e
		}
	}
	var current uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT journal_generation FROM report_privacy_journal_metadata WHERE singleton`).Scan(&current); err != nil {
		return ReportRejectionRepairBatch{}, err
	}
	if current != generation {
		return ReportRejectionRepairBatch{}, ErrReportPrivacy
	}
	if err = check(ctx); err != nil {
		return ReportRejectionRepairBatch{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return ReportRejectionRepairBatch{}, err
	}
	return ReportRejectionRepairBatch{len(ids), more}, nil
}
