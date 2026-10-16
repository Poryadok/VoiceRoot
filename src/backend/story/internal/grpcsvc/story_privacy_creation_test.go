package grpcsvc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/pkg/privacy"
	grpcsvc "voice/backend/story/internal/grpcsvc"
	"voice/backend/story/internal/storyevents"

	storyv1 "voice.app/voice/story/v1"
)

type unavailableStoryPrivacy struct{}

func (unavailableStoryPrivacy) ShowStoriesAudience(context.Context, uuid.UUID) (privacy.Audience, error) {
	return privacy.Audience{}, errors.New("user unavailable")
}

type storyCreationEventCounter struct {
	story, lfp int
}

func (e *storyCreationEventCounter) PublishStoryCreated(context.Context, string, string, string, string, []string) error {
	e.story++
	return nil
}

func (*storyCreationEventCounter) PublishStoryViewed(context.Context, string, string) error {
	return nil
}
func (*storyCreationEventCounter) PublishStoryReacted(context.Context, string, string, string) error {
	return nil
}
func (*storyCreationEventCounter) PublishStoryExpired(context.Context, string) error { return nil }
func (*storyCreationEventCounter) PublishStoryHighlightCreated(context.Context, string, string) error {
	return nil
}
func (e *storyCreationEventCounter) PublishStoryLfpCreated(context.Context, string, string, string) error {
	e.lfp++
	return nil
}
func (*storyCreationEventCounter) PublishStoryLfpResponse(context.Context, string, string, string, string) error {
	return nil
}
func (*storyCreationEventCounter) PublishStoryMention(context.Context, string, string, string) error {
	return nil
}

func (*storyCreationEventCounter) Close() error { return nil }

var _ storyevents.Publisher = (*storyCreationEventCounter)(nil)

func TestStoryCreationRejectsUnavailablePrivacyBeforePersistenceOrEvents(t *testing.T) {
	for _, tc := range []struct {
		name        string
		unavailable bool
	}{
		{name: "unwired checker"},
		{name: "User lookup unavailable", unavailable: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := grpcsvc.NewStoryGRPC(nil) // Any store call would fail with Internal.
			if tc.unavailable {
				svc.Privacy = unavailableStoryPrivacy{}
			}
			events := &storyCreationEventCounter{}
			svc.Events = events
			client, cleanup := startStoryGRPCFromService(t, svc)
			defer cleanup()
			ctx := withProfile(context.Background(), uuid.New(), uuid.New())
			text := "story"

			for _, req := range []*storyv1.CreateStoryRequest{
				{Type: "text", TextContent: &text, Visibility: "everyone"},
				{Type: "text", TextContent: &text},
			} {
				_, err := client.CreateStory(ctx, req)
				require.Equal(t, codes.Unavailable, status.Code(err))
			}
			_, err := client.CreateLookingForParty(ctx, &storyv1.CreateLookingForPartyRequest{
				CriteriaJson: `{"game_id":"dota-2","visibility":"everyone"}`,
			})
			require.Equal(t, codes.Unavailable, status.Code(err))
			require.Zero(t, events.story)
			require.Zero(t, events.lfp)
		})
	}
}
