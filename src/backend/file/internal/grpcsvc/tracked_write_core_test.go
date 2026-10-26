package grpcsvc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"voice/backend/file/internal/store"
)

// This is a physical-ledger fixture, not catalog source authorization or READY
// proof. No actor/import helper or executor dependency is present in this branch.
func TestTrackedWriteCorePinsDeletionAndSettlesAfterPublicationClosure(t *testing.T) {
	for _, ack := range []bool{true, false} {
		name := "ack"
		if !ack {
			name = "lost-ack"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			pool := startR23FilePostgres(t, ctx)
			applyFileGateSQL(t, ctx, pool, filepath.Join("src", "backend", "migrations", "file_db", "000010_catalog_asset_import.up.sql"))
			files := store.NewFilesStore(pool)
			file, gen := uuid.New(), uuid.New()
			key := "catalog/" + uuid.NewString() + "/" + gen.String() + "/source.gif"
			digest := sha256.Sum256([]byte("GIF89a-source"))
			_, err := pool.Exec(ctx, `INSERT INTO files(id,uploader_profile_id,original_name,mime_type,size_bytes,sha256_hash,r2_key,status,file_type,scan_result,upload_intent,catalog_asset,created_at)
VALUES($1,$2,'source.gif','image/gif',13,$3,$4,'pending_upload','image','skipped','gif',true,clock_timestamp()-interval '2 hours')`, file, uuid.New(), hex.EncodeToString(digest[:]), key)
			require.NoError(t, err)
			intent := store.TrackedWriteIntent{Key: key, FileID: file, Generation: gen, Kind: "catalog_source", ExpectedSHA256: digest[:], ExpectedSize: 13}
			tx, err := pool.Begin(ctx)
			require.NoError(t, err)
			require.NoError(t, store.DeclareTrackedWriteTx(ctx, tx, intent))
			require.NoError(t, tx.Commit(ctx))
			actual, err := files.StartTrackedWrite(ctx, key, gen)
			require.NoError(t, err)
			require.Equal(t, intent, actual)
			_, err = files.StartTrackedWrite(ctx, key, gen)
			require.Error(t, err, "no second PUT under one declaration")
			require.NoError(t, files.MarkDeleted(ctx, file))
			tx, err = pool.Begin(ctx)
			require.NoError(t, err)
			var blob uuid.UUID
			require.NoError(t, tx.QueryRow(ctx, `SELECT blob_id FROM files WHERE id=$1 FOR UPDATE`, file).Scan(&blob))
			eligible, err := store.CanCollectTrackedBlobTx(ctx, tx, blob)
			require.NoError(t, err)
			require.False(t, eligible, "deletion cannot clear STARTED pin")
			require.NoError(t, tx.Commit(ctx))
			// Kill the exact idle pooled session with a separate admin connection.
			// Settlement must retry only its idempotent DB bookkeeping, never PUT.
			cached, err := pool.Acquire(ctx)
			require.NoError(t, err)
			pid := cached.Conn().PgConn().PID()
			cached.Release()
			admin, err := pgx.Connect(ctx, pool.Config().ConnString())
			require.NoError(t, err)
			var killed bool
			require.NoError(t, admin.QueryRow(ctx, `SELECT pg_terminate_backend($1)`, pid).Scan(&killed))
			require.True(t, killed)
			require.NoError(t, admin.Close(ctx))
			// Do not consume/reconnect the dead pool session before Settle.
			canceled, stop := context.WithCancel(ctx)
			stop()
			require.NoError(t, files.SettleTrackedWrite(canceled, key, gen, ack))
			tx, err = pool.Begin(ctx)
			require.NoError(t, err)
			require.NoError(t, tx.QueryRow(ctx, `SELECT blob_id FROM files WHERE id=$1 FOR UPDATE`, file).Scan(&blob))
			eligible, err = store.CanCollectTrackedObjectTx(ctx, tx, file, key)
			require.NoError(t, err)
			require.Equal(t, ack, eligible)
			eligible, err = store.CanCollectTrackedBlobTx(ctx, tx, blob)
			require.NoError(t, err)
			require.Equal(t, ack, eligible)
			require.NoError(t, tx.Commit(ctx))
			canceled, stop = context.WithCancel(ctx)
			stop()
			require.NoError(t, files.SettleTrackedWrite(canceled, key, gen, ack))
			tx, err = pool.Begin(ctx)
			require.NoError(t, err)
			eligible, err = store.CanCollectTrackedBlobTx(ctx, tx, blob)
			require.NoError(t, err)
			require.Equal(t, ack, eligible)
			require.NoError(t, tx.Commit(ctx))
			if !ack {
				require.Error(t, files.SettleTrackedWrite(ctx, key, gen, true), "lost ACK cannot be re-cleared by assertion")
			}
		})
	}
}
