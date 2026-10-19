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
 SELECT 1 WHERE NOT EXISTS(SELECT 1 FROM report_privacy_journal_metadata
  WHERE singleton AND journal_generation<>'00000000-0000-0000-0000-000000000000'::uuid)
 UNION ALL
 SELECT 1 FROM report_privacy_journal j WHERE
  j.report_id='00000000-0000-0000-0000-000000000000'::uuid
  OR j.evidence_id='00000000-0000-0000-0000-000000000000'::uuid
  OR j.subject_id='00000000-0000-0000-0000-000000000000'::uuid
  OR (j.evidence_kind='AUTH_TERMINAL' AND (
   j.subject_id IS NULL OR j.subject_id<>j.evidence_id
   OR NOT EXISTS(SELECT 1 FROM automod_terminal_subjects t WHERE t.subject_id=j.subject_id)))
 UNION ALL
 SELECT 1 FROM reports r
 LEFT JOIN report_privacy_journal jr ON jr.report_id=r.id AND jr.slot='REPORTER'
 LEFT JOIN report_privacy_journal jt ON jt.report_id=r.id AND jt.slot='USER_TARGET'
 WHERE
  r.reporter_provenance NOT IN ('VERIFIED_AT_SUBMISSION','OWNER_RECOVERED_LEGACY')
  OR jr.disposition IS DISTINCT FROM 'BOUND'
  OR jr.subject_id IS DISTINCT FROM r.reporter_subject_id
  OR (r.reporter_provenance='VERIFIED_AT_SUBMISSION' AND jr.evidence_kind IS DISTINCT FROM 'AUTH_SUBJECT')
  OR (r.reporter_provenance='OWNER_RECOVERED_LEGACY' AND jr.evidence_kind IS DISTINCT FROM 'USER_OWNER_FACT')
  OR (jt.report_id IS NOT NULL AND (r.target_type<>'user' OR jt.disposition<>'BOUND'))
  OR
  r.reporter_account_id IS NULL OR r.reporter_subject_id IS NULL OR r.reporter_subject_revision IS NULL
  OR NOT EXISTS(SELECT 1 FROM automod_subjects a
   WHERE a.account_id=r.reporter_account_id AND a.subject_id=r.reporter_subject_id
   AND a.lifecycle_revision>=r.reporter_subject_revision
   AND ((a.lifecycle_state='LIVE' AND a.purge_at IS NULL) OR (a.lifecycle_state='DELETED' AND a.purge_at>clock_timestamp()))
   AND NOT EXISTS(SELECT 1 FROM automod_terminal_subjects t WHERE t.subject_id=a.subject_id))
  OR (r.target_type='user' AND (r.validation_state='VALID' OR jt.report_id IS NOT NULL) AND (
   r.validated_target_provenance NOT IN ('VERIFIED_AT_VALIDATION','OWNER_RECOVERED_LEGACY')
   OR jt.disposition IS DISTINCT FROM 'BOUND'
   OR jt.subject_id IS DISTINCT FROM r.validated_target_subject_id
   OR (r.validated_target_provenance='VERIFIED_AT_VALIDATION' AND jt.evidence_kind IS DISTINCT FROM 'AUTH_SUBJECT')
   OR (r.validated_target_provenance='OWNER_RECOVERED_LEGACY' AND jt.evidence_kind IS DISTINCT FROM 'USER_OWNER_FACT')
   OR
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
