package store

import (
	"context"
	"errors"
	"reflect"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ReportTerminalRepairBatch reports one bounded local observation, not a current
// source cut or readiness proof. The final read-only checker is still required.
type ReportTerminalRepairBatch struct {
	Removed int
	More    bool
}

// RepairReportTerminalReceipts is private boot repair, not effects admission.
// The caller must supply AcquirePrivateRepair from the shared coordinator and
// establish actual writer fencing/restore continuity before using restored data.
func (s *ReportStore) RepairReportTerminalReceipts(ctx context.Context, privateAcquire ReportPrivacyAcquire, limit int) (ReportTerminalRepairBatch, error) {
	if s == nil || s.Pool == nil || limit < 1 || limit > 1000 {
		return ReportTerminalRepairBatch{}, ErrReportPrivacy
	}
	check, err := AcquireReportPrivacy(ctx, privateAcquire)
	if err != nil {
		return ReportTerminalRepairBatch{}, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return ReportTerminalRepairBatch{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var generation uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT journal_generation FROM report_privacy_journal_metadata WHERE singleton`).Scan(&generation); err != nil {
		return ReportTerminalRepairBatch{}, err
	}
	if generation == uuid.Nil {
		return ReportTerminalRepairBatch{}, ErrReportPrivacy
	}
	rows, err := tx.Query(ctx, `SELECT r.id FROM reports r WHERE EXISTS(
 SELECT 1 FROM report_privacy_journal j WHERE j.report_id=r.id AND (
  (j.disposition='REDACTED' AND j.evidence_kind='USER_ERASURE_FACT')
  OR EXISTS(SELECT 1 FROM automod_terminal_subjects t WHERE t.subject_id=j.subject_id)))
 ORDER BY r.id LIMIT $1`, limit+1)
	if err != nil {
		return ReportTerminalRepairBatch{}, err
	}
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return ReportTerminalRepairBatch{}, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return ReportTerminalRepairBatch{}, err
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
	var bindings []reportDispatchBinding
	for _, id := range ids {
		raw, err := readReportRecoveryRaw(ctx, tx, id, false)
		if err != nil {
			return ReportTerminalRepairBatch{}, err
		}
		journal, err := readReportRecoveryJournal(ctx, tx, id, false)
		if err != nil {
			return ReportTerminalRepairBatch{}, err
		}
		observations = append(observations, observation{raw, journal})
		for _, account := range []*uuid.UUID{raw.ReporterAccount, raw.TargetAccount} {
			if account != nil {
				accounts = append(accounts, *account)
			}
		}
		for _, subject := range []*uuid.UUID{raw.ReporterSubject, raw.TargetSubject} {
			if subject != nil {
				bindings = append(bindings, reportDispatchBinding{subject: *subject})
			}
		}
		for _, record := range journal {
			if record.SubjectID != nil {
				bindings = append(bindings, reportDispatchBinding{subject: *record.SubjectID})
			}
		}
	}
	if err := lockReportAccounts(ctx, tx, accounts...); err != nil {
		return ReportTerminalRepairBatch{}, err
	}
	if err := lockDispatchSubjects(ctx, tx, bindings); err != nil {
		return ReportTerminalRepairBatch{}, err
	}
	// Own every report lock before requesting any journal lock. A changed tuple
	// is retried from new account/subject observations; never acquire a newly
	// discovered account after taking a subject or report lock.
	for _, observed := range observations {
		raw, err := readReportRecoveryRaw(ctx, tx, observed.raw.ID, true)
		if err != nil {
			return ReportTerminalRepairBatch{}, err
		}
		if !reflect.DeepEqual(raw, observed.raw) {
			return ReportTerminalRepairBatch{}, ErrReportPrivacy
		}
	}
	for _, observed := range observations {
		journal, err := readReportRecoveryJournal(ctx, tx, observed.raw.ID, true)
		if err != nil {
			return ReportTerminalRepairBatch{}, err
		}
		if !reflect.DeepEqual(journal, observed.journal) {
			return ReportTerminalRepairBatch{}, ErrReportPrivacy
		}
		eligible := false
		for _, record := range journal {
			if !validReportRecoveryRecord(record) {
				return ReportTerminalRepairBatch{}, ErrReportPrivacy
			}
			rawSubject := observed.raw.ReporterSubject
			if record.Slot == "USER_TARGET" {
				if observed.raw.TargetType != "user" {
					return ReportTerminalRepairBatch{}, ErrReportRecoveryDivergence
				}
				rawSubject = observed.raw.TargetSubject
			}
			if rawSubject != nil && record.SubjectID != nil && *rawSubject != *record.SubjectID {
				return ReportTerminalRepairBatch{}, ErrReportRecoveryDivergence
			}
			var ended bool
			if record.SubjectID != nil {
				if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM automod_terminal_subjects WHERE subject_id=$1)`, *record.SubjectID).Scan(&ended); err != nil {
					return ReportTerminalRepairBatch{}, err
				}
			}
			if record.EvidenceKind == "AUTH_TERMINAL" && (record.SubjectID == nil || *record.SubjectID != record.EvidenceID || !ended) {
				return ReportTerminalRepairBatch{}, ErrReportPrivacy
			}
			if ended {
				// Report and journal locks are already owned; this participant
				// recheck cannot request a new row or account lock after them.
				if err := s.recordTerminalReportSlotTx(ctx, tx, record.ReportID, record.Slot, *record.SubjectID); err != nil {
					return ReportTerminalRepairBatch{}, err
				}
				eligible = true
			} else if record.Disposition == "REDACTED" && record.EvidenceKind == "USER_ERASURE_FACT" {
				eligible = true
			}
		}
		if !eligible {
			return ReportTerminalRepairBatch{}, ErrReportPrivacy
		}
	}
	if err := finishReportPurgeTx(ctx, tx, ids); err != nil {
		return ReportTerminalRepairBatch{}, err
	}
	var current uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT journal_generation FROM report_privacy_journal_metadata WHERE singleton`).Scan(&current); err != nil {
		return ReportTerminalRepairBatch{}, err
	}
	if current != generation {
		return ReportTerminalRepairBatch{}, ErrReportPrivacy
	}
	if err := check(ctx); err != nil {
		return ReportTerminalRepairBatch{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ReportTerminalRepairBatch{}, err
	}
	return ReportTerminalRepairBatch{Removed: len(ids), More: more}, nil
}

type reportRecoveryRaw struct {
	ID, ReporterProfile                                            uuid.UUID
	TargetID                                                       *uuid.UUID
	TargetType                                                     string
	ReporterAccount, ReporterSubject, TargetAccount, TargetSubject *uuid.UUID
	ReporterRevision, TargetRevision                               *int64
	ReporterProvenance, TargetProvenance, ValidationState          string
}

func readReportRecoveryRaw(ctx context.Context, tx pgx.Tx, id uuid.UUID, locked bool) (reportRecoveryRaw, error) {
	query := `SELECT id,reporter_profile_id,target_type,target_id,reporter_account_id,reporter_subject_id,
 validated_target_account_id,validated_target_subject_id,reporter_subject_revision,validated_target_subject_revision,
 reporter_provenance,validated_target_provenance,validation_state FROM reports WHERE id=$1`
	if locked {
		query += ` FOR UPDATE`
	}
	var r reportRecoveryRaw
	err := tx.QueryRow(ctx, query, id).Scan(&r.ID, &r.ReporterProfile, &r.TargetType, &r.TargetID, &r.ReporterAccount, &r.ReporterSubject, &r.TargetAccount, &r.TargetSubject, &r.ReporterRevision, &r.TargetRevision, &r.ReporterProvenance, &r.TargetProvenance, &r.ValidationState)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, errors.Join(ErrReportPrivacy, err)
	}
	return r, err
}

func readReportRecoveryJournal(ctx context.Context, tx pgx.Tx, id uuid.UUID, locked bool) ([]ReportRecoveryRecord, error) {
	query := `SELECT report_id,slot,disposition,subject_id,evidence_kind,evidence_id,evidence_version FROM report_privacy_journal WHERE report_id=$1 ORDER BY slot COLLATE "C"`
	if locked {
		query += ` FOR UPDATE`
	}
	rows, err := tx.Query(ctx, query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []ReportRecoveryRecord
	for rows.Next() {
		var r ReportRecoveryRecord
		if err := rows.Scan(&r.ReportID, &r.Slot, &r.Disposition, &r.SubjectID, &r.EvidenceKind, &r.EvidenceID, &r.EvidenceVersion); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, rows.Err()
}
