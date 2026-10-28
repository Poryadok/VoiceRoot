package grpcsvc

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	filev1 "voice.app/voice/file/v1"
	storyv1 "voice.app/voice/story/v1"
	"voice/backend/file/internal/store"
)

// StoryReferenceReleaseAuthority must prove the complete global owner backfill
// and current File-ledger activation for this installation/restore cohort.
// No production provider is installed yet. A configuration boolean, an empty
// ledger or Story's own reference is not this evidence.
type StoryReferenceReleaseAuthority interface {
	RequireCompleteReferenceLedger(context.Context) error
}

// ClaimStoryMedia atomically attests admission and acquires the exact Story key.
// A replay returns its original receipt, even after that key was released.
func (s *FileGRPC) ClaimStoryMedia(ctx context.Context, req *filev1.ClaimStoryMediaRequest) (*filev1.ClaimStoryMediaResponse, error) {
	if _, err := requireFileService(ctx, "story", filev1.FileService_ClaimStoryMedia_FullMethodName, req); err != nil {
		return nil, err
	}
	if req == nil || len(req.ProtoReflect().GetUnknown()) != 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	if err := requireLifecycleStore(s); err != nil {
		return nil, err
	}
	op, err := parseUUID("operation_id", req.GetOperationId())
	if err != nil {
		return nil, err
	}
	story, err := parseUUID("story_id", req.GetStoryId())
	if err != nil {
		return nil, err
	}
	file, err := parseUUID("file_id", req.GetFileId())
	if err != nil {
		return nil, err
	}
	author, err := parseUUID("author_profile_id", req.GetAuthorProfileId())
	if err != nil {
		return nil, err
	}
	if req.GetExpectedStoryType() != storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO && req.GetExpectedStoryType() != storyv1.StoryMediaType_STORY_MEDIA_TYPE_VIDEO {
		return nil, status.Error(codes.InvalidArgument, "invalid story media type")
	}
	wire, hash, err := lifecycleRequest(req)
	if err != nil {
		return nil, err
	}
	tx, err := store.BeginInventoryMutation(ctx, s.files.Pool, false)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "file persistence unavailable")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// Serialize before the receipt lookup: a concurrent lost-response retry must
	// never race compensation and recreate a reference after release.
	if err = lockStoryMediaOperation(ctx, tx, op, story); err != nil {
		return nil, err
	}
	response := &filev1.ClaimStoryMediaResponse{}
	if found, e := loadReferenceOperation(ctx, tx, "story", op, hash, response); e != nil {
		return nil, e
	} else if found {
		return response, nil
	}
	var fileLocked uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT id FROM files WHERE id=$1 FOR NO KEY UPDATE`, file).Scan(&fileLocked); errors.Is(err, pgx.ErrNoRows) {
		return nil, status.Error(codes.NotFound, "file not found")
	} else if err != nil {
		return nil, err
	}
	var blob uuid.UUID
	var blobState string
	var deletion *time.Time
	err = tx.QueryRow(ctx, `SELECT b.blob_id,b.state,b.deleted_at FROM files f JOIN file_blobs b ON b.blob_id=f.blob_id WHERE f.id=$1 FOR UPDATE OF b`, file).Scan(&blob, &blobState, &deletion)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, status.Error(codes.NotFound, "file not found")
	}
	if err != nil {
		return nil, status.Error(codes.Unavailable, "file persistence unavailable")
	}
	if (blobState != "LIVE" && blobState != "GC_PENDING") || deletion != nil {
		return nil, status.Error(codes.FailedPrecondition, "blob already collected")
	}
	var uploader uuid.UUID
	var chat, existingStory *uuid.UUID
	var e2e bool
	var lifecycle, scan, category string
	var duration *int32
	err = tx.QueryRow(ctx, `SELECT uploader_profile_id,chat_id,story_id,is_e2e,status,scan_result,file_type,duration_seconds FROM files WHERE id=$1 AND blob_id=$2`, file, blob).Scan(&uploader, &chat, &existingStory, &e2e, &lifecycle, &scan, &category, &duration)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "file persistence unavailable")
	}
	if uploader != author {
		return nil, status.Error(codes.PermissionDenied, "file belongs to another author")
	}
	valid := chat == nil && existingStory == nil && !e2e && lifecycle == "ready" && (scan == "clean" || scan == "skipped")
	if req.GetExpectedStoryType() == storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO {
		valid = valid && category == "image"
	} else {
		valid = valid && category == "video" && duration != nil && *duration >= 1 && *duration <= 60
	}
	if !valid {
		return nil, status.Error(codes.FailedPrecondition, "file is not eligible story media")
	}
	var conflicting bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM file_references WHERE owner_type=2 AND owner_id=$1 AND (file_id<>$2 OR released_at IS NOT NULL OR subresource_id IS NOT NULL OR scope_space_id IS NOT NULL))`, story, file).Scan(&conflicting)
	if err != nil {
		return nil, err
	}
	if conflicting {
		return nil, status.Error(codes.AlreadyExists, "story media binding is immutable")
	}
	_, err = tx.Exec(ctx, `INSERT INTO file_references(file_id,owner_type,owner_id) SELECT $1,2,$2 WHERE NOT EXISTS(SELECT 1 FROM file_references WHERE file_id=$1 AND owner_type=2 AND owner_id=$2 AND subresource_id IS NULL AND scope_space_id IS NULL AND released_at IS NULL)`, file, story)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `UPDATE file_blobs SET state='LIVE',next_attempt_at=NULL WHERE blob_id=$1 AND state='GC_PENDING'`, blob)
	if err != nil {
		return nil, err
	}
	response = &filev1.ClaimStoryMediaResponse{ReceiptId: uuid.NewString(), OperationId: req.OperationId, RequestSha256: hash}
	if err = saveReferenceOperation(ctx, tx, "story", op, wire, hash, response); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, status.Error(codes.Unavailable, "file persistence unavailable")
	}
	return response, nil
}

func (s *FileGRPC) ReleaseStoryMedia(ctx context.Context, req *filev1.ReleaseStoryMediaRequest) (*filev1.ReleaseStoryMediaResponse, error) {
	if _, err := requireFileService(ctx, "story", filev1.FileService_ReleaseStoryMedia_FullMethodName, req); err != nil {
		return nil, err
	}
	if req == nil || len(req.ProtoReflect().GetUnknown()) != 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	if err := requireLifecycleStore(s); err != nil {
		return nil, err
	}
	op, err := parseUUID("operation_id", req.GetOperationId())
	if err != nil {
		return nil, err
	}
	story, err := parseUUID("story_id", req.GetStoryId())
	if err != nil {
		return nil, err
	}
	file, err := parseUUID("file_id", req.GetFileId())
	if err != nil {
		return nil, err
	}
	wire, hash, err := lifecycleRequest(req)
	if err != nil {
		return nil, err
	}
	tx, err := store.BeginInventoryMutation(ctx, s.files.Pool, false)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = lockStoryMediaOperation(ctx, tx, op, story); err != nil {
		return nil, err
	}
	response := &filev1.ReleaseStoryMediaResponse{}
	if found, e := loadReferenceOperation(ctx, tx, "story", op, hash, response); e != nil {
		return nil, e
	} else if found {
		return response, nil
	}
	// Keep the reference live while global legacy owners may still be absent
	// from File's ledger. Merely avoiding this GC enqueue would not protect
	// against another producer's release or the independently running worker.
	if s.storyReleaseAuthority == nil {
		return nil, status.Error(codes.FailedPrecondition, "Story reference release awaits verified ledger activation")
	}
	if err = s.storyReleaseAuthority.RequireCompleteReferenceLedger(ctx); err != nil {
		return nil, status.Error(codes.FailedPrecondition, "Story reference release awaits verified ledger activation")
	}
	var blob uuid.UUID
	err = tx.QueryRow(ctx, `SELECT b.blob_id FROM files f JOIN file_blobs b ON b.blob_id=f.blob_id WHERE f.id=$1 FOR UPDATE OF b`, file).Scan(&blob)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if err == nil {
		tag, releaseErr := tx.Exec(ctx, `UPDATE file_references SET released_at=clock_timestamp(),release_operation_id=$3 WHERE file_id=$1 AND owner_type=2 AND owner_id=$2 AND subresource_id IS NULL AND scope_space_id IS NULL AND released_at IS NULL`, file, story, op)
		err = releaseErr
		if err != nil {
			return nil, err
		}
		if tag.RowsAffected() > 0 {
			_, err = tx.Exec(ctx, `UPDATE file_blobs b SET state='GC_PENDING',gc_operation_id=COALESCE(gc_operation_id,$2),next_attempt_at=clock_timestamp() WHERE blob_id=$1 AND state='LIVE' AND NOT EXISTS(SELECT 1 FROM files f JOIN file_references r ON r.file_id=f.id WHERE f.blob_id=b.blob_id AND r.released_at IS NULL)`, blob, uuid.New())
			if err != nil {
				return nil, err
			}
		}
	}
	// Retire the exact key even if acquisition has not reached File yet. Do not
	// infer ownership or initiate GC for historical unclaimed Story media.
	_, err = tx.Exec(ctx, `INSERT INTO file_references(file_id,owner_type,owner_id,released_at,release_operation_id) SELECT f.id,2,$2,clock_timestamp(),$3 FROM files f WHERE f.id=$1 AND NOT EXISTS(SELECT 1 FROM file_references r WHERE r.file_id=f.id AND r.owner_type=2 AND r.owner_id=$2 AND r.subresource_id IS NULL AND r.scope_space_id IS NULL)`, file, story, op)
	if err != nil {
		return nil, err
	}
	response = &filev1.ReleaseStoryMediaResponse{ReceiptId: uuid.NewString(), OperationId: req.OperationId, RequestSha256: hash}
	if err = saveReferenceOperation(ctx, tx, "story", op, wire, hash, response); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return response, nil
}

func lockStoryMediaOperation(ctx context.Context, tx pgx.Tx, op, story uuid.UUID) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('story-media-operation:'||$1,0))`, op.String()); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('story-media-owner:'||$1,0))`, story.String())
	return err
}
