package store

import (
	"context"
	"errors"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/moderation/internal/automod"
	"voice/backend/pkg/automodclient"
	automodv1 "voice/backend/pkg/pb/voice/automod/v1"
)

// ReportPrivacyCheck rechecks the captured process boot and recovery generation.
// A check obtained before closure must remain invalid after the gate reopens.
type ReportPrivacyCheck func(context.Context) error

// ReportPrivacyAcquire captures the shared process boot and recovery generation.
type ReportPrivacyAcquire func(context.Context) (ReportPrivacyCheck, error)

type ReportSubjectResolver interface {
	ResolveSubject(context.Context, uuid.UUID, uuid.UUID) (*automodv1.ResolveModerationSubjectResponse, error)
}

var ErrReportPrivacy = errors.New("report privacy authority unavailable")

func AcquireReportPrivacy(ctx context.Context, acquire ReportPrivacyAcquire) (ReportPrivacyCheck, error) {
	if acquire == nil {
		return nil, ErrReportPrivacy
	}
	check, err := acquire(ctx)
	if err != nil || check == nil {
		return nil, ErrReportPrivacy
	}
	if err = check(ctx); err != nil {
		return nil, err
	}
	return check, nil
}

// ResolveReportSubject runs before the caller takes any SQL lock. Auth errors
// are never interpreted as proof of terminal deletion.
func ResolveReportSubject(ctx context.Context, resolver ReportSubjectResolver, account, expected uuid.UUID) (automod.Subject, uint64, error) {
	if resolver == nil {
		return automod.Subject{}, 0, ErrReportPrivacy
	}
	resolveCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	response, err := resolver.ResolveSubject(resolveCtx, account, expected)
	if err != nil {
		return automod.Subject{}, 0, err
	}
	if response == nil || !automodclient.ValidSubject(response.GetSubject(), account, expected) || response.GetSessionEpoch() == 0 || response.GetSessionEpoch() > math.MaxInt64 {
		return automod.Subject{}, 0, ErrReportPrivacy
	}
	binding := response.GetSubject()
	subject := automod.Subject{ID: uuid.MustParse(binding.GetSubjectId()), AccountID: account, Revision: int64(binding.GetLifecycleRevision()), State: "LIVE"}
	if binding.GetState() == automodv1.SubjectState_SUBJECT_STATE_DELETED {
		if !response.GetAuthRestricted() {
			return automod.Subject{}, 0, ErrReportPrivacy
		}
		deadline := binding.GetPurgeAt().AsTime()
		subject.State = "DELETED"
		subject.PurgeAt = &deadline
	}
	return subject, response.GetSessionEpoch(), nil
}

// Every participating account precedes every subject and report row. This also
// covers reciprocal A-reports-B/B-reports-A and the threshold hook's account lock.
func lockReportAccounts(ctx context.Context, tx pgx.Tx, accounts ...uuid.UUID) error {
	ids := map[string]bool{}
	for _, id := range accounts {
		if id != uuid.Nil {
			ids[id.String()] = true
		}
	}
	ordered := make([]string, 0, len(ids))
	for id := range ids {
		ordered = append(ordered, id)
	}
	sort.Strings(ordered)
	for _, id := range ordered {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('moderation-account-sanction:' || $1::text,0))`, id); err != nil {
			return err
		}
	}
	return nil
}

type reportDispatchBinding struct {
	account, subject uuid.UUID
	revision         int64
}

func lockDispatchSubjects(ctx context.Context, tx pgx.Tx, bindings []reportDispatchBinding) error {
	ids := map[string]bool{}
	for _, binding := range bindings {
		ids[binding.subject.String()] = true
	}
	ordered := make([]string, 0, len(ids))
	for id := range ids {
		ordered = append(ordered, id)
	}
	sort.Strings(ordered)
	for _, id := range ordered {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('moderation-subject-lifecycle:' || $1::text,0))`, id); err != nil {
			return err
		}
	}
	return nil
}

// InsertSubjectReport records current requester lifecycle provenance and the
// receipt in one account->subject serialized transaction. It never reads target.
func (s *ReportStore) InsertSubjectReport(ctx context.Context, reporterProfile uuid.UUID, subject automod.Subject, check ReportPrivacyCheck, targetType string, targetID uuid.UUID, category string, description *string, evidence string) (*ReportRow, error) {
	if s == nil || s.Pool == nil || s.Subjects == nil || check == nil {
		return nil, ErrReportPrivacy
	}
	if subject.State != "LIVE" || subject.PurgeAt != nil {
		return nil, ErrReportPrivacy
	}
	if err := check(ctx); err != nil {
		return nil, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err = lockReportAccounts(ctx, tx, subject.AccountID); err != nil {
		return nil, err
	}
	if err = s.Subjects.BindSubjectTx(ctx, tx, subject); err != nil {
		return nil, err
	}
	if err = s.Subjects.RequireLiveSubjectTx(ctx, tx, subject.AccountID, subject.ID, subject.Revision); err != nil {
		return nil, err
	}
	row := &ReportRow{}
	err = tx.QueryRow(ctx, `INSERT INTO reports(reporter_profile_id,reporter_account_id,reporter_subject_id,reporter_subject_revision,target_type,target_id,category,description,evidence,reporter_provenance)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,'VERIFIED_AT_SUBMISSION') RETURNING id,reporter_profile_id,target_type,target_id,category,description,evidence::text,status,assigned_to,resolved_at,resolution::text,created_at`, reporterProfile, subject.AccountID, subject.ID, subject.Revision, targetType, targetID, category, description, evidence).Scan(&row.ID, &row.ReporterProfileID, &row.TargetType, &row.TargetID, &row.Category, &row.Description, &row.EvidenceJSON, &row.Status, &row.AssignedToProfile, &row.ResolvedAt, &row.ResolutionJSON, &row.CreatedAt)
	if err != nil {
		return nil, err
	}
	if err = s.RecordReportRecoveryTx(ctx, tx, ReportRecoveryRecord{ReportID: row.ID, Slot: "REPORTER", Disposition: "BOUND", SubjectID: &subject.ID, EvidenceKind: "AUTH_SUBJECT", EvidenceID: subject.ID, EvidenceVersion: 1}); err != nil {
		return nil, err
	}
	if err = check(ctx); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return row, nil
}
