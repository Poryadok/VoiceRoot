package automod

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Store) ThroughSequence(ctx context.Context, id uuid.UUID) (uint64, error) {
	var through uint64
	if s == nil || s.Pool == nil || id == uuid.Nil {
		return 0, ErrSubject
	}
	err := s.Pool.QueryRow(ctx, `SELECT through_sequence FROM automod_subjects WHERE subject_id=$1`, id).Scan(&through)
	return through, err
}

// RefreshSubject consumes fresh Auth lifecycle authority for an already
// provisioned stream. Evidence alone cannot create/reset the stream baseline.
func (s *Store) RefreshSubject(ctx context.Context, fresh Subject) error {
	if !validSubject(fresh) {
		return ErrEvidence
	}
	return s.accountTx(ctx, fresh.AccountID, func(tx pgx.Tx) error {
		if err := lockSubject(ctx, tx, fresh.ID); err != nil {
			return err
		}
		var terminal bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM automod_terminal_subjects WHERE subject_id=$1)`, fresh.ID).Scan(&terminal); err != nil {
			return err
		}
		if terminal {
			return ErrPurged
		}
		tag, err := tx.Exec(ctx, `UPDATE automod_subjects SET lifecycle_revision=$4,lifecycle_state=$5,purge_at=$6
   WHERE subject_id=$1 AND account_id=$2 AND source_epoch=$3 AND (lifecycle_revision<$4 OR
     (lifecycle_revision=$4 AND lifecycle_state=$5 AND purge_at IS NOT DISTINCT FROM $6))`, fresh.ID, fresh.AccountID, fresh.Epoch, fresh.Revision, fresh.State, fresh.PurgeAt)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return ErrSubject
		}
		return nil
	})
}
