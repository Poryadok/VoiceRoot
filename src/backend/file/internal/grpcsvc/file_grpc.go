package grpcsvc

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/file/internal/authctx"
	"voice/backend/file/internal/r2file"
	"voice/backend/file/internal/store"

	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	filev1 "voice.app/voice/file/v1"
)

var sha256Re = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)

var ErrNotChatMember = errors.New("not a chat member")

type ChatGuard interface {
	EnsureMember(ctx context.Context, chatID, profileID uuid.UUID) error
	ChatE2EState(ctx context.Context, chatID uuid.UUID) (chatType string, e2eEnabled bool, err error)
}

type ImageProcessingResult struct {
	ConvertedR2Key string
	ThumbnailR2Key string
	Width          int32
	Height         int32
}

type ImageProcessor interface {
	ProcessImage(ctx context.Context, row store.FileRow) (ImageProcessingResult, error)
}

type ObjectReader interface {
	ReadObject(ctx context.Context, key string, maxBytes int64) ([]byte, error)
}

type Scanner interface {
	ScanBytes(ctx context.Context, data []byte) (string, error)
}

type keyDerivingImageProcessor struct{}

func (keyDerivingImageProcessor) ProcessImage(_ context.Context, row store.FileRow) (ImageProcessingResult, error) {
	prefix := "processed/" + row.ID.String()
	return ImageProcessingResult{
		ConvertedR2Key: prefix + "/full.webp",
		ThumbnailR2Key: prefix + "/thumb.webp",
	}, nil
}

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
	ChatGuard ChatGuard
	Processor ImageProcessor
	Reader    ObjectReader
	Scanner   Scanner
}

type FileGRPC struct {
	filev1.UnimplementedFileServiceServer
	files     *store.FilesStore
	presigner r2file.Presigner
	clock     Clock
	chatGuard ChatGuard
	processor ImageProcessor
	reader    ObjectReader
	scanner   Scanner
}

func New(deps Deps) *FileGRPC {
	clock := deps.Clock
	if clock == nil {
		clock = realClock{}
	}
	processor := deps.Processor
	if processor == nil {
		processor = keyDerivingImageProcessor{}
	}
	return &FileGRPC{
		files:     deps.Files,
		presigner: deps.Presigner,
		clock:     clock,
		chatGuard: deps.ChatGuard,
		processor: processor,
		reader:    deps.Reader,
		scanner:   deps.Scanner,
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
	maxBytes := uploadMaxBytes(ctx)
	if err := r2file.ValidateUpload(originalName, mimeType, sizeBytes, maxBytes); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	chatID, chatType, err := s.chatContext(ctx, req.GetContextChat(), profileID)
	if err != nil {
		return nil, err
	}

	isE2E := req.IsE2E != nil && *req.IsE2E
	if chatID != nil && s.chatGuard != nil {
		typ, e2eEnabled, err := s.chatGuard.ChatE2EState(ctx, *chatID)
		if err != nil {
			if errors.Is(err, ErrNotChatMember) {
				return nil, status.Error(codes.PermissionDenied, "not a chat member")
			}
			return nil, status.Error(codes.Internal, err.Error())
		}
		if isE2E {
			if typ != "dm" || !e2eEnabled {
				return nil, status.Error(codes.FailedPrecondition, "e2e file upload requires e2e-enabled dm chat")
			}
		} else if e2eEnabled {
			return nil, status.Error(codes.FailedPrecondition, "e2e chat requires encrypted file upload")
		}
	}

	fileID := uuid.New()
	r2Key := r2file.ObjectKey(fileID, originalName)
	var expiresAt *time.Time
	if isE2E {
		t := time.Now().UTC().Add(90 * 24 * time.Hour)
		expiresAt = &t
	}
	putURL, err := s.presigner.PresignPut(ctx, r2file.PutPresignInput{
		Key:           r2Key,
		ContentType:   mimeType,
		ContentLength: sizeBytes,
		MaxBytes:      maxBytes,
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
		ChatID:            chatID,
		ChatType:          chatType,
		IsE2E:             isE2E,
		ExpiresAt:         expiresAt,
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
	if err := s.ensureFileAccess(ctx, row, profileID); err != nil {
		return nil, err
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

func (s *FileGRPC) ConfirmUpload(ctx context.Context, req *filev1.ConfirmUploadRequest) (*filev1.ConfirmUploadResponse, error) {
	profileID, err := s.requireProfile(ctx)
	if err != nil {
		return nil, err
	}
	fileID, err := parseUUID("file_id", req.GetFileId())
	if err != nil {
		return nil, err
	}
	sha := strings.TrimSpace(req.GetSha256Hash())
	if !sha256Re.MatchString(sha) {
		return nil, status.Error(codes.InvalidArgument, "invalid sha256_hash")
	}
	row, err := s.fileOwnedByUploader(ctx, fileID, profileID)
	if err != nil {
		return nil, err
	}
	if row.Status != "pending_upload" {
		return nil, status.Error(codes.FailedPrecondition, "file upload is already confirmed")
	}
	row, err = s.files.ConfirmUpload(ctx, fileID, strings.ToLower(sha))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	row, err = s.scanConfirmedFile(ctx, row)
	if err != nil {
		return nil, err
	}
	if row.Status != "ready" {
		return &filev1.ConfirmUploadResponse{FileMetadata: fileRowToProto(row)}, nil
	}
	if row.FileType == "image" && s.processor != nil {
		processed, err := s.processor.ProcessImage(ctx, row)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		row, err = s.files.ApplyImageProcessing(ctx, fileID, processed.ConvertedR2Key, processed.ThumbnailR2Key, processed.Width, processed.Height)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	return &filev1.ConfirmUploadResponse{FileMetadata: fileRowToProto(row)}, nil
}

func (s *FileGRPC) scanConfirmedFile(ctx context.Context, row store.FileRow) (store.FileRow, error) {
	if !shouldScan(row.OriginalName, row.MimeType) {
		return row, nil
	}
	if s.scanner == nil || s.reader == nil {
		updated, err := s.files.ApplyScanResult(ctx, row.ID, "ready", "skipped")
		if err != nil {
			return store.FileRow{}, status.Error(codes.Internal, err.Error())
		}
		return updated, nil
	}
	bytes, err := s.reader.ReadObject(ctx, row.R2Key, row.SizeBytes)
	if err != nil {
		updated, uerr := s.files.ApplyScanResult(ctx, row.ID, "failed", "error")
		if uerr != nil {
			return store.FileRow{}, status.Error(codes.Internal, uerr.Error())
		}
		return updated, status.Error(codes.Internal, err.Error())
	}
	outcome, err := s.scanner.ScanBytes(ctx, bytes)
	if err != nil {
		updated, uerr := s.files.ApplyScanResult(ctx, row.ID, "failed", "error")
		if uerr != nil {
			return store.FileRow{}, status.Error(codes.Internal, uerr.Error())
		}
		return updated, status.Error(codes.Internal, err.Error())
	}
	statusValue := "ready"
	if outcome == "infected" || outcome == "error" {
		statusValue = "failed"
	}
	updated, err := s.files.ApplyScanResult(ctx, row.ID, statusValue, outcome)
	if err != nil {
		return store.FileRow{}, status.Error(codes.Internal, err.Error())
	}
	return updated, nil
}

func (s *FileGRPC) GetFileMetadata(ctx context.Context, req *filev1.GetFileMetadataRequest) (*filev1.GetFileMetadataResponse, error) {
	profileID, err := s.requireProfile(ctx)
	if err != nil {
		return nil, err
	}
	fileID, err := parseUUID("file_id", req.GetFileId())
	if err != nil {
		return nil, err
	}
	row, err := s.fileAccessibleByProfile(ctx, fileID, profileID)
	if err != nil {
		return nil, err
	}
	return &filev1.GetFileMetadataResponse{FileMetadata: fileRowToProto(row)}, nil
}

func (s *FileGRPC) GetBulkMetadata(ctx context.Context, req *filev1.GetBulkMetadataRequest) (*filev1.GetBulkMetadataResponse, error) {
	profileID, err := s.requireProfile(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(req.GetFileIds()))
	for _, raw := range req.GetFileIds() {
		id, err := parseUUID("file_ids", raw)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	rows, err := s.files.GetFilesByIDs(ctx, ids)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	out := map[string]*filev1.FileMetadata{}
	for _, id := range ids {
		row, ok := rows[id]
		if !ok || row.Status == "deleted" || s.ensureFileAccess(ctx, row, profileID) != nil {
			continue
		}
		out[id.String()] = fileRowToProto(row)
	}
	return &filev1.GetBulkMetadataResponse{
		BulkFileMetadata: &filev1.BulkFileMetadata{ByFileId: out},
	}, nil
}

func (s *FileGRPC) DeleteFile(ctx context.Context, req *filev1.DeleteFileRequest) (*filev1.DeleteFileResponse, error) {
	profileID, err := s.requireProfile(ctx)
	if err != nil {
		return nil, err
	}
	fileID, err := parseUUID("file_id", req.GetFileId())
	if err != nil {
		return nil, err
	}
	if _, err := s.fileOwnedByUploader(ctx, fileID, profileID); err != nil {
		return nil, err
	}
	if err := s.files.MarkDeleted(ctx, fileID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "file not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &filev1.DeleteFileResponse{}, nil
}

func (s *FileGRPC) ListFiles(ctx context.Context, req *filev1.ListFilesRequest) (*filev1.ListFilesResponse, error) {
	profileID, err := s.requireProfile(ctx)
	if err != nil {
		return nil, err
	}
	if req.GetFilterChat() != nil {
		return nil, status.Error(codes.FailedPrecondition, "chat-scoped file listing requires chat access guard")
	}
	pageSize := int32(50)
	if req.GetPage() != nil && req.GetPage().GetPageSize() > 0 {
		pageSize = req.GetPage().GetPageSize()
	}
	rows, err := s.files.ListFilesForProfile(ctx, profileID, pageSize)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	files := make([]*filev1.FileMetadata, 0, len(rows))
	for _, row := range rows {
		files = append(files, fileRowToProto(row))
	}
	return &filev1.ListFilesResponse{
		FileList: &filev1.FileList{
			Files: files,
			Page:  &commonv1.CursorPageResponse{},
		},
	}, nil
}

func (s *FileGRPC) CheckQuota(ctx context.Context, req *filev1.CheckQuotaRequest) (*filev1.CheckQuotaResponse, error) {
	profileID, err := s.requireProfile(ctx)
	if err != nil {
		return nil, err
	}
	if raw := strings.TrimSpace(req.GetProfileId()); raw != "" {
		requested, err := parseUUID("profile_id", raw)
		if err != nil {
			return nil, err
		}
		if requested != profileID {
			return nil, status.Error(codes.PermissionDenied, "cannot read quota for another profile")
		}
	}
	used, err := s.files.BytesUsedByProfile(ctx, profileID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &filev1.CheckQuotaResponse{
		QuotaResponse: &filev1.QuotaResponse{
			BytesUsed:  used,
			BytesLimit: r2file.MaxFreeFileBytes,
		},
	}, nil
}

func (s *FileGRPC) requireProfile(ctx context.Context) (uuid.UUID, error) {
	if s == nil || s.files == nil {
		return uuid.Nil, status.Error(codes.FailedPrecondition, "file persistence not configured")
	}
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return uuid.Nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	return profileID, nil
}

func (s *FileGRPC) chatContext(ctx context.Context, ref *chatv1.ChatRef, profileID uuid.UUID) (*uuid.UUID, *string, error) {
	if ref == nil || strings.TrimSpace(ref.GetId()) == "" {
		return nil, nil, nil
	}
	if s.chatGuard == nil {
		return nil, nil, status.Error(codes.FailedPrecondition, "chat access guard is not configured")
	}
	chatID, err := parseUUID("context_chat.id", ref.GetId())
	if err != nil {
		return nil, nil, err
	}
	if err := s.chatGuard.EnsureMember(ctx, chatID, profileID); err != nil {
		if errors.Is(err, ErrNotChatMember) {
			return nil, nil, status.Error(codes.PermissionDenied, "not a chat member")
		}
		return nil, nil, status.Error(codes.Internal, err.Error())
	}
	var typ string
	switch ref.GetType() {
	case chatv1.ChatType_CHAT_TYPE_DM:
		typ = "dm"
	case chatv1.ChatType_CHAT_TYPE_GROUP:
		typ = "group"
	case chatv1.ChatType_CHAT_TYPE_CHANNEL:
		typ = "channel"
	default:
		return nil, nil, status.Error(codes.InvalidArgument, "context_chat.type is required")
	}
	return &chatID, &typ, nil
}

func (s *FileGRPC) fileOwnedByUploader(ctx context.Context, fileID, profileID uuid.UUID) (store.FileRow, error) {
	row, err := s.files.GetFileByID(ctx, fileID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.FileRow{}, status.Error(codes.NotFound, "file not found")
		}
		return store.FileRow{}, status.Error(codes.Internal, err.Error())
	}
	if row.UploaderProfileID != profileID {
		return store.FileRow{}, status.Error(codes.PermissionDenied, "file is not owned by profile")
	}
	return row, nil
}

func (s *FileGRPC) fileAccessibleByProfile(ctx context.Context, fileID, profileID uuid.UUID) (store.FileRow, error) {
	row, err := s.files.GetFileByID(ctx, fileID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.FileRow{}, status.Error(codes.NotFound, "file not found")
		}
		return store.FileRow{}, status.Error(codes.Internal, err.Error())
	}
	if err := s.ensureFileAccess(ctx, row, profileID); err != nil {
		return store.FileRow{}, err
	}
	return row, nil
}

func (s *FileGRPC) ensureFileAccess(ctx context.Context, row store.FileRow, profileID uuid.UUID) error {
	if row.UploaderProfileID == profileID {
		return nil
	}
	if row.ChatID == nil {
		return status.Error(codes.PermissionDenied, "file is not owned by profile")
	}
	if s.chatGuard == nil {
		return status.Error(codes.FailedPrecondition, "chat access guard is not configured")
	}
	if err := s.chatGuard.EnsureMember(ctx, *row.ChatID, profileID); err != nil {
		if errors.Is(err, ErrNotChatMember) {
			return status.Error(codes.PermissionDenied, "not a chat member")
		}
		return status.Error(codes.Internal, err.Error())
	}
	return nil
}

func parseUUID(field, value string) (uuid.UUID, error) {
	id, err := uuid.Parse(strings.TrimSpace(value))
	if err != nil || id == uuid.Nil {
		return uuid.Nil, status.Errorf(codes.InvalidArgument, "invalid %s", field)
	}
	return id, nil
}

func uploadMaxBytes(ctx context.Context) int64 {
	if tier, ok := subscriptionTier(ctx); ok && tier == "premium" {
		return r2file.MaxPremiumFileBytes
	}
	return r2file.MaxFreeFileBytes
}

func subscriptionTier(ctx context.Context) (string, bool) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", false
	}
	vals := md.Get("x-voice-subscription-tier")
	if len(vals) == 0 {
		return "", false
	}
	tier := strings.TrimSpace(strings.ToLower(vals[0]))
	if tier == "" {
		return "", false
	}
	return tier, true
}

func shouldScan(originalName, mimeType string) bool {
	lowerName := strings.ToLower(strings.TrimSpace(originalName))
	lowerMIME := strings.ToLower(strings.TrimSpace(mimeType))
	return strings.HasSuffix(lowerName, ".exe") ||
		strings.HasSuffix(lowerName, ".zip") ||
		strings.HasSuffix(lowerName, ".bat") ||
		lowerMIME == "application/zip" ||
		lowerMIME == "application/x-msdownload"
}

func fileRowToProto(row store.FileRow) *filev1.FileMetadata {
	meta := &filev1.FileMetadata{
		Id:                row.ID.String(),
		UploaderProfileId: row.UploaderProfileID.String(),
		OriginalName:      row.OriginalName,
		MimeType:          row.MimeType,
		SizeBytes:         row.SizeBytes,
		R2Key:             row.R2Key,
		Status:            row.Status,
		FileType:          row.FileType,
		IsE2E:             row.IsE2E,
		ScanResult:        row.ScanResult,
		CreatedAt:         timestamppb.New(row.CreatedAt),
	}
	if row.SHA256Hash != nil {
		meta.Sha256Hash = *row.SHA256Hash
	}
	if row.ExpiresAt != nil {
		meta.ExpiresAt = timestamppb.New(*row.ExpiresAt)
	}
	if row.Width != nil {
		meta.Width = row.Width
	}
	if row.Height != nil {
		meta.Height = row.Height
	}
	if row.DurationSeconds != nil {
		meta.DurationSeconds = row.DurationSeconds
	}
	if row.ThumbnailR2Key != nil {
		meta.ThumbnailR2Key = row.ThumbnailR2Key
	}
	if row.ConvertedR2Key != nil {
		meta.ConvertedR2Key = row.ConvertedR2Key
	}
	if row.ChatID != nil {
		meta.Chat = chatRef(row.ChatID.String(), row.ChatType)
	}
	statusEnum := lifecycleStatus(row.Status)
	fileTypeEnum := mediaCategory(row.FileType)
	scanEnum := scanOutcome(row.ScanResult)
	meta.StatusEnum = &statusEnum
	meta.FileTypeEnum = &fileTypeEnum
	meta.ScanResultEnum = &scanEnum
	return meta
}

func chatRef(id string, typ *string) *chatv1.ChatRef {
	ref := &chatv1.ChatRef{Id: id}
	if typ == nil {
		return ref
	}
	switch *typ {
	case "dm":
		t := chatv1.ChatType_CHAT_TYPE_DM
		ref.Type = &t
	case "group":
		t := chatv1.ChatType_CHAT_TYPE_GROUP
		ref.Type = &t
	case "channel":
		t := chatv1.ChatType_CHAT_TYPE_CHANNEL
		ref.Type = &t
	}
	return ref
}

func lifecycleStatus(status string) filev1.FileLifecycleStatus {
	switch status {
	case "pending_upload":
		return filev1.FileLifecycleStatus_FILE_LIFECYCLE_STATUS_PENDING_UPLOAD
	case "processing":
		return filev1.FileLifecycleStatus_FILE_LIFECYCLE_STATUS_PROCESSING
	case "ready":
		return filev1.FileLifecycleStatus_FILE_LIFECYCLE_STATUS_READY
	case "failed":
		return filev1.FileLifecycleStatus_FILE_LIFECYCLE_STATUS_FAILED
	case "deleted", "expired":
		return filev1.FileLifecycleStatus_FILE_LIFECYCLE_STATUS_DELETED
	default:
		return filev1.FileLifecycleStatus_FILE_LIFECYCLE_STATUS_UNSPECIFIED
	}
}

func mediaCategory(kind string) filev1.FileMediaCategory {
	switch kind {
	case "image":
		return filev1.FileMediaCategory_FILE_MEDIA_CATEGORY_IMAGE
	case "video":
		return filev1.FileMediaCategory_FILE_MEDIA_CATEGORY_VIDEO
	case "audio":
		return filev1.FileMediaCategory_FILE_MEDIA_CATEGORY_AUDIO
	case "document":
		return filev1.FileMediaCategory_FILE_MEDIA_CATEGORY_DOCUMENT
	case "other":
		return filev1.FileMediaCategory_FILE_MEDIA_CATEGORY_OTHER
	default:
		return filev1.FileMediaCategory_FILE_MEDIA_CATEGORY_UNSPECIFIED
	}
}

func scanOutcome(outcome string) filev1.FileScanOutcome {
	switch outcome {
	case "pending":
		return filev1.FileScanOutcome_FILE_SCAN_OUTCOME_PENDING
	case "clean":
		return filev1.FileScanOutcome_FILE_SCAN_OUTCOME_CLEAN
	case "infected":
		return filev1.FileScanOutcome_FILE_SCAN_OUTCOME_INFECTED
	case "error":
		return filev1.FileScanOutcome_FILE_SCAN_OUTCOME_ERROR
	case "skipped":
		return filev1.FileScanOutcome_FILE_SCAN_OUTCOME_SKIPPED
	default:
		return filev1.FileScanOutcome_FILE_SCAN_OUTCOME_UNSPECIFIED
	}
}
