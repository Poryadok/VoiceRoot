package grpcsvc

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	filev1 "voice.app/voice/file/v1"
	storyv1 "voice.app/voice/story/v1"
	"voice/backend/file/internal/store"
	"voice/backend/pkg/principal"
)

// RED handler contract: WithVerified models only the server-owned context that
// the future protected TLS/JWKS/replay interceptor must construct.
func TestValidateStoryMediaRED_requiresVerifiedExactStoryPrincipal(t *testing.T) {
	if testing.Short() {
		t.Skip("requires isolated PostgreSQL")
	}
	ctx := context.Background()
	pool := startFileGatePostgres(t, ctx)
	fileID, authorID := uuid.New(), uuid.New()
	seedStoryMediaFile(t, ctx, pool, fileID, authorID, "image", "ready", "clean", nil, false, nil, nil)
	req := &filev1.ValidateStoryMediaRequest{FileId: fileID.String(), AuthorProfileId: authorID.String(), ExpectedStoryType: storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO}
	wrong, _ := principal.FromContext(storyMediaVerifiedContext(t, req))
	wrong.Subject, wrong.Issuer = "service:gateway", "gateway"
	badBinding, _ := principal.FromContext(storyMediaVerifiedContext(t, req))
	badBinding.RequestID = "different-request"
	for _, tc := range []struct {
		name string
		ctx  context.Context
		want codes.Code
	}{
		{"missing verified principal", context.Background(), codes.Unauthenticated},
		{"different verified service", principal.WithVerified(storyMediaVerifiedContext(t, req), wrong), codes.PermissionDenied},
		{"mismatched verified request binding", principal.WithVerified(storyMediaVerifiedContext(t, req), badBinding), codes.Unauthenticated},
		{"verified story reaches predicate", storyMediaVerifiedContext(t, req), codes.OK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := storyMediaFileSnapshot(t, pool, fileID)
			_, err := New(Deps{Files: store.NewFilesStore(pool)}).ValidateStoryMedia(tc.ctx, req)
			require.Equal(t, tc.want, status.Code(err))
			require.Equal(t, before, storyMediaFileSnapshot(t, pool, fileID))
		})
	}
}

func TestValidateStoryMediaRED_atomicFilePredicate(t *testing.T) {
	if testing.Short() {
		t.Skip("requires isolated PostgreSQL")
	}
	ctx := context.Background()
	pool := startFileGatePostgres(t, ctx)
	cases := []struct {
		name, fileType, state, scan string
		chatID, storyID             *uuid.UUID
		e2e, authorSame             bool
		duration                    *int32
		expected                    storyv1.StoryMediaType
		want                        codes.Code
	}{
		{"clean image", "image", "ready", "clean", nil, nil, false, true, nil, storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO, codes.OK},
		{"skipped image", "image", "ready", "skipped", nil, nil, false, true, nil, storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO, codes.OK},
		{"one second video", "video", "ready", "clean", nil, nil, false, true, int32p(1), storyv1.StoryMediaType_STORY_MEDIA_TYPE_VIDEO, codes.OK},
		{"sixty second video", "video", "ready", "clean", nil, nil, false, true, int32p(60), storyv1.StoryMediaType_STORY_MEDIA_TYPE_VIDEO, codes.OK},
		{"skipped video", "video", "ready", "skipped", nil, nil, false, true, int32p(1), storyv1.StoryMediaType_STORY_MEDIA_TYPE_VIDEO, codes.OK},
		{"foreign uploader", "image", "ready", "clean", nil, nil, false, false, nil, storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO, codes.PermissionDenied},
		{"chat scoped", "image", "ready", "clean", uuidPtr(uuid.New()), nil, false, true, nil, storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO, codes.FailedPrecondition},
		{"e2e", "image", "ready", "clean", nil, nil, true, true, nil, storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO, codes.FailedPrecondition},
		{"already story scoped", "image", "ready", "clean", nil, uuidPtr(uuid.New()), false, true, nil, storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO, codes.FailedPrecondition},
		{"pending lifecycle", "image", "pending_upload", "pending", nil, nil, false, true, nil, storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO, codes.FailedPrecondition},
		{"infected scan", "image", "failed", "infected", nil, nil, false, true, nil, storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO, codes.FailedPrecondition},
		{"scan error", "image", "failed", "error", nil, nil, false, true, nil, storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO, codes.FailedPrecondition},
		{"wrong media category", "video", "ready", "clean", nil, nil, false, true, int32p(10), storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO, codes.FailedPrecondition},
		{"image is not video", "image", "ready", "clean", nil, nil, false, true, int32p(10), storyv1.StoryMediaType_STORY_MEDIA_TYPE_VIDEO, codes.FailedPrecondition},
		{"audio is not video", "audio", "ready", "clean", nil, nil, false, true, int32p(10), storyv1.StoryMediaType_STORY_MEDIA_TYPE_VIDEO, codes.FailedPrecondition},
		{"document is not photo", "document", "ready", "clean", nil, nil, false, true, nil, storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO, codes.FailedPrecondition},
		{"other is not photo", "other", "ready", "clean", nil, nil, false, true, nil, storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO, codes.FailedPrecondition},
		{"missing duration", "video", "ready", "clean", nil, nil, false, true, nil, storyv1.StoryMediaType_STORY_MEDIA_TYPE_VIDEO, codes.FailedPrecondition},
		{"zero duration", "video", "ready", "clean", nil, nil, false, true, int32p(0), storyv1.StoryMediaType_STORY_MEDIA_TYPE_VIDEO, codes.FailedPrecondition},
		{"overlong video", "video", "ready", "clean", nil, nil, false, true, int32p(61), storyv1.StoryMediaType_STORY_MEDIA_TYPE_VIDEO, codes.FailedPrecondition},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fileID, ownerID, authorID := uuid.New(), uuid.New(), uuid.New()
			if tc.authorSame {
				authorID = ownerID
			}
			seedStoryMediaFile(t, ctx, pool, fileID, ownerID, tc.fileType, tc.state, tc.scan, tc.chatID, tc.e2e, tc.storyID, tc.duration)
			req := &filev1.ValidateStoryMediaRequest{FileId: fileID.String(), AuthorProfileId: authorID.String(), ExpectedStoryType: tc.expected}
			before := storyMediaFileSnapshot(t, pool, fileID)
			_, err := New(Deps{Files: store.NewFilesStore(pool)}).ValidateStoryMedia(storyMediaVerifiedContext(t, req), req)
			require.Equal(t, tc.want, status.Code(err))
			require.Equal(t, before, storyMediaFileSnapshot(t, pool, fileID), "validation must not mutate even accepted uploads")
		})
	}
}

func TestValidateStoryMediaRED_dependencyAndAbsentFile(t *testing.T) {
	req := &filev1.ValidateStoryMediaRequest{FileId: uuid.NewString(), AuthorProfileId: uuid.NewString(), ExpectedStoryType: storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO}
	t.Run("missing persistence", func(t *testing.T) {
		_, err := New(Deps{}).ValidateStoryMedia(storyMediaVerifiedContext(t, req), req)
		require.Equal(t, codes.Unavailable, status.Code(err))
	})
	t.Run("absent file", func(t *testing.T) {
		if testing.Short() {
			t.Skip("requires isolated PostgreSQL")
		}
		pool := startFileGatePostgres(t, context.Background())
		_, err := New(Deps{Files: store.NewFilesStore(pool)}).ValidateStoryMedia(storyMediaVerifiedContext(t, req), req)
		require.Equal(t, codes.NotFound, status.Code(err))
	})
}

func storyMediaVerifiedContext(t *testing.T, req proto.Message) context.Context {
	t.Helper()
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer verified-fixture", "x-request-id", "story-media-red"))
	return principal.WithVerified(ctx, principal.Principal{Kind: "service", Issuer: "story", Subject: "service:story", Audience: "file", RPC: filev1.FileService_ValidateStoryMedia_FullMethodName, RequestID: "story-media-red", RequestHash: hash})
}

func storyMediaFileSnapshot(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) string {
	t.Helper()
	var snapshot string
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT row_to_json(f)::text FROM files f WHERE id=$1`, id).Scan(&snapshot))
	return snapshot
}

func TestValidateStoryMediaRED_inputRejectedBeforePersistence(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*filev1.ValidateStoryMediaRequest)
	}{
		{"missing file", func(r *filev1.ValidateStoryMediaRequest) { r.FileId = "" }},
		{"malformed file", func(r *filev1.ValidateStoryMediaRequest) { r.FileId = "invalid" }},
		{"missing author", func(r *filev1.ValidateStoryMediaRequest) { r.AuthorProfileId = "" }},
		{"malformed author", func(r *filev1.ValidateStoryMediaRequest) { r.AuthorProfileId = "invalid" }},
		{"unspecified category", func(r *filev1.ValidateStoryMediaRequest) {
			r.ExpectedStoryType = storyv1.StoryMediaType_STORY_MEDIA_TYPE_UNSPECIFIED
		}},
		{"text category", func(r *filev1.ValidateStoryMediaRequest) {
			r.ExpectedStoryType = storyv1.StoryMediaType_STORY_MEDIA_TYPE_TEXT
		}},
		{"unknown category", func(r *filev1.ValidateStoryMediaRequest) { r.ExpectedStoryType = storyv1.StoryMediaType(99) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := &filev1.ValidateStoryMediaRequest{FileId: uuid.NewString(), AuthorProfileId: uuid.NewString(), ExpectedStoryType: storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO}
			tc.edit(req)
			_, err := New(Deps{}).ValidateStoryMedia(storyMediaVerifiedContext(t, req), req)
			require.Equal(t, codes.InvalidArgument, status.Code(err), "input denial precedes unavailable persistence")
		})
	}
}

func TestValidateStoryMediaRED_principalBindingBeforePersistence(t *testing.T) {
	req := &filev1.ValidateStoryMediaRequest{FileId: uuid.NewString(), AuthorProfileId: uuid.NewString(), ExpectedStoryType: storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO}
	for _, tc := range []struct {
		name string
		edit func(*principal.Principal)
		want codes.Code
	}{
		{"kind", func(p *principal.Principal) { p.Kind = "delegated_user" }, codes.PermissionDenied},
		{"issuer", func(p *principal.Principal) { p.Issuer = "gateway" }, codes.PermissionDenied},
		{"subject", func(p *principal.Principal) { p.Subject = "service:gateway" }, codes.PermissionDenied},
		{"audience", func(p *principal.Principal) { p.Audience = "story" }, codes.Unauthenticated},
		{"rpc", func(p *principal.Principal) { p.RPC = filev1.FileService_GetFileMetadata_FullMethodName }, codes.Unauthenticated},
		{"request id", func(p *principal.Principal) { p.RequestID = "different" }, codes.Unauthenticated},
		{"request hash", func(p *principal.Principal) { p.RequestHash = strings.Repeat("0", 64) }, codes.Unauthenticated},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := storyMediaVerifiedContext(t, req)
			p, _ := principal.FromContext(ctx)
			tc.edit(&p)
			_, err := New(Deps{}).ValidateStoryMedia(principal.WithVerified(ctx, p), req)
			require.Equal(t, tc.want, status.Code(err))
		})
	}
	for _, ids := range [][]string{nil, {"story-media-red", "story-media-red"}, {"different"}} {
		ctx := storyMediaVerifiedContext(t, req)
		md := metadata.Pairs("authorization", "Bearer verified-fixture")
		md["x-request-id"] = ids
		_, err := New(Deps{}).ValidateStoryMedia(metadata.NewIncomingContext(ctx, md), req)
		require.Equal(t, codes.Unauthenticated, status.Code(err))
	}
}

func TestValidateStoryMediaRED_lifecycleAndScanIndependently(t *testing.T) {
	if testing.Short() {
		t.Skip("requires isolated PostgreSQL")
	}
	ctx := context.Background()
	pool := startFileGatePostgres(t, ctx)
	for _, tc := range []struct{ state, scan string }{
		{"pending_upload", "clean"}, {"processing", "clean"}, {"failed", "clean"}, {"deleted", "clean"}, {"expired", "clean"},
		{"ready", "pending"}, {"ready", "infected"}, {"ready", "error"},
	} {
		t.Run(tc.state+"_"+tc.scan, func(t *testing.T) {
			id, owner := uuid.New(), uuid.New()
			seedStoryMediaFile(t, ctx, pool, id, owner, "image", tc.state, tc.scan, nil, false, nil, nil)
			req := &filev1.ValidateStoryMediaRequest{FileId: id.String(), AuthorProfileId: owner.String(), ExpectedStoryType: storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO}
			before := storyMediaFileSnapshot(t, pool, id)
			_, err := New(Deps{Files: store.NewFilesStore(pool)}).ValidateStoryMedia(storyMediaVerifiedContext(t, req), req)
			require.Equal(t, codes.FailedPrecondition, status.Code(err))
			require.Equal(t, before, storyMediaFileSnapshot(t, pool, id))
		})
	}
}

// The barrier stops after PostgreSQL has returned the locked row but before the
// handler can finish validation/commit. It observes real pgx I/O, not a store fake.
type storyMediaLockTrace struct {
	started, locked chan uint32
	release         chan struct{}
	once            sync.Once
	stopAfterRead   bool
}
type storyMediaLockTraceKey struct{}

func (b *storyMediaLockTrace) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if strings.Contains(data.SQL, "FROM files") && (strings.Contains(data.SQL, "FOR SHARE") || strings.Contains(data.SQL, "FOR UPDATE")) {
		b.started <- conn.PgConn().PID()
		return context.WithValue(ctx, storyMediaLockTraceKey{}, true)
	}
	return ctx
}
func (b *storyMediaLockTrace) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	if ctx.Value(storyMediaLockTraceKey{}) != true || data.Err != nil {
		return
	}
	b.locked <- conn.PgConn().PID()
	if b.stopAfterRead {
		select {
		case <-b.release:
		case <-ctx.Done():
		}
	}
}
func (b *storyMediaLockTrace) unblock() { b.once.Do(func() { close(b.release) }) }

func TestValidateStoryMediaRED_realPostgresLockAtomicity(t *testing.T) {
	if testing.Short() {
		t.Skip("requires isolated PostgreSQL")
	}
	for _, writerFirst := range []bool{false, true} {
		name := "validation_blocks_lifecycle_writer"
		if writerFirst {
			name = "committed_transition_before_lock_is_rejected"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			pool := startFileGatePostgres(t, ctx)
			id, author := uuid.New(), uuid.New()
			seedStoryMediaFile(t, ctx, pool, id, author, "image", "ready", "clean", nil, false, nil, nil)
			barrier := &storyMediaLockTrace{started: make(chan uint32, 1), locked: make(chan uint32, 1), release: make(chan struct{}), stopAfterRead: !writerFirst}
			defer barrier.unblock()
			config := pool.Config()
			config.ConnConfig.Tracer = barrier
			reader, err := pgxpool.NewWithConfig(ctx, config)
			require.NoError(t, err)
			defer func() { cancel(); barrier.unblock(); reader.Close() }()
			writer, err := pool.Acquire(ctx)
			require.NoError(t, err)
			defer writer.Release()
			writerPID := writer.Conn().PgConn().PID()
			var tx pgx.Tx
			if writerFirst {
				tx, err = writer.Begin(ctx)
				require.NoError(t, err)
				defer func() { _ = tx.Rollback(context.Background()) }()
				_, err = tx.Exec(ctx, `UPDATE files SET status='deleted' WHERE id=$1`, id)
				require.NoError(t, err)
			}
			req := &filev1.ValidateStoryMediaRequest{FileId: id.String(), AuthorProfileId: author.String(), ExpectedStoryType: storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO}
			result := make(chan error, 1)
			verifiedCtx := storyMediaVerifiedContext(t, req)
			go func() {
				callCtx, stop := context.WithCancel(verifiedCtx)
				defer stop()
				stopDeadline := context.AfterFunc(ctx, stop)
				defer stopDeadline()
				_, callErr := New(Deps{Files: store.NewFilesStore(reader)}).ValidateStoryMedia(callCtx, req)
				result <- callErr
			}()
			var readerPID uint32
			select {
			case readerPID = <-barrier.started:
			case err := <-result:
				t.Fatalf("validation returned without acquiring row lock: %v", err)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			if writerFirst {
				storyMediaRequireBlockedBy(t, ctx, pool, readerPID, writerPID)
				require.NoError(t, tx.Commit(ctx))
				select {
				case err := <-result:
					require.Equal(t, codes.FailedPrecondition, status.Code(err))
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
				return
			}
			select {
			case <-barrier.locked:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			written := make(chan error, 1)
			go func() {
				_, err := writer.Exec(ctx, `UPDATE files SET status='deleted' WHERE id=$1`, id)
				written <- err
			}()
			storyMediaRequireBlockedBy(t, ctx, pool, writerPID, readerPID)
			barrier.unblock()
			select {
			case err := <-result:
				require.NoError(t, err)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			select {
			case err := <-written:
				require.NoError(t, err)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
		})
	}
}

func storyMediaRequireBlockedBy(t *testing.T, ctx context.Context, pool *pgxpool.Pool, blocked, blocker uint32) {
	t.Helper()
	require.Eventually(t, func() bool {
		var found bool
		err := pool.QueryRow(ctx, `SELECT $2::int = ANY(pg_blocking_pids($1::int))`, blocked, blocker).Scan(&found)
		return err == nil && found
	}, 5*time.Second, 10*time.Millisecond, "PostgreSQL must report the actual conflicting row lock")
}

func seedStoryMediaFile(t *testing.T, ctx context.Context, pool *pgxpool.Pool, fileID, ownerID uuid.UUID, fileType, state, scan string, chatID *uuid.UUID, e2e bool, storyID *uuid.UUID, duration *int32) {
	t.Helper()
	_, err := pool.Exec(ctx, `INSERT INTO files (id, uploader_profile_id, original_name, mime_type, size_bytes, r2_key, status, file_type, chat_id, is_e2e, story_id, scan_result, duration_seconds) VALUES ($1,$2,'media','image/png',1,$3,$4,$5,$6,$7,$8,$9,$10)`, fileID, ownerID, "attachments/"+fileID.String(), state, fileType, chatID, e2e, storyID, scan, duration)
	require.NoError(t, err)
}

func int32p(value int32) *int32 { return &value }
