package store

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/pkg/integrationtest"
)

func TestProfileSpaceSearchStore_DeleteChatRequiresPermanentP3Fence_postgres(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	root := searchModuleRepoRoot(t)
	pool := integrationtest.StartPostgres(t, ctx, "searchdb", filepath.Join(root, "src", "backend", "migrations", "search_db", "000001_init.up.sql"))
	for _, name := range []string{"000003_space_lifecycle.up.sql", "000009_managed_chat_message_purge.up.sql", "000010_chat_manifest_root_binding.up.sql", "000011_chat_deleted_fence_binding.up.sql"} {
		integrationtest.ApplySQLFile(t, ctx, pool, root, filepath.Join("src", "backend", "migrations", "search_db", name))
	}
	st := NewProfileSpaceSearchStore(pool)
	spaceID, operationID, manifestID, chatID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	manifestHash := sha256.Sum256([]byte("exact chat purge manifest"))
	require.NoError(t, st.UpsertChat(ctx, chatID, "Purge fixture"))
	_, err := pool.Exec(ctx, `INSERT INTO search_space_lifecycle_fences(space_id,generation,state,deletion_operation_id,updated_at) VALUES($1,2,'PURGE_DECIDED',$2,clock_timestamp())`, spaceID, operationID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO search_space_chat_manifests(space_id,deletion_operation_id,generation,manifest_id,manifest_sha256,item_count,page_count,sealed,created_at,root_manifest_id,root_manifest_sha256,root_manifest_item_count)
		VALUES($1,$2,1,$3,$4,1,1,true,clock_timestamp(),$3,$4,1)`, spaceID, operationID, manifestID.String(), manifestHash[:])
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO search_space_chat_manifest_items(space_id,deletion_operation_id,generation,page_index,item_index,chat_id) VALUES($1,$2,1,0,0,$3)`, spaceID, operationID, chatID)
	require.NoError(t, err)
	delete := func(chat, space, op uuid.UUID) error {
		return st.DeleteChat(ctx, chat, space, op, 2, manifestID, manifestHash[:])
	}
	require.ErrorIs(t, delete(chatID, spaceID, operationID), ErrChatDeletedProjectionNotReady, "PurgeDecided is not terminal")
	_, err = pool.Exec(ctx, `UPDATE search_space_lifecycle_fences SET state='PURGED' WHERE space_id=$1`, spaceID)
	require.NoError(t, err)
	require.ErrorIs(t, delete(uuid.New(), spaceID, operationID), ErrChatDeletedProjectionConflict, "chat must be in the exact purged manifest")
	require.ErrorIs(t, delete(chatID, uuid.New(), operationID), ErrChatDeletedProjectionNotReady, "space binding must have its own terminal fence")
	require.NoError(t, delete(chatID, spaceID, operationID))
	require.NoError(t, delete(chatID, spaceID, operationID), "terminal projection deletion is idempotent")
	var remaining int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM chat_search_documents WHERE chat_id=$1`, chatID).Scan(&remaining))
	require.Zero(t, remaining)
	require.Error(t, st.UpsertChat(ctx, chatID, "late replay"), "permanent P3 fence must block late chat.updated/created projection replay")
}

func TestProfileSpaceSearchStore_DeleteChatPostRetentionBinding_postgres(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	root := searchModuleRepoRoot(t)
	pool := integrationtest.StartPostgres(t, ctx, "searchdb", filepath.Join(root, "src", "backend", "migrations", "search_db", "000001_init.up.sql"))
	for _, name := range []string{
		"000003_space_lifecycle.up.sql",
		"000009_managed_chat_message_purge.up.sql",
		"000010_chat_manifest_root_binding.up.sql",
		"000011_chat_deleted_fence_binding.up.sql",
	} {
		integrationtest.ApplySQLFile(t, ctx, pool, root, filepath.Join("src", "backend", "migrations", "search_db", name))
	}
	st := NewProfileSpaceSearchStore(pool)
	_, err := pool.Exec(ctx, `INSERT INTO search_space_purged_chat_fences(space_id,chat_id,deletion_operation_id)
		VALUES($1,$2,$3)`, uuid.New(), uuid.New(), uuid.New())
	require.Error(t, err, "compact tuple must be complete or entirely legacy-null")
	spaceID, operationID, manifestID, chatID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	manifestHash := sha256.Sum256([]byte("post-retention chat purge manifest"))
	require.NoError(t, st.UpsertChat(ctx, chatID, "Purge fixture"))
	_, err = pool.Exec(ctx, `INSERT INTO search_space_lifecycle_fences(space_id,generation,state,deletion_operation_id,updated_at)
		VALUES($1,2,'PURGED',$2,clock_timestamp())`, spaceID, operationID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO search_space_purge_receipts(space_id,deletion_operation_id,generation,request_bytes,request_sha256,receipt_bytes,completed_at,retain_until)
		VALUES($1,$2,2,$3,$4,$5,clock_timestamp(),clock_timestamp()-interval '1 second')`, spaceID, operationID, []byte("request"), manifestHash[:], []byte("receipt"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO search_space_chat_manifests(space_id,deletion_operation_id,generation,manifest_id,manifest_sha256,item_count,page_count,sealed,created_at,root_manifest_id,root_manifest_sha256,root_manifest_item_count)
		VALUES($1,$2,1,$3,$4,1,1,true,clock_timestamp(),$3,$4,1)`, spaceID, operationID, manifestID.String(), manifestHash[:])
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO search_space_chat_manifest_items(space_id,deletion_operation_id,generation,page_index,item_index,chat_id)
		VALUES($1,$2,1,0,0,$3)`, spaceID, operationID, chatID)
	require.NoError(t, err)

	// A conflicting pre-existing tombstone must abort compaction without
	// deleting the retained manifest or purge receipt that can prove the event.
	conflictingHash := sha256.Sum256([]byte("conflicting binding"))
	_, err = pool.Exec(ctx, `INSERT INTO search_space_purged_chat_fences(space_id,chat_id,deletion_operation_id,event_generation,manifest_id,manifest_sha256)
		VALUES($1,$2,$3,2,$4,$5)`, spaceID, chatID, uuid.New(), uuid.NewString(), conflictingHash[:])
	require.NoError(t, err)
	require.ErrorIs(t, st.DeleteChat(ctx, chatID, spaceID, operationID, 2, manifestID, manifestHash[:]), ErrChatDeletedProjectionConflict,
		"a contradictory complete compact tuple must fail closed even while source evidence remains")
	require.Error(t, PruneExpiredLifecycleEvidence(ctx, pool))
	var retainedManifests, retainedReceipts int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM search_space_chat_manifests WHERE space_id=$1`, spaceID).Scan(&retainedManifests))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM search_space_purge_receipts WHERE space_id=$1`, spaceID).Scan(&retainedReceipts))
	require.Equal(t, 1, retainedManifests)
	require.Equal(t, 1, retainedReceipts)
	_, err = pool.Exec(ctx, `DELETE FROM search_space_purged_chat_fences WHERE space_id=$1 AND chat_id=$2`, spaceID, chatID)
	require.NoError(t, err)

	require.NoError(t, PruneExpiredLifecycleEvidence(ctx, pool))
	var compactOperation, compactManifest string
	var compactGeneration int64
	var compactHash []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT deletion_operation_id::text,event_generation,manifest_id,manifest_sha256
		FROM search_space_purged_chat_fences WHERE space_id=$1 AND chat_id=$2`, spaceID, chatID).Scan(&compactOperation, &compactGeneration, &compactManifest, &compactHash))
	require.Equal(t, operationID.String(), compactOperation)
	require.EqualValues(t, 2, compactGeneration)
	require.Equal(t, manifestID.String(), compactManifest)
	require.Equal(t, manifestHash[:], compactHash)
	var deletedEvidence int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM search_space_chat_manifests WHERE space_id=$1`, spaceID).Scan(&deletedEvidence))
	require.Zero(t, deletedEvidence)
	require.NoError(t, st.DeleteChat(ctx, chatID, spaceID, operationID, 2, manifestID, manifestHash[:]))
	require.NoError(t, st.DeleteChat(ctx, chatID, spaceID, operationID, 2, manifestID, manifestHash[:]), "exact post-retention delete is idempotent")
	require.ErrorIs(t, st.DeleteChat(ctx, chatID, spaceID, operationID, 2, uuid.New(), manifestHash[:]), ErrChatDeletedProjectionConflict)
	wrongHash := sha256.Sum256([]byte("different manifest"))
	require.ErrorIs(t, st.DeleteChat(ctx, chatID, spaceID, operationID, 2, manifestID, wrongHash[:]), ErrChatDeletedProjectionConflict)
	require.ErrorIs(t, st.DeleteChat(ctx, chatID, spaceID, uuid.New(), 2, manifestID, manifestHash[:]), ErrChatDeletedProjectionConflict)
	require.ErrorIs(t, st.DeleteChat(ctx, chatID, spaceID, operationID, 3, manifestID, manifestHash[:]), ErrChatDeletedProjectionConflict)

	legacyChat := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO search_space_purged_chat_fences(space_id,chat_id) VALUES($1,$2)`, spaceID, legacyChat)
	require.NoError(t, err)
	require.ErrorIs(t, st.DeleteChat(ctx, legacyChat, spaceID, operationID, 2, manifestID, manifestHash[:]), ErrChatDeletedProjectionNotReady,
		"an all-null legacy row has no authoritative binding and must remain retryable")
	deadCtx, canceled := context.WithCancel(ctx)
	canceled()
	require.ErrorIs(t, st.DeleteChat(deadCtx, chatID, spaceID, operationID, 2, manifestID, manifestHash[:]), context.Canceled,
		"database/context failures remain retryable errors")

	downSQL, err := os.ReadFile(filepath.Join(root, "src", "backend", "migrations", "search_db", "000011_chat_deleted_fence_binding.down.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(downSQL))
	require.Error(t, err, "DOWN must refuse while immutable binding evidence remains")
	var preservedBindings int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM search_space_purged_chat_fences
		WHERE deletion_operation_id IS NOT NULL OR event_generation IS NOT NULL OR manifest_id IS NOT NULL OR manifest_sha256 IS NOT NULL`).Scan(&preservedBindings))
	require.Equal(t, 1, preservedBindings)
	_, err = pool.Exec(ctx, `DELETE FROM search_space_purged_chat_fences WHERE space_id=$1 AND chat_id=$2`, spaceID, chatID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(downSQL))
	require.NoError(t, err, "DOWN may remove empty binding columns while preserving legacy rows")
	var legacyRows int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM search_space_purged_chat_fences WHERE space_id=$1 AND chat_id=$2`, spaceID, legacyChat).Scan(&legacyRows))
	require.Equal(t, 1, legacyRows)
}

func TestProfileSpaceSearchStore_ProfileILIKE_postgres(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	migrationPath := filepath.Join(searchModuleRepoRoot(t), "src", "backend", "migrations", "search_db", "000001_init.up.sql")
	pool := integrationtest.StartPostgres(t, ctx, "searchdb", migrationPath)
	integrationtest.ApplySQLFile(t, ctx, pool, searchModuleRepoRoot(t), filepath.Join("src", "backend", "migrations", "search_db", "000003_space_lifecycle.up.sql"))
	integrationtest.ApplySQLFile(t, ctx, pool, searchModuleRepoRoot(t), filepath.Join("src", "backend", "migrations", "search_db", "000002_verification_type.up.sql"))
	integrationtest.ApplySQLFile(t, ctx, pool, searchModuleRepoRoot(t), filepath.Join("src", "backend", "migrations", "search_db", "000004_user_profile_projection.up.sql"))
	integrationtest.ApplySQLFile(t, ctx, pool, searchModuleRepoRoot(t), filepath.Join("src", "backend", "migrations", "search_db", "000005_user_profile_projection_snapshot.up.sql"))
	integrationtest.ApplySQLFile(t, ctx, pool, searchModuleRepoRoot(t), filepath.Join("src", "backend", "migrations", "search_db", "000006_user_profile_projection_quarantine.up.sql"))
	integrationtest.ApplySQLFile(t, ctx, pool, searchModuleRepoRoot(t), filepath.Join("src", "backend", "migrations", "search_db", "000007_user_profile_projection_fence.up.sql"))
	integrationtest.ApplySQLFile(t, ctx, pool, searchModuleRepoRoot(t), filepath.Join("src", "backend", "migrations", "search_db", "000008_user_profile_projection_generations.up.sql"))

	viewer := uuid.New()
	pidCarol := uuid.New()
	pidDave := uuid.New()

	st := NewProfileSpaceSearchStore(pool)
	require.NoError(t, st.UpsertProfile(ctx, ProfileDocument{
		ProfileID:     pidCarol,
		AccountID:     uuid.New(),
		Username:      "carol",
		Discriminator: "0001",
		DisplayName:   "Carol Literal 50%_off",
	}))
	require.NoError(t, st.UpsertProfile(ctx, ProfileDocument{
		ProfileID:     pidDave,
		AccountID:     uuid.New(),
		Username:      "dave",
		Discriminator: "0002",
		DisplayName:   "Dave",
	}))

	t.Run("username ilike match", func(t *testing.T) {
		hits, err := st.SearchProfiles(ctx, viewer, "carol", nil, 20)
		require.NoError(t, err)
		require.Len(t, hits, 1)
		require.Equal(t, pidCarol, hits[0].ProfileID)
	})

	t.Run("literal percent and underscore in display_name", func(t *testing.T) {
		hits, err := st.SearchProfiles(ctx, viewer, "50%_off", nil, 20)
		require.NoError(t, err)
		require.Len(t, hits, 1)
		require.Equal(t, pidCarol, hits[0].ProfileID)
	})

	t.Run("own account is excluded before limit", func(t *testing.T) {
		ownProfile := uuid.New()
		siblingProfile := uuid.New()
		foreignProfile := uuid.New()
		for _, doc := range []ProfileDocument{
			{ProfileID: ownProfile, AccountID: viewer, Username: "a_shared", Discriminator: "0001", DisplayName: "Shared"},
			{ProfileID: siblingProfile, AccountID: viewer, Username: "b_shared", Discriminator: "0002", DisplayName: "Shared"},
			{ProfileID: foreignProfile, AccountID: uuid.New(), Username: "c_shared", Discriminator: "0003", DisplayName: "Shared"},
		} {
			require.NoError(t, st.UpsertProfile(ctx, doc))
		}
		hits, err := st.SearchProfiles(ctx, viewer, "shared", nil, 1)
		require.NoError(t, err)
		require.Len(t, hits, 1)
		require.Equal(t, foreignProfile, hits[0].ProfileID)
	})
}

func TestProfileSpaceSearchStore_ExcludesBlockedProfiles_postgres(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	migrationPath := filepath.Join(searchModuleRepoRoot(t), "src", "backend", "migrations", "search_db", "000001_init.up.sql")
	pool := integrationtest.StartPostgres(t, ctx, "searchdb", migrationPath)
	integrationtest.ApplySQLFile(t, ctx, pool, searchModuleRepoRoot(t), filepath.Join("src", "backend", "migrations", "search_db", "000003_space_lifecycle.up.sql"))
	integrationtest.ApplySQLFile(t, ctx, pool, searchModuleRepoRoot(t), filepath.Join("src", "backend", "migrations", "search_db", "000002_verification_type.up.sql"))
	integrationtest.ApplySQLFile(t, ctx, pool, searchModuleRepoRoot(t), filepath.Join("src", "backend", "migrations", "search_db", "000004_user_profile_projection.up.sql"))
	integrationtest.ApplySQLFile(t, ctx, pool, searchModuleRepoRoot(t), filepath.Join("src", "backend", "migrations", "search_db", "000005_user_profile_projection_snapshot.up.sql"))
	integrationtest.ApplySQLFile(t, ctx, pool, searchModuleRepoRoot(t), filepath.Join("src", "backend", "migrations", "search_db", "000006_user_profile_projection_quarantine.up.sql"))
	integrationtest.ApplySQLFile(t, ctx, pool, searchModuleRepoRoot(t), filepath.Join("src", "backend", "migrations", "search_db", "000007_user_profile_projection_fence.up.sql"))
	integrationtest.ApplySQLFile(t, ctx, pool, searchModuleRepoRoot(t), filepath.Join("src", "backend", "migrations", "search_db", "000008_user_profile_projection_generations.up.sql"))

	viewer := uuid.New()
	blockedAccount := uuid.New()
	allowedProfile := uuid.New()
	allowedAccount := uuid.New()

	st := NewProfileSpaceSearchStore(pool)
	require.NoError(t, st.UpsertProfile(ctx, ProfileDocument{
		ProfileID:     uuid.New(),
		AccountID:     blockedAccount,
		Username:      "blockeduser",
		Discriminator: "0001",
		DisplayName:   "Blocked User",
	}))
	require.NoError(t, st.UpsertProfile(ctx, ProfileDocument{
		ProfileID:     allowedProfile,
		AccountID:     allowedAccount,
		Username:      "alloweduser",
		Discriminator: "0002",
		DisplayName:   "Allowed User",
	}))

	hits, err := st.SearchProfiles(ctx, viewer, "user", []uuid.UUID{blockedAccount}, 20)
	require.NoError(t, err)
	require.Len(t, hits, 1)
	require.Equal(t, allowedProfile, hits[0].ProfileID)
}

func TestProfileSpaceSearchStore_SpaceVisibilityFilter_postgres(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	migrationPath := filepath.Join(searchModuleRepoRoot(t), "src", "backend", "migrations", "search_db", "000001_init.up.sql")
	pool := integrationtest.StartPostgres(t, ctx, "searchdb", migrationPath)
	integrationtest.ApplySQLFile(t, ctx, pool, searchModuleRepoRoot(t), filepath.Join("src", "backend", "migrations", "search_db", "000003_space_lifecycle.up.sql"))

	publicID := uuid.New()
	inviteID := uuid.New()
	privateID := uuid.New()

	st := NewProfileSpaceSearchStore(pool)
	require.NoError(t, st.UpsertSpace(ctx, SpaceDocument{
		SpaceID:    publicID,
		Name:       "Public Raiders",
		Visibility: "public",
	}))
	require.NoError(t, st.UpsertSpace(ctx, SpaceDocument{
		SpaceID:    inviteID,
		Name:       "Invite Raiders",
		Visibility: "invite_only",
	}))
	require.NoError(t, st.UpsertSpace(ctx, SpaceDocument{
		SpaceID:    privateID,
		Name:       "Private Raiders",
		Visibility: "private",
	}))

	hits, _, err := st.SearchSpaces(ctx, "Raiders", nil, 20)
	require.NoError(t, err)
	require.Len(t, hits, 2)
	ids := map[uuid.UUID]bool{hits[0].SpaceID: true, hits[1].SpaceID: true}
	require.True(t, ids[publicID])
	require.True(t, ids[inviteID])
	require.False(t, ids[privateID])
}
