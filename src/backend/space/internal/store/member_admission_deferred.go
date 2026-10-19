package store

import "context"

// PostgreSQL otherwise executes deferred constraints inside COMMIT, after the
// final initiating-call restriction. Flush the real checks as the last SQL so
// their waits are included in that oldest ingress budget. No SQL/source write
// follows before the pure continuation/call checks and COMMIT.
func (s *SpaceStore) flushMemberAdmissionDeferredChecks(ctx context.Context, capture *effectiveMemberMutations) error {
	if s == nil || s.tx == nil {
		return ErrEffectiveEpisodeUnavailable
	}
	if capture != nil && (capture.tx != s.tx || !capture.used || (capture.dbMutation != nil && !capture.dbMutation.used)) {
		return ErrEffectiveEpisodeUnavailable
	}
	if _, err := s.tx.Exec(ctx, `SET CONSTRAINTS ALL IMMEDIATE`); err != nil {
		return ErrEffectiveEpisodeUnavailable
	}
	return nil
}
