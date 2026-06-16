package grpcsvc_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	storyv1 "voice.app/voice/story/v1"
)

func TestCreateStory_requiresProfile(t *testing.T) {
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()
	_, err := client.CreateStory(context.Background(), &storyv1.CreateStoryRequest{Type: "text"})
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestGetViewers_authorOnly(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	author := uuid.New()
	stranger := uuid.New()
	ctxAuthor := withProfile(context.Background(), uuid.New(), author)
	ctxStranger := withProfile(context.Background(), uuid.New(), stranger)
	text := "private viewers"
	created, err := client.CreateStory(ctxAuthor, &storyv1.CreateStoryRequest{
		Type: "text", TextContent: &text, Visibility: "everyone",
	})
	require.NoError(t, err)

	_, err = client.GetViewers(ctxStranger, &storyv1.GetViewersRequest{StoryId: created.GetStory().GetId()})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestHighlight_updateDeleteRemove(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	profile := uuid.New()
	ctx := withProfile(context.Background(), uuid.New(), profile)
	text := "hl item"
	created, err := client.CreateStory(ctx, &storyv1.CreateStoryRequest{
		Type: "text", TextContent: &text, Visibility: "friends",
	})
	require.NoError(t, err)

	hl, err := client.CreateHighlight(ctx, &storyv1.CreateHighlightRequest{Name: "A"})
	require.NoError(t, err)

	newName := "B"
	updated, err := client.UpdateHighlight(ctx, &storyv1.UpdateHighlightRequest{
		HighlightId: hl.GetHighlight().GetId(),
		Name:        &newName,
	})
	require.NoError(t, err)
	require.Equal(t, "B", updated.GetHighlight().GetName())

	_, err = client.AddToHighlight(ctx, &storyv1.AddToHighlightRequest{
		HighlightId: hl.GetHighlight().GetId(),
		StoryId:     created.GetStory().GetId(),
	})
	require.NoError(t, err)

	_, err = client.RemoveFromHighlight(ctx, &storyv1.RemoveFromHighlightRequest{
		HighlightId: hl.GetHighlight().GetId(),
		StoryId:     created.GetStory().GetId(),
	})
	require.NoError(t, err)

	_, err = client.DeleteHighlight(ctx, &storyv1.DeleteHighlightRequest{HighlightId: hl.GetHighlight().GetId()})
	require.NoError(t, err)
}

func TestCreateLookingForParty_requiresCriteria(t *testing.T) {
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()
	profile := uuid.New()
	ctx := withProfile(context.Background(), uuid.New(), profile)
	_, err := client.CreateLookingForParty(ctx, &storyv1.CreateLookingForPartyRequest{})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}
