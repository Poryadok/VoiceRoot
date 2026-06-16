package grpcsvc

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/story/internal/authctx"
	"voice/backend/story/internal/store"

	commonv1 "voice.app/voice/common/v1"
	storyv1 "voice.app/voice/story/v1"
)

// FriendChecker resolves whether viewer can see author's friends-only stories.
type FriendChecker interface {
	IsFriend(ctx context.Context, viewerProfileID, authorProfileID uuid.UUID) (bool, error)
}

// StoryGRPC implements voice.story.v1.StoryService.
type StoryGRPC struct {
	storyv1.UnimplementedStoryServiceServer
	Store   *store.StoryStore
	Friends FriendChecker
}

// NewStoryGRPC wires a StoryGRPC handler.
func NewStoryGRPC(st *store.StoryStore) *StoryGRPC {
	return &StoryGRPC{Store: st}
}

func (s *StoryGRPC) CreateStory(ctx context.Context, req *storyv1.CreateStoryRequest) (*storyv1.CreateStoryResponse, error) {
	profileID, err := authctx.ProfileID(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "profile required")
	}
	storyType := strings.TrimSpace(req.GetType())
	if storyType == "" {
		storyType = storyMediaTypeString(req.GetTypeEnum())
	}
	if storyType == "" {
		return nil, status.Error(codes.InvalidArgument, "type is required")
	}
	visibility := strings.TrimSpace(req.GetVisibility())
	if visibility == "" {
		visibility = storyAudienceString(req.GetVisibilityEnum())
	}
	if visibility == "" {
		visibility = "friends"
	}
	var mediaID *uuid.UUID
	if req.MediaFileId != nil && strings.TrimSpace(*req.MediaFileId) != "" {
		parsed, parseErr := uuid.Parse(strings.TrimSpace(*req.MediaFileId))
		if parseErr != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid media_file_id")
		}
		mediaID = &parsed
	}
	row, err := s.Store.CreateStory(ctx, store.CreateStoryInput{
		AuthorProfileID: profileID,
		Type:            storyType,
		MediaFileID:     mediaID,
		TextContent:     req.TextContent,
		TextStyleJSON:   req.TextStyleJson,
		GameTag:         req.GameTag,
		Visibility:      visibility,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &storyv1.CreateStoryResponse{Story: rowToProto(row)}, nil
}

func (s *StoryGRPC) DeleteStory(ctx context.Context, req *storyv1.DeleteStoryRequest) (*storyv1.DeleteStoryResponse, error) {
	profileID, err := authctx.ProfileID(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "profile required")
	}
	storyID, err := uuid.Parse(strings.TrimSpace(req.GetStoryId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid story_id")
	}
	if err := s.Store.DeleteStory(ctx, storyID, profileID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "story not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &storyv1.DeleteStoryResponse{}, nil
}

func (s *StoryGRPC) GetStory(ctx context.Context, req *storyv1.GetStoryRequest) (*storyv1.GetStoryResponse, error) {
	viewerID, err := authctx.ProfileID(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "profile required")
	}
	storyID, err := uuid.Parse(strings.TrimSpace(req.GetStoryId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid story_id")
	}
	row, err := s.Store.GetStory(ctx, storyID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "story not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !s.canViewStory(ctx, viewerID, row) {
		return nil, status.Error(codes.PermissionDenied, "story not visible")
	}
	return &storyv1.GetStoryResponse{Story: rowToProto(row)}, nil
}

func (s *StoryGRPC) GetStoryFeed(ctx context.Context, req *storyv1.GetStoryFeedRequest) (*storyv1.GetStoryFeedResponse, error) {
	viewerID, err := authctx.ProfileID(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "profile required")
	}
	limit := int32(50)
	if req.GetPage() != nil && req.GetPage().GetPageSize() > 0 {
		limit = req.GetPage().GetPageSize()
	}
	rows, err := s.Store.ListActiveStories(ctx, int(limit))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	var visible []*storyv1.Story
	for i := range rows {
		if s.canViewStory(ctx, viewerID, &rows[i]) {
			visible = append(visible, rowToProto(&rows[i]))
		}
	}
	return &storyv1.GetStoryFeedResponse{
		Stories: visible,
		Page:    &commonv1.CursorPageResponse{HasMore: false},
	}, nil
}

func (s *StoryGRPC) GetProfileStories(ctx context.Context, req *storyv1.GetProfileStoriesRequest) (*storyv1.GetProfileStoriesResponse, error) {
	viewerID, err := authctx.ProfileID(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "profile required")
	}
	profileID, err := uuid.Parse(strings.TrimSpace(req.GetProfileId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid profile_id")
	}
	rows, err := s.Store.ListActiveStoriesByAuthor(ctx, profileID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	var visible []*storyv1.Story
	for i := range rows {
		if s.canViewStory(ctx, viewerID, &rows[i]) {
			visible = append(visible, rowToProto(&rows[i]))
		}
	}
	return &storyv1.GetProfileStoriesResponse{
		StoryList: &storyv1.StoryList{Stories: visible},
	}, nil
}

func (s *StoryGRPC) MarkViewed(ctx context.Context, req *storyv1.MarkViewedRequest) (*storyv1.MarkViewedResponse, error) {
	viewerID, err := authctx.ProfileID(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "profile required")
	}
	storyID, err := uuid.Parse(strings.TrimSpace(req.GetStoryId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid story_id")
	}
	row, err := s.Store.GetStory(ctx, storyID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "story not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !s.canViewStory(ctx, viewerID, row) {
		return nil, status.Error(codes.PermissionDenied, "story not visible")
	}
	if err := s.Store.MarkViewed(ctx, storyID, viewerID, req.GetAnonymous()); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &storyv1.MarkViewedResponse{}, nil
}

func (s *StoryGRPC) GetViewers(ctx context.Context, req *storyv1.GetViewersRequest) (*storyv1.GetViewersResponse, error) {
	profileID, err := authctx.ProfileID(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "profile required")
	}
	storyID, err := uuid.Parse(strings.TrimSpace(req.GetStoryId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid story_id")
	}
	row, err := s.Store.GetStory(ctx, storyID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "story not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	if row.AuthorProfileID != profileID {
		return nil, status.Error(codes.PermissionDenied, "only author can list viewers")
	}
	if row.ExpiredAt != nil {
		return &storyv1.GetViewersResponse{ViewerList: &storyv1.ViewerList{}}, nil
	}
	ids, err := s.Store.ListViewers(ctx, storyID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	strIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		strIDs = append(strIDs, id.String())
	}
	return &storyv1.GetViewersResponse{
		ViewerList: &storyv1.ViewerList{ViewerProfileIds: strIDs},
	}, nil
}

func (s *StoryGRPC) ReactToStory(ctx context.Context, req *storyv1.ReactToStoryRequest) (*storyv1.ReactToStoryResponse, error) {
	profileID, err := authctx.ProfileID(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "profile required")
	}
	storyID, err := uuid.Parse(strings.TrimSpace(req.GetStoryId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid story_id")
	}
	row, err := s.Store.GetStory(ctx, storyID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "story not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !s.canViewStory(ctx, profileID, row) {
		return nil, status.Error(codes.PermissionDenied, "story not visible")
	}
	emoji := strings.TrimSpace(req.GetEmoji())
	if emoji == "" {
		return nil, status.Error(codes.InvalidArgument, "emoji is required")
	}
	if err := s.Store.ReactToStory(ctx, storyID, profileID, emoji); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &storyv1.ReactToStoryResponse{}, nil
}

func (s *StoryGRPC) GetArchive(ctx context.Context, req *storyv1.GetArchiveRequest) (*storyv1.GetArchiveResponse, error) {
	profileID, err := authctx.ProfileID(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "profile required")
	}
	target := profileID
	if strings.TrimSpace(req.GetProfileId()) != "" {
		parsed, parseErr := uuid.Parse(strings.TrimSpace(req.GetProfileId()))
		if parseErr != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid profile_id")
		}
		target = parsed
	}
	if target != profileID {
		return nil, status.Error(codes.PermissionDenied, "archive is private")
	}
	rows, err := s.Store.ListArchive(ctx, target)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	stories := make([]*storyv1.Story, 0, len(rows))
	for i := range rows {
		stories = append(stories, rowToProto(&rows[i]))
	}
	return &storyv1.GetArchiveResponse{StoryList: &storyv1.StoryList{Stories: stories}}, nil
}

func (s *StoryGRPC) CreateHighlight(ctx context.Context, req *storyv1.CreateHighlightRequest) (*storyv1.CreateHighlightResponse, error) {
	profileID, err := authctx.ProfileID(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "profile required")
	}
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	row, err := s.Store.CreateHighlight(ctx, profileID, name)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &storyv1.CreateHighlightResponse{Highlight: highlightToProto(row)}, nil
}

func (s *StoryGRPC) UpdateHighlight(ctx context.Context, req *storyv1.UpdateHighlightRequest) (*storyv1.UpdateHighlightResponse, error) {
	profileID, err := authctx.ProfileID(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "profile required")
	}
	highlightID, err := uuid.Parse(strings.TrimSpace(req.GetHighlightId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid highlight_id")
	}
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	row, err := s.Store.UpdateHighlight(ctx, highlightID, profileID, name)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "highlight not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &storyv1.UpdateHighlightResponse{Highlight: highlightToProto(row)}, nil
}

func (s *StoryGRPC) DeleteHighlight(ctx context.Context, req *storyv1.DeleteHighlightRequest) (*storyv1.DeleteHighlightResponse, error) {
	profileID, err := authctx.ProfileID(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "profile required")
	}
	highlightID, err := uuid.Parse(strings.TrimSpace(req.GetHighlightId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid highlight_id")
	}
	if err := s.Store.DeleteHighlight(ctx, highlightID, profileID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "highlight not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &storyv1.DeleteHighlightResponse{}, nil
}

func (s *StoryGRPC) AddToHighlight(ctx context.Context, req *storyv1.AddToHighlightRequest) (*storyv1.AddToHighlightResponse, error) {
	profileID, err := authctx.ProfileID(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "profile required")
	}
	highlightID, err := uuid.Parse(strings.TrimSpace(req.GetHighlightId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid highlight_id")
	}
	storyID, err := uuid.Parse(strings.TrimSpace(req.GetStoryId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid story_id")
	}
	if err := s.Store.AddToHighlight(ctx, highlightID, profileID, storyID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "highlight or story not found")
		}
		if errors.Is(err, store.ErrForbidden) {
			return nil, status.Error(codes.PermissionDenied, "forbidden")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &storyv1.AddToHighlightResponse{}, nil
}

func (s *StoryGRPC) RemoveFromHighlight(ctx context.Context, req *storyv1.RemoveFromHighlightRequest) (*storyv1.RemoveFromHighlightResponse, error) {
	profileID, err := authctx.ProfileID(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "profile required")
	}
	highlightID, err := uuid.Parse(strings.TrimSpace(req.GetHighlightId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid highlight_id")
	}
	storyID, err := uuid.Parse(strings.TrimSpace(req.GetStoryId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid story_id")
	}
	if err := s.Store.RemoveFromHighlight(ctx, highlightID, profileID, storyID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &storyv1.RemoveFromHighlightResponse{}, nil
}

func (s *StoryGRPC) GetHighlights(ctx context.Context, req *storyv1.GetHighlightsRequest) (*storyv1.GetHighlightsResponse, error) {
	if _, err := authctx.ProfileID(ctx); err != nil {
		return nil, status.Error(codes.Unauthenticated, "profile required")
	}
	profileID, err := uuid.Parse(strings.TrimSpace(req.GetProfileId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid profile_id")
	}
	rows, err := s.Store.GetHighlights(ctx, profileID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	highlights := make([]*storyv1.Highlight, 0, len(rows))
	for i := range rows {
		highlights = append(highlights, highlightToProto(&rows[i]))
	}
	return &storyv1.GetHighlightsResponse{
		HighlightList: &storyv1.HighlightList{Highlights: highlights},
	}, nil
}

func (s *StoryGRPC) CreateLookingForParty(ctx context.Context, req *storyv1.CreateLookingForPartyRequest) (*storyv1.CreateLookingForPartyResponse, error) {
	profileID, err := authctx.ProfileID(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "profile required")
	}
	criteria := strings.TrimSpace(req.GetCriteriaJson())
	if criteria == "" {
		return nil, status.Error(codes.InvalidArgument, "criteria_json is required")
	}
	var mediaID *uuid.UUID
	if req.MediaFileId != nil && strings.TrimSpace(*req.MediaFileId) != "" {
		parsed, parseErr := uuid.Parse(strings.TrimSpace(*req.MediaFileId))
		if parseErr != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid media_file_id")
		}
		mediaID = &parsed
	}
	row, err := s.Store.CreateStory(ctx, store.CreateStoryInput{
		AuthorProfileID:   profileID,
		Type:              "text",
		MediaFileID:       mediaID,
		IsLookingForParty: true,
		LFPCriteriaJSON:   &criteria,
		Visibility:        "everyone",
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &storyv1.CreateLookingForPartyResponse{Story: rowToProto(row)}, nil
}

func (s *StoryGRPC) canViewStory(ctx context.Context, viewerID uuid.UUID, row *store.StoryRow) bool {
	if row == nil {
		return false
	}
	if viewerID == row.AuthorProfileID {
		return true
	}
	switch row.Visibility {
	case "everyone":
		return true
	case "friends":
		if s.Friends == nil {
			return false
		}
		ok, err := s.Friends.IsFriend(ctx, viewerID, row.AuthorProfileID)
		return err == nil && ok
	default:
		return false
	}
}

func rowToProto(row *store.StoryRow) *storyv1.Story {
	if row == nil {
		return nil
	}
	out := &storyv1.Story{
		Id:                row.ID.String(),
		AuthorProfileId:   row.AuthorProfileID.String(),
		Type:              row.Type,
		TextContent:       row.TextContent,
		TextStyleJson:     row.TextStyleJSON,
		GameTag:           row.GameTag,
		IsLookingForParty: row.IsLookingForParty,
		LfpCriteriaJson:   row.LFPCriteriaJSON,
		MentionProfileIdsJson: row.MentionProfileIDs,
		ViewCount:         int32(row.ViewCount),
		Visibility:        row.Visibility,
		ExpiresAt:         timestamppb.New(row.ExpiresAt),
		ArchivedUntil:     timestamppb.New(row.ArchivedUntil),
		CreatedAt:         timestamppb.New(row.CreatedAt),
	}
	if row.MediaFileID != nil {
		s := row.MediaFileID.String()
		out.MediaFileId = &s
	}
	if row.DeletedAt != nil {
		out.DeletedAt = timestamppb.New(*row.DeletedAt)
	}
	return out
}

func highlightToProto(row *store.HighlightRow) *storyv1.Highlight {
	if row == nil {
		return nil
	}
	ids := make([]string, 0, len(row.StoryIDs))
	for _, id := range row.StoryIDs {
		ids = append(ids, id.String())
	}
	return &storyv1.Highlight{
		Id:        row.ID.String(),
		ProfileId: row.ProfileID.String(),
		Name:      row.Name,
		StoryIds:  ids,
	}
}

func storyMediaTypeString(v storyv1.StoryMediaType) string {
	switch v {
	case storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO:
		return "photo"
	case storyv1.StoryMediaType_STORY_MEDIA_TYPE_VIDEO:
		return "video"
	case storyv1.StoryMediaType_STORY_MEDIA_TYPE_TEXT:
		return "text"
	default:
		return ""
	}
}

func storyAudienceString(v storyv1.StoryAudience) string {
	switch v {
	case storyv1.StoryAudience_STORY_AUDIENCE_PUBLIC:
		return "everyone"
	case storyv1.StoryAudience_STORY_AUDIENCE_FRIENDS:
		return "friends"
	default:
		return ""
	}
}
