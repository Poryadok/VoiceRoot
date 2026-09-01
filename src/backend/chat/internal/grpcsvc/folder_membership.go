package grpcsvc

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/chat/internal/authctx"
	"voice/backend/chat/internal/store"

	chatv1 "voice.app/voice/chat/v1"
)

func (s *ChatGRPC) AddChatToFolder(ctx context.Context, req *chatv1.AddChatToFolderRequest) (*chatv1.AddChatToFolderResponse, error) {
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
	chatID, err := parseUUIDField("chat_id", req.GetChatId())
	if err != nil {
		return nil, err
	}
	var sortOrder *int32
	if req.SortOrder != nil {
		sortOrder = req.SortOrder
	}
	if err := s.DM.AddChatToFolder(ctx, profileID, folderID, chatID, sortOrder); err != nil {
		return nil, folderMembershipGRPCError(err)
	}
	return &chatv1.AddChatToFolderResponse{}, nil
}

func (s *ChatGRPC) RemoveChatFromFolder(ctx context.Context, req *chatv1.RemoveChatFromFolderRequest) (*chatv1.RemoveChatFromFolderResponse, error) {
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
	chatID, err := parseUUIDField("chat_id", req.GetChatId())
	if err != nil {
		return nil, err
	}
	if err := s.DM.RemoveChatFromFolder(ctx, profileID, folderID, chatID); err != nil {
		return nil, folderMembershipGRPCError(err)
	}
	return &chatv1.RemoveChatFromFolderResponse{}, nil
}

func (s *ChatGRPC) ReorderFolderChats(ctx context.Context, req *chatv1.ReorderFolderChatsRequest) (*chatv1.ReorderFolderChatsResponse, error) {
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
	chatIDs := make([]uuid.UUID, 0, len(req.GetChatIds()))
	for _, raw := range req.GetChatIds() {
		id, parseErr := parseUUIDField("chat_id", raw)
		if parseErr != nil {
			return nil, parseErr
		}
		chatIDs = append(chatIDs, id)
	}
	if err := s.DM.ReorderFolderChats(ctx, profileID, folderID, chatIDs); err != nil {
		return nil, folderMembershipGRPCError(err)
	}
	return &chatv1.ReorderFolderChatsResponse{}, nil
}

func (s *ChatGRPC) PinChatInFolder(ctx context.Context, req *chatv1.PinChatInFolderRequest) (*chatv1.PinChatInFolderResponse, error) {
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
	chatID, err := parseUUIDField("chat_id", req.GetChatId())
	if err != nil {
		return nil, err
	}
	var pinOrder *int32
	if req.PinOrder != nil {
		pinOrder = req.PinOrder
	}
	if err := s.DM.PinChatInFolder(ctx, profileID, folderID, chatID, pinOrder); err != nil {
		return nil, folderMembershipGRPCError(err)
	}
	return &chatv1.PinChatInFolderResponse{}, nil
}

func (s *ChatGRPC) UnpinChatInFolder(ctx context.Context, req *chatv1.UnpinChatInFolderRequest) (*chatv1.UnpinChatInFolderResponse, error) {
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
	chatID, err := parseUUIDField("chat_id", req.GetChatId())
	if err != nil {
		return nil, err
	}
	if err := s.DM.UnpinChatInFolder(ctx, profileID, folderID, chatID); err != nil {
		return nil, folderMembershipGRPCError(err)
	}
	return &chatv1.UnpinChatInFolderResponse{}, nil
}

func folderMembershipGRPCError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, store.ErrFolderNotFound) {
		return status.Error(codes.NotFound, "folder not found")
	}
	if errors.Is(err, store.ErrSystemFolderMembership) {
		return status.Error(codes.FailedPrecondition, err.Error())
	}
	if errors.Is(err, store.ErrFolderChatNotMember) {
		return status.Error(codes.NotFound, err.Error())
	}
	if errors.Is(err, store.ErrFolderChatPredicate) {
		return status.Error(codes.FailedPrecondition, err.Error())
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return status.Error(codes.NotFound, "chat not found")
	}
	if strings.Contains(err.Error(), "archived") {
		return status.Error(codes.FailedPrecondition, err.Error())
	}
	return status.Error(codes.Internal, err.Error())
}
