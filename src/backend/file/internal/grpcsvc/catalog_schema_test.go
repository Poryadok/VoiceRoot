package grpcsvc

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"voice/backend/file/internal/store"
)

func TestCatalogSchemaUnknownSizeIsInternalAndOrdinaryUploadConstraintSurvives(t *testing.T) {
	ctx := context.Background()
	pool := startR23FilePostgres(t, ctx)
	applyFileGateSQL(t, ctx, pool, filepath.Join("src", "backend", "migrations", "file_db", "000010_catalog_asset_import.up.sql"))
	id, profile := uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO files(id,uploader_profile_id,original_name,mime_type,size_bytes,r2_key,status,file_type,scan_result,upload_intent,catalog_asset,catalog_source_size_unknown) VALUES($1::uuid,$2,'catalog.gif','image/gif',0,$1::uuid::text||'/source.gif','pending_upload','image','pending','gif',true,true)`, id, profile)
	require.NoError(t, err)
	_, err = store.NewFilesStore(pool).GetFileByID(ctx, id)
	require.ErrorIs(t, err, pgx.ErrNoRows, "unknown size must never appear as measured metadata")
	bulk, err := store.NewFilesStore(pool).GetFilesByIDs(ctx, []uuid.UUID{id})
	require.NoError(t, err)
	require.Empty(t, bulk)
	listed, err := store.NewFilesStore(pool).ListFilesForProfile(ctx, profile, 10)
	require.NoError(t, err)
	require.Empty(t, listed)
	_, err = pool.Exec(ctx, `INSERT INTO files(id,uploader_profile_id,original_name,mime_type,size_bytes,r2_key,status,file_type,scan_result) VALUES($1::uuid,$2,'ordinary.gif','image/gif',0,$1::uuid::text,'pending_upload','image','pending')`, uuid.New(), profile)
	require.Error(t, err, "ordinary zero-byte uploads must remain invalid")
	_, err = pool.Exec(ctx, `UPDATE files SET status='ready' WHERE id=$1`, id)
	require.Error(t, err, "unknown bytes cannot become READY")
	_, err = pool.Exec(ctx, `UPDATE files SET sha256_hash=repeat('ab',32) WHERE id=$1`, id)
	require.Error(t, err, "digest alone is not a measured positive size")
	_, err = pool.Exec(ctx, `UPDATE files SET size_bytes=42,sha256_hash=repeat('ab',32),catalog_source_size_unknown=false WHERE id=$1`, id)
	require.NoError(t, err)
	row, err := store.NewFilesStore(pool).GetFileByID(ctx, id)
	require.NoError(t, err)
	require.EqualValues(t, 42, row.SizeBytes)
	_, err = pool.Exec(ctx, `UPDATE files SET catalog_asset=false WHERE id=$1`, id)
	require.Error(t, err, "catalog File must never become ordinary by marker mutation")
}

func TestCatalogSchemaReadyPublicationRequiresCurrentActorGeneration(t *testing.T) {
	ctx := context.Background()
	pool := startR23FilePostgres(t, ctx)
	applyFileGateSQL(t, ctx, pool, filepath.Join("src", "backend", "migrations", "file_db", "000010_catalog_asset_import.up.sql"))
	account, profile, fileID, source := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	accountGate, profileGate, accountGen, profileGen := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO file_catalog_actor_gates(id,actor_kind,key_version,actor_hmac,generation,state) VALUES($1,'account','fixture',decode(repeat('01',32),'hex'),$3,'LIVE'),($2,'profile','fixture',decode(repeat('02',32),'hex'),$4,'LIVE')`, accountGate, profileGate, accountGen, profileGen)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO files(id,uploader_profile_id,original_name,mime_type,size_bytes,sha256_hash,r2_key,status,file_type,scan_result,upload_intent,catalog_asset) VALUES($1::uuid,$2,'catalog.gif','image/gif',42,repeat('ab',32),$1::uuid::text,'processing','image','skipped','gif',true)`, fileID, profile)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO file_catalog_assets(file_id,source_id,actor_account_id,actor_profile_id,provider,provider_id,account_gate_id,profile_gate_id,account_generation,profile_generation,chat_account_generation,chat_profile_generation,source_sha256) VALUES($1,$2,$3,$4,'fixture','asset-1',$5,$6,$7,$8,$9,$10,decode(repeat('ab',32),'hex'))`, fileID, source, account, profile, accountGate, profileGate, accountGen, profileGen, uuid.New(), uuid.New())
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE files SET status='ready',file_type='video' WHERE id=$1`, fileID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE files SET status='processing' WHERE id=$1`, fileID)
	require.NoError(t, err)
	closing, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = closing.Rollback(context.Background()) }()
	_, err = closing.Exec(ctx, `UPDATE file_catalog_actor_gates SET state='CLOSED',generation=$2 WHERE id=$1`, profileGate, uuid.New())
	require.NoError(t, err)
	worker, err := pool.Acquire(ctx)
	require.NoError(t, err)
	var workerPID int
	require.NoError(t, worker.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&workerPID))
	workerContext, cancelWorker := context.WithCancel(ctx)
	finished := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		_, updateErr := worker.Exec(workerContext, `UPDATE files SET status='ready' WHERE id=$1`, fileID)
		result <- updateErr
		close(finished)
	}()
	defer func() { cancelWorker(); <-finished; worker.Release() }()
	require.Eventually(t, func() bool {
		var wait string
		err := pool.QueryRow(ctx, `SELECT coalesce(wait_event_type,'') FROM pg_stat_activity WHERE pid=$1`, workerPID).Scan(&wait)
		return err == nil && wait == "Lock"
	}, 2*time.Second, 10*time.Millisecond, "old publisher must actually wait on the closing actor gate")
	require.NoError(t, closing.Commit(ctx))
	select {
	case err = <-result:
		require.Error(t, err, "publisher started before close must recheck the committed actor generation after waiting")
	case <-time.After(2 * time.Second):
		t.Fatal("late publisher did not settle")
	}
	_, err = pool.Exec(ctx, `UPDATE file_catalog_actor_gates SET state='LIVE' WHERE id=$1`, profileGate)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE files SET status='ready' WHERE id=$1`, fileID)
	require.Error(t, err, "restore generation does not revive old source binding")
	_, err = pool.Exec(ctx, `UPDATE file_catalog_assets SET source_id=$2 WHERE file_id=$1`, fileID, uuid.New())
	require.Error(t, err, "source generation cannot mutate under same File")
}

func TestCatalogSchemaEmptyDownIsReversible(t *testing.T) {
	ctx := context.Background()
	pool := startR23FilePostgres(t, ctx)
	applyFileGateSQL(t, ctx, pool, filepath.Join("src", "backend", "migrations", "file_db", "000010_catalog_asset_import.up.sql"))
	applyFileGateSQL(t, ctx, pool, filepath.Join("src", "backend", "migrations", "file_db", "000010_catalog_asset_import.down.sql"))
	var absent bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass('file_catalog_terminal_fences') IS NULL`).Scan(&absent))
	require.True(t, absent)
}

func TestCatalogSchemaDownRefusesPermanentReplayAuthority(t *testing.T) {
	ctx := context.Background()
	pool := startR23FilePostgres(t, ctx)
	applyFileGateSQL(t, ctx, pool, filepath.Join("src", "backend", "migrations", "file_db", "000010_catalog_asset_import.up.sql"))
	_, err := pool.Exec(ctx, `INSERT INTO file_catalog_terminal_fences(purpose,key_version,identity_hmac,outcome,fence_id) VALUES('operation','fixture',decode(repeat('ab',32),'hex'),'failed',$1)`, uuid.New())
	require.NoError(t, err)
	down, err := os.ReadFile(filepath.Join(fileGateRepoRoot(t), "src", "backend", "migrations", "file_db", "000010_catalog_asset_import.down.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(down))
	require.Error(t, err, "DOWN cannot erase replay fences")
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM file_catalog_terminal_fences`).Scan(&count))
	require.Equal(t, 1, count)
}
