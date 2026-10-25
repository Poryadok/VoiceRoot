package store

import (
	"context"
	"time"
)

// CleanupMemberAdmissionEvidence erases private membership evidence at the
// existing Role P3 P30D boundary. The permanent Space retirement fence remains;
// no subsequent request may reuse the retired Space or its deleted operations.
func (s *RoleStore) CleanupMemberAdmissionEvidence(ctx context.Context) error {
	if s == nil || s.Pool == nil {
		return ErrScopeUnavailable
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	for _, table := range []string{"role_member_contributions", "role_member_owner_authority", "role_member_admission_preparations", "role_member_source_floors", "role_member_admission_receipts"} {
		if _, err := tx.Exec(ctx, "DELETE FROM "+table+" r USING role_space_lifecycle l WHERE l.space_id=r.space_id AND l.retired_at IS NOT NULL AND clock_timestamp()>=l.retired_at+interval '30 days'"); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
