package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func checkNativeTerminal47Triggers(ctx context.Context, tx pgx.Tx) error {
	var valid bool
	if err := tx.QueryRow(ctx, `SELECT count(*)=2 FROM pg_trigger t
 JOIN pg_class c ON c.oid=t.tgrelid JOIN pg_namespace n ON n.oid=c.relnamespace
 JOIN pg_proc p ON p.oid=t.tgfoid JOIN pg_namespace pn ON pn.oid=p.pronamespace
 WHERE n.nspname='native_membership_guard' AND c.relname='catalog47_transition'
 AND ((t.tgname='terminal_catalog47_immutable' AND t.tgtype=27)
  OR (t.tgname='terminal_catalog47_no_truncate' AND t.tgtype=34))
 AND t.tgenabled='O' AND NOT t.tgisinternal AND NOT t.tgdeferrable AND NOT t.tginitdeferred
 AND t.tgnargs=0 AND t.tgargs=''::bytea AND pn.nspname='native_membership_guard'
 AND p.proname='bot_global_immutable' AND p.pronargs=0`).Scan(&valid); err != nil || !valid {
		return ErrMemberAuthorityUnavailable
	}
	return nil
}

func (s *SpaceStore) cleanupLifecycleEvidence47(ctx context.Context) error {
	if s.tx != nil {
		return ErrLifecycleEvidenceInvalid
	}
	// Discovery confers no authority. Each selected Space is reread under its
	// own RC transaction/fence; no global DELETE outlives a previous Space lock.
	rows, err := s.Pool.Query(ctx, `SELECT space_id FROM (
 SELECT a.space_id FROM public.space_lifecycle_aggregates a WHERE
  (a.phase='PURGED' AND NOT a.evidence_compacted AND a.completed_at+interval '30 days'<=clock_timestamp()) OR EXISTS(
   SELECT 1 FROM public.space_lifecycle_operations o WHERE o.space_id=a.space_id AND o.state='COMPLETED'
    AND o.completed_at+interval '30 days'<=clock_timestamp()
    AND NOT(o.method='DELETE' AND o.operation_id=a.deletion_operation_id AND a.phase<>'LIVE'))
 UNION SELECT space_id FROM public.space_lifecycle_outbox WHERE state='DELIVERED' AND delivered_at+interval '30 days'<=clock_timestamp()
 UNION SELECT space_id FROM public.space_deletion_tombstones WHERE retain_until<=clock_timestamp() AT TIME ZONE 'UTC'
 UNION SELECT f.space_id FROM public.space_chat_creation_fences f JOIN public.space_chat_creation_evidence e USING(operation_id)
  WHERE f.full_evidence_expires_at<=clock_timestamp()
 ) candidates ORDER BY space_id LIMIT 100`)
	if err != nil {
		return err
	}
	var spaces []uuid.UUID
	for rows.Next() {
		var space uuid.UUID
		if err = rows.Scan(&space); err != nil {
			rows.Close()
			return err
		}
		spaces = append(spaces, space)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, space := range spaces {
		if err := s.cleanupLifecycleSpace47(ctx, space); err != nil {
			return err
		}
	}
	// Audit ingress minimization has its own immutable row/deadline guards.
	// No Space/advisory lock is acquired after these row locks.
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer rollbackLifecycleTx(ctx, tx)
	if err = cleanupAuditIngressEvidence(ctx, tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *SpaceStore) cleanupLifecycleSpace47(ctx context.Context, space uuid.UUID) error {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer rollbackLifecycleTx(ctx, tx)
	var started time.Time
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&started); err != nil {
		return err
	}
	if err = lockLifecycleSpace(ctx, tx, space); err != nil {
		return err
	}
	if err = checkMemberAuthorityCatalog(ctx, tx); err != nil {
		return err
	}
	var terminal bool
	if err = tx.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM public.spaces WHERE id=$1)
 AND EXISTS(SELECT 1 FROM public.space_permanent_purge_fences WHERE space_id=$1)`, space).Scan(&terminal); err != nil {
		return err
	}
	scoped := *s
	scoped.tx = tx
	var capture *effectiveMemberMutations
	if terminal {
		var version int64
		if err = tx.QueryRow(ctx, `SELECT version FROM public.schema_migrations WHERE NOT dirty`).Scan(&version); err != nil {
			return err
		}
		if version == 49 {
			// Child evidence shares the parent's first PURGED+30d deadline.
			// Erase its complete row before the fixed parent compaction removes
			// full coordinator evidence, in this same fenced transaction.
			if _, err = tx.Exec(ctx, `SELECT public.space_bot_purge_child_expire($1)`, space); err != nil {
				return err
			}
		}
		for _, query := range []string{
			`SELECT public.space_native_terminal_compact($1)`,
			`SELECT public.space_native_terminal_expire_events($1)`,
			`SELECT public.space_native_terminal_expire($1)`,
		} {
			if _, err = tx.Exec(ctx, query, space); err != nil {
				return err
			}
		}
	} else {
		capture, err = scoped.captureLifecycleMaintenanceMutation(ctx, space)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `DELETE FROM public.space_lifecycle_operations o WHERE o.space_id=$1 AND o.state='COMPLETED'
 AND o.completed_at+interval '30 days'<=clock_timestamp() AND NOT EXISTS(
  SELECT 1 FROM public.space_lifecycle_aggregates a WHERE a.space_id=o.space_id AND a.deletion_operation_id=o.operation_id AND a.phase<>'LIVE')`, space); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `DELETE FROM public.space_lifecycle_outbox WHERE space_id=$1 AND state='DELIVERED' AND delivered_at+interval '30 days'<=clock_timestamp()`, space); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, `DELETE FROM public.space_chat_creation_evidence e USING public.space_chat_creation_fences f
 WHERE e.operation_id=f.operation_id AND f.space_id=$1 AND f.full_evidence_expires_at<=clock_timestamp()`, space); err != nil {
		return err
	}
	if !terminal {
		if err = scoped.finishLifecycleMaintenanceMutation(ctx, capture); err != nil {
			return err
		}
	}
	var fresh bool
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()<$1::timestamptz+interval '5 seconds'`, started).Scan(&fresh); err != nil || !fresh {
		return ErrMemberAuthorityUnavailable
	}
	return tx.Commit(ctx)
}

// Terminal operations never open a membership mutation frame. The fixed DB
// helpers authenticate the serving session, retained P3 disposition and exact
// representation, and consume their own private scope before returning.
func (s *SpaceStore) markTerminalLifecycleEventDelivered(ctx context.Context, tx pgx.Tx, space, event uuid.UUID, raw, hash []byte) (bool, error) {
	var terminal bool
	if err := tx.QueryRow(ctx, `SELECT to_regprocedure('public.space_native_membership_require(uuid)') IS NOT NULL
 AND NOT EXISTS(SELECT 1 FROM public.spaces WHERE id=$1)
 AND EXISTS(SELECT 1 FROM public.space_permanent_purge_fences WHERE space_id=$1)`, space).Scan(&terminal); err != nil {
		return false, err
	}
	if !terminal {
		return false, nil
	}
	var version int64
	if err := tx.QueryRow(ctx, `SELECT version FROM public.schema_migrations WHERE NOT dirty`).Scan(&version); err != nil {
		return true, err
	}
	if version != 47 && (version != 48 && version != 49) {
		// Preserve historical44/45/46 replay and its existing fail-closed FIRST
		// behavior. This forward helper does not reinterpret those schemas.
		if version == 44 || version == 45 || version == 46 {
			return false, nil
		}
		return true, ErrMemberAuthorityUnavailable
	}
	if err := checkMemberAuthorityCatalog(ctx, tx); err != nil {
		return true, err
	}
	_, err := tx.Exec(ctx, `SELECT public.space_native_terminal_event_delivered($1,$2,$3,$4)`, space, event, raw, hash)
	return true, err
}
