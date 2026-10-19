package grpcsvc

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/file/internal/jobs"
	"voice/backend/file/internal/store"
)

// Both flags intentionally remain false: disabling a protected listener cannot
// turn existing durable reference evidence into deletion permission.
func TestBE264LegacyDeleteProtectsReferencesAndAliases(t *testing.T) {
	for _, protection := range []string{"reference", "alias-reference", "empty-ledger-alias"} {
		for _, expiry := range []bool{false, true} {
			t.Run(protection+map[bool]string{false: "/delete", true: "/expiry"}[expiry], func(t *testing.T) {
				ctx := r23TestContext(t)
				pool := startR23FilePostgres(t, ctx)
				id := insertR23ReadyFile(t, ctx, pool, "protected")
				subject := uuid.New()
				_, err := pool.Exec(ctx, `UPDATE files SET uploader_profile_id=$2,expires_at=$3 WHERE id=$1`, id, subject, time.Now().Add(-time.Hour))
				require.NoError(t, err)
				referenceFile := id
				if protection != "reference" {
					referenceFile = insertR23AliasFile(t, ctx, pool, id, "survivor")
				}
				if protection != "empty-ledger-alias" {
					insertR23Reference(t, ctx, pool, r23MessageReference(referenceFile, uuid.New(), uuid.New()))
				}
				deleter := &r23ScriptedDeleter{}
				files := store.NewFilesStore(pool)
				client := New(Deps{Files: files, Deleter: deleter})
				if expiry {
					n, e := jobs.RunExpiryPurgeOnce(ctx, files, deleter, nil, time.Now())
					require.NoError(t, e)
					require.Zero(t, n)
				} else {
					response, e := client.DeleteFile(r23UserContext(ctx, uuid.New(), subject), &filev1.DeleteFileRequest{FileId: id.String()})
					require.Nil(t, response)
					require.Equal(t, codes.FailedPrecondition, status.Code(e))
				}
				row, err := files.GetFileByID(ctx, id)
				require.NoError(t, err)
				require.Equal(t, "ready", row.Status)
				require.Equal(t, "LIVE", r23BlobGCState(t, ctx, pool, id))
				require.Empty(t, deleter.calls)
			})
		}
	}
}

func TestBE264LegacyDeleteIntentSurvivesMissingDeleterAndRestart(t *testing.T) {
	ctx := r23TestContext(t)
	pool := startR23FilePostgres(t, ctx)
	id := insertR23ReadyFile(t, ctx, pool, "durable-intent")
	subject := uuid.New()
	_, err := pool.Exec(ctx, `UPDATE files SET uploader_profile_id=$2 WHERE id=$1`, id, subject)
	require.NoError(t, err)
	files := store.NewFilesStore(pool)
	client := New(Deps{Files: files})
	response, err := client.DeleteFile(r23UserContext(ctx, uuid.New(), subject), &filev1.DeleteFileRequest{FileId: id.String()})
	require.Error(t, err)
	require.Nil(t, response)
	row, err := files.GetFileByID(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "deleted", row.Status)
	var started bool
	var operation uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL,gc_operation_id FROM file_blobs WHERE blob_id=(SELECT blob_id FROM files WHERE id=$1)`, id).Scan(&started, &operation))
	require.True(t, started)
	require.NotEqual(t, uuid.Nil, operation)
	acquire := r23AcquireRequest(r23MessageReference(id, uuid.New(), uuid.New()))
	authority := r23FileServer(pool)
	_, err = authority.AcquireFileReferences(r23ServiceContext(t, ctx, "messaging", filev1.FileService_AcquireFileReferences_FullMethodName, acquire), acquire)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	deleter := &r23ScriptedDeleter{}
	n, err := store.NewFilesStore(pool).RunReferenceGCOnce(ctx, deleter, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, n)
	require.Equal(t, "GC_COMPLETE", r23BlobGCState(t, ctx, pool, id))
	client = New(Deps{Files: store.NewFilesStore(pool), Deleter: deleter})
	_, err = client.DeleteFile(r23UserContext(ctx, uuid.New(), subject), &filev1.DeleteFileRequest{FileId: id.String()})
	require.NoError(t, err)
	require.Len(t, deleter.calls, 1)
}

func TestBE264ExpiryProtectedFirstBatchDoesNotStarveSingleton(t *testing.T) {
	ctx := r23TestContext(t)
	pool := startR23FilePostgres(t, ctx)
	files := store.NewFilesStore(pool)
	for i := 0; i < 101; i++ {
		id := insertR23ReadyFile(t, ctx, pool, uuid.NewString())
		insertR23Reference(t, ctx, pool, r23MessageReference(id, uuid.New(), uuid.New()))
		_, err := pool.Exec(ctx, `UPDATE files SET expires_at=$2 WHERE id=$1`, id, time.Now().Add(-2*time.Hour))
		require.NoError(t, err)
	}
	id := insertR23ReadyFile(t, ctx, pool, "eligible")
	_, err := pool.Exec(ctx, `UPDATE files SET expires_at=$2 WHERE id=$1`, id, time.Now().Add(-time.Hour))
	require.NoError(t, err)
	deleter := &r23ScriptedDeleter{}
	n, err := jobs.RunExpiryPurgeOnce(ctx, files, deleter, nil, time.Now())
	require.NoError(t, err)
	require.EqualValues(t, 1, n)
	row, err := files.GetFileByID(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, "expired", row.Status)
	require.Len(t, deleter.calls, 1)
}

func TestBE264LegacyDeleteFreshSnapshotAndFKLockCompatibility(t *testing.T) {
	ctx := r23TestContext(t)
	pool := startR23FilePostgres(t, ctx)
	id := insertR23ReadyFile(t, ctx, pool, "reference-wins")
	subject := uuid.New()
	_, err := pool.Exec(ctx, `UPDATE files SET uploader_profile_id=$2 WHERE id=$1`, id, subject)
	require.NoError(t, err)
	barrier, err := pool.Begin(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = barrier.Rollback(context.Background()) })
	var blob uuid.UUID
	require.NoError(t, barrier.QueryRow(ctx, `SELECT blob_id FROM file_blobs WHERE blob_id=(SELECT blob_id FROM files WHERE id=$1) FOR UPDATE`, id).Scan(&blob))
	deleter := &r23ScriptedDeleter{}
	client := New(Deps{Files: store.NewFilesStore(pool), Deleter: deleter})
	done := make(chan error, 1)
	go func() {
		_, e := client.DeleteFile(r23UserContext(ctx, uuid.New(), subject), &filev1.DeleteFileRequest{FileId: id.String()})
		done <- e
	}()
	r23WaitForBlockedTransactions(t, ctx, pool, 1)
	// Same FK insert performed by Acquire while it owns the blob. A target-file
	// FOR UPDATE lock would deadlock this insertion; NO KEY UPDATE is compatible.
	insertCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err = barrier.Exec(insertCtx, `INSERT INTO file_references(file_id,owner_type,owner_id) VALUES($1,1,$2)`, id, uuid.New())
	require.NoError(t, err)
	require.NoError(t, barrier.Commit(ctx))
	require.Equal(t, codes.FailedPrecondition, status.Code(<-done))
	require.Empty(t, deleter.calls)
}

func TestBE264LegacySingletonDeletesAllVariantsAndReplays(t *testing.T) {
	ctx := r23TestContext(t)
	pool := startR23FilePostgres(t, ctx)
	id := insertR23ReadyFile(t, ctx, pool, "singleton")
	subject := uuid.New()
	_, err := pool.Exec(ctx, `UPDATE files SET uploader_profile_id=$2,converted_r2_key='variant',thumbnail_r2_key='thumbnail' WHERE id=$1`, id, subject)
	require.NoError(t, err)
	deleter := &r23ScriptedDeleter{}
	client := New(Deps{Files: store.NewFilesStore(pool), Deleter: deleter})
	userCtx := r23UserContext(ctx, uuid.New(), subject)
	request := &filev1.DeleteFileRequest{FileId: id.String()}
	_, err = client.DeleteFile(r23UserContext(ctx, uuid.New(), uuid.New()), request)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Empty(t, deleter.calls)
	for i := 0; i < 2; i++ {
		_, err = client.DeleteFile(userCtx, request)
		require.NoError(t, err)
	}
	require.ElementsMatch(t, []string{"r23/singleton", "variant", "thumbnail"}, deleter.calls)
	require.Equal(t, "GC_COMPLETE", r23BlobGCState(t, ctx, pool, id))
}
