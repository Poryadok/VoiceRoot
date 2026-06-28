package grpcsvc

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/pkg/guestguard"
	"voice/backend/pkg/privacy"
	"voice/backend/story/internal/authctx"
	"voice/backend/story/internal/s2s"
	"voice/backend/story/internal/store"
	"voice/backend/story/internal/storyevents"

	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	storyv1 "voice.app/voice/story/v1"
)

// FriendChecker resolves whether viewer can see author's friends-only stories.
type FriendChecker interface {
	IsFriend(ctx context.Context, viewerProfileID, authorProfileID uuid.UUID) (bool, error)
}

// FeedAuthorLister returns author profile ids for feed prefilter.
type FeedAuthorLister interface {
	ListFeedAuthorIDs(ctx context.Context, viewerProfileID uuid.UUID) ([]uuid.UUID, error)
}

// StoryAudienceChecker supplies social/space facts for privacy.Matcher.
type StoryAudienceChecker interface {
	privacy.SocialGraph
	privacy.SpaceCoMembership
}

// FileMetadataChecker validates uploaded media metadata (e.g. video duration).
type FileMetadataChecker interface {
	GetFileDurationSeconds(ctx context.Context, fileID uuid.UUID) (int32, error)
}

// SubscriptionChecker resolves Premium entitlement for anonymous story views.
type SubscriptionChecker interface {
	HasActivePremium(ctx context.Context, accountID uuid.UUID) (bool, error)
}

// StoryPrivacyChecker loads show_stories audience for LFP visibility floor.
type StoryPrivacyChecker interface {
	ShowStoriesAudience(ctx context.Context, profileID uuid.UUID) (privacy.Audience, error)
}

// ChatClient opens DM chats for private story replies.
type ChatClient interface {
	CreateDM(ctx context.Context, in *chatv1.CreateDMRequest) (*chatv1.CreateDMResponse, error)
}

// MessagingClient sends DM messages for private story replies.
type MessagingClient interface {
	SendMessage(ctx context.Context, in *messagingv1.SendMessageRequest) (*messagingv1.SendMessageResponse, error)
}

// StoryGRPC implements voice.story.v1.StoryService.
type StoryGRPC struct {
	storyv1.UnimplementedStoryServiceServer
	Store         *store.StoryStore
	Friends       FriendChecker
	Audience      StoryAudienceChecker
	FeedAuthors   FeedAuthorLister
	Files         FileMetadataChecker
	Subscriptions SubscriptionChecker
	Privacy       StoryPrivacyChecker
	Chat          ChatClient
	Messaging     MessagingClient
	Events        storyevents.Publisher
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
	var mediaID *uuid.UUID
	if req.MediaFileId != nil && strings.TrimSpace(*req.MediaFileId) != "" {
		parsed, parseErr := uuid.Parse(strings.TrimSpace(*req.MediaFileId))
		if parseErr != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid media_file_id")
		}
		mediaID = &parsed
	}
	visibility, visAudienceJSON := visibilityFromRequest(strings.TrimSpace(req.GetVisibility()), req.GetVisibilityEnum())
	if visibility == "" {
		if s.Privacy != nil {
			aud, privErr := s.Privacy.ShowStoriesAudience(ctx, profileID)
			if privErr == nil {
				visibility, visAudienceJSON = audienceToStoryVisibility(aud)
			}
		}
		if visibility == "" {
			visibility = "friends"
		}
	}
	if storyType == "video" && mediaID != nil && s.Files != nil {
		secs, durErr := s.Files.GetFileDurationSeconds(ctx, *mediaID)
		if durErr != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid media_file_id")
		}
		if secs > 60 {
			return nil, status.Error(codes.InvalidArgument, "video duration must be at most 60 seconds")
		}
	}
	row, err := s.Store.CreateStory(ctx, store.CreateStoryInput{
		AuthorProfileID:        profileID,
		Type:                   storyType,
		MediaFileID:            mediaID,
		TextContent:            req.TextContent,
		TextStyleJSON:          req.TextStyleJson,
		GameTag:                req.GameTag,
		MentionProfileIDs:      mentionIDsToJSON(req.GetMentionProfileIds()),
		Visibility:             visibility,
		VisibilityAudienceJSON: visAudienceJSON,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if s.Events != nil {
		gameTag := ""
		if req.GameTag != nil {
			gameTag = *req.GameTag
		}
		_ = s.Events.PublishStoryCreated(ctx, row.ID.String(), profileID.String(), storyType, gameTag, req.GetMentionProfileIds())
	}
	return &storyv1.CreateStoryResponse{Story: rowToProtoForViewer(row, profileID)}, nil
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
	return &storyv1.GetStoryResponse{Story: rowToProtoForViewer(row, viewerID)}, nil
}

func (s *StoryGRPC) GetStoryFeed(ctx context.Context, req *storyv1.GetStoryFeedRequest) (*storyv1.GetStoryFeedResponse, error) {
	viewerID, err := authctx.ProfileID(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "profile required")
	}
	limit := int32(50)
	cursor := ""
	if req.GetPage() != nil {
		if req.GetPage().GetPageSize() > 0 {
			limit = req.GetPage().GetPageSize()
		}
		cursor = req.GetPage().GetCursor()
	}
	var page *store.PaginatedStories
	var pageErr error
	if s.FeedAuthors != nil {
		authorIDs, listErr := s.FeedAuthors.ListFeedAuthorIDs(ctx, viewerID)
		if listErr == nil {
			page, pageErr = s.Store.ListActiveStoriesForAuthorsPaginated(ctx, authorIDs, int(limit), cursor)
		}
	}
	if page == nil {
		page, pageErr = s.Store.ListActiveStoriesPaginated(ctx, int(limit), cursor)
	}
	if pageErr != nil {
		return nil, status.Error(codes.Internal, pageErr.Error())
	}
	var visible []*storyv1.Story
	for i := range page.Rows {
		if s.canViewStory(ctx, viewerID, &page.Rows[i]) {
			visible = append(visible, rowToProtoForViewer(&page.Rows[i], viewerID))
		}
	}
	return &storyv1.GetStoryFeedResponse{
		Stories:    visible,
		NextCursor: page.NextCursor,
		Page: &commonv1.CursorPageResponse{
			NextCursor: page.NextCursor,
			HasMore:    page.HasMore,
		},
		FeedGroups: groupStoriesByAuthorCentric(visible),
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
			visible = append(visible, rowToProtoForViewer(&rows[i], viewerID))
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
	if req.GetAnonymous() {
		accountID, err := accountIDFromContext(ctx)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "account required")
		}
		if s.Subscriptions == nil {
			return nil, status.Error(codes.PermissionDenied, "premium subscription required")
		}
		ok, err := s.Subscriptions.HasActivePremium(ctx, accountID)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		if !ok {
			return nil, status.Error(codes.PermissionDenied, "premium subscription required")
		}
	}
	if err := s.Store.MarkViewed(ctx, storyID, viewerID, req.GetAnonymous()); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if s.Events != nil {
		_ = s.Events.PublishStoryViewed(ctx, storyID.String(), viewerID.String())
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
	if s.Events != nil {
		_ = s.Events.PublishStoryReacted(ctx, storyID.String(), profileID.String(), emoji)
	}
	return &storyv1.ReactToStoryResponse{}, nil
}

func (s *StoryGRPC) GetStoryReactions(ctx context.Context, req *storyv1.GetStoryReactionsRequest) (*storyv1.GetStoryReactionsResponse, error) {
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
		return nil, status.Error(codes.PermissionDenied, "only author can list reactions")
	}
	reactions, err := s.Store.ListStoryReactions(ctx, storyID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	out := make([]*storyv1.StoryReaction, 0, len(reactions))
	for _, r := range reactions {
		out = append(out, &storyv1.StoryReaction{
			ReactorProfileId: r.ReactorProfileID.String(),
			Emoji:            r.Emoji,
		})
	}
	return &storyv1.GetStoryReactionsResponse{Reactions: out}, nil
}

func (s *StoryGRPC) ReplyToStory(ctx context.Context, req *storyv1.ReplyToStoryRequest) (*storyv1.ReplyToStoryResponse, error) {
	profileID, err := authctx.ProfileID(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "profile required")
	}
	if s.Chat == nil || s.Messaging == nil {
		return nil, status.Error(codes.FailedPrecondition, "reply not configured")
	}
	storyID, err := uuid.Parse(strings.TrimSpace(req.GetStoryId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid story_id")
	}
	text := strings.TrimSpace(req.GetText())
	if text == "" {
		return nil, status.Error(codes.InvalidArgument, "text is required")
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
	if row.AuthorProfileID == profileID {
		return nil, status.Error(codes.InvalidArgument, "cannot reply to own story")
	}
	callCtx := s2s.ForwardIncomingMetadata(ctx)
	dm, err := s.Chat.CreateDM(callCtx, &chatv1.CreateDMRequest{OtherProfileId: row.AuthorProfileID.String()})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	sent, err := s.Messaging.SendMessage(callCtx, &messagingv1.SendMessageRequest{
		Chat:    &chatv1.ChatRef{Id: dm.GetChat().GetId()},
		Content: text,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &storyv1.ReplyToStoryResponse{
		ChatId:    dm.GetChat().GetId(),
		MessageId: sent.GetMessage().GetId(),
	}, nil
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
	row, err := s.Store.CreateHighlight(ctx, profileID, name, highlightVisibilityFromRequest(req.GetVisibility(), req.GetVisibilityEnum()))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if s.Events != nil {
		_ = s.Events.PublishStoryHighlightCreated(ctx, row.ID.String(), profileID.String())
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
	visibility := highlightVisibilityFromRequest(req.GetVisibility(), req.GetVisibilityEnum())
	if name == "" && visibility == "" {
		return nil, status.Error(codes.InvalidArgument, "name or visibility is required")
	}
	row, err := s.Store.UpdateHighlight(ctx, highlightID, profileID, name, visibility)
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
	viewerID, err := authctx.ProfileID(ctx)
	if err != nil {
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
		if s.canViewHighlight(ctx, viewerID, &rows[i]) {
			highlights = append(highlights, highlightToProto(&rows[i]))
		}
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
	lfpVisibility, err := lfpVisibilityFromCriteria(criteria)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	floor := s.storyPrivacyFloor(ctx, profileID)
	if visibilityRestrictiveness(lfpVisibility) > visibilityRestrictiveness(floor) {
		return nil, status.Error(codes.InvalidArgument, "lfp visibility cannot be narrower than story privacy")
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
		Visibility:        lfpVisibility,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if s.Events != nil {
		_ = s.Events.PublishStoryLfpCreated(ctx, row.ID.String(), profileID.String(), criteria)
	}
	return &storyv1.CreateLookingForPartyResponse{Story: rowToProtoForViewer(row, profileID)}, nil
}

func (s *StoryGRPC) canViewStory(ctx context.Context, viewerID uuid.UUID, row *store.StoryRow) bool {
	if row == nil {
		return false
	}
	if viewerID == row.AuthorProfileID {
		return true
	}
	storyAudience := audienceFromStoryRow(row.Visibility, row.VisibilityAudienceJSON)
	if s.Privacy != nil {
		floor, err := s.Privacy.ShowStoriesAudience(ctx, row.AuthorProfileID)
		if err == nil && !floor.IsNobody() {
			matcher := s.privacyMatcher()
			ok, matchErr := matcher.Allowed(ctx, row.AuthorProfileID, viewerID, floor, guestguard.IsGuest(ctx))
			if matchErr != nil || !ok {
				return false
			}
		}
	}
	matcher := s.privacyMatcher()
	ok, err := matcher.Allowed(ctx, row.AuthorProfileID, viewerID, storyAudience, guestguard.IsGuest(ctx))
	return err == nil && ok
}

func (s *StoryGRPC) privacyMatcher() privacy.Matcher {
	var social privacy.SocialGraph
	var space privacy.SpaceCoMembership
	if s.Audience != nil {
		social = s.Audience
		space = s.Audience
	} else if s.Friends != nil {
		if sg, ok := s.Friends.(privacy.SocialGraph); ok {
			social = sg
		}
		if sc, ok := s.Friends.(privacy.SpaceCoMembership); ok {
			space = sc
		}
	}
	return privacy.Matcher{Social: social, Space: space}
}

func (s *StoryGRPC) canViewHighlight(ctx context.Context, viewerID uuid.UUID, row *store.HighlightRow) bool {
	if row == nil {
		return false
	}
	if viewerID == row.ProfileID {
		return true
	}
	hlAudience := audienceFromStoryRow(row.Visibility, nil)
	matcher := s.privacyMatcher()
	ok, err := matcher.Allowed(ctx, row.ProfileID, viewerID, hlAudience, guestguard.IsGuest(ctx))
	return err == nil && ok
}

func (s *StoryGRPC) storyPrivacyFloor(ctx context.Context, profileID uuid.UUID) string {
	if s.Privacy != nil {
		audience, err := s.Privacy.ShowStoriesAudience(ctx, profileID)
		if err == nil {
			return audienceFloorVisibility(audience)
		}
	}
	return audienceFloorVisibility(privacy.FriendsAndFoF())
}

func rowToProtoForViewer(row *store.StoryRow, viewerID uuid.UUID) *storyv1.Story {
	out := rowToProto(row)
	if out == nil {
		return nil
	}
	if row.AuthorProfileID != viewerID {
		out.ViewCount = 0
	}
	return out
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
		Id:         row.ID.String(),
		ProfileId:  row.ProfileID.String(),
		Name:       row.Name,
		StoryIds:   ids,
		Visibility: row.Visibility,
	}
}

func groupStoriesByAuthorCentric(stories []*storyv1.Story) []*storyv1.StoryFeedGroup {
	if len(stories) == 0 {
		return nil
	}
	byAuthor := make(map[string][]*storyv1.Story)
	latest := make(map[string]int64)
	for _, st := range stories {
		aid := st.GetAuthorProfileId()
		byAuthor[aid] = append(byAuthor[aid], st)
		ts := st.GetCreatedAt().AsTime().UnixNano()
		if ts > latest[aid] {
			latest[aid] = ts
		}
	}
	authorIDs := make([]string, 0, len(byAuthor))
	for aid := range byAuthor {
		authorIDs = append(authorIDs, aid)
	}
	for i := 0; i < len(authorIDs); i++ {
		for j := i + 1; j < len(authorIDs); j++ {
			li, lj := latest[authorIDs[i]], latest[authorIDs[j]]
			if lj > li || (lj == li && authorIDs[j] > authorIDs[i]) {
				authorIDs[i], authorIDs[j] = authorIDs[j], authorIDs[i]
			}
		}
	}
	groups := make([]*storyv1.StoryFeedGroup, 0, len(authorIDs))
	for _, aid := range authorIDs {
		groups = append(groups, &storyv1.StoryFeedGroup{
			AuthorProfileId: aid,
			Stories:         byAuthor[aid],
		})
	}
	return groups
}

func mentionIDsToJSON(ids []string) string {
	valid := make([]string, 0, len(ids))
	for _, raw := range ids {
		id, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			continue
		}
		valid = append(valid, id.String())
	}
	b, err := json.Marshal(valid)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func accountIDFromContext(ctx context.Context) (uuid.UUID, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return uuid.Nil, authctx.ErrMissingAccount()
	}
	vals := md.Get(authctx.HeaderUserID)
	if len(vals) == 0 || vals[0] == "" {
		return uuid.Nil, authctx.ErrMissingAccount()
	}
	return uuid.Parse(vals[0])
}

type lfpCriteria struct {
	Visibility string `json:"visibility"`
}

func lfpVisibilityFromCriteria(raw string) (string, error) {
	var parsed lfpCriteria
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return "", err
	}
	v := strings.TrimSpace(parsed.Visibility)
	if v == "" {
		return "everyone", nil
	}
	switch v {
	case "everyone", "friends", "custom":
		return v, nil
	default:
		return "", errors.New("invalid lfp visibility")
	}
}

func audienceFloorVisibility(a privacy.Audience) string {
	if a.IsEveryoneShortcut() {
		return "everyone"
	}
	if a.IsNobody() {
		return "custom"
	}
	return "friends"
}

func visibilityRestrictiveness(v string) int {
	switch strings.TrimSpace(v) {
	case "everyone":
		return 0
	case "friends":
		return 1
	default:
		return 2
	}
}

func highlightVisibilityFromRequest(visibility string, enum storyv1.StoryAudience) string {
	if v := strings.TrimSpace(visibility); v != "" {
		return v
	}
	if v := storyAudienceString(enum); v != "" {
		return v
	}
	return ""
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
