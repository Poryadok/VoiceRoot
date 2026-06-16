package grpcsvc_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	storyv1 "voice.app/voice/story/v1"
)

func TestDeleteStory_andGetArchive(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, cleanup := startStoryGRPC(t)
	defer cleanup()

	profile := uuid.New()
	ctx := withProfile(context.Background(), uuid.New(), profile)
	text := "archive flow"
	created, err := client.CreateStory(ctx, &storyv1.CreateStoryRequest{
		Type: "text", TextContent: &text, Visibility: "friends",
	})
	require.NoError(t, err)

	_, err = client.DeleteStory(ctx, &storyv1.DeleteStoryRequest{StoryId: created.GetStory().GetId()})
	require.NoError(t, err)

	rowID, _ := uuid.Parse(created.GetStory().GetId())
	_, err = st.GetStory(context.Background(), rowID)
	require.Error(t, err)
}

func TestReactToStory_andGetStory(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	author := uuid.New()
	viewer := uuid.New()
	ctxAuthor := withProfile(context.Background(), uuid.New(), author)
	ctxViewer := withProfile(context.Background(), uuid.New(), viewer)
	text := "react path"
	created, err := client.CreateStory(ctxAuthor, &storyv1.CreateStoryRequest{
		Type: "text", TextContent: &text, Visibility: "everyone",
	})
	require.NoError(t, err)

	_, err = client.ReactToStory(ctxViewer, &storyv1.ReactToStoryRequest{
		StoryId: created.GetStory().GetId(),
		Emoji:   "👍",
	})
	require.NoError(t, err)

	got, err := client.GetStory(ctxViewer, &storyv1.GetStoryRequest{StoryId: created.GetStory().GetId()})
	require.NoError(t, err)
	require.Equal(t, created.GetStory().GetId(), got.GetStory().GetId())
}

func TestGetArchive_afterExpiry(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, st, cleanup := startStoryGRPC(t)
	defer cleanup()

	profile := uuid.New()
	ctx := withProfile(context.Background(), uuid.New(), profile)
	text := "to archive"
	created, err := client.CreateStory(ctx, &storyv1.CreateStoryRequest{
		Type: "text", TextContent: &text, Visibility: "friends",
	})
	require.NoError(t, err)

	rowID, _ := uuid.Parse(created.GetStory().GetId())
	row, err := st.GetStory(context.Background(), rowID)
	require.NoError(t, err)
	_, err = st.MarkExpiredStories(context.Background(), row.ExpiresAt.Add(1))
	require.NoError(t, err)

	archive, err := client.GetArchive(ctx, &storyv1.GetArchiveRequest{})
	require.NoError(t, err)
	require.Len(t, archive.GetStoryList().GetStories(), 1)
}

func TestGetProfileStories_authorVisible(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	profile := uuid.New()
	ctx := withProfile(context.Background(), uuid.New(), profile)
	text := "profile"
	_, err := client.CreateStory(ctx, &storyv1.CreateStoryRequest{
		Type: "text", TextContent: &text, Visibility: "friends",
	})
	require.NoError(t, err)

	resp, err := client.GetProfileStories(ctx, &storyv1.GetProfileStoriesRequest{ProfileId: profile.String()})
	require.NoError(t, err)
	require.Len(t, resp.GetStoryList().GetStories(), 1)
}
