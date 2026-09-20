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

// ListQuickAccess returns the caller's quick-access slots with hydrated chats.
func (s *ChatGRPC) ListQuickAccess(ctx context.Context, _ *chatv1.ListQuickAccessRequest) (*chatv1.ListQuickAccessResponse, error) {
	if s == nil || s.DM == nil {
		return nil, status.Error(codes.FailedPrecondition, "chat persistence not configured")
	}
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	rows, err := s.DM.ListQuickAccess(ctx, profileID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	chatIDs := make([]uuid.UUID, 0, len(rows))
	chatRows := make(map[uuid.UUID]*store.ChatRow, len(rows))
	for _, row := range rows {
		chatRow, err := s.DM.FindChatByID(ctx, row.ChatID)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		if chatRow == nil {
			// A stored shortcut cannot be safely disclosed when its target cannot
			// be classified for the deleted-peer lifecycle gate.
			return nil, status.Error(codes.Unavailable, "quick access availability unavailable")
		}
		chatIDs = append(chatIDs, row.ChatID)
		chatRows[row.ChatID] = chatRow
	}
	peers, err := s.DM.DMPeerProfileIDs(ctx, profileID, chatIDs)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "quick access availability unavailable")
	}
	classified := make([]*store.ChatRow, 0, len(rows))
	for _, row := range rows {
		classified = append(classified, chatRows[row.ChatID])
	}
	visibleRows, err := s.filterQuickAccessDeletedPeerDMs(ctx, classified, peers)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "quick access availability unavailable")
	}
	visible := make(map[uuid.UUID]struct{}, len(visibleRows))
	for _, row := range visibleRows {
		visible[row.ID] = struct{}{}
	}
	items := make([]*chatv1.QuickAccessItem, 0, len(visibleRows))
	for _, row := range rows {
		if _, ok := visible[row.ChatID]; !ok {
			continue
		}
		items = append(items, &chatv1.QuickAccessItem{
			ChatId:    row.ChatID.String(),
			SortOrder: row.SortOrder,
			Chat:      chatRowToProto(chatRows[row.ChatID]),
		})
	}
	return &chatv1.ListQuickAccessResponse{Items: items}, nil
}

// AddQuickAccess adds a chat to the caller's quick-access list.
func (s *ChatGRPC) AddQuickAccess(ctx context.Context, req *chatv1.AddQuickAccessRequest) (*chatv1.AddQuickAccessResponse, error) {
	if s == nil || s.DM == nil {
		return nil, status.Error(codes.FailedPrecondition, "chat persistence not configured")
	}
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	chatID, err := parseUUIDField("chat_id", req.GetChatId())
	if err != nil {
		return nil, err
	}
	var sortOrder *int32
	if req.SortOrder != nil {
		sortOrder = req.SortOrder
	}
	if err := s.DM.AddQuickAccess(ctx, profileID, chatID, sortOrder); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "chat not found")
		}
		if errors.Is(err, store.ErrQuickAccessLimit) {
			return nil, status.Error(codes.FailedPrecondition, "quick access limit reached")
		}
		if strings.Contains(err.Error(), "archived") {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &chatv1.AddQuickAccessResponse{}, nil
}

// RemoveQuickAccess removes a chat from the caller's quick-access list.
func (s *ChatGRPC) RemoveQuickAccess(ctx context.Context, req *chatv1.RemoveQuickAccessRequest) (*chatv1.RemoveQuickAccessResponse, error) {
	if s == nil || s.DM == nil {
		return nil, status.Error(codes.FailedPrecondition, "chat persistence not configured")
	}
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	chatID, err := parseUUIDField("chat_id", req.GetChatId())
	if err != nil {
		return nil, err
	}
	if err := s.DM.RemoveQuickAccess(ctx, profileID, chatID); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &chatv1.RemoveQuickAccessResponse{}, nil
}

// ReorderQuickAccess sets the full ordered quick-access list for the caller.
func (s *ChatGRPC) ReorderQuickAccess(ctx context.Context, req *chatv1.ReorderQuickAccessRequest) (*chatv1.ReorderQuickAccessResponse, error) {
	if s == nil || s.DM == nil {
		return nil, status.Error(codes.FailedPrecondition, "chat persistence not configured")
	}
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	chatIDs := make([]uuid.UUID, 0, len(req.GetChatIds()))
	for _, raw := range req.GetChatIds() {
		id, err := parseUUIDField("chat_id", raw)
		if err != nil {
			return nil, err
		}
		chatIDs = append(chatIDs, id)
	}
	if err := s.DM.ReorderQuickAccess(ctx, profileID, chatIDs); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "quick access entry not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &chatv1.ReorderQuickAccessResponse{}, nil
}
