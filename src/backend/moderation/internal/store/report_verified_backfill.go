package store

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/moderation/internal/automod"
)

// ReportJournalBackfillBatch describes bounded local progress, not source-tail
// completeness or readiness. Journaled counts slots and Purged counts reports.
type ReportJournalBackfillBatch struct {
	Journaled int
	Purged    int
	More      bool
}

// BackfillVerifiedReportJournal only journals the trusted pre-journal writer's
// already verified pairs. It cannot manufacture User owner facts for legacy
// classes, change submission provenance, validate reports or emit effects.
func (s *ReportStore) BackfillVerifiedReportJournal(ctx context.Context, resolver ReportSubjectResolver, privateAcquire ReportPrivacyAcquire, limit int) (ReportJournalBackfillBatch, error) {
	if s == nil || s.Pool == nil || s.Subjects == nil || resolver == nil || limit < 1 || limit > 100 {
		return ReportJournalBackfillBatch{}, ErrReportPrivacy
	}
	check, err := AcquireReportPrivacy(ctx, privateAcquire)
	if err != nil {
		return ReportJournalBackfillBatch{}, err
	}
	generation, observations, more, err := s.readVerifiedJournalCandidates(ctx, limit)
	if err != nil {
		return ReportJournalBackfillBatch{}, err
	}
	// The repeatable-read transaction is closed before any reciprocal RPC.
	terminal, err := s.verifiedJournalTerminals(ctx, observations)
	if err != nil {
		return ReportJournalBackfillBatch{}, err
	}
	resolved := map[verifiedPairKey]automod.Subject{}
	for _, observed := range observations {
		if hasVerifiedTerminal(observed, terminal) {
			continue
		}
		for _, pair := range observed.pairs {
			key := pair.key()
			binding, found := resolved[key]
			if !found {
				binding, _, err = ResolveReportSubject(ctx, resolver, pair.account, pair.subject)
				if err != nil {
					return ReportJournalBackfillBatch{}, err
				}
				resolved[key] = binding
			}
			if binding.Revision < pair.revision {
				return ReportJournalBackfillBatch{}, ErrReportPrivacy
			}
		}
	}
	if err := check(ctx); err != nil {
		return ReportJournalBackfillBatch{}, err
	}
	if len(observations) == 0 {
		return ReportJournalBackfillBatch{More: more}, nil
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return ReportJournalBackfillBatch{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var current uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT journal_generation FROM report_privacy_journal_metadata WHERE singleton`).Scan(&current); err != nil {
		return ReportJournalBackfillBatch{}, err
	}
	if current != generation {
		return ReportJournalBackfillBatch{}, ErrReportPrivacy
	}
	var accounts []uuid.UUID
	var subjectLocks []reportDispatchBinding
	for _, observed := range observations {
		for _, account := range []*uuid.UUID{observed.raw.ReporterAccount, observed.raw.TargetAccount} {
			if account != nil {
				accounts = append(accounts, *account)
			}
		}
		for _, subject := range []*uuid.UUID{observed.raw.ReporterSubject, observed.raw.TargetSubject} {
			if subject != nil {
				subjectLocks = append(subjectLocks, reportDispatchBinding{subject: *subject})
			}
		}
		for _, record := range observed.journal {
			if record.SubjectID != nil {
				subjectLocks = append(subjectLocks, reportDispatchBinding{subject: *record.SubjectID})
			}
		}
	}
	if err := lockReportAccounts(ctx, tx, accounts...); err != nil {
		return ReportJournalBackfillBatch{}, err
	}
	if err := lockDispatchSubjects(ctx, tx, subjectLocks); err != nil {
		return ReportJournalBackfillBatch{}, err
	}
	// Observe every actual permanent fence before binding any retained subject.
	terminal = map[uuid.UUID]bool{}
	for _, observed := range observations {
		for _, pair := range observed.pairs {
			var ended bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM automod_terminal_subjects WHERE subject_id=$1)`, pair.subject).Scan(&ended); err != nil {
				return ReportJournalBackfillBatch{}, err
			}
			terminal[pair.subject] = ended
		}
	}
	needed := map[verifiedPairKey]automod.Subject{}
	for _, observed := range observations {
		if hasVerifiedTerminal(observed, terminal) {
			continue
		}
		for _, pair := range observed.pairs {
			binding, found := resolved[pair.key()]
			if !found || binding.Revision < pair.revision {
				return ReportJournalBackfillBatch{}, ErrReportPrivacy
			}
			needed[pair.key()] = binding
		}
	}
	bindings := make([]automod.Subject, 0, len(needed))
	for _, binding := range needed {
		bindings = append(bindings, binding)
	}
	slices.SortFunc(bindings, func(a, b automod.Subject) int { return strings.Compare(a.ID.String(), b.ID.String()) })
	// Bind/lock lifecycle rows before taking report rows. These helpers create
	// no source epoch, cursor, vote or validation authority.
	for _, binding := range bindings {
		if err := s.Subjects.BindRetainedSubjectTx(ctx, tx, binding); err != nil {
			return ReportJournalBackfillBatch{}, errors.Join(ErrReportPrivacy, err)
		}
	}
	for _, observed := range observations {
		raw, err := readReportRecoveryRaw(ctx, tx, observed.raw.ID, true)
		if err != nil {
			return ReportJournalBackfillBatch{}, err
		}
		if !reflect.DeepEqual(raw, observed.raw) {
			return ReportJournalBackfillBatch{}, ErrReportPrivacy
		}
	}
	for _, observed := range observations {
		journal, err := readReportRecoveryJournal(ctx, tx, observed.raw.ID, true)
		if err != nil {
			return ReportJournalBackfillBatch{}, err
		}
		if !reflect.DeepEqual(journal, observed.journal) {
			return ReportJournalBackfillBatch{}, ErrReportPrivacy
		}
	}
	result := ReportJournalBackfillBatch{More: more}
	var purged []uuid.UUID
	for _, observed := range observations {
		ended := hasVerifiedTerminal(observed, terminal)
		for _, pair := range observed.pairs {
			if ended {
				if !terminal[pair.subject] {
					continue
				}
				if err := s.recordTerminalReportSlotTx(ctx, tx, observed.raw.ID, pair.slot, pair.subject); err != nil {
					return ReportJournalBackfillBatch{}, err
				}
			} else {
				subject := pair.subject
				if err := s.RecordReportRecoveryTx(ctx, tx, ReportRecoveryRecord{ReportID: observed.raw.ID, Slot: pair.slot, Disposition: "BOUND", SubjectID: &subject, EvidenceKind: "AUTH_SUBJECT", EvidenceID: subject, EvidenceVersion: 1}); err != nil {
					return ReportJournalBackfillBatch{}, err
				}
			}
			result.Journaled++
		}
		if ended {
			purged = append(purged, observed.raw.ID)
		}
	}
	if err := finishReportPurgeTx(ctx, tx, purged); err != nil {
		return ReportJournalBackfillBatch{}, err
	}
	for _, binding := range bindings {
		if err := s.Subjects.RequireRetainedSubjectTx(ctx, tx, binding); err != nil {
			return ReportJournalBackfillBatch{}, errors.Join(ErrReportPrivacy, err)
		}
	}
	if err := tx.QueryRow(ctx, `SELECT journal_generation FROM report_privacy_journal_metadata WHERE singleton`).Scan(&current); err != nil {
		return ReportJournalBackfillBatch{}, err
	}
	if current != generation {
		return ReportJournalBackfillBatch{}, ErrReportPrivacy
	}
	if err := check(ctx); err != nil {
		return ReportJournalBackfillBatch{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ReportJournalBackfillBatch{}, err
	}
	result.Purged = len(purged)
	return result, nil
}

type verifiedPairKey struct{ account, subject uuid.UUID }
type verifiedJournalPair struct {
	slot             string
	account, subject uuid.UUID
	revision         int64
}

func (p verifiedJournalPair) key() verifiedPairKey { return verifiedPairKey{p.account, p.subject} }

type verifiedJournalObservation struct {
	raw     reportRecoveryRaw
	journal []ReportRecoveryRecord
	pairs   []verifiedJournalPair
}

func hasVerifiedTerminal(observed verifiedJournalObservation, terminal map[uuid.UUID]bool) bool {
	for _, pair := range observed.pairs {
		if terminal[pair.subject] {
			return true
		}
	}
	return false
}

func (s *ReportStore) readVerifiedJournalCandidates(ctx context.Context, limit int) (uuid.UUID, []verifiedJournalObservation, bool, error) {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return uuid.Nil, nil, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var generation uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT journal_generation FROM report_privacy_journal_metadata WHERE singleton`).Scan(&generation); err != nil {
		return uuid.Nil, nil, false, err
	}
	if generation == uuid.Nil {
		return uuid.Nil, nil, false, ErrReportPrivacy
	}
	rows, err := tx.Query(ctx, `SELECT r.id FROM reports r WHERE
 (r.reporter_provenance='VERIFIED_AT_SUBMISSION' AND NOT EXISTS(SELECT 1 FROM report_privacy_journal j WHERE j.report_id=r.id AND j.slot='REPORTER'))
 OR (r.target_type='user' AND r.validation_state NOT IN ('MISSING','DELETED')
 AND NOT EXISTS(SELECT 1 FROM report_target_rejections q WHERE q.report_id=r.id)
 AND r.validated_target_provenance='VERIFIED_AT_VALIDATION' AND NOT EXISTS(SELECT 1 FROM report_privacy_journal j WHERE j.report_id=r.id AND j.slot='USER_TARGET'))
 ORDER BY r.id LIMIT $1`, limit+1)
	if err != nil {
		return uuid.Nil, nil, false, err
	}
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return uuid.Nil, nil, false, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return uuid.Nil, nil, false, err
	}
	more := len(ids) > limit
	if more {
		ids = ids[:limit]
	}
	var observations []verifiedJournalObservation
	subjectAccounts := map[uuid.UUID]uuid.UUID{}
	for _, id := range ids {
		raw, err := readReportRecoveryRaw(ctx, tx, id, false)
		if err != nil {
			return uuid.Nil, nil, false, err
		}
		journal, err := readReportRecoveryJournal(ctx, tx, id, false)
		if err != nil {
			return uuid.Nil, nil, false, err
		}
		existing := map[string]bool{}
		for _, record := range journal {
			if !validReportRecoveryRecord(record) || record.Disposition == "REDACTED" {
				return uuid.Nil, nil, false, ErrReportPrivacy
			}
			existing[record.Slot] = true
		}
		observed := verifiedJournalObservation{raw: raw, journal: journal}
		add := func(slot string, account, subject *uuid.UUID, revision *int64) error {
			if account == nil || subject == nil || revision == nil || *account == uuid.Nil || *subject == uuid.Nil || *revision <= 0 {
				return ErrReportPrivacy
			}
			if old, found := subjectAccounts[*subject]; found && old != *account {
				return ErrReportRecoveryDivergence
			}
			subjectAccounts[*subject] = *account
			observed.pairs = append(observed.pairs, verifiedJournalPair{slot, *account, *subject, *revision})
			return nil
		}
		if raw.ReporterProvenance == "VERIFIED_AT_SUBMISSION" && !existing["REPORTER"] {
			if err := add("REPORTER", raw.ReporterAccount, raw.ReporterSubject, raw.ReporterRevision); err != nil {
				return uuid.Nil, nil, false, err
			}
		}
		var rejected bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM report_target_rejections WHERE report_id=$1)`, raw.ID).Scan(&rejected); err != nil {
			return uuid.Nil, nil, false, err
		}
		if raw.TargetType == "user" && raw.ValidationState != "MISSING" && raw.ValidationState != "DELETED" && !rejected && raw.TargetProvenance == "VERIFIED_AT_VALIDATION" && !existing["USER_TARGET"] {
			if err := add("USER_TARGET", raw.TargetAccount, raw.TargetSubject, raw.TargetRevision); err != nil {
				return uuid.Nil, nil, false, err
			}
		}
		observations = append(observations, observed)
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, nil, false, err
	}
	return generation, observations, more, nil
}

func (s *ReportStore) verifiedJournalTerminals(ctx context.Context, observations []verifiedJournalObservation) (map[uuid.UUID]bool, error) {
	var subjects []uuid.UUID
	for _, observed := range observations {
		for _, pair := range observed.pairs {
			subjects = append(subjects, pair.subject)
		}
	}
	terminal := map[uuid.UUID]bool{}
	if len(subjects) == 0 {
		return terminal, nil
	}
	rows, err := s.Pool.Query(ctx, `SELECT subject_id FROM automod_terminal_subjects WHERE subject_id=ANY($1::uuid[])`, subjects)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		terminal[id] = true
	}
	return terminal, rows.Err()
}
