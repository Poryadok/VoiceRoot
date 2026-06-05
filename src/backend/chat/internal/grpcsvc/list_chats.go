package grpcsvc

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"voice/backend/chat/internal/authctx"
	"voice/backend/chat/internal/store"

	chatv1 "voice.app/voice/chat/v1"
)

// ListChatsEnrichment is optional S2S to Messaging: last message preview and unread counts for list rows.
type ListChatsEnrichment interface {
	EnrichListChats(ctx context.Context, viewerProfileID uuid.UUID, chatIDs []uuid.UUID) (map[uuid.UUID]ListChatExtra, error)
}

// ListChatExtra is per-chat list metadata from Messaging (or a future denormalized path).
type ListChatExtra struct {
	LastMessagePreview string
	UnreadCount        int64
}

func (s *ChatGRPC) ListChats(ctx context.Context, req *chatv1.ListChatsRequest) (*chatv1.ListChatsResponse, error) {
	if s == nil || s.DM == nil {
		return nil, status.Error(codes.FailedPrecondition, "chat persistence not configured")
	}
	caller, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}

	limit := 50
	cursor := ""
	if req != nil {
		if p := req.GetPage(); p != nil {
			cursor = p.GetCursor()
			if ps := int(p.GetPageSize()); ps > 0 {
				limit = ps
			}
		}
	}
	if limit > 100 {
		limit = 100
	}

	inbox := strings.TrimSpace(req.GetInbox())
	if inbox == "" {
		inbox = "main"
	}
	if inbox != "main" && inbox != "requests" {
		return nil, status.Error(codes.InvalidArgument, "invalid inbox")
	}
	page, err := s.DM.ListChatsPage(ctx, caller, cursor, limit, inbox)
	if err != nil {
		if errors.Is(err, store.ErrInvalidListCursor) {
			return nil, status.Error(codes.InvalidArgument, "invalid page cursor")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	ids := make([]uuid.UUID, 0, len(page.Rows))
	for _, row := range page.Rows {
		ids = append(ids, row.ID)
	}

	extras := map[uuid.UUID]ListChatExtra{}
	if s.ListEnrich != nil && len(ids) > 0 {
		extras, err = s.ListEnrich.EnrichListChats(ctx, caller, ids)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	peers := map[uuid.UUID]uuid.UUID{}
	if len(ids) > 0 {
		peers, err = s.DM.DMPeerProfileIDs(ctx, caller, ids)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	items := make([]*chatv1.ChatListItem, 0, len(page.Rows))
	for _, row := range page.Rows {
		item := &chatv1.ChatListItem{
			Chat:        chatRowToProto(row),
			UnreadCount: 0,
		}
		item.Inbox = proto.String(row.InboxBucket)
		item.IsStranger = proto.Bool(row.InboxBucket == "requests")
		if peerID, ok := peers[row.ID]; ok {
			item.DmPeerProfileId = proto.String(peerID.String())
		}
		if x, ok := extras[row.ID]; ok {
			item.UnreadCount = x.UnreadCount
			if x.LastMessagePreview != "" {
				item.LastMessagePreview = proto.String(x.LastMessagePreview)
			}
		}
		items = append(items, item)
	}

	return &chatv1.ListChatsResponse{
		ChatList: &chatv1.ChatList{
			Items:      items,
			NextCursor: page.NextCursor,
		},
	}, nil
}
