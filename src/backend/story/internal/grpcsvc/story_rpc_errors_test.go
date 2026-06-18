package grpcsvc_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	storyv1 "voice.app/voice/story/v1"
)

func TestStoryRPCs_invalidIDs(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	ctx := withProfile(context.Background(), uuid.New(), uuid.New())
	bad := "not-a-uuid"

	_, err := client.GetStory(ctx, &storyv1.GetStoryRequest{StoryId: bad})
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	_, err = client.DeleteStory(ctx, &storyv1.DeleteStoryRequest{StoryId: bad})
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	_, err = client.MarkViewed(ctx, &storyv1.MarkViewedRequest{StoryId: bad})
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	_, err = client.GetViewers(ctx, &storyv1.GetViewersRequest{StoryId: bad})
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	_, err = client.ReactToStory(ctx, &storyv1.ReactToStoryRequest{StoryId: bad, Emoji: "x"})
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	_, err = client.GetStoryReactions(ctx, &storyv1.GetStoryReactionsRequest{StoryId: bad})
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	_, err = client.ReplyToStory(ctx, &storyv1.ReplyToStoryRequest{StoryId: bad, Text: "hi"})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))

	_, err = client.GetHighlights(ctx, &storyv1.GetHighlightsRequest{ProfileId: bad})
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	_, err = client.GetProfileStories(ctx, &storyv1.GetProfileStoriesRequest{ProfileId: bad})
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	_, err = client.UpdateHighlight(ctx, &storyv1.UpdateHighlightRequest{HighlightId: bad})
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	_, err = client.DeleteHighlight(ctx, &storyv1.DeleteHighlightRequest{HighlightId: bad})
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	_, err = client.AddToHighlight(ctx, &storyv1.AddToHighlightRequest{HighlightId: bad, StoryId: uuid.NewString()})
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	_, err = client.RemoveFromHighlight(ctx, &storyv1.RemoveFromHighlightRequest{HighlightId: bad, StoryId: uuid.NewString()})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestCreateStory_requiresType(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	ctx := withProfile(context.Background(), uuid.New(), uuid.New())
	_, err := client.CreateStory(ctx, &storyv1.CreateStoryRequest{Visibility: "everyone"})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestCreateStory_videoAndCloseFriendsEnum(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	profile := uuid.New()
	ctx := withProfile(context.Background(), uuid.New(), profile)
	video := storyv1.StoryMediaType_STORY_MEDIA_TYPE_VIDEO
	closeFriends := storyv1.StoryAudience_STORY_AUDIENCE_CLOSE_FRIENDS
	text := "vid"
	resp, err := client.CreateStory(ctx, &storyv1.CreateStoryRequest{
		TypeEnum:       &video,
		VisibilityEnum: &closeFriends,
		TextContent:    &text,
	})
	require.NoError(t, err)
	require.Equal(t, "video", resp.GetStory().GetType())
}

func TestCreateLookingForParty_invalidCriteriaJSON(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, cleanup := startStoryGRPCWithFriends(t)
	defer cleanup()

	ctx := withProfile(context.Background(), uuid.New(), uuid.New())
	_, err := client.CreateLookingForParty(ctx, &storyv1.CreateLookingForPartyRequest{CriteriaJson: "{"})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestReplyToStory_requiresText(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, cleanup := startStoryGRPCWithClients(t)
	defer cleanup()

	author := uuid.New()
	replier := uuid.New()
	ctxAuthor := withProfile(context.Background(), uuid.New(), author)
	ctxReplier := withProfile(context.Background(), uuid.New(), replier)
	text := "target"
	created, err := client.CreateStory(ctxAuthor, &storyv1.CreateStoryRequest{
		Type: "text", TextContent: &text, Visibility: "everyone",
	})
	require.NoError(t, err)

	_, err = client.ReplyToStory(ctxReplier, &storyv1.ReplyToStoryRequest{
		StoryId: created.GetStory().GetId(),
	})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestGetStory_customVisibilityHiddenFromStranger(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	author := uuid.New()
	stranger := uuid.New()
	ctxAuthor := withProfile(context.Background(), uuid.New(), author)
	ctxStranger := withProfile(context.Background(), uuid.New(), stranger)
	text := "custom"
	created, err := client.CreateStory(ctxAuthor, &storyv1.CreateStoryRequest{
		Type: "text", TextContent: &text, Visibility: "custom",
	})
	require.NoError(t, err)

	_, err = client.GetStory(ctxStranger, &storyv1.GetStoryRequest{StoryId: created.GetStory().GetId()})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestGetArchive_otherProfileDenied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	ctx := withProfile(context.Background(), uuid.New(), uuid.New())
	_, err := client.GetArchive(ctx, &storyv1.GetArchiveRequest{ProfileId: uuid.NewString()})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestCreateHighlight_requiresName(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	ctx := withProfile(context.Background(), uuid.New(), uuid.New())
	_, err := client.CreateHighlight(ctx, &storyv1.CreateHighlightRequest{})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestAddToHighlight_foreignStoryDenied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	owner := uuid.New()
	other := uuid.New()
	ctxOwner := withProfile(context.Background(), uuid.New(), owner)
	ctxOther := withProfile(context.Background(), uuid.New(), other)

	hl, err := client.CreateHighlight(ctxOwner, &storyv1.CreateHighlightRequest{Name: "Mine"})
	require.NoError(t, err)

	text := "other story"
	story, err := client.CreateStory(ctxOther, &storyv1.CreateStoryRequest{
		Type: "text", TextContent: &text, Visibility: "everyone",
	})
	require.NoError(t, err)

	_, err = client.AddToHighlight(ctxOwner, &storyv1.AddToHighlightRequest{
		HighlightId: hl.GetHighlight().GetId(),
		StoryId:     story.GetStory().GetId(),
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestGetViewers_expiredStoryEmpty(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, cleanup := startStoryGRPC(t)
	defer cleanup()

	author := uuid.New()
	ctx := withProfile(context.Background(), uuid.New(), author)
	text := "expire viewers"
	created, err := client.CreateStory(ctx, &storyv1.CreateStoryRequest{
		Type: "text", TextContent: &text, Visibility: "everyone",
	})
	require.NoError(t, err)

	rowID, _ := uuid.Parse(created.GetStory().GetId())
	row, err := st.GetStory(context.Background(), rowID)
	require.NoError(t, err)
	_, err = st.MarkExpiredStories(context.Background(), row.ExpiresAt.Add(time.Second))
	require.NoError(t, err)

	viewers, err := client.GetViewers(ctx, &storyv1.GetViewersRequest{StoryId: created.GetStory().GetId()})
	require.NoError(t, err)
	require.Empty(t, viewers.GetViewerList().GetViewerProfileIds())
}

func TestCreateLookingForParty_invalidLFPVisibility(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, cleanup := startStoryGRPCWithFriends(t)
	defer cleanup()

	ctx := withProfile(context.Background(), uuid.New(), uuid.New())
	_, err := client.CreateLookingForParty(ctx, &storyv1.CreateLookingForPartyRequest{
		CriteriaJson: `{"visibility":"secret"}`,
	})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestUpdateHighlight_notFound(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	ctx := withProfile(context.Background(), uuid.New(), uuid.New())
	name := "nope"
	_, err := client.UpdateHighlight(ctx, &storyv1.UpdateHighlightRequest{
		HighlightId: uuid.NewString(),
		Name:        &name,
	})
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestRemoveFromHighlight_notFound(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	ctx := withProfile(context.Background(), uuid.New(), uuid.New())
	_, err := client.RemoveFromHighlight(ctx, &storyv1.RemoveFromHighlightRequest{
		HighlightId: uuid.NewString(),
		StoryId:     uuid.NewString(),
	})
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestMentionIDs_skipsInvalidUUIDs(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	profile := uuid.New()
	valid := uuid.New()
	ctx := withProfile(context.Background(), uuid.New(), profile)
	text := "mentions"
	resp, err := client.CreateStory(ctx, withMentionProfileIDs(&storyv1.CreateStoryRequest{
		Type: "text", TextContent: &text, Visibility: "everyone",
	}, valid))
	require.NoError(t, err)
	got := mentionIDsFromStoryJSON(resp.GetStory().GetMentionProfileIdsJson())
	require.Equal(t, []string{valid.String()}, got)
}

func TestCreateStory_friendsAudienceEnum(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	profile := uuid.New()
	ctx := withProfile(context.Background(), uuid.New(), profile)
	friends := storyv1.StoryAudience_STORY_AUDIENCE_FRIENDS
	text := "friends enum"
	resp, err := client.CreateStory(ctx, &storyv1.CreateStoryRequest{
		VisibilityEnum: &friends,
		TextContent:    &text,
		Type:           "text",
	})
	require.NoError(t, err)
	require.Equal(t, "friends", resp.GetStory().GetVisibility())
}

func TestMarkViewed_storyNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	ctx := withProfile(context.Background(), uuid.New(), uuid.New())
	_, err := client.MarkViewed(ctx, &storyv1.MarkViewedRequest{StoryId: uuid.NewString()})
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestReactToStory_storyNotVisible(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	author := uuid.New()
	stranger := uuid.New()
	ctxAuthor := withProfile(context.Background(), uuid.New(), author)
	ctxStranger := withProfile(context.Background(), uuid.New(), stranger)
	text := "hidden"
	created, err := client.CreateStory(ctxAuthor, &storyv1.CreateStoryRequest{
		Type: "text", TextContent: &text, Visibility: "friends",
	})
	require.NoError(t, err)

	_, err = client.ReactToStory(ctxStranger, &storyv1.ReactToStoryRequest{
		StoryId: created.GetStory().GetId(),
		Emoji:   "x",
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
