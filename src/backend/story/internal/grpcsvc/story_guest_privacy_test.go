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

func withGuestProfile(ctx context.Context, accountID, profileID uuid.UUID) context.Context {
	md := metadata.Pairs(
		"x-voice-user-id", accountID.String(),
		"x-voice-profile-id", profileID.String(),
		"x-voice-account-type", "guest",
	)
	return metadata.NewOutgoingContext(ctx, md)
}

func TestGetStory_GuestViewerDeniedWhenGuestAudienceExcluded(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, cleanup := startStoryGRPCWithPrivacy(t, privacy.FriendsOnly())
	defer cleanup()

	author := uuid.New()
	viewer := uuid.New()
	viewerAcct := uuid.New()

	authorCtx := withProfile(context.Background(), uuid.New(), author)
	text := "guest privacy floor"
	created, err := client.CreateStory(authorCtx, &storyv1.CreateStoryRequest{
		Type:        "text",
		TextContent: &text,
		Visibility:  "everyone",
	})
	require.NoError(t, err)

	guestCtx := withGuestProfile(context.Background(), viewerAcct, viewer)
	_, err = client.GetStory(guestCtx, &storyv1.GetStoryRequest{StoryId: created.GetStory().GetId()})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
