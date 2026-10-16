package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PruneExpiredLifecycleEvidence removes only replay evidence after its fixed
// retention period. Permanent fences and purged chat tombstones are retained.
func PruneExpiredLifecycleEvidence(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return nil
	}
	ready, err := lifecycleSchemaReady(ctx, pool)
	if err != nil {
		return err
	}
	if !ready {
		return errors.New("search lifecycle schema unavailable")
	}
	rows, err := pool.Query(ctx, `
		SELECT DISTINCT space_id FROM (
			SELECT space_id FROM search_space_lifecycle_receipts WHERE retain_until < clock_timestamp()
			UNION ALL
			SELECT space_id FROM search_space_lifecycle_operations WHERE retain_until < clock_timestamp()
			UNION ALL
			SELECT space_id FROM search_space_purge_receipts WHERE retain_until < clock_timestamp()
		) expired`)
	if err != nil {
		return err
	}
	var spaces []uuid.UUID
	for rows.Next() {
		var spaceID uuid.UUID
		if err = rows.Scan(&spaceID); err != nil {
			rows.Close()
			return err
		}
		spaces = append(spaces, spaceID)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	for _, spaceID := range spaces {
		if err = pruneExpiredLifecycleSpace(ctx, pool, spaceID); err != nil {
			return err
		}
	}
	return nil
}

func pruneExpiredLifecycleSpace(ctx context.Context, pool *pgxpool.Pool, spaceID uuid.UUID) (err error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(context.Background())
		}
	}()
	if err = AcquireLifecycleSpaceLock(ctx, tx, spaceID); err != nil {
		return err
	}
	if ready, readyErr := lifecycleSchemaReady(ctx, tx); readyErr != nil {
		return readyErr
	} else if !ready {
		return errors.New("search lifecycle schema unavailable")
	}

	var expiredPurge bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(
		SELECT 1 FROM search_space_purge_receipts p
		JOIN search_space_lifecycle_fences f USING(space_id)
		WHERE p.space_id=$1 AND p.retain_until < clock_timestamp() AND f.state='PURGED'
	)`, spaceID).Scan(&expiredPurge); err != nil {
		return err
	}
	if expiredPurge {
		var sourceOperations, sourceItems int64
		var sourceComplete bool
		if err = tx.QueryRow(ctx, `
			WITH expired AS (
				SELECT p.space_id,p.deletion_operation_id,p.generation,
				       f.deletion_operation_id AS fence_operation_id,f.generation AS fence_generation,
				       f.state,m.manifest_id,m.manifest_sha256,m.item_count,
				       count(i.chat_id) AS actual_item_count
				FROM search_space_purge_receipts p
				LEFT JOIN search_space_lifecycle_fences f ON f.space_id=p.space_id
				LEFT JOIN search_space_chat_manifests m
				  ON m.space_id=p.space_id AND m.deletion_operation_id=p.deletion_operation_id
				 AND m.generation=p.generation-1
				LEFT JOIN search_space_chat_manifest_items i
				  ON i.space_id=m.space_id AND i.deletion_operation_id=m.deletion_operation_id
			 AND i.generation=m.generation
			WHERE p.space_id=$1 AND p.retain_until < clock_timestamp()
			GROUP BY p.space_id,p.deletion_operation_id,p.generation,
			         f.deletion_operation_id,f.generation,f.state,
			         m.manifest_id,m.manifest_sha256,m.item_count
			)
			SELECT count(*),coalesce(sum(actual_item_count),0),coalesce(bool_and(
				state='PURGED' AND fence_operation_id=deletion_operation_id
				AND fence_generation=generation AND generation>=2
				AND manifest_id IS NOT NULL AND manifest_id<>''
				AND octet_length(manifest_sha256)=32 AND item_count=actual_item_count
			),false)
			FROM expired`, spaceID).Scan(&sourceOperations, &sourceItems, &sourceComplete); err != nil {
			return err
		}
		if sourceOperations == 0 || !sourceComplete {
			return errors.New("cannot compact incomplete Search Chat purge evidence")
		}
		var inserted pgconn.CommandTag
		inserted, err = tx.Exec(ctx, `
			INSERT INTO search_space_purged_chat_fences
				(space_id,chat_id,deletion_operation_id,event_generation,manifest_id,manifest_sha256)
			SELECT i.space_id,i.chat_id,p.deletion_operation_id,p.generation,m.manifest_id,m.manifest_sha256
			FROM search_space_purge_receipts p
			JOIN search_space_lifecycle_fences f
			  ON f.space_id=p.space_id AND f.deletion_operation_id=p.deletion_operation_id
			 AND f.generation=p.generation AND f.state='PURGED'
			JOIN search_space_chat_manifests m
			  ON m.space_id=p.space_id AND m.deletion_operation_id=p.deletion_operation_id
			 AND m.generation=p.generation-1
			JOIN search_space_chat_manifest_items i
			  ON i.space_id=m.space_id AND i.deletion_operation_id=m.deletion_operation_id
			 AND i.generation=m.generation
			WHERE p.space_id=$1 AND p.retain_until < clock_timestamp()
			ON CONFLICT (space_id,chat_id) DO UPDATE SET
				deletion_operation_id=EXCLUDED.deletion_operation_id,
				event_generation=EXCLUDED.event_generation,
				manifest_id=EXCLUDED.manifest_id,
				manifest_sha256=EXCLUDED.manifest_sha256
			WHERE (search_space_purged_chat_fences.deletion_operation_id IS NULL
			  AND search_space_purged_chat_fences.event_generation IS NULL
			  AND search_space_purged_chat_fences.manifest_id IS NULL
			  AND search_space_purged_chat_fences.manifest_sha256 IS NULL
			   OR (search_space_purged_chat_fences.deletion_operation_id=EXCLUDED.deletion_operation_id
			  AND search_space_purged_chat_fences.event_generation=EXCLUDED.event_generation
			  AND search_space_purged_chat_fences.manifest_id=EXCLUDED.manifest_id
			  AND search_space_purged_chat_fences.manifest_sha256=EXCLUDED.manifest_sha256))`, spaceID)
		if err != nil {
			return err
		}
		if inserted.RowsAffected() != sourceItems {
			return errors.New("search chat purge evidence conflicts with compact terminal binding")
		}
		for _, statement := range []string{
			`DELETE FROM search_space_chat_manifest_items i USING search_space_purge_receipts p WHERE i.space_id=$1 AND p.space_id=i.space_id AND p.deletion_operation_id=i.deletion_operation_id AND p.retain_until < clock_timestamp()`,
			`DELETE FROM search_space_chat_manifest_pages p USING search_space_purge_receipts r WHERE p.space_id=$1 AND r.space_id=p.space_id AND r.deletion_operation_id=p.deletion_operation_id AND r.retain_until < clock_timestamp()`,
			`DELETE FROM search_space_chat_manifests m USING search_space_purge_receipts p WHERE m.space_id=$1 AND p.space_id=m.space_id AND p.deletion_operation_id=m.deletion_operation_id AND p.retain_until < clock_timestamp()`,
		} {
			if _, err = tx.Exec(ctx, statement, spaceID); err != nil {
				return err
			}
		}
	}
	for _, table := range []string{"search_space_lifecycle_receipts", "search_space_lifecycle_operations", "search_space_purge_receipts"} {
		if _, err = tx.Exec(ctx, "DELETE FROM "+table+" WHERE space_id=$1 AND retain_until < clock_timestamp()", spaceID); err != nil {
			return err
		}
	}
	err = tx.Commit(ctx)
	return err
}
