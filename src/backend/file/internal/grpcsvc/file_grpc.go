package grpcsvc

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/file/internal/authctx"
	"voice/backend/file/internal/r2file"
	"voice/backend/file/internal/store"

	filev1 "voice.app/voice/file/v1"
)

type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time {
	return time.Now()
}

type Deps struct {
	Files     *store.FilesStore
	Presigner r2file.Presigner
	Clock     Clock
}

type FileGRPC struct {
	filev1.UnimplementedFileServiceServer
	files     *store.FilesStore
	presigner r2file.Presigner
	clock     Clock
}

func New(deps Deps) *FileGRPC {
	clock := deps.Clock
	if clock == nil {
		clock = realClock{}
	}
	return &FileGRPC{
		files:     deps.Files,
		presigner: deps.Presigner,
		clock:     clock,
	}
}

func (s *FileGRPC) RequestUpload(ctx context.Context, req *filev1.RequestUploadRequest) (*filev1.RequestUploadResponse, error) {
	if s == nil || s.files == nil {
		return nil, status.Error(codes.FailedPrecondition, "file persistence not configured")
	}
	if s.presigner == nil {
		return nil, status.Error(codes.FailedPrecondition, "file upload is not configured")
	}
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	originalName := strings.TrimSpace(req.GetOriginalName())
	mimeType := strings.TrimSpace(strings.ToLower(req.GetMimeType()))
	sizeBytes := req.GetSizeBytes()
	if err := r2file.ValidateUpload(originalName, mimeType, sizeBytes); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if req.GetContextChat() != nil {
		return nil, status.Error(codes.FailedPrecondition, "chat-scoped file uploads require chat access guard")
	}

	fileID := uuid.New()
	r2Key := r2file.ObjectKey(fileID, originalName)
	putURL, err := s.presigner.PresignPut(ctx, r2file.PutPresignInput{
		Key:           r2Key,
		ContentType:   mimeType,
		ContentLength: sizeBytes,
		TTL:           r2file.DefaultURLTTL,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	_, err = s.files.InsertPendingFile(ctx, store.FileRow{
		ID:                fileID,
		UploaderProfileID: profileID,
		OriginalName:      originalName,
		MimeType:          mimeType,
		SizeBytes:         sizeBytes,
		R2Key:             r2Key,
		Status:            "pending_upload",
		FileType:          r2file.MediaCategory(mimeType),
		ScanResult:        "pending",
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &filev1.RequestUploadResponse{
		UploadResponse: &filev1.UploadResponse{
			FileId:          fileID.String(),
			PresignedPutUrl: putURL,
			R2Key:           r2Key,
		},
	}, nil
}

func (s *FileGRPC) GetFileURL(ctx context.Context, req *filev1.GetFileURLRequest) (*filev1.GetFileURLResponse, error) {
	if s == nil || s.files == nil {
		return nil, status.Error(codes.FailedPrecondition, "file persistence not configured")
	}
	if s.presigner == nil {
		return nil, status.Error(codes.FailedPrecondition, "file upload is not configured")
	}
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	fileID, err := parseUUID("file_id", req.GetFileId())
	if err != nil {
		return nil, err
	}
	row, err := s.files.GetFileByID(ctx, fileID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "file not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	if row.UploaderProfileID != profileID {
		return nil, status.Error(codes.PermissionDenied, "file is not owned by profile")
	}
	if row.Status != "ready" {
		return nil, status.Error(codes.FailedPrecondition, "file is not ready")
	}
	ttl := r2file.DefaultURLTTL
	getURL, err := s.presigner.PresignGet(ctx, r2file.GetPresignInput{Key: row.R2Key, TTL: ttl})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &filev1.GetFileURLResponse{
		PresignedGetUrl: getURL,
		ExpiresAt:       timestamppb.New(s.clock.Now().Add(ttl)),
	}, nil
}

func parseUUID(field, value string) (uuid.UUID, error) {
	id, err := uuid.Parse(strings.TrimSpace(value))
	if err != nil || id == uuid.Nil {
		return uuid.Nil, status.Errorf(codes.InvalidArgument, "invalid %s", field)
	}
	return id, nil
}
