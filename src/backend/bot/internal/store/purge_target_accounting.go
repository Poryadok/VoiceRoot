package store

import (
	"bytes"
	"context"
	"sort"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func sameAcceptedIDs(a, b []uuid.UUID) bool {
	if len(a) != len(b) {
		return false
	}
	a, b = append([]uuid.UUID(nil), a...), append([]uuid.UUID(nil), b...)
	sort.Slice(a, func(i, j int) bool { return bytes.Compare(a[i][:], a[j][:]) < 0 })
	sort.Slice(b, func(i, j int) bool { return bytes.Compare(b[i][:], b[j][:]) < 0 })
	for i := range a {
		if a[i] == uuid.Nil || a[i] != b[i] || (i > 0 && a[i] == a[i-1]) {
			return false
		}
	}
	return true
}

func sealLifecycleMembershipTx(ctx context.Context, tx pgx.Tx, claim *BotDeletion) error {
	_, err := tx.Exec(ctx, lifecycleQuery(claim, `UPDATE bot_deletion_operations o SET target_ids=ARRAY(SELECT t.target_id FROM bot_deletion_installations t WHERE t.operation_id=o.operation_id ORDER BY t.target_id),target_membership_sealed=true WHERE operation_id=$1`), claim.OperationID)
	return err
}

// The caller owns the root lock. Minimal local rows are an explicit positive
// outcome, never an inference from a disappeared rich target or missing effect.
func lifecycleAccountingTx(ctx context.Context, tx pgx.Tx, claim *BotDeletion) (complete bool, localOnly bool, err error) {
	var expected []uuid.UUID
	var sealed bool
	if err = tx.QueryRow(ctx, lifecycleQuery(claim, `SELECT target_ids,target_membership_sealed FROM bot_deletion_operations WHERE operation_id=$1`), claim.OperationID).Scan(&expected, &sealed); err != nil {
		return false, false, err
	}
	if !sealed {
		return false, false, ErrInstallationConflict
	}
	rows, e := tx.Query(ctx, lifecycleQuery(claim, `SELECT target_id,effect_ids,disposition,local_result_id FROM bot_deletion_installations WHERE operation_id=$1 ORDER BY target_id`), claim.OperationID)
	if e != nil {
		return false, false, e
	}
	type target struct {
		id          uuid.UUID
		effects     []uuid.UUID
		disposition string
		result      *uuid.UUID
	}
	var targets []target
	var actual []uuid.UUID
	for rows.Next() {
		var v target
		if e = rows.Scan(&v.id, &v.effects, &v.disposition, &v.result); e != nil {
			break
		}
		targets = append(targets, v)
		actual = append(actual, v.id)
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		return false, false, e
	}
	if !sameAcceptedIDs(expected, actual) {
		return false, false, ErrInstallationConflict
	}
	complete, localOnly = true, len(targets) > 0
	for _, t := range targets {
		if len(t.effects) < 2 {
			return false, false, ErrInstallationConflict
		}
		local := t.disposition == "LOCAL_P3_COVERED"
		if (!local && t.disposition != "ACTIVE") || (local && (t.result == nil || *t.result == uuid.Nil)) {
			return false, false, ErrInstallationConflict
		}
		localOnly = localOnly && local
		rows, e = tx.Query(ctx, lifecycleQuery(claim, `SELECT effect_id,state FROM bot_deletion_effects WHERE target_id=$1 ORDER BY effect_id`), t.id)
		if e != nil {
			return false, false, e
		}
		actual = nil
		for rows.Next() {
			var id uuid.UUID
			var state string
			if e = rows.Scan(&id, &state); e != nil {
				break
			}
			actual = append(actual, id)
			if local {
				if state != "LOCAL_P3_COVERED" {
					e = ErrInstallationConflict
					break
				}
			} else {
				if state == "LOCAL_P3_COVERED" {
					e = ErrInstallationConflict
					break
				}
				complete = complete && state == "COMPLETE"
			}
		}
		if e == nil {
			e = rows.Err()
		}
		rows.Close()
		if e != nil {
			return false, false, e
		}
		if !sameAcceptedIDs(t.effects, actual) {
			return false, false, ErrInstallationConflict
		}
	}
	return complete, localOnly, nil
}

func (s *BotStore) completeLocallyRetiredUninstall(ctx context.Context, claim *BotDeletion) (bool, error) {
	var exists bool
	err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM bot_uninstall_installations WHERE operation_id=$1 AND disposition='LOCAL_P3_COVERED')`, claim.OperationID).Scan(&exists)
	if err != nil || !exists {
		return false, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return true, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	// No erased Space/intent is reconstructed and no Space lock follows Bot.
	var bot uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT b.id FROM bots b JOIN bot_uninstall_operations o ON o.bot_id=b.id WHERE o.operation_id=$1 FOR UPDATE OF b`, claim.OperationID).Scan(&bot); err != nil {
		return true, err
	}
	if err = s.lockLifecycleClaim(ctx, tx, claim); err != nil {
		return true, err
	}
	complete, local, err := lifecycleAccountingTx(ctx, tx, claim)
	if err != nil {
		return true, err
	}
	if !complete || !local {
		return true, ErrDeletionPending
	}
	var count int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM bot_uninstall_installations WHERE operation_id=$1`, claim.OperationID).Scan(&count); err != nil {
		return true, err
	}
	if count != 1 {
		return true, ErrInstallationConflict
	}
	r, err := tx.Exec(ctx, `UPDATE bot_uninstall_operations SET state='COMPLETE',completed_at=clock_timestamp(),claimed_until=NULL,last_error='' WHERE operation_id=$1 AND claim_generation=$2 AND claimed_until>clock_timestamp()`, claim.OperationID, claim.ClaimGeneration)
	if err != nil {
		return true, err
	}
	if r.RowsAffected() != 1 {
		return true, ErrDeletionClaimLost
	}
	return true, tx.Commit(ctx)
}
