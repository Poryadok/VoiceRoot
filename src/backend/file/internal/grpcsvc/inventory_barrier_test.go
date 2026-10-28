package grpcsvc

import (
	"context"
	"github.com/stretchr/testify/require"
	"path/filepath"
	"testing"
	"voice/backend/file/internal/store"
)

// This regression deliberately uses the actual GC and legacy delete consumers.
// The historical owner's page has not arrived, so a per-blob hold cannot help.
func TestInventoryBarrierProtectsUnimportedHistory(t *testing.T) {
	ctx := r23TestContext(t)
	pool := startR23FilePostgres(t, ctx)
	file := insertR23ReadyFile(t, ctx, pool, "unimported-history")
	applyFileGateSQL(t, ctx, pool, filepath.Join("src", "backend", "migrations", "file_db", "000016_reference_inventory.up.sql"))
	_, err := pool.Exec(ctx, `UPDATE file_reference_inventory_gate SET blocked=true`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE file_blobs SET state='GC_PENDING' WHERE blob_id=(SELECT blob_id FROM files WHERE id=$1)`, file)
	require.NoError(t, err)
	files := store.NewFilesStore(pool)
	deleter := &r23ScriptedDeleter{}
	n, err := files.RunReferenceGCOnce(ctx, deleter, 1)
	require.NoError(t, err)
	require.Zero(t, n, "incomplete owner inventory must stop GC before pages arrive")
	require.Empty(t, deleter.calls)
	_, err = files.CleanupGIFOnce(ctx, deleter)
	require.ErrorIs(t, err, store.ErrInventoryBarrier, "actual alternate derivative cleanup obeys migration barrier")
	require.Empty(t, deleter.calls)
	_, err = files.DeleteLegacyFile(ctx, file, nil, deleter)
	require.Error(t, err, "uploader deletion cannot bypass an incomplete inventory")
	row, err := files.GetFileByID(context.Background(), file)
	require.NoError(t, err)
	require.Equal(t, "ready", row.Status)
}
