package grpcsvc

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/file/internal/catalogasset"
	"voice/backend/file/internal/store"
)

type catalogPurgeFixture struct {
	pool                                                    *pgxpool.Pool
	keys                                                    *catalogasset.ReplayKeys
	actor                                                   catalogasset.ActorPair
	file, source, operation, profileGate, profileGeneration uuid.UUID
}

func seedCatalogPurgeFixture(t *testing.T, ctx context.Context, ready bool) catalogPurgeFixture {
	t.Helper()
	pool := startR23FilePostgres(t, ctx)
	applyFileGateSQL(t, ctx, pool, filepath.Join("src", "backend", "migrations", "file_db", "000010_catalog_asset_import.up.sql"))
	applyFileGateSQL(t, ctx, pool, filepath.Join("src", "backend", "migrations", "file_db", "000012_catalog_selection_authority.up.sql"))
	keys, err := catalogasset.NewReplayKeys("fixture", map[string][]byte{"fixture": bytes.Repeat([]byte{1}, 32)})
	require.NoError(t, err)
	f := catalogPurgeFixture{pool: pool, keys: keys, actor: catalogasset.ActorPair{AccountID: uuid.New(), ProfileID: uuid.New()}, file: uuid.New(), source: uuid.New(), operation: uuid.New(), profileGate: uuid.New(), profileGeneration: uuid.New()}
	accountGate, accountGeneration := uuid.New(), uuid.New()
	accountHash, err := keys.Account(f.actor.AccountID)
	require.NoError(t, err)
	profileHash, err := keys.Profile(f.actor)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO file_catalog_actor_gates(id,actor_kind,key_version,actor_hmac,generation,state) VALUES($1,'account','fixture',$3,$5,'LIVE'),($2,'profile','fixture',$4,$6,'LIVE')`, accountGate, f.profileGate, accountHash[0].HMAC, profileHash[0].HMAC, accountGeneration, f.profileGeneration)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO files(id,uploader_profile_id,original_name,mime_type,size_bytes,sha256_hash,r2_key,status,file_type,scan_result,upload_intent,catalog_asset) VALUES($1::uuid,$2,'catalog.gif','image/gif',42,repeat('ab',32),$1::uuid::text,'processing','image','skipped','gif',true)`, f.file, f.actor.ProfileID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO file_catalog_assets(file_id,source_id,actor_account_id,actor_profile_id,provider,provider_id,account_gate_id,profile_gate_id,account_generation,profile_generation,chat_account_generation,chat_profile_generation,source_sha256) VALUES($1,$2,$3,$4,'fixture','asset-1',$5,$6,$7,$8,$9,$10,decode(repeat('ab',32),'hex'))`, f.file, f.source, f.actor.AccountID, f.actor.ProfileID, accountGate, f.profileGate, accountGeneration, f.profileGeneration, uuid.New(), uuid.New())
	require.NoError(t, err)
	if ready {
		_, err = pool.Exec(ctx, `UPDATE files SET status='ready',file_type='video' WHERE id=$1`, f.file)
		require.NoError(t, err)
	}
	op, err := keys.Operation(f.actor, f.operation)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO file_catalog_import_operations(key_version,operation_hmac,generation,phase,operation_id,file_id,actor_account_id,actor_profile_id,destination_chat_id,destination_chat_type,request_sha256,selection_sha256,max_fetch_bytes,source_id,provider,provider_id,chat_account_generation,chat_profile_generation)
SELECT 'fixture',$1,$2,'PROCESSING',$3,$4,$5,$6,$7,'group',$8,$8,52428800,source_id,provider,provider_id,chat_account_generation,chat_profile_generation FROM file_catalog_assets WHERE file_id=$4`, op[0].HMAC, uuid.New(), f.operation, f.file, f.actor.AccountID, f.actor.ProfileID, uuid.New(), bytes.Repeat([]byte{3}, 32))
	require.NoError(t, err)
	// This hand-built lifecycle fixture represents an already admitted v2 row.
	// Admission-order behavior itself is tested through BeginCatalogImport.
	var ca, cp uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT chat_account_generation,chat_profile_generation FROM file_catalog_assets WHERE file_id=$1`, f.file).Scan(&ca, &cp))
	selector, err := keys.Selection(f.actor, accountGeneration, f.profileGeneration, ca, cp, "fixture", "asset-1")
	require.NoError(t, err)
	head := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO file_catalog_selection_heads(id,key_version,selector_hmac,revision,current_source_id,current_file_id) VALUES($1,'fixture',$2,1,$3,$4)`, head, selector[0].HMAC, f.source, f.file)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE file_catalog_assets SET authority_protocol=2,selection_head_id=$2,selection_revision=1 WHERE file_id=$1;`, f.file, head)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE file_catalog_import_operations SET authority_protocol=2,selection_head_id=$2,selection_revision=1 WHERE file_id=$1`, f.file, head)
	require.NoError(t, err)
	return f
}

func TestCatalogProfilePurgeErasesPrivateMappingsAndPreservesCommittedHistory(t *testing.T) {
	ctx := context.Background()
	f := seedCatalogPurgeFixture(t, ctx, true)
	message := uuid.New()
	_, err := f.pool.Exec(ctx, `INSERT INTO file_references(file_id,owner_type,owner_id) VALUES($1,1,$2)`, f.file, message)
	require.NoError(t, err)
	require.NoError(t, store.NewFilesStore(f.pool).PurgeCatalogProfile(ctx, f.actor, f.keys))
	var state string
	var generation uuid.UUID
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT state,generation FROM file_catalog_actor_gates WHERE id=$1`, f.profileGate).Scan(&state, &generation))
	require.Equal(t, "CLOSED", state)
	require.NotEqual(t, f.profileGeneration, generation)
	for _, table := range []string{"file_catalog_assets", "file_catalog_import_operations"} {
		var count int
		require.NoError(t, f.pool.QueryRow(ctx, `SELECT count(*) FROM `+table).Scan(&count))
		require.Zero(t, count)
	}
	var released bool
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT released_at IS NOT NULL FROM file_references WHERE file_id=$1 AND owner_id=$2`, f.file, message).Scan(&released))
	require.False(t, released)
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT status FROM files WHERE id=$1`, f.file).Scan(&state))
	require.Equal(t, "ready", state)
	var fences int
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT count(*) FROM file_catalog_terminal_fences`).Scan(&fences))
	require.Equal(t, 3, fences)
	require.NoError(t, store.NewFilesStore(f.pool).PurgeCatalogProfile(ctx, f.actor, f.keys))
	var replayGeneration uuid.UUID
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT generation FROM file_catalog_actor_gates WHERE id=$1`, f.profileGate).Scan(&replayGeneration))
	require.Equal(t, generation, replayGeneration)
}

func TestCatalogProfilePurgeDoesNotClaimUnsettledObjectWriteCleanup(t *testing.T) {
	ctx := context.Background()
	f := seedCatalogPurgeFixture(t, ctx, false)
	_, err := f.pool.Exec(ctx, `INSERT INTO file_catalog_writes(object_key,file_id,generation,state,expected_sha256,expected_size_bytes) VALUES('owned-attempt/source.gif',$1,$2,'UNSETTLED',decode(repeat('ab',32),'hex'),42)`, f.file, uuid.New())
	require.NoError(t, err)
	require.NoError(t, store.NewFilesStore(f.pool).PurgeCatalogProfile(ctx, f.actor, f.keys))
	var state string
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT state FROM file_catalog_writes WHERE file_id=$1`, f.file).Scan(&state))
	require.Equal(t, "UNSETTLED", state)
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT status FROM files WHERE id=$1`, f.file).Scan(&state))
	require.Equal(t, "failed", state)
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT state FROM file_blobs WHERE blob_id=$1`, f.file).Scan(&state))
	require.Equal(t, "LIVE", state, "missing runtime/storage fence must not authorize blob GC")
}

func TestCatalogProfilePurgeRejectsMissingHistoricalKey(t *testing.T) {
	ctx := context.Background()
	f := seedCatalogPurgeFixture(t, ctx, true)
	keys, err := catalogasset.NewReplayKeys("rotated", map[string][]byte{"rotated": bytes.Repeat([]byte{2}, 32)})
	require.NoError(t, err)
	require.ErrorIs(t, store.NewFilesStore(f.pool).PurgeCatalogProfile(ctx, f.actor, keys), catalogasset.ErrReplayKeys)
	var state string
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT state FROM file_catalog_actor_gates WHERE id=$1`, f.profileGate).Scan(&state))
	require.Equal(t, "LIVE", state)
	var count int
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT count(*) FROM file_catalog_assets`).Scan(&count))
	require.Equal(t, 1, count)
}

func TestCatalogProfilePurgeRemovesUnknownSizeWithoutInventingObjectFacts(t *testing.T) {
	ctx := context.Background()
	f := seedCatalogPurgeFixture(t, ctx, true)
	pending := uuid.New()
	_, err := f.pool.Exec(ctx, `INSERT INTO files(id,uploader_profile_id,original_name,mime_type,size_bytes,r2_key,status,file_type,scan_result,upload_intent,catalog_asset,catalog_source_size_unknown)
VALUES($1::uuid,$2,'catalog.gif','image/gif',0,$1::uuid::text,'pending_upload','image','skipped','gif',true,true)`, pending, f.actor.ProfileID)
	require.NoError(t, err)
	_, err = f.pool.Exec(ctx, `INSERT INTO file_catalog_assets(file_id,source_id,actor_account_id,actor_profile_id,provider,provider_id,account_gate_id,profile_gate_id,account_generation,profile_generation,chat_account_generation,chat_profile_generation)
SELECT $1,$2,actor_account_id,actor_profile_id,provider,provider_id,account_gate_id,profile_gate_id,account_generation,profile_generation,chat_account_generation,chat_profile_generation FROM file_catalog_assets WHERE file_id=$3`, pending, uuid.New(), f.file)
	require.NoError(t, err)
	require.NoError(t, store.NewFilesStore(f.pool).PurgeCatalogProfile(ctx, f.actor, f.keys))
	var count int
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT count(*) FROM files WHERE id=$1`, pending).Scan(&count))
	require.Zero(t, count)
	var state string
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT state FROM file_blobs WHERE blob_id=$1`, pending).Scan(&state))
	require.Equal(t, "GC_PENDING", state)
}
