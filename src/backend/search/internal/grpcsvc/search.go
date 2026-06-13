package grpcsvc

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/search/internal/authctx"

	commonv1 "voice.app/voice/common/v1"
	searchv1 "voice.app/voice/search/v1"

	chatv1 "voice.app/voice/chat/v1"
)

const (
	defaultPageSize = 20
	maxPageSize     = 50
)

// MessageHit is a ranked message search result.
type MessageHit struct {
	MessageID uuid.UUID
	ChatID    uuid.UUID
	Snippet   string
	Score     float64
}

// MessageSearcher queries indexed messages.
type MessageSearcher interface {
	SearchInChat(ctx context.Context, chatID uuid.UUID, query string, cursor *string, limit int) ([]MessageHit, string, error)
	SearchGlobalMessages(ctx context.Context, viewer uuid.UUID, query string, cursor *string, limit int, accessibleChatIDs []uuid.UUID) ([]MessageHit, string, error)
}

// ProfileSearcher queries indexed profiles.
type ProfileSearcher interface {
	SearchProfiles(ctx context.Context, viewer uuid.UUID, query string, excludeBlocked []uuid.UUID, limit int) ([]uuid.UUID, error)
}

// SpaceSearcher queries indexed public spaces.
type SpaceSearcher interface {
	SearchSpaces(ctx context.Context, query string, cursor *string, limit int) ([]uuid.UUID, string, error)
}

// RoleChecker validates read access to a chat channel.
type RoleChecker interface {
	CanReadMessages(ctx context.Context, viewer, chatID uuid.UUID) (bool, error)
}

// BlockList returns account IDs blocked by the viewer.
type BlockList interface {
	BlockedAccountIDs(ctx context.Context) ([]uuid.UUID, error)
}

// ChatAccess lists and searches chats visible to the viewer.
type ChatAccess interface {
	AccessibleChatIDs(ctx context.Context, viewer uuid.UUID) ([]uuid.UUID, error)
	SearchChats(ctx context.Context, query string, limit int) ([]uuid.UUID, error)
}

// ChatReindexer backfills search index for one chat.
type ChatReindexer interface {
	ReindexChat(ctx context.Context, chatID uuid.UUID) error
}

// SearchGRPC implements voice.search.v1.SearchService.
type SearchGRPC struct {
	searchv1.UnimplementedSearchServiceServer
	Messages MessageSearcher
	Profiles ProfileSearcher
	Spaces   SpaceSearcher
	Chats    ChatAccess
	Roles    RoleChecker
	Blocks   BlockList
	Reindex  ChatReindexer
}

func pageSize(page *commonv1.CursorPageRequest) int {
	if page == nil || page.GetPageSize() <= 0 {
		return defaultPageSize
	}
	if page.GetPageSize() > maxPageSize {
		return maxPageSize
	}
	return int(page.GetPageSize())
}

func cursorPtr(page *commonv1.CursorPageRequest) *string {
	if page == nil {
		return nil
	}
	c := strings.TrimSpace(page.GetCursor())
	if c == "" {
		return nil
	}
	return &c
}

func requireProfile(ctx context.Context) (uuid.UUID, error) {
	pid, ok := authctx.ProfileID(ctx)
	if !ok {
		return uuid.Nil, status.Error(codes.Unauthenticated, "missing credentials")
	}
	return pid, nil
}

func requireQuery(q string) (string, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return "", status.Error(codes.InvalidArgument, "query required")
	}
	return q, nil
}

func toProtoHits(hits []MessageHit) []*searchv1.SearchHit {
	out := make([]*searchv1.SearchHit, 0, len(hits))
	for _, h := range hits {
		out = append(out, &searchv1.SearchHit{
			MessageId: h.MessageID.String(),
			Snippet:   h.Snippet,
			Score:     h.Score,
		})
	}
	return out
}

func (s *SearchGRPC) SearchInChat(ctx context.Context, req *searchv1.SearchInChatRequest) (*searchv1.SearchInChatResponse, error) {
	viewer, err := requireProfile(ctx)
	if err != nil {
		return nil, err
	}
	q, err := requireQuery(req.GetQuery())
	if err != nil {
		return nil, err
	}
	if s.Messages == nil {
		return nil, status.Error(codes.Unavailable, "message search unavailable")
	}
	if req.GetChat() == nil || strings.TrimSpace(req.GetChat().GetId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "chat required")
	}
	chatID, err := uuid.Parse(req.GetChat().GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid chat_id")
	}
	if s.Roles != nil {
		ok, err := s.Roles.CanReadMessages(ctx, viewer, chatID)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		if !ok {
			return nil, status.Error(codes.PermissionDenied, "read access denied")
		}
	}
	limit := pageSize(req.GetPage())
	hits, next, err := s.Messages.SearchInChat(ctx, chatID, q, cursorPtr(req.GetPage()), limit)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &searchv1.SearchInChatResponse{
		SearchResults: &searchv1.SearchResults{
			Hits:       toProtoHits(hits),
			NextCursor: next,
		},
	}, nil
}

func (s *SearchGRPC) SearchGlobal(ctx context.Context, req *searchv1.SearchGlobalRequest) (*searchv1.SearchGlobalResponse, error) {
	viewer, err := requireProfile(ctx)
	if err != nil {
		return nil, err
	}
	q, err := requireQuery(req.GetQuery())
	if err != nil {
		return nil, err
	}
	limit := pageSize(req.GetPage())

	var blockedAccounts []uuid.UUID
	if s.Blocks != nil {
		blockedAccounts, err = s.Blocks.BlockedAccountIDs(ctx)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	var profileIDs []string
	if s.Profiles != nil {
		ids, err := s.Profiles.SearchProfiles(ctx, viewer, q, blockedAccounts, limit)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		profileIDs = make([]string, 0, len(ids))
		for _, id := range ids {
			profileIDs = append(profileIDs, id.String())
		}
	}

	var spaceIDs []string
	if s.Spaces != nil {
		ids, _, err := s.Spaces.SearchSpaces(ctx, q, cursorPtr(req.GetPage()), limit)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		spaceIDs = make([]string, 0, len(ids))
		for _, id := range ids {
			spaceIDs = append(spaceIDs, id.String())
		}
	}

	var matchedChats []*chatv1.ChatRef
	var accessible []uuid.UUID
	if s.Chats != nil {
		accessible, err = s.Chats.AccessibleChatIDs(ctx, viewer)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		chatIDs, err := s.Chats.SearchChats(ctx, q, limit)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		for _, id := range chatIDs {
			matchedChats = append(matchedChats, &chatv1.ChatRef{Id: id.String()})
		}
	}

	var msgHits []*searchv1.SearchHit
	var next string
	if s.Messages != nil && len(accessible) > 0 {
		hits, cursor, err := s.Messages.SearchGlobalMessages(ctx, viewer, q, cursorPtr(req.GetPage()), limit, accessible)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		msgHits = toProtoHits(hits)
		next = cursor
	}

	return &searchv1.SearchGlobalResponse{
		GlobalSearchResults: &searchv1.GlobalSearchResults{
			Messages:     msgHits,
			ProfileIds:   profileIDs,
			MatchedChats: matchedChats,
			SpaceIds:     spaceIDs,
			NextCursor:   next,
		},
	}, nil
}

func (s *SearchGRPC) SearchUsers(ctx context.Context, req *searchv1.SearchUsersRequest) (*searchv1.SearchUsersResponse, error) {
	viewer, err := requireProfile(ctx)
	if err != nil {
		return nil, err
	}
	q, err := requireQuery(req.GetQuery())
	if err != nil {
		return nil, err
	}
	if s.Profiles == nil {
		return nil, status.Error(codes.Unavailable, "profile search unavailable")
	}
	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = defaultPageSize
	}
	var blockedAccounts []uuid.UUID
	if s.Blocks != nil {
		blockedAccounts, err = s.Blocks.BlockedAccountIDs(ctx)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	ids, err := s.Profiles.SearchProfiles(ctx, viewer, q, blockedAccounts, limit)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, id.String())
	}
	return &searchv1.SearchUsersResponse{
		UserSearchResults: &searchv1.UserSearchResults{ProfileIds: out},
	}, nil
}

func (s *SearchGRPC) SearchSpaces(ctx context.Context, req *searchv1.SearchSpacesRequest) (*searchv1.SearchSpacesResponse, error) {
	if _, err := requireProfile(ctx); err != nil {
		return nil, err
	}
	q, err := requireQuery(req.GetQuery())
	if err != nil {
		return nil, err
	}
	if s.Spaces == nil {
		return nil, status.Error(codes.Unavailable, "space search unavailable")
	}
	limit := pageSize(req.GetPage())
	ids, next, err := s.Spaces.SearchSpaces(ctx, q, cursorPtr(req.GetPage()), limit)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, id.String())
	}
	return &searchv1.SearchSpacesResponse{
		SpaceSearchResults: &searchv1.SpaceSearchResults{
			SpaceIds:   out,
			NextCursor: next,
		},
	}, nil
}

func (s *SearchGRPC) ReindexChat(ctx context.Context, req *searchv1.ReindexChatRequest) (*searchv1.ReindexChatResponse, error) {
	if _, err := requireProfile(ctx); err != nil {
		return nil, err
	}
	if req.GetChat() == nil || strings.TrimSpace(req.GetChat().GetId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "chat required")
	}
	if s.Reindex == nil {
		return nil, status.Error(codes.Unavailable, "chat reindex unavailable")
	}
	chatID, err := uuid.Parse(strings.TrimSpace(req.GetChat().GetId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid chat id")
	}
	if s.Roles != nil {
		viewer, _ := authctx.ProfileID(ctx)
		allowed, perr := s.Roles.CanReadMessages(ctx, viewer, chatID)
		if perr != nil {
			return nil, status.Error(codes.Internal, perr.Error())
		}
		if !allowed {
			return nil, status.Error(codes.PermissionDenied, "cannot read chat")
		}
	}
	if err := s.Reindex.ReindexChat(ctx, chatID); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &searchv1.ReindexChatResponse{}, nil
}
