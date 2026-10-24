package grpcsvc

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/file/internal/store"
	"voice/backend/pkg/principal"
)

func (s *FileGRPC) ResolveMessageAttachmentPreview(ctx context.Context, req *filev1.ResolveMessageAttachmentPreviewRequest) (*filev1.ResolveMessageAttachmentPreviewResponse, error) {
	if _, err := requireFileService(ctx, "messaging", filev1.FileService_ResolveMessageAttachmentPreview_FullMethodName, req); err != nil {
		return nil, err
	}
	p, _ := principal.FromContext(ctx)
	if p.AccountID != "" || p.ProfileID != "" || p.SessionEpoch != 0 || p.RequestID == "" {
		return nil, status.Error(codes.PermissionDenied, "service principal required")
	}
	if err := requireLifecycleStore(s); err != nil {
		return nil, err
	}
	if req.GetProtocolVersion() != 1 || req.GetReference().GetOwnerType() != filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE || req.GetReference().SubresourceId != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid MESSAGE preview reference")
	}
	ids, err := parseReference(req.Reference)
	if err != nil {
		return nil, err
	}
	tx, err := s.files.Pool.Begin(ctx)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "preview persistence unavailable")
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if ids.scope != nil {
		var state string
		err = tx.QueryRow(ctx, `SELECT state FROM file_space_lifecycle_fences WHERE space_id=$1 FOR SHARE`, *ids.scope).Scan(&state)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.Unavailable, "Space fence unavailable")
		}
		if err != nil {
			return nil, status.Error(codes.Unavailable, "Space fence unavailable")
		}
		if state != "LIVE" {
			return nil, status.Error(codes.FailedPrecondition, "Space is not LIVE")
		}
	}
	// File SHARE is compatible with Acquire's FK KEY SHARE. The explicit blob
	// KEY SHARE below conflicts with physical GC's FOR UPDATE without changing it.
	row, err := s.files.GetFileByIDTx(ctx, tx, ids.file, false)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, status.Error(codes.FailedPrecondition, "file unavailable")
	}
	if err != nil {
		return nil, status.Error(codes.Unavailable, "file unavailable")
	}
	var blob string
	err = tx.QueryRow(ctx, `SELECT b.state FROM file_blobs b JOIN files f ON f.blob_id=b.blob_id WHERE f.id=$1 FOR KEY SHARE OF b`, ids.file).Scan(&blob)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "blob unavailable")
	}
	if blob != "LIVE" && row.Status != "deleted" && row.Status != "expired" {
		return nil, status.Error(codes.FailedPrecondition, "blob unavailable")
	}
	// Release owns blob before updating a reference. Taking the reference first
	// here would deadlock an unscoped concurrent release waiting for this SHARE.
	var reference uuid.UUID
	err = tx.QueryRow(ctx, `SELECT file_id FROM file_references WHERE file_id=$1 AND owner_type=$2 AND owner_id=$3 AND subresource_id IS NULL AND scope_space_id IS NOT DISTINCT FROM $4 AND released_at IS NULL FOR SHARE`, ids.file, int32(req.Reference.OwnerType), ids.owner, ids.scope).Scan(&reference)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, status.Error(codes.FailedPrecondition, "reference unavailable")
	}
	if err != nil {
		return nil, status.Error(codes.Unavailable, "reference unavailable")
	}
	var clockExpired bool
	if err = tx.QueryRow(ctx, `SELECT expires_at IS NOT NULL AND expires_at<=clock_timestamp() FROM files WHERE id=$1`, ids.file).Scan(&clockExpired); err != nil {
		return nil, status.Error(codes.Unavailable, "file unavailable")
	}
	if clockExpired && row.Status != "expired" && row.Status != "deleted" {
		return nil, status.Error(codes.FailedPrecondition, "expiry transition pending")
	}
	snapshot, err := store.PreviewSnapshotTx(ctx, tx, row)
	if err != nil {
		return nil, status.Error(codes.FailedPrecondition, "preview unsupported or invalid")
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, status.Error(codes.Unavailable, "preview persistence unavailable")
	}
	return &filev1.ResolveMessageAttachmentPreviewResponse{Reference: proto.Clone(req.Reference).(*filev1.FileReferenceKey), Snapshot: snapshot}, nil
}
