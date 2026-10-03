package store

import (
	"context"
	"github.com/google/uuid"
)

// CleanupLifecycleEvidence removes private coordinator wire bodies thirty days
// after the first local completion. Compact tuples expire with the tombstone.
func (s *SpaceStore) CleanupLifecycleEvidence(ctx context.Context) error {
	if s == nil || s.Pool == nil {
		return ErrLifecycleEvidenceInvalid
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollbackLifecycleTx(ctx, tx)
	rows, err := tx.Query(ctx, `SELECT a.space_id FROM space_lifecycle_aggregates a WHERE (a.phase='PURGED' AND a.completed_at<=clock_timestamp()-interval '30 days' AND NOT a.evidence_compacted) OR EXISTS(SELECT 1 FROM space_lifecycle_operations o WHERE o.space_id=a.space_id AND o.state='COMPLETED' AND o.completed_at<=clock_timestamp()-interval '30 days' AND NOT(o.method='DELETE' AND o.operation_id=a.deletion_operation_id AND a.phase<>'LIVE')) ORDER BY a.space_id LIMIT 100`)
	if err != nil {
		return err
	}
	var spaces []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		spaces = append(spaces, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range spaces {
		if err = lockLifecycleSpace(ctx, tx, id); err != nil {
			return err
		}
		var ready bool
		if err = tx.QueryRow(ctx, `SELECT phase='PURGED' AND local_purge_completed AND completed_at<=clock_timestamp()-interval '30 days' AND NOT evidence_compacted AND EXISTS(SELECT 1 FROM space_deletion_tombstones t WHERE t.space_id=a.space_id) FROM space_lifecycle_aggregates a WHERE space_id=$1 FOR UPDATE`, id).Scan(&ready); err != nil {
			return err
		}
		if !ready {
			// Historical API outcomes use their own first-completion clock. The
			// current DELETE proof remains coordinator evidence until PURGED.
			if _, err = tx.Exec(ctx, `DELETE FROM space_lifecycle_operations o WHERE o.space_id=$1 AND o.state='COMPLETED' AND o.completed_at<=clock_timestamp()-interval '30 days' AND NOT EXISTS(SELECT 1 FROM space_lifecycle_aggregates a WHERE a.space_id=o.space_id AND a.deletion_operation_id=o.operation_id AND a.phase<>'LIVE')`, id); err != nil {
				return err
			}
			continue
		}
		command, err := tx.Exec(ctx, `INSERT INTO space_lifecycle_completions(space_id,deletion_operation_id,generation,participant_id,request_kind,request_sha256,receipt_sha256,manifest_sha256,completed_at) SELECT p.space_id,p.deletion_operation_id,p.generation,p.participant_id,p.request_kind,p.request_sha256,p.receipt_sha256,p.manifest_sha256,p.completed_at FROM space_lifecycle_participants p JOIN space_lifecycle_aggregates a ON a.space_id=p.space_id AND a.deletion_operation_id=p.deletion_operation_id AND a.generation=p.generation WHERE p.space_id=$1 AND p.progress='COMPLETE' AND p.request_kind IN ('ROLE_RETIREMENT','PURGE')`, id)
		if err != nil {
			return err
		}
		if command.RowsAffected() != 10 {
			return ErrLifecycleEvidenceInvalid
		}
		for _, query := range []string{`DELETE FROM space_lifecycle_participants WHERE space_id=$1`, `DELETE FROM space_lifecycle_operations WHERE space_id=$1`, `UPDATE space_lifecycle_aggregates SET evidence_compacted=TRUE WHERE space_id=$1`} {
			if _, err = tx.Exec(ctx, query, id); err != nil {
				return err
			}
		}
	}
	// UTC scalar tombstone timestamps are intentional in migration 000013.
	if _, err = tx.Exec(ctx, `DELETE FROM space_lifecycle_outbox WHERE state='DELIVERED' AND delivered_at<=clock_timestamp()-interval '30 days'`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM space_lifecycle_aggregates a USING space_deletion_tombstones t WHERE a.space_id=t.space_id AND a.phase='PURGED' AND a.evidence_compacted AND t.retain_until<=clock_timestamp() AT TIME ZONE 'UTC'`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM space_deletion_tombstones WHERE retain_until<=clock_timestamp() AT TIME ZONE 'UTC'`); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
