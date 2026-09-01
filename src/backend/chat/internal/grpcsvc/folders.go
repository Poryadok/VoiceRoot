package grpcsvc

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/chat/internal/authctx"
	"voice/backend/chat/internal/store"

	chatv1 "voice.app/voice/chat/v1"
)

// ListFolders returns system + custom folders for the caller, seeding system folders on first access.
func (s *ChatGRPC) ListFolders(ctx context.Context, _ *chatv1.ListFoldersRequest) (*chatv1.ListFoldersResponse, error) {
	if s == nil || s.DM == nil {
		return nil, status.Error(codes.FailedPrecondition, "chat persistence not configured")
	}
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	rows, err := s.DM.ListFolders(ctx, profileID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	folders := make([]*chatv1.Folder, 0, len(rows))
	for _, row := range rows {
		folders = append(folders, folderRowToProto(row))
	}
	return &chatv1.ListFoldersResponse{
		FolderList: &chatv1.FolderList{Folders: folders},
	}, nil
}

// CreateFolder creates a custom folder for the caller.
func (s *ChatGRPC) CreateFolder(ctx context.Context, req *chatv1.CreateFolderRequest) (*chatv1.CreateFolderResponse, error) {
	if s == nil || s.DM == nil {
		return nil, status.Error(codes.FailedPrecondition, "chat persistence not configured")
	}
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	row, err := s.DM.CreateFolder(ctx, profileID, name, req.GetFilterConfigJson())
	if err != nil {
		if strings.Contains(err.Error(), "folder name is required") {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &chatv1.CreateFolderResponse{Folder: folderRowToProto(*row)}, nil
}

// UpdateFolder renames or reorders a custom folder (system folders are immutable).
func (s *ChatGRPC) UpdateFolder(ctx context.Context, req *chatv1.UpdateFolderRequest) (*chatv1.UpdateFolderResponse, error) {
	if s == nil || s.DM == nil {
		return nil, status.Error(codes.FailedPrecondition, "chat persistence not configured")
	}
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	folderID, err := parseUUIDField("folder_id", req.GetFolderId())
	if err != nil {
		return nil, err
	}
	upd := store.FolderUpdate{}
	if req.Name != nil {
		name := strings.TrimSpace(req.GetName())
		upd.Name = &name
	}
	if req.FilterConfigJson != nil {
		filter := req.GetFilterConfigJson()
		upd.FilterConfigJSON = &filter
	}
	if req.SortOrder != nil {
		sortOrder := req.GetSortOrder()
		upd.SortOrder = &sortOrder
	}
	row, err := s.DM.UpdateFolder(ctx, profileID, folderID, upd)
	if err != nil {
		return nil, folderCRUDGRPCError(err)
	}
	return &chatv1.UpdateFolderResponse{Folder: folderRowToProto(*row)}, nil
}

// DeleteFolder removes a custom folder owned by the caller.
func (s *ChatGRPC) DeleteFolder(ctx context.Context, req *chatv1.DeleteFolderRequest) (*chatv1.DeleteFolderResponse, error) {
	if s == nil || s.DM == nil {
		return nil, status.Error(codes.FailedPrecondition, "chat persistence not configured")
	}
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	folderID, err := parseUUIDField("folder_id", req.GetFolderId())
	if err != nil {
		return nil, err
	}
	if err := s.DM.DeleteFolder(ctx, profileID, folderID); err != nil {
		return nil, folderCRUDGRPCError(err)
	}
	return &chatv1.DeleteFolderResponse{}, nil
}

func folderCRUDGRPCError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, store.ErrFolderNotFound) {
		return status.Error(codes.NotFound, "folder not found")
	}
	if errors.Is(err, store.ErrSystemFolderImmutable) {
		return status.Error(codes.FailedPrecondition, err.Error())
	}
	if errors.Is(err, store.ErrFolderNoUpdates) {
		return status.Error(codes.InvalidArgument, err.Error())
	}
	if strings.Contains(err.Error(), "folder name is required") {
		return status.Error(codes.InvalidArgument, err.Error())
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return status.Error(codes.NotFound, "folder not found")
	}
	return status.Error(codes.Internal, err.Error())
}

func folderRowToProto(row store.FolderRow) *chatv1.Folder {
	return &chatv1.Folder{
		Id:               row.ID.String(),
		Name:             row.Name,
		FolderType:       row.FolderType,
		FilterConfigJson: row.FilterConfigJSON,
		SortOrder:        row.SortOrder,
	}
}
