package grpcsvc

import (
	"context"
	"errors"
	"log"
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
	if inbox != "main" && inbox != "requests" && inbox != "archive" {
		return nil, status.Error(codes.InvalidArgument, "invalid inbox")
	}

	folderRaw := strings.TrimSpace(req.GetFolderId())
	var folderID *uuid.UUID
	if folderRaw != "" && inbox != "archive" {
		id, parseErr := uuid.Parse(folderRaw)
		if parseErr != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid folder_id")
		}
		folderID = &id
	}

	var spaceIDs []uuid.UUID
	if (inbox == "main" || folderID != nil) && s.SpaceMembers != nil {
		var spaceErr error
		spaceIDs, spaceErr = s.SpaceMembers.ListMemberSpaceIDs(ctx, caller)
		if spaceErr != nil {
			log.Printf("chat: ListChats space membership skipped: %v", spaceErr)
			spaceIDs = nil
		}
	}

	var page *store.ListChatsPage
	var err error
	if folderID != nil {
		page, err = s.DM.ListChatsPageByFolder(ctx, caller, *folderID, cursor, limit, spaceIDs)
	} else {
		page, err = s.DM.ListChatsPage(ctx, caller, cursor, limit, inbox, spaceIDs)
	}
	if err != nil {
		if errors.Is(err, store.ErrInvalidListCursor) {
			return nil, status.Error(codes.InvalidArgument, "invalid page cursor")
		}
		if errors.Is(err, store.ErrFolderNotFound) {
			return nil, status.Error(codes.NotFound, "folder not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	rows := page.Rows
	nextCursor := page.NextCursor

	peers := map[uuid.UUID]uuid.UUID{}
	if len(rows) > 0 {
		rawIDs := make([]uuid.UUID, 0, len(rows))
		for _, row := range rows {
			rawIDs = append(rawIDs, row.ID)
		}
		peers, err = s.DM.DMPeerProfileIDs(ctx, caller, rawIDs)
		if err != nil {
			return nil, status.Error(codes.Unavailable, "chat snapshot unavailable")
		}
	}
	rows, err = s.filterListChatsDeletedPeerDMs(ctx, rows, peers)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "chat snapshot unavailable")
	}

	ids := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}

	extras := map[uuid.UUID]ListChatExtra{}
	if s.ListEnrich != nil && len(ids) > 0 {
		extras, err = s.ListEnrich.EnrichListChats(ctx, caller, ids)
		if err != nil {
			// Degrade: chat rows are still useful without preview/unread when Messaging
			// is temporarily unavailable during stack startup or S2S errors.
			log.Printf("chat: ListChats enrichment skipped: %v", err)
			extras = map[uuid.UUID]ListChatExtra{}
		}
	}

	items := make([]*chatv1.ChatListItem, 0, len(rows))
	for _, row := range rows {
		item := &chatv1.ChatListItem{
			Chat:        chatRowToProto(row),
			UnreadCount: 0,
		}
		item.Inbox = proto.String(row.InboxBucket)
		item.IsStranger = proto.Bool(row.InboxBucket == "requests")
		if folderID != nil {
			item.IsPinned = proto.Bool(row.IsPinned)
		}
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
			NextCursor: nextCursor,
		},
	}, nil
}
