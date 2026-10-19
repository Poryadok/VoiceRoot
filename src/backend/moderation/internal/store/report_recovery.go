package store

import "context"

// RequirePrivacyRecovery verifies report participants after authoritative replay
// and before the shared coordinator admits effects. It never manufactures owner
// provenance, performs reciprocal RPCs, or opens a report-owned readiness flag.
func (s *ReportStore) RequirePrivacyRecovery(ctx context.Context) error {
	if s == nil || s.Pool == nil {
		return ErrReportPrivacy
	}
	var unresolved bool
	err := s.Pool.QueryRow(ctx, `SELECT EXISTS(
 SELECT 1 FROM reports r WHERE
  r.reporter_account_id IS NULL OR r.reporter_subject_id IS NULL OR r.reporter_subject_revision IS NULL
  OR NOT EXISTS(SELECT 1 FROM automod_subjects a
   WHERE a.account_id=r.reporter_account_id AND a.subject_id=r.reporter_subject_id
   AND a.lifecycle_revision>=r.reporter_subject_revision
   AND ((a.lifecycle_state='LIVE' AND a.purge_at IS NULL) OR (a.lifecycle_state='DELETED' AND a.purge_at>clock_timestamp()))
   AND NOT EXISTS(SELECT 1 FROM automod_terminal_subjects t WHERE t.subject_id=a.subject_id))
  OR (r.target_type='user' AND r.validation_state='VALID' AND (
   r.validated_target_account_id IS NULL OR r.validated_target_subject_id IS NULL OR r.validated_target_subject_revision IS NULL
   OR NOT EXISTS(SELECT 1 FROM automod_subjects a
    WHERE a.account_id=r.validated_target_account_id AND a.subject_id=r.validated_target_subject_id
    AND a.lifecycle_revision>=r.validated_target_subject_revision
    AND ((a.lifecycle_state='LIVE' AND a.purge_at IS NULL) OR (a.lifecycle_state='DELETED' AND a.purge_at>clock_timestamp()))
    AND NOT EXISTS(SELECT 1 FROM automod_terminal_subjects t WHERE t.subject_id=a.subject_id))
  )))`).Scan(&unresolved)
	if err != nil {
		return err
	}
	if unresolved {
		return ErrReportPrivacy
	}
	return nil
}
