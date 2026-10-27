package grpcsvc

import (
	"context"
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/require"
	"voice/backend/file/internal/store"
)

func TestTrackedWriteDeclarationRequiresMeasuredFileAndImmutableIdentity(t *testing.T) {
	ctx := context.Background()
	f := seedPendingCatalog(t, ctx)
	digest := sha256.Sum256([]byte("GIF89a-source"))
	intent := store.TrackedWriteIntent{Key: f.imported.R2Key, FileID: f.imported.FileID, Generation: f.imported.Generation, Kind: "catalog_source", ExpectedSHA256: digest[:], ExpectedSize: 13}
	tx, err := f.pool.Begin(ctx)
	require.NoError(t, err)
	require.Error(t, store.DeclareTrackedWriteTx(ctx, tx, intent))
	require.NoError(t, tx.Rollback(ctx))
	require.NoError(t, f.files.RecordCatalogFetched(ctx, f.imported, f.capture, f.keys, 13, digest[:]))
	tx, err = f.pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, store.DeclareTrackedWriteTx(ctx, tx, intent))
	require.NoError(t, tx.Commit(ctx))
	wrong := intent
	wrong.ExpectedSize++
	tx, err = f.pool.Begin(ctx)
	require.NoError(t, err)
	require.Error(t, store.DeclareTrackedWriteTx(ctx, tx, wrong))
	require.NoError(t, tx.Rollback(ctx))
}

func TestTrackedWriteSettlementDoesNotDependOnDeletedPublicationAuthority(t *testing.T) {
	for _, ack := range []bool{true, false} {
		name := "ack"
		if !ack {
			name = "lost-ack"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			f := seedPendingCatalog(t, ctx)
			digest := sha256.Sum256([]byte("GIF89a-source"))
			require.NoError(t, f.files.RecordCatalogFetched(ctx, f.imported, f.capture, f.keys, 13, digest[:]))
			intent, err := f.files.StartTrackedWrite(ctx, f.imported.R2Key, f.imported.Generation)
			require.NoError(t, err)
			require.Equal(t, f.imported.FileID, intent.FileID)
			require.Equal(t, digest[:], intent.ExpectedSHA256)
			_, err = f.files.StartTrackedWrite(ctx, f.imported.R2Key, f.imported.Generation)
			require.Error(t, err, "one declaration cannot create a second network attempt")
			require.NoError(t, f.files.PurgeCatalogProfile(ctx, f.actor, f.keys))
			settleContext, stop := context.WithCancel(ctx)
			stop()
			require.NoError(t, f.files.SettleTrackedWrite(settleContext, f.imported.R2Key, f.imported.Generation, ack), "physical outcome survives caller cancellation and actor/File publication closure")
			tx, err := f.pool.Begin(ctx)
			require.NoError(t, err)
			_, err = tx.Exec(ctx, `SELECT id FROM files WHERE id=$1 FOR UPDATE`, f.imported.FileID)
			require.NoError(t, err)
			eligible, err := store.CanCollectTrackedObjectTx(ctx, tx, f.imported.FileID, f.imported.R2Key)
			require.NoError(t, err)
			require.Equal(t, ack, eligible)
			require.NoError(t, tx.Commit(ctx))
			if !ack {
				require.Error(t, f.files.SettleTrackedWrite(ctx, f.imported.R2Key, f.imported.Generation, true), "an assertion cannot turn an already lost ACK into remote quiescence")
			}
		})
	}
}
