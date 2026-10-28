package store

import (
	"bytes"
	"context"
	"github.com/google/uuid"
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
	defer func() { _ = tx.Rollback(context.Background()) }()
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
	if err = d.next.DeleteObject(ctx, key); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
