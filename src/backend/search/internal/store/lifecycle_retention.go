package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
		if _, err = tx.Exec(ctx, `
			INSERT INTO search_space_purged_chat_fences(space_id,chat_id)
			SELECT DISTINCT i.space_id,i.chat_id
			FROM search_space_chat_manifest_items i
			JOIN search_space_purge_receipts p USING(space_id,deletion_operation_id)
			WHERE i.space_id=$1 AND p.retain_until < clock_timestamp()
			ON CONFLICT DO NOTHING`, spaceID); err != nil {
			return err
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
