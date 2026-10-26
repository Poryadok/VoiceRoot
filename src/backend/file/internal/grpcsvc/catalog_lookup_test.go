package grpcsvc

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/file/internal/catalogasset"
	"voice/backend/file/internal/store"
)

func seedCatalogReadyRepresentation(t *testing.T, ctx context.Context, f catalogPurgeFixture) {
	t.Helper()
	full, err := json.Marshal(store.Representation{Key: "owned/full.mp4", MIMEType: "video/mp4", SizeBytes: 512, SHA256: strings.Repeat("cd", 32), Filename: "catalog.mp4", ProcessorVersion: store.GIFProcessorVersion})
	require.NoError(t, err)
	_, err = f.pool.Exec(ctx, `INSERT INTO file_gif_outputs(file_id,full_descriptor,thumbnail_descriptor,processor_version) VALUES($1,$2,$2,$3)`, f.file, full, store.GIFProcessorVersion)
	require.NoError(t, err)
}

func TestCatalogLookupRequiresRealReadyBindingAndLiveBlob(t *testing.T) {
	ctx := context.Background()
	f := seedCatalogPurgeFixture(t, ctx, true)
	seedCatalogReadyRepresentation(t, ctx, f)
	files := store.NewFilesStore(f.pool)
	out, err := files.LookupCatalogFile(ctx, f.actor, f.file, time.Now())
	require.NoError(t, err)
	require.False(t, out.GetOrdinaryFile())
	require.Equal(t, f.source.String(), out.GetCatalog().SourceId)
	r := &filev1.ValidateCatalogAssetRequest{ProtocolVersion: 1, SourceId: f.source.String(), Provider: "fixture", ProviderId: "asset-1", FileId: f.file.String(), ActorAccountId: f.actor.AccountID.String(), ActorProfileId: f.actor.ProfileID.String()}
	_, err = files.ValidateCatalogFile(ctx, r, time.Now())
	require.NoError(t, err)
	r.SourceId = uuid.NewString()
	_, err = files.ValidateCatalogFile(ctx, r, time.Now())
	require.Error(t, err, "forged source must not reuse READY proof")
	wrong := catalogasset.ActorPair{AccountID: uuid.New(), ProfileID: f.actor.ProfileID}
	_, err = files.LookupCatalogFile(ctx, wrong, f.file, time.Now())
	require.Error(t, err, "wrong actor cannot cause ordinary fallback")
	for _, mutation := range []string{`UPDATE files SET status='processing'`, `UPDATE files SET status='failed'`, `UPDATE files SET expires_at=clock_timestamp()-interval '1 second'`, `UPDATE file_blobs SET state='GC_PENDING'`, `UPDATE file_blobs SET deleted_at=clock_timestamp()`, `DELETE FROM file_catalog_assets`} {
		t.Run(mutation, func(t *testing.T) {
			tx, e := f.pool.Begin(ctx)
			require.NoError(t, e)
			// Mutate one actual database fact, then commit so the independent reader
			// observes it. Each case restores only the changed fact afterward.
			_, e = tx.Exec(ctx, mutation)
			require.NoError(t, e)
			require.NoError(t, tx.Commit(ctx))
			_, e = files.LookupCatalogFile(ctx, f.actor, f.file, time.Now())
			require.Error(t, e)
			if strings.Contains(mutation, "DELETE") {
				return
			}
			_, e = f.pool.Exec(ctx, `UPDATE file_blobs SET state='LIVE',deleted_at=NULL; UPDATE files SET status='ready',expires_at=NULL`)
			require.NoError(t, e)
		})
	}
}

func TestCatalogLookupOrdinaryIsExplicitAndMissingIsNeverOrdinary(t *testing.T) {
	ctx := context.Background()
	f := seedCatalogPurgeFixture(t, ctx, true)
	files := store.NewFilesStore(f.pool)
	_, err := files.LookupCatalogFile(ctx, f.actor, uuid.New(), time.Now())
	require.Error(t, err)
	id := uuid.New()
	_, err = f.pool.Exec(ctx, `INSERT INTO files(id,uploader_profile_id,original_name,mime_type,size_bytes,r2_key,status,file_type,scan_result) VALUES($1::uuid,$2,'image.png','image/png',42,$1::uuid::text,'ready','image','skipped')`, id, f.actor.ProfileID)
	require.NoError(t, err)
	out, err := files.LookupCatalogFile(ctx, f.actor, id, time.Now())
	require.NoError(t, err)
	require.True(t, out.GetOrdinaryFile())
	require.Nil(t, out.GetCatalog())
	// The server marker remains authoritative after private mapping erasure.
	_, err = f.pool.Exec(ctx, `DELETE FROM file_catalog_assets WHERE file_id=$1`, f.file)
	require.NoError(t, err)
	_, err = files.LookupCatalogFile(ctx, f.actor, f.file, time.Now())
	require.Error(t, err)
}
