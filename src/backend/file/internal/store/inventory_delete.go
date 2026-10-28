package store

import (
	"bytes"
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"sort"
	"time"
	"voice/backend/file/internal/r2file"
)

type inventoryDeleteContextKey struct{}
type inventoryObjectDeleter struct {
	files *FilesStore
	next  r2file.ObjectDeleter
}

// InventoryGuardedDeleter covers legacy image/original/error cleanup paths that
// perform storage I/O outside blob GC. It holds the same first guard throughout
// one bounded DELETE. GC already owns that guard and its blob lock; its private
// context marker avoids recursively opening a second transaction.
func (s *FilesStore) InventoryGuardedDeleter(next r2file.ObjectDeleter) r2file.ObjectDeleter {
	if next == nil {
		return nil
	}
	if _, ok := next.(*inventoryObjectDeleter); ok {
		return next
	}
	return &inventoryObjectDeleter{files: s, next: next}
}
func guardedInventoryDeleteContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, inventoryDeleteContextKey{}, true)
}
func (d *inventoryObjectDeleter) DeleteObject(ctx context.Context, key string) error {
	if ctx.Value(inventoryDeleteContextKey{}) == true {
		return d.next.DeleteObject(ctx, key)
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := BeginInventoryMutation(ctx, d.files.Pool, true)
	if err != nil {
		return err
	}
	admissionTx := tx
	defer func() { _ = admissionTx.Rollback(context.Background()) }()
	// An absent migration schema preserves pre-reference legacy cleanup; no
	// inventory holds can exist in that older schema.
	if inventory, e := inventoryHoldsExistTx(ctx, tx); e != nil {
		return e
	} else if !inventory {
		return d.next.DeleteObject(ctx, key)
	}
	generation, err := LockInventoryGuardTx(ctx, tx, true)
	if err != nil {
		return err
	}
	attempt, err := insertInventoryDelete(ctx, tx, key, generation)
	if err != nil {
		return err
	}
	// STARTED is durable before touching storage. Cohort creation now either
	// waits for the physical guard or rejects this outstanding remote attempt.
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	submitted := false
	defer func() {
		if !submitted {
			if tx != nil {
				_ = tx.Rollback(context.Background())
			}
			settle, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, _ = d.files.Pool.Exec(settle, `UPDATE file_reference_inventory_deletes SET state='NOT_SENT' WHERE attempt_id=$1 AND state='STARTED'`, attempt)
		}
	}()
	tx, err = BeginInventoryMutation(ctx, d.files.Pool, true)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	currentGeneration, e := LockInventoryGuardTx(ctx, tx, true)
	if e != nil {
		return e
	}
	if currentGeneration != generation {
		return ErrInventoryBarrier
	}
	var state string
	if err = tx.QueryRow(ctx, `SELECT state FROM file_reference_inventory_deletes WHERE attempt_id=$1 AND object_key=$2 AND guard_generation=$3 FOR UPDATE`, attempt, key, generation).Scan(&state); err != nil {
		return err
	}
	if state != "STARTED" {
		return ErrInventoryBarrier
	}
	rows, err := tx.Query(ctx, `SELECT id,blob_id FROM files WHERE r2_key=$1 OR converted_r2_key=$1 OR thumbnail_r2_key=$1 ORDER BY id FOR NO KEY UPDATE`, key)
	if err != nil {
		return err
	}
	var blobs []uuid.UUID
	for rows.Next() {
		var file uuid.UUID
		var blob *uuid.UUID
		if err = rows.Scan(&file, &blob); err != nil {
			rows.Close()
			return err
		}
		if blob != nil {
			blobs = append(blobs, *blob)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	// Blob key custody survives retirement/removal of the File alias metadata.
	rows, err = tx.Query(ctx, `SELECT blob_id FROM file_blobs WHERE original_r2_key=$1 OR converted_r2_key=$1 OR thumbnail_r2_key=$1 ORDER BY blob_id`, key)
	if err != nil {
		return err
	}
	for rows.Next() {
		var blob uuid.UUID
		if err = rows.Scan(&blob); err != nil {
			rows.Close()
			return err
		}
		blobs = append(blobs, blob)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	// Ordering at the blob layer is independent of alias/File ordering.
	sort.Slice(blobs, func(i, j int) bool { return bytes.Compare(blobs[i][:], blobs[j][:]) < 0 })
	for _, blob := range blobs {
		var id uuid.UUID
		if err = tx.QueryRow(ctx, `SELECT blob_id FROM file_blobs WHERE blob_id=$1 FOR UPDATE`, blob).Scan(&id); err != nil {
			return err
		}
		held, e := inventoryBlobHeldTx(ctx, tx, blob)
		if e != nil {
			return e
		}
		if held {
			return ErrInventoryBarrier
		}
	}
	submitted = true
	physicalErr := d.next.DeleteObject(ctx, key)
	return finishInventoryDeleteTx(tx, attempt, physicalErr)
}

// q must be a transaction holding the shared guard, or a pool called while the
// same caller retains that guard in its physical transaction. There are no File
// FKs or cohort lock upgrades in this insert-only storage admission.
func insertInventoryDelete(ctx context.Context, q fileRowQueryer, key string, generation int64) (uuid.UUID, error) {
	attempt := uuid.New()
	var inserted uuid.UUID
	err := q.QueryRow(ctx, `INSERT INTO file_reference_inventory_deletes(attempt_id,object_key,guard_generation,state) VALUES($1,$2,$3,'STARTED') ON CONFLICT DO NOTHING RETURNING attempt_id`, attempt, key, generation).Scan(&inserted)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrInventoryBarrier
	}
	return inserted, err
}

func finishInventoryDeleteTx(tx pgx.Tx, attempt uuid.UUID, physicalErr error) error {
	// An actual ACK is independent of caller cancellation. A failed SQL update or
	// commit conservatively leaves STARTED; neither timeout nor restart clears it.
	settle, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := setInventoryDeleteOutcomeTx(settle, tx, attempt, physicalErr)
	if err != nil {
		return errors.Join(physicalErr, err)
	}
	return errors.Join(physicalErr, tx.Commit(settle))
}

func setInventoryDeleteOutcomeTx(ctx context.Context, tx pgx.Tx, attempt uuid.UUID, physicalErr error) error {
	state := "ACKED"
	if physicalErr != nil {
		state = "UNSETTLED"
	}
	_, err := tx.Exec(ctx, `UPDATE file_reference_inventory_deletes SET state=$2,acknowledged_at=CASE WHEN $2='ACKED' THEN clock_timestamp() ELSE NULL END WHERE attempt_id=$1 AND state='STARTED'`, attempt, state)
	return err
}
