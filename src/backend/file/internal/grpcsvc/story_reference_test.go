package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	filev1 "voice.app/voice/file/v1"
	storyv1 "voice.app/voice/story/v1"
	"voice/backend/file/internal/store"
)

func TestStoryReferenceClaimReleaseReplayAndSharedBlob(t *testing.T) {
	if testing.Short() {
		t.Skip("isolated PostgreSQL")
	}
	ctx := context.Background()
	pool := startR23FilePostgres(t, ctx)
	file, author := uuid.New(), uuid.New()
	seedStoryMediaFile(t, ctx, pool, file, author, "image", "ready", "clean", nil, false, nil, nil)
	_, err := pool.Exec(ctx, `INSERT INTO file_blobs(blob_id,sha256,original_r2_key,state) SELECT id,decode(repeat('a',64),'hex'),r2_key,'LIVE' FROM files WHERE id=$1 ON CONFLICT DO NOTHING`, file)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE files SET blob_id=id WHERE id=$1`, file)
	require.NoError(t, err)
	svc := New(Deps{Files: store.NewFilesStore(pool)})
	claim := func(story uuid.UUID) *filev1.ClaimStoryMediaRequest {
		return &filev1.ClaimStoryMediaRequest{OperationId: uuid.NewString(), StoryId: story.String(), FileId: file.String(), AuthorProfileId: author.String(), ExpectedStoryType: storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO}
	}
	a, b := claim(uuid.New()), claim(uuid.New())
	first, err := svc.ClaimStoryMedia(catalogVerifiedContext(t, a, filev1.FileService_ClaimStoryMedia_FullMethodName, "story"), a)
	require.NoError(t, err)
	replay, err := svc.ClaimStoryMedia(catalogVerifiedContext(t, a, filev1.FileService_ClaimStoryMedia_FullMethodName, "story"), a)
	require.NoError(t, err)
	require.True(t, proto.Equal(first, replay), "immutable acquire receipt survives lost response")
	_, err = svc.ClaimStoryMedia(catalogVerifiedContext(t, b, filev1.FileService_ClaimStoryMedia_FullMethodName, "story"), b)
	require.NoError(t, err)
	drop := &filev1.ReleaseStoryMediaRequest{OperationId: uuid.NewString(), StoryId: a.StoryId, FileId: a.FileId}
	_, err = svc.ReleaseStoryMedia(catalogVerifiedContext(t, drop, filev1.FileService_ReleaseStoryMedia_FullMethodName, "story"), drop)
	require.NoError(t, err)
	_, err = svc.ClaimStoryMedia(catalogVerifiedContext(t, a, filev1.FileService_ClaimStoryMedia_FullMethodName, "story"), a)
	require.NoError(t, err, "replay succeeds without resurrecting released reference")
	var live int
	var state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM file_references WHERE file_id=$1 AND released_at IS NULL`, file).Scan(&live))
	require.Equal(t, 1, live)
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM file_blobs WHERE blob_id=$1`, file).Scan(&state))
	require.Equal(t, "LIVE", state)
	gc := &r23ScriptedDeleter{}
	processed, err := svc.files.RunReferenceGCOnce(ctx, gc, 1)
	require.NoError(t, err)
	require.Zero(t, processed)
	require.Empty(t, gc.calls, "shared media cannot reach physical deletion")
	changed := proto.Clone(a).(*filev1.ClaimStoryMediaRequest)
	changed.StoryId = uuid.NewString()
	_, err = svc.ClaimStoryMedia(catalogVerifiedContext(t, changed, filev1.FileService_ClaimStoryMedia_FullMethodName, "story"), changed)
	require.Equal(t, codes.AlreadyExists, status.Code(err))
	drop.OperationId, drop.StoryId = uuid.NewString(), b.StoryId
	_, err = svc.ReleaseStoryMedia(catalogVerifiedContext(t, drop, filev1.FileService_ReleaseStoryMedia_FullMethodName, "story"), drop)
	require.NoError(t, err)
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM file_blobs WHERE blob_id=$1`, file).Scan(&state))
	require.Equal(t, "GC_PENDING", state)
	// An abandoned creation may be retired before its delayed acquire arrives.
	late := claim(uuid.New())
	cancel := &filev1.ReleaseStoryMediaRequest{OperationId: uuid.NewString(), StoryId: late.StoryId, FileId: late.FileId}
	_, err = svc.ReleaseStoryMedia(catalogVerifiedContext(t, cancel, filev1.FileService_ReleaseStoryMedia_FullMethodName, "story"), cancel)
	require.NoError(t, err)
	_, err = svc.ClaimStoryMedia(catalogVerifiedContext(t, late, filev1.FileService_ClaimStoryMedia_FullMethodName, "story"), late)
	require.Equal(t, codes.AlreadyExists, status.Code(err), "release must fence an acquire which had not reached File yet")
	var original string
	require.NoError(t, pool.QueryRow(ctx, `SELECT original_r2_key FROM file_blobs WHERE blob_id=$1`, file).Scan(&original))
	_, err = pool.Exec(ctx, `UPDATE file_blobs SET converted_r2_key='story/converted',thumbnail_r2_key='story/thumbnail' WHERE blob_id=$1`, file)
	require.NoError(t, err)
	gc.failures = map[string]int{original: 1}
	_, err = svc.files.RunReferenceGCOnce(ctx, gc, 1)
	require.Error(t, err, "an ambiguous object outcome remains retryable")
	processed, err = svc.files.RunReferenceGCOnce(ctx, gc, 1)
	require.NoError(t, err)
	require.EqualValues(t, 1, processed)
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM file_blobs WHERE blob_id=$1`, file).Scan(&state))
	require.Equal(t, "GC_COMPLETE", state)
	require.Contains(t, gc.calls, original)
	require.Contains(t, gc.calls, "story/converted")
	require.Contains(t, gc.calls, "story/thumbnail")
}

func TestStoryReferenceRechecksAdmissionAfterValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("isolated PostgreSQL")
	}
	ctx := context.Background()
	pool := startR23FilePostgres(t, ctx)
	file, author := uuid.New(), uuid.New()
	seedStoryMediaFile(t, ctx, pool, file, author, "image", "ready", "clean", nil, false, nil, nil)
	svc := New(Deps{Files: store.NewFilesStore(pool)})
	validation := &filev1.ValidateStoryMediaRequest{FileId: file.String(), AuthorProfileId: author.String(), ExpectedStoryType: storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO}
	_, err := svc.ValidateStoryMedia(storyMediaVerifiedContext(t, validation), validation)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE files SET scan_result='infected' WHERE id=$1`, file)
	require.NoError(t, err)
	claim := &filev1.ClaimStoryMediaRequest{OperationId: uuid.NewString(), StoryId: uuid.NewString(), FileId: file.String(), AuthorProfileId: author.String(), ExpectedStoryType: storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO}
	_, err = svc.ClaimStoryMedia(catalogVerifiedContext(t, claim, filev1.FileService_ClaimStoryMedia_FullMethodName, "story"), claim)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	var live int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM file_references WHERE file_id=$1 AND released_at IS NULL`, file).Scan(&live))
	require.Zero(t, live)
}

func TestStoryReferenceRejectsRawAuthority(t *testing.T) {
	claim := &filev1.ClaimStoryMediaRequest{OperationId: uuid.NewString(), StoryId: uuid.NewString(), FileId: uuid.NewString(), AuthorProfileId: uuid.NewString(), ExpectedStoryType: storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO}
	raw := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-internal-caller", "story", "x-voice-profile-id", claim.AuthorProfileId))
	_, err := (&FileGRPC{}).ClaimStoryMedia(raw, claim)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
