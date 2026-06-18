package grpcsvc_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	commonv1 "voice.app/voice/common/v1"
	storyv1 "voice.app/voice/story/v1"
)

// withMentionProfileIDs sets mention_profile_ids on CreateStoryRequest.
func withMentionProfileIDs(req *storyv1.CreateStoryRequest, ids ...uuid.UUID) *storyv1.CreateStoryRequest {
	strIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		strIDs = append(strIDs, id.String())
	}
	req.MentionProfileIds = strIDs
	return req
}

func mentionIDsFromStoryJSON(raw string) []string {
	var ids []string
	_ = json.Unmarshal([]byte(raw), &ids)
	return ids
}

func TestCreateStory_persistsMentionProfileIDs(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	author := uuid.New()
	mentioned := uuid.New()
	ctx := withProfile(context.Background(), uuid.New(), author)
	text := "hello @friend"
	resp, err := client.CreateStory(ctx, withMentionProfileIDs(&storyv1.CreateStoryRequest{
		Type:        "text",
		TextContent: &text,
		Visibility:  "everyone",
	}, mentioned))
	require.NoError(t, err)
	got := mentionIDsFromStoryJSON(resp.GetStory().GetMentionProfileIdsJson())
	require.Contains(t, got, mentioned.String(),
		"CreateStory must persist mention_profile_ids for @username notifications")
}

func TestGetStoryFeed_hasMoreWhenAdditionalStoriesExist(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	author := uuid.New()
	ctx := withProfile(context.Background(), uuid.New(), author)
	for i := 0; i < 3; i++ {
		text := "feed story"
		_, err := client.CreateStory(ctx, &storyv1.CreateStoryRequest{
			Type: "text", TextContent: &text, Visibility: "everyone",
		})
		require.NoError(t, err)
	}

	feed, err := client.GetStoryFeed(ctx, &storyv1.GetStoryFeedRequest{
		Page: &commonv1.CursorPageRequest{PageSize: 2},
	})
	require.NoError(t, err)
	require.Len(t, feed.GetStories(), 2)
	require.True(t, feed.GetPage().GetHasMore(),
		"GetStoryFeed must set HasMore when more active stories exist beyond page size")
}

func TestGetStoryFeed_cursorReturnsNextPage(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	author := uuid.New()
	ctx := withProfile(context.Background(), uuid.New(), author)
	var ids []string
	for i := 0; i < 3; i++ {
		text := "paged story"
		created, err := client.CreateStory(ctx, &storyv1.CreateStoryRequest{
			Type: "text", TextContent: &text, Visibility: "everyone",
		})
		require.NoError(t, err)
		ids = append(ids, created.GetStory().GetId())
	}

	page1, err := client.GetStoryFeed(ctx, &storyv1.GetStoryFeedRequest{
		Page: &commonv1.CursorPageRequest{PageSize: 1},
	})
	require.NoError(t, err)
	cursor := page1.GetPage().GetNextCursor()
	if cursor == "" {
		cursor = page1.GetNextCursor()
	}
	require.NotEmpty(t, cursor, "feed must return next_cursor for pagination")

	page2, err := client.GetStoryFeed(ctx, &storyv1.GetStoryFeedRequest{
		Page: &commonv1.CursorPageRequest{
			PageSize: 1,
			Cursor:   cursor,
		},
	})
	require.NoError(t, err)
	require.NotEqual(t, page1.GetStories()[0].GetId(), page2.GetStories()[0].GetId(),
		"cursor page must return the next story")
	_ = ids
}

func TestGetStory_viewCountHiddenFromNonAuthor(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	author := uuid.New()
	viewer := uuid.New()
	ctxAuthor := withProfile(context.Background(), uuid.New(), author)
	ctxViewer := withProfile(context.Background(), uuid.New(), viewer)
	text := "count me"
	created, err := client.CreateStory(ctxAuthor, &storyv1.CreateStoryRequest{
		Type: "text", TextContent: &text, Visibility: "everyone",
	})
	require.NoError(t, err)
	storyID := created.GetStory().GetId()

	_, err = client.MarkViewed(ctxViewer, &storyv1.MarkViewedRequest{StoryId: storyID})
	require.NoError(t, err)

	authorView, err := client.GetStory(ctxAuthor, &storyv1.GetStoryRequest{StoryId: storyID})
	require.NoError(t, err)
	require.Greater(t, authorView.GetStory().GetViewCount(), int32(0),
		"author sees view_count per stories.md")

	viewerView, err := client.GetStory(ctxViewer, &storyv1.GetStoryRequest{StoryId: storyID})
	require.NoError(t, err)
	require.Zero(t, viewerView.GetStory().GetViewCount(),
		"view_count must be hidden from non-author")
}

func TestGetStoryFeed_viewCountHiddenFromNonAuthor(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	author := uuid.New()
	viewer := uuid.New()
	ctxAuthor := withProfile(context.Background(), uuid.New(), author)
	ctxViewer := withProfile(context.Background(), uuid.New(), viewer)
	text := "feed count"
	created, err := client.CreateStory(ctxAuthor, &storyv1.CreateStoryRequest{
		Type: "text", TextContent: &text, Visibility: "everyone",
	})
	require.NoError(t, err)
	storyID := created.GetStory().GetId()

	_, err = client.MarkViewed(ctxViewer, &storyv1.MarkViewedRequest{StoryId: storyID})
	require.NoError(t, err)

	feed, err := client.GetStoryFeed(ctxViewer, &storyv1.GetStoryFeedRequest{})
	require.NoError(t, err)
	for _, s := range feed.GetStories() {
		if s.GetId() == storyID {
			require.Zero(t, s.GetViewCount(), "feed must hide view_count from non-author")
			return
		}
	}
	require.Fail(t, "story not found in viewer feed")
}

// GetStoryReactions RPC is author-only per docs/features/stories.md (reactions visible only to author).
func callGetStoryReactions(client storyv1.StoryServiceClient, ctx context.Context, storyID string) error {
	_, err := client.GetStoryReactions(ctx, &storyv1.GetStoryReactionsRequest{StoryId: storyID})
	return err
}

func TestGetStoryReactions_authorOnly(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	author := uuid.New()
	viewer := uuid.New()
	ctxAuthor := withProfile(context.Background(), uuid.New(), author)
	ctxViewer := withProfile(context.Background(), uuid.New(), viewer)
	text := "react"
	created, err := client.CreateStory(ctxAuthor, &storyv1.CreateStoryRequest{
		Type: "text", TextContent: &text, Visibility: "everyone",
	})
	require.NoError(t, err)
	storyID := created.GetStory().GetId()

	_, err = client.ReactToStory(ctxViewer, &storyv1.ReactToStoryRequest{
		StoryId: storyID,
		Emoji:   "🔥",
	})
	require.NoError(t, err)

	err = callGetStoryReactions(client, ctxAuthor, storyID)
	require.NoError(t, err, "author must list reactions via GetStoryReactions")

	err = callGetStoryReactions(client, ctxViewer, storyID)
	require.Equal(t, codes.PermissionDenied, status.Code(err),
		"non-author must not call GetStoryReactions")
}

func TestMarkViewed_anonymousRejectedWithoutSubscription(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	author := uuid.New()
	viewer := uuid.New()
	ctxAuthor := withProfile(context.Background(), uuid.New(), author)
	ctxViewer := withProfile(context.Background(), uuid.New(), viewer)
	text := "anon view"
	created, err := client.CreateStory(ctxAuthor, &storyv1.CreateStoryRequest{
		Type: "text", TextContent: &text, Visibility: "everyone",
	})
	require.NoError(t, err)

	_, err = client.MarkViewed(ctxViewer, &storyv1.MarkViewedRequest{
		StoryId:   created.GetStory().GetId(),
		Anonymous: true,
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err),
		"anonymous view requires active Premium subscription per stories.md")
}

func TestGetHighlights_filtersByHighlightVisibility(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, cleanup := startStoryGRPC(t)
	defer cleanup()

	author := uuid.New()
	stranger := uuid.New()
	ctxAuthor := withProfile(context.Background(), uuid.New(), author)
	ctxStranger := withProfile(context.Background(), uuid.New(), stranger)

	text := "highlight me"
	created, err := client.CreateStory(ctxAuthor, &storyv1.CreateStoryRequest{
		Type: "text", TextContent: &text, Visibility: "everyone",
	})
	require.NoError(t, err)

	hl, err := client.CreateHighlight(ctxAuthor, &storyv1.CreateHighlightRequest{Name: "Friends only"})
	require.NoError(t, err)
	_, err = client.AddToHighlight(ctxAuthor, &storyv1.AddToHighlightRequest{
		HighlightId: hl.GetHighlight().GetId(),
		StoryId:     created.GetStory().GetId(),
	})
	require.NoError(t, err)

	_, err = st.Pool.Exec(context.Background(),
		`UPDATE highlights SET visibility = 'friends' WHERE id = $1`, hl.GetHighlight().GetId())
	require.NoError(t, err)

	list, err := client.GetHighlights(ctxStranger, &storyv1.GetHighlightsRequest{
		ProfileId: author.String(),
	})
	require.NoError(t, err)
	require.Empty(t, list.GetHighlightList().GetHighlights(),
		"GetHighlights must filter by highlight visibility independent of story visibility")
}

func TestCreateLookingForParty_enforcesVisibilityFloor(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, cleanup := startStoryGRPCWithFriends(t)
	defer cleanup()

	author := uuid.New()
	ctx := withProfile(context.Background(), uuid.New(), author)

	// Per stories.md: LFP visibility cannot be narrower than the author's story privacy setting.
	_, err := client.CreateLookingForParty(ctx, &storyv1.CreateLookingForPartyRequest{
		CriteriaJson: `{"game_id":"dota-2","visibility":"custom"}`,
	})
	require.Equal(t, codes.InvalidArgument, status.Code(err),
		"LFP story more restrictive than story privacy must be rejected")
}
