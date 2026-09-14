package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
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
		{"different verified service", principal.WithVerified(context.Background(), wrong), codes.PermissionDenied},
		{"mismatched verified request binding", principal.WithVerified(context.Background(), badBinding), codes.Unauthenticated},
		{"verified story reaches predicate", storyMediaVerifiedContext(t, req), codes.OK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := New(Deps{Files: store.NewFilesStore(pool)}).ValidateStoryMedia(tc.ctx, req)
			require.Equal(t, tc.want, status.Code(err))
		})
	}
}

func TestValidateStoryMediaRED_atomicFilePredicate(t *testing.T) {
	if testing.Short() {
		t.Skip("requires isolated PostgreSQL")
	}
	ctx := context.Background()
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
		{"foreign uploader", "image", "ready", "clean", nil, nil, false, false, nil, storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO, codes.PermissionDenied},
		{"chat scoped", "image", "ready", "clean", uuidPtr(uuid.New()), nil, false, true, nil, storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO, codes.FailedPrecondition},
		{"e2e", "image", "ready", "clean", nil, nil, true, true, nil, storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO, codes.FailedPrecondition},
		{"already story scoped", "image", "ready", "clean", nil, uuidPtr(uuid.New()), false, true, nil, storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO, codes.FailedPrecondition},
		{"pending lifecycle", "image", "pending_upload", "pending", nil, nil, false, true, nil, storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO, codes.FailedPrecondition},
		{"infected scan", "image", "failed", "infected", nil, nil, false, true, nil, storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO, codes.FailedPrecondition},
		{"scan error", "image", "failed", "error", nil, nil, false, true, nil, storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO, codes.FailedPrecondition},
		{"wrong media category", "video", "ready", "clean", nil, nil, false, true, int32p(10), storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO, codes.FailedPrecondition},
		{"missing duration", "video", "ready", "clean", nil, nil, false, true, nil, storyv1.StoryMediaType_STORY_MEDIA_TYPE_VIDEO, codes.FailedPrecondition},
		{"zero duration", "video", "ready", "clean", nil, nil, false, true, int32p(0), storyv1.StoryMediaType_STORY_MEDIA_TYPE_VIDEO, codes.FailedPrecondition},
		{"overlong video", "video", "ready", "clean", nil, nil, false, true, int32p(61), storyv1.StoryMediaType_STORY_MEDIA_TYPE_VIDEO, codes.FailedPrecondition},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pool := startFileGatePostgres(t, ctx)
			fileID, ownerID, authorID := uuid.New(), uuid.New(), uuid.New()
			if tc.authorSame {
				authorID = ownerID
			}
			seedStoryMediaFile(t, ctx, pool, fileID, ownerID, tc.fileType, tc.state, tc.scan, tc.chatID, tc.e2e, tc.storyID, tc.duration)
			req := &filev1.ValidateStoryMediaRequest{FileId: fileID.String(), AuthorProfileId: authorID.String(), ExpectedStoryType: tc.expected}
			_, err := New(Deps{Files: store.NewFilesStore(pool)}).ValidateStoryMedia(storyMediaVerifiedContext(t, req), req)
			require.Equal(t, tc.want, status.Code(err))
		})
	}
}

func storyMediaVerifiedContext(t *testing.T, req proto.Message) context.Context {
	t.Helper()
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	return principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "story", Subject: "service:story", Audience: "file", RPC: filev1.FileService_ValidateStoryMedia_FullMethodName, RequestID: "story-media-red", RequestHash: hash})
}

func seedStoryMediaFile(t *testing.T, ctx context.Context, pool *pgxpool.Pool, fileID, ownerID uuid.UUID, fileType, state, scan string, chatID *uuid.UUID, e2e bool, storyID *uuid.UUID, duration *int32) {
	t.Helper()
	_, err := pool.Exec(ctx, `INSERT INTO files (id, uploader_profile_id, original_name, mime_type, size_bytes, r2_key, status, file_type, chat_id, is_e2e, story_id, scan_result, duration_seconds) VALUES ($1,$2,'media','image/png',1,$3,$4,$5,$6,$7,$8,$9,$10)`, fileID, ownerID, "attachments/"+fileID.String(), state, fileType, chatID, e2e, storyID, scan, duration)
	require.NoError(t, err)
}

func int32p(value int32) *int32 { return &value }
