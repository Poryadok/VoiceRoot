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
	"voice/backend/file/internal/store"
)

// The triggers are transaction barriers only. Both operations execute their real
// handlers and commit paths; no reference or deletion outcome is synthesized.
func TestBE264RealAcquireAndLegacyDeleteBothWinners(t *testing.T) {
	for _, acquireFirst := range []bool{true, false} {
		name := "delete-intent-first"
		if acquireFirst {
			name = "acquire-first"
		}
		t.Run(name, func(t *testing.T) {
			// Nested isolated PostgreSQL startup is outside the five-second
			// lock-barrier oracle; allow setup without relaxing that oracle.
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			t.Cleanup(cancel)
			pool := startR23FilePostgres(t, ctx)
			id := insertR23ReadyFile(t, ctx, pool, name)
			uploader := uuid.New()
			_, err := pool.Exec(ctx, `UPDATE files SET uploader_profile_id=$2 WHERE id=$1`, id, uploader)
			require.NoError(t, err)
			gate, err := pool.Begin(ctx)
			require.NoError(t, err)
			t.Cleanup(func() { _ = gate.Rollback(context.Background()) })
			_, err = gate.Exec(ctx, `SELECT pg_advisory_xact_lock(264404)`)
			require.NoError(t, err)
			trigger := `CREATE FUNCTION be264_gate() RETURNS trigger LANGUAGE plpgsql AS $$
 BEGIN PERFORM pg_advisory_xact_lock(264404); RETURN NEW; END $$;
 CREATE TRIGGER be264_gate BEFORE INSERT ON file_references FOR EACH ROW EXECUTE FUNCTION be264_gate()`
			if !acquireFirst {
				trigger = `CREATE FUNCTION be264_gate() RETURNS trigger LANGUAGE plpgsql AS $$
 BEGIN PERFORM pg_advisory_xact_lock(264404); RETURN NEW; END $$;
 CREATE TRIGGER be264_gate BEFORE UPDATE ON file_blobs FOR EACH ROW
 WHEN (OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL) EXECUTE FUNCTION be264_gate()`
			}
			_, err = pool.Exec(ctx, trigger)
			require.NoError(t, err)
			files := store.NewFilesStore(pool)
			deleter := &r23ScriptedDeleter{}
			legacy := New(Deps{Files: files, Deleter: deleter})
			if !acquireFirst {
				// The failed synchronous storage phase leaves the committed intent
				// available to the normal worker, independently of process memory.
				legacy = New(Deps{Files: files})
			}
			authority := r23FileServer(pool)
			request := r23AcquireRequest(r23MessageReference(id, uuid.New(), uuid.New()))
			acquireCtx := r23ServiceContext(t, ctx, "messaging", filev1.FileService_AcquireFileReferences_FullMethodName, request)
			deleteCtx := r23UserContext(ctx, uuid.New(), uploader)
			acquired := make(chan error, 1)
			deleted := make(chan error, 1)
			acquire := func() {
				_, e := authority.AcquireFileReferences(acquireCtx, request)
				acquired <- e
			}
			remove := func() {
				_, e := legacy.DeleteFile(deleteCtx, &filev1.DeleteFileRequest{FileId: id.String()})
				deleted <- e
			}
			if acquireFirst {
				go acquire()
			} else {
				go remove()
			}
			r23WaitForBlockedTransactions(t, ctx, pool, 1)
			if acquireFirst {
				go remove()
			} else {
				go acquire()
			}
			// Both transactions reached real locks before the winner can commit.
			// In acquire-first, Delete already owns the file lock: the later
			// genuine reference FK check detects an incompatible FOR UPDATE lock.
			r23WaitForBlockedTransactions(t, ctx, pool, 2)
			require.NoError(t, gate.Commit(ctx))
			var acquireErr, deleteErr error
			select {
			case acquireErr = <-acquired:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			select {
			case deleteErr = <-deleted:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			var references int
			var started bool
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM file_references WHERE file_id=$1 AND released_at IS NULL`, id).Scan(&references))
			require.NoError(t, pool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM file_blobs WHERE blob_id=(SELECT blob_id FROM files WHERE id=$1)`, id).Scan(&started))
			row, err := files.GetFileByID(ctx, id)
			require.NoError(t, err)
			if acquireFirst {
				require.NoError(t, acquireErr)
				require.Equal(t, codes.FailedPrecondition, status.Code(deleteErr))
				require.Equal(t, 1, references)
				require.False(t, started)
				require.Equal(t, "ready", row.Status)
				require.Equal(t, "LIVE", r23BlobGCState(t, ctx, pool, id))
			} else {
				require.Equal(t, codes.FailedPrecondition, status.Code(acquireErr))
				require.Error(t, deleteErr, "nil storage must not claim completed deletion")
				require.Zero(t, references)
				require.True(t, started)
				require.Equal(t, "deleted", row.Status)
				require.Equal(t, "GC_PENDING", r23BlobGCState(t, ctx, pool, id))
			}
			require.Empty(t, deleter.calls)
		})
	}
}
