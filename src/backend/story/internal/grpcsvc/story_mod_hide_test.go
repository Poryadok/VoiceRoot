package grpcsvc_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"voice/backend/pkg/privacy"

	storyv1 "voice.app/voice/story/v1"
)

func withInternalModCtx(ctx context.Context, moderatorProfileID uuid.UUID) context.Context {
	ctx = metadata.AppendToOutgoingContext(ctx, "x-voice-internal", "true")
	return metadata.AppendToOutgoingContext(ctx, "x-voice-profile-id", moderatorProfileID.String())
}

func storyIDsContain(stories []*storyv1.Story, storyID string) bool {
	for _, s := range stories {
		if s.GetId() == storyID {
			return true
		}
	}
	return false
}

// ST-05: staff hide removes story from non-author GetStory / feeds; author retains GetStory.
func TestHideStoryFromFeed_hidesFromNonAuthorAndFeeds(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	author := uuid.New()
	viewer := uuid.New()
	moderator := uuid.New()
	authorCtx := withProfile(context.Background(), uuid.New(), author)
	viewerCtx := withProfile(context.Background(), uuid.New(), viewer)
	modCtx := withInternalModCtx(context.Background(), moderator)

	text := "moderation hide"
	created, err := client.CreateStory(authorCtx, &storyv1.CreateStoryRequest{
		Type: "text", TextContent: &text, Visibility: "everyone",
	})
	require.NoError(t, err)
	storyID := created.GetStory().GetId()

	_, err = client.GetStory(viewerCtx, &storyv1.GetStoryRequest{StoryId: storyID})
	require.NoError(t, err, "precondition: non-author can view before hide")

	_, err = client.HideStoryFromFeed(modCtx, &storyv1.HideStoryFromFeedRequest{StoryId: storyID})
	require.NoError(t, err)

	_, err = client.GetStory(viewerCtx, &storyv1.GetStoryRequest{StoryId: storyID})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	got, err := client.GetStory(authorCtx, &storyv1.GetStoryRequest{StoryId: storyID})
	require.NoError(t, err)
	require.Equal(t, storyID, got.GetStory().GetId())

	feed, err := client.GetStoryFeed(viewerCtx, &storyv1.GetStoryFeedRequest{})
	require.NoError(t, err)
	require.False(t, storyIDsContain(feed.GetStories(), storyID),
		"hidden story must not appear in GetStoryFeed")

	profileStories, err := client.GetProfileStories(viewerCtx, &storyv1.GetProfileStoriesRequest{
		ProfileId: author.String(),
	})
	require.NoError(t, err)
	require.False(t, storyIDsContain(profileStories.GetStoryList().GetStories(), storyID),
		"hidden story must not appear in GetProfileStories")
}

// ST-06: show_stories=Nobody floors CreateStory(everyone) → custom/nobody; non-author denied.
func TestCreateStory_capsVisibilityToNobodyFloor(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, cleanup := startStoryGRPCWithPrivacy(t, privacy.Nobody())
	defer cleanup()

	author := uuid.New()
	viewer := uuid.New()
	authorCtx := withProfile(context.Background(), uuid.New(), author)
	viewerCtx := withProfile(context.Background(), uuid.New(), viewer)

	text := "nobody floor"
	resp, err := client.CreateStory(authorCtx, &storyv1.CreateStoryRequest{
		Type: "text", TextContent: &text, Visibility: "everyone",
	})
	require.NoError(t, err)
	vis := resp.GetStory().GetVisibility()
	require.NotEqual(t, "everyone", vis,
		"CreateStory must cap visibility to show_stories floor (Nobody), not leave everyone")
	require.True(t, vis == "custom" || vis == "nobody",
		"expected nobody-floor storage as custom/nobody, got %q", vis)

	_, err = client.GetStory(viewerCtx, &storyv1.GetStoryRequest{StoryId: resp.GetStory().GetId()})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

// ST-06: show_stories=FriendsOnly floors CreateStory(everyone) → friends.
func TestCreateStory_capsVisibilityToFriendsPrivacyFloor(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, cleanup := startStoryGRPCWithPrivacy(t, privacy.FriendsOnly())
	defer cleanup()

	author := uuid.New()
	authorCtx := withProfile(context.Background(), uuid.New(), author)

	text := "friends floor"
	resp, err := client.CreateStory(authorCtx, &storyv1.CreateStoryRequest{
		Type: "text", TextContent: &text, Visibility: "everyone",
	})
	require.NoError(t, err)
	require.Equal(t, "friends", resp.GetStory().GetVisibility(),
		"CreateStory must cap visibility=everyone down to FriendsOnly show_stories floor")
}
