package grpcsvc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/file/internal/authctx"
	"voice/backend/file/internal/fileevents"
	"voice/backend/file/internal/jobs"
	"voice/backend/file/internal/r2file"
	"voice/backend/file/internal/store"

	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	filev1 "voice.app/voice/file/v1"
	storyv1 "voice.app/voice/story/v1"
)

var sha256Re = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)

const defaultRetentionDays = 90

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
	Files                    *store.FilesStore
	Presigner                r2file.Presigner
	Deleter                  r2file.ObjectDeleter
	Clock                    Clock
	ChatGuard                ChatGuard
	Processor                ImageProcessor
	Reader                   ObjectReader
	Scanner                  Scanner
	Events                   fileevents.Publisher
	ReferenceAuthorityActive bool
}

type FileGRPC struct {
	filev1.UnimplementedFileServiceServer
	files                    *store.FilesStore
	presigner                r2file.Presigner
	deleter                  r2file.ObjectDeleter
	clock                    Clock
	chatGuard                ChatGuard
	processor                ImageProcessor
	reader                   ObjectReader
	scanner                  Scanner
	events                   fileevents.Publisher
	referenceAuthorityActive bool
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
	events := deps.Events
	if events == nil {
		events = fileevents.NoopPublisher{}
	}
	return &FileGRPC{
		files:                    deps.Files,
		presigner:                deps.Presigner,
		deleter:                  deps.Deleter,
		clock:                    clock,
		chatGuard:                deps.ChatGuard,
		processor:                processor,
		reader:                   deps.Reader,
		scanner:                  deps.Scanner,
		events:                   events,
		referenceAuthorityActive: deps.ReferenceAuthorityActive,
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
	storyID, err := s.storyContext(req.GetContextStory())
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
	expiresAt := retentionExpiresAt(s.clock, ctx, isE2E)
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
		StoryID:           storyID,
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
	var row store.FileRow
	if req.GetAccess() != nil {
		row, err = s.fileAccessibleBySelector(ctx, fileID, profileID, req.GetAccess(), filev1.FileReadSurface_FILE_READ_SURFACE_URL)
	} else {
		row, err = s.fileAccessibleByProfile(ctx, fileID, profileID)
	}
	if err != nil {
		return nil, err
	}
	if row.Status != "ready" {
		return nil, status.Error(codes.FailedPrecondition, "file is not ready")
	}
	ttl := r2file.DefaultURLTTL
	getURL, err := s.presigner.PresignGet(ctx, r2file.GetPresignInput{Key: downloadKey(row), TTL: ttl})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if err := s.events.PublishFileDownloaded(ctx, row.ID.String(), profileID.String()); err != nil {
		slog.Default().WarnContext(ctx, "file.downloaded publish failed",
			slog.String("file_id", row.ID.String()),
			slog.String("downloader_profile_id", profileID.String()),
			slog.String("error", err.Error()),
		)
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
	uploaded, err := s.readUploadBytes(ctx, row)
	if err != nil {
		return nil, err
	}
	if err := verifySHA256(uploaded, sha); err != nil {
		return nil, err
	}
	row, err = s.files.ConfirmUpload(ctx, fileID, strings.ToLower(sha))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	row, err = s.scanConfirmedFile(ctx, row, uploaded)
	if err != nil {
		return nil, err
	}
	if row.ScanResult == "infected" {
		_ = s.events.PublishFileScanInfected(ctx, row.ID.String(), row.UploaderProfileID.String())
		return &filev1.ConfirmUploadResponse{FileMetadata: fileRowToProto(row)}, nil
	}
	if row.Status != "ready" {
		return &filev1.ConfirmUploadResponse{FileMetadata: fileRowToProto(row)}, nil
	}
	_ = s.events.PublishFileUploaded(ctx, row.ID.String(), row.UploaderProfileID.String())
	originalKey := row.R2Key
	if row.FileType == "image" && !row.IsE2E && s.processor != nil {
		processed, err := s.processor.ProcessImage(ctx, row)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		row, err = s.files.ApplyImageProcessing(ctx, fileID, processed.ConvertedR2Key, processed.ThumbnailR2Key, processed.Width, processed.Height)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		_ = s.events.PublishFileProcessed(ctx, row.ID.String(), row.Status, processed.ConvertedR2Key, processed.ThumbnailR2Key)
		if s.deleter != nil {
			if err := s.deleter.DeleteObject(ctx, originalKey); err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
		}
	}
	return &filev1.ConfirmUploadResponse{FileMetadata: fileRowToProto(row)}, nil
}

func (s *FileGRPC) scanConfirmedFile(ctx context.Context, row store.FileRow, uploaded []byte) (store.FileRow, error) {
	if row.IsE2E {
		updated, err := s.files.ApplyScanResult(ctx, row.ID, "ready", "skipped")
		if err != nil {
			return store.FileRow{}, status.Error(codes.Internal, err.Error())
		}
		return updated, nil
	}
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
	bytes := uploaded
	if bytes == nil {
		var readErr error
		bytes, readErr = s.reader.ReadObject(ctx, row.R2Key, row.SizeBytes)
		if readErr != nil {
			updated, uerr := s.files.ApplyScanResult(ctx, row.ID, "failed", "error")
			if uerr != nil {
				return store.FileRow{}, status.Error(codes.Internal, uerr.Error())
			}
			return updated, status.Error(codes.Internal, readErr.Error())
		}
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
	var row store.FileRow
	if req.GetAccess() != nil {
		row, err = s.fileAccessibleBySelector(ctx, fileID, profileID, req.GetAccess(), filev1.FileReadSurface_FILE_READ_SURFACE_METADATA)
	} else {
		row, err = s.fileAccessibleByProfile(ctx, fileID, profileID)
	}
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
	if len(req.GetItems()) > 0 {
		items, prepareErr := prepareBulkMetadataItems(req.GetItems())
		if prepareErr != nil {
			return nil, prepareErr
		}
		tx, beginErr := s.files.Pool.Begin(ctx)
		if beginErr != nil {
			return nil, status.Error(codes.Internal, beginErr.Error())
		}
		defer func() { _ = tx.Rollback(ctx) }()
		out := make(map[string]*filev1.FileMetadata, len(items))
		for _, item := range items {
			row, accessErr := s.fileAccessibleBySelectorTx(ctx, tx, item.fileID, profileID, item.selector, filev1.FileReadSurface_FILE_READ_SURFACE_METADATA)
			if accessErr != nil {
				return nil, accessErr
			}
			out[item.fileID.String()] = fileRowToProto(row)
		}
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return nil, status.Error(codes.Internal, commitErr.Error())
		}
		return &filev1.GetBulkMetadataResponse{BulkFileMetadata: &filev1.BulkFileMetadata{ByFileId: out}}, nil
	}
	legacyFileIDs := req.GetFileIds() //nolint:staticcheck // R23 compatibility: accept deprecated file_ids until callers migrate to access-scoped items.
	ids := make([]uuid.UUID, 0, len(legacyFileIDs))
	for _, raw := range legacyFileIDs {
		id, err := parseUUID("file_ids", raw)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if s.referenceAuthorityActive {
		lockedIDs := append([]uuid.UUID(nil), ids...)
		sort.Slice(lockedIDs, func(i, j int) bool { return lockedIDs[i].String() < lockedIDs[j].String() })
		tx, beginErr := s.files.Pool.Begin(ctx)
		if beginErr != nil {
			return nil, status.Error(codes.Internal, beginErr.Error())
		}
		defer func() { _ = tx.Rollback(ctx) }()
		out := make(map[string]*filev1.FileMetadata, len(lockedIDs))
		for _, id := range lockedIDs {
			row, accessErr := s.fileAccessibleByLegacyTx(ctx, tx, id, profileID)
			if accessErr != nil {
				return nil, accessErr
			}
			out[id.String()] = fileRowToProto(row)
		}
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return nil, status.Error(codes.Internal, commitErr.Error())
		}
		return &filev1.GetBulkMetadataResponse{BulkFileMetadata: &filev1.BulkFileMetadata{ByFileId: out}}, nil
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

type preparedBulkMetadataItem struct {
	fileID        uuid.UUID
	selector      *filev1.FileAccessSelector
	selectorKey   []byte
	originalIndex int
}

func prepareBulkMetadataItems(items []*filev1.FileAccessItem) ([]preparedBulkMetadataItem, error) {
	prepared := make([]preparedBulkMetadataItem, 0, len(items))
	for index, item := range items {
		fileID, err := parseUUID("items.file_id", item.GetFileId())
		if err != nil {
			return nil, err
		}
		selector := item.GetAccess()
		if selector == nil {
			return nil, status.Error(codes.PermissionDenied, "exact access selector required")
		}
		switch selected := selector.GetSelector().(type) {
		case *filev1.FileAccessSelector_Reference:
			ids, parseErr := parseReference(selected.Reference)
			if parseErr != nil || ids.file != fileID {
				return nil, status.Error(codes.PermissionDenied, "reference does not match file")
			}
		case *filev1.FileAccessSelector_CapabilityId:
			if _, parseErr := parseUUID("capability_id", selected.CapabilityId); parseErr != nil {
				return nil, status.Error(codes.PermissionDenied, "invalid capability")
			}
		default:
			return nil, status.Error(codes.PermissionDenied, "exact access selector required")
		}
		selectorKey, marshalErr := proto.MarshalOptions{Deterministic: true}.Marshal(selector)
		if marshalErr != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid access selector")
		}
		prepared = append(prepared, preparedBulkMetadataItem{fileID: fileID, selector: selector, selectorKey: selectorKey, originalIndex: index})
	}
	sort.Slice(prepared, func(i, j int) bool {
		if comparison := bytes.Compare(prepared[i].fileID[:], prepared[j].fileID[:]); comparison != 0 {
			return comparison < 0
		}
		if comparison := bytes.Compare(prepared[i].selectorKey, prepared[j].selectorKey); comparison != 0 {
			return comparison < 0
		}
		return prepared[i].originalIndex < prepared[j].originalIndex
	})
	return prepared, nil
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
	row, err := s.files.GetFileByID(ctx, fileID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "file not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	if s.deleter != nil {
		if err := r2file.DeleteKeys(ctx, s.deleter, jobs.FileStorageKeys(row)...); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
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
		if s.chatGuard == nil {
			return nil, status.Error(codes.FailedPrecondition, "chat access guard is not configured")
		}
		chatID, err := parseUUID("filter_chat.id", req.GetFilterChat().GetId())
		if err != nil {
			return nil, err
		}
		if err := s.chatGuard.EnsureMember(ctx, chatID, profileID); err != nil {
			if errors.Is(err, ErrNotChatMember) {
				return nil, status.Error(codes.PermissionDenied, "not a chat member")
			}
			return nil, status.Error(codes.Internal, err.Error())
		}
		pageSize := int32(50)
		if req.GetPage() != nil && req.GetPage().GetPageSize() > 0 {
			pageSize = req.GetPage().GetPageSize()
		}
		rows, err := s.files.ListFilesForChat(ctx, chatID, pageSize)
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
			BytesLimit: quotaMaxBytes(ctx),
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

func (s *FileGRPC) storyContext(ref *storyv1.StoryRef) (*uuid.UUID, error) {
	if ref == nil || strings.TrimSpace(ref.GetStoryId()) == "" {
		return nil, nil
	}
	storyID, err := parseUUID("context_story.story_id", ref.GetStoryId())
	if err != nil {
		return nil, err
	}
	return &storyID, nil
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
	if s.referenceAuthorityActive {
		tx, err := s.files.Pool.Begin(ctx)
		if err != nil {
			return store.FileRow{}, status.Error(codes.Internal, err.Error())
		}
		defer func() { _ = tx.Rollback(ctx) }()
		row, err := s.fileAccessibleByLegacyTx(ctx, tx, fileID, profileID)
		if err != nil {
			return store.FileRow{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return store.FileRow{}, status.Error(codes.Internal, err.Error())
		}
		return row, nil
	}
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

func (s *FileGRPC) fileAccessibleByLegacyTx(ctx context.Context, tx pgx.Tx, fileID, profileID uuid.UUID) (store.FileRow, error) {
	rows, err := tx.Query(ctx, `SELECT owner_type,owner_id,subresource_id,scope_space_id FROM file_references WHERE file_id=$1 AND released_at IS NULL ORDER BY owner_type,owner_id,subresource_id NULLS FIRST,scope_space_id NULLS FIRST LIMIT 2`, fileID)
	if err != nil {
		return store.FileRow{}, status.Error(codes.Internal, err.Error())
	}
	var refs []*filev1.FileReferenceKey
	for rows.Next() {
		var ownerType int32
		var owner uuid.UUID
		var subresource, scope *uuid.UUID
		if err := rows.Scan(&ownerType, &owner, &subresource, &scope); err != nil {
			rows.Close()
			return store.FileRow{}, status.Error(codes.Internal, err.Error())
		}
		ref := &filev1.FileReferenceKey{FileId: fileID.String(), OwnerType: filev1.FileReferenceOwnerType(ownerType), OwnerId: owner.String()}
		if subresource != nil {
			value := subresource.String()
			ref.SubresourceId = &value
		}
		if scope != nil {
			value := scope.String()
			ref.ScopeSpaceId = &value
		}
		refs = append(refs, ref)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return store.FileRow{}, status.Error(codes.Internal, err.Error())
	}
	if len(refs) != 1 {
		return store.FileRow{}, status.Error(codes.PermissionDenied, "legacy access requires exactly one live reference")
	}
	row, err := s.authorizeExactReferenceTx(ctx, tx, fileID, profileID, refs[0], false)
	if err != nil {
		return store.FileRow{}, err
	}
	var lockedCardinality int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM file_references WHERE file_id=$1 AND released_at IS NULL`, fileID).Scan(&lockedCardinality); err != nil {
		return store.FileRow{}, status.Error(codes.Internal, err.Error())
	}
	if lockedCardinality != 1 {
		return store.FileRow{}, status.Error(codes.PermissionDenied, "legacy access requires exactly one live reference after locking")
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
	return quotaMaxBytes(ctx)
}

func quotaMaxBytes(ctx context.Context) int64 {
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

func retentionDuration() time.Duration {
	if raw := strings.TrimSpace(os.Getenv("FILE_RETENTION_DEV")); raw != "" {
		if secs, err := strconv.Atoi(raw); err == nil && secs > 0 {
			return time.Duration(secs) * time.Second
		}
	}
	return defaultRetentionDays * 24 * time.Hour
}

func retentionExpiresAt(clock Clock, ctx context.Context, isE2E bool) *time.Time {
	if !isE2E {
		if tier, ok := subscriptionTier(ctx); ok && tier == "premium" {
			return nil
		}
	}
	t := clock.Now().UTC().Add(retentionDuration())
	return &t
}

func downloadKey(row store.FileRow) string {
	if row.ConvertedR2Key != nil {
		if key := strings.TrimSpace(*row.ConvertedR2Key); key != "" {
			return key
		}
	}
	return row.R2Key
}

func (s *FileGRPC) readUploadBytes(ctx context.Context, row store.FileRow) ([]byte, error) {
	if s.reader == nil {
		return nil, status.Error(codes.FailedPrecondition, "upload verification not configured")
	}
	bytes, err := s.reader.ReadObject(ctx, row.R2Key, row.SizeBytes)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return bytes, nil
}

func verifySHA256(data []byte, claimed string) error {
	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:])
	if !strings.EqualFold(got, strings.TrimSpace(claimed)) {
		return status.Error(codes.FailedPrecondition, "sha256_hash mismatch")
	}
	return nil
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
	case "deleted":
		return filev1.FileLifecycleStatus_FILE_LIFECYCLE_STATUS_DELETED
	case "expired":
		return filev1.FileLifecycleStatus_FILE_LIFECYCLE_STATUS_EXPIRED
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
