package grpcsvc

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/pkg/privacy"

	storyv1 "voice.app/voice/story/v1"
)

func TestAudienceFromStoryRow_everyone(t *testing.T) {
	a := audienceFromStoryRow("everyone", nil)
	require.True(t, a.IsEveryoneShortcut())
}

func TestAudienceFromStoryRow_closeFriends(t *testing.T) {
	a := audienceFromStoryRow("close_friends", nil)
	require.True(t, a.FriendsOfFriends)
}

func TestAudienceToStoryVisibility_friendsAndFoF(t *testing.T) {
	vis, json := audienceToStoryVisibility(privacy.FriendsAndFoF())
	require.Equal(t, "close_friends", vis)
	require.Nil(t, json)
}

func TestVisibilityFromRequest_closeFriendsEnum(t *testing.T) {
	enum := storyv1.StoryAudience_STORY_AUDIENCE_CLOSE_FRIENDS
	vis, _ := visibilityFromRequest("", enum)
	require.Equal(t, "close_friends", vis)
}

func TestStoryAudienceString_custom(t *testing.T) {
	require.Equal(t, "custom", storyAudienceString(storyv1.StoryAudience_STORY_AUDIENCE_CUSTOM))
}

func TestGroupStoriesByAuthorCentric_ordersByLatest(t *testing.T) {
	now := timestamppb.Now()
	earlier := timestamppb.New(now.AsTime().Add(-time.Hour))
	stories := []*storyv1.Story{
		{Id: "s1", AuthorProfileId: "a", CreatedAt: earlier},
		{Id: "s2", AuthorProfileId: "b", CreatedAt: now},
		{Id: "s3", AuthorProfileId: "a", CreatedAt: now},
	}
	groups := groupStoriesByAuthorCentric(stories)
	require.Len(t, groups, 2)
	require.Equal(t, "b", groups[0].AuthorProfileId)
	require.Equal(t, "a", groups[1].AuthorProfileId)
	require.Len(t, groups[1].Stories, 2)
}
