package grpcsvc

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	filev1 "voice.app/voice/file/v1"
	storyv1 "voice.app/voice/story/v1"
	"voice/backend/pkg/principal"
)

// ValidateStoryMedia attests only the current File-owned admission predicate.
// It neither claims the upload nor grants authority over later lifecycle reads.
func (s *FileGRPC) ValidateStoryMedia(ctx context.Context, req *filev1.ValidateStoryMediaRequest) (*filev1.ValidateStoryMediaResponse, error) {
	p, ok := principal.FromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "verified principal required")
	}
	if p.Kind != "service" || p.Issuer != "story" || p.Subject != "service:story" || p.AccountID != "" || p.ProfileID != "" || p.SessionEpoch != 0 {
		return nil, status.Error(codes.PermissionDenied, "story service required")
	}
	if req == nil || len(req.ProtoReflect().GetUnknown()) != 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	hash, err := principal.RequestHash(req)
	md, _ := metadata.FromIncomingContext(ctx)
	ids := md.Get("x-request-id")
	if err != nil || p.Audience != "file" || p.RPC != filev1.FileService_ValidateStoryMedia_FullMethodName || p.RequestHash != hash || len(ids) != 1 || ids[0] == "" || p.RequestID != ids[0] {
		return nil, status.Error(codes.Unauthenticated, "invalid request binding")
	}
	fileID, fileErr := uuid.Parse(req.GetFileId())
	authorID, authorErr := uuid.Parse(req.GetAuthorProfileId())
	expected := req.GetExpectedStoryType()
	if fileErr != nil || authorErr != nil || fileID == uuid.Nil || authorID == uuid.Nil || (expected != storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO && expected != storyv1.StoryMediaType_STORY_MEDIA_TYPE_VIDEO) {
		return nil, status.Error(codes.InvalidArgument, "invalid story media request")
	}
	if s == nil || s.files == nil || s.files.Pool == nil {
		return nil, status.Error(codes.Unavailable, "file persistence unavailable")
	}
	tx, err := s.files.Pool.Begin(ctx)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "file persistence unavailable")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var uploader uuid.UUID
	var chatID, storyID *uuid.UUID
	var e2e bool
	var lifecycle, scan, category string
	var duration *int32
	err = tx.QueryRow(ctx, `SELECT uploader_profile_id, chat_id, story_id,
is_e2e, status, scan_result, file_type, duration_seconds
FROM files WHERE id = $1 FOR SHARE`, fileID).Scan(&uploader, &chatID, &storyID, &e2e, &lifecycle, &scan, &category, &duration)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, status.Error(codes.NotFound, "file not found")
	}
	if err != nil {
		return nil, status.Error(codes.Unavailable, "file persistence unavailable")
	}
	if uploader != authorID {
		return nil, status.Error(codes.PermissionDenied, "file belongs to another author")
	}
	valid := chatID == nil && storyID == nil && !e2e && lifecycle == "ready" && (scan == "clean" || scan == "skipped")
	if expected == storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO {
		valid = valid && category == "image"
	} else {
		valid = valid && category == "video" && duration != nil && *duration >= 1 && *duration <= 60
	}
	if !valid {
		return nil, status.Error(codes.FailedPrecondition, "file is not eligible story media")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, status.Error(codes.Unavailable, "file persistence unavailable")
	}
	return &filev1.ValidateStoryMediaResponse{}, nil
}
