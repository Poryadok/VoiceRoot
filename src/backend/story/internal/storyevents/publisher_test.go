package storyevents

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type recordingPublisher struct {
	events []string
}

func (r *recordingPublisher) PublishStoryCreated(context.Context, string, string, string, string, []string) error {
	r.events = append(r.events, "StoryCreated")
	return nil
}
func (r *recordingPublisher) PublishStoryViewed(context.Context, string, string) error {
	r.events = append(r.events, "StoryViewed")
	return nil
}
func (r *recordingPublisher) PublishStoryReacted(context.Context, string, string, string) error {
	r.events = append(r.events, "StoryReacted")
	return nil
}
func (r *recordingPublisher) PublishStoryExpired(context.Context, string) error {
	r.events = append(r.events, "StoryExpired")
	return nil
}
func (r *recordingPublisher) PublishStoryHighlightCreated(context.Context, string, string) error {
	r.events = append(r.events, "StoryHighlightCreated")
	return nil
}
func (r *recordingPublisher) PublishStoryLfpCreated(context.Context, string, string, string) error {
	r.events = append(r.events, "StoryLfpCreated")
	return nil
}
func (r *recordingPublisher) Close() error { return nil }

func TestRecordingPublisher_emitsAllStoryEventTypes(t *testing.T) {
	t.Parallel()
	rec := &recordingPublisher{}
	var p Publisher = rec
	ctx := context.Background()

	require.NoError(t, p.PublishStoryCreated(ctx, "story-1", "author-1", "text", "", nil))
	require.NoError(t, p.PublishStoryViewed(ctx, "story-1", "viewer-1"))
	require.NoError(t, p.PublishStoryReacted(ctx, "story-1", "viewer-1", "🔥"))
	require.NoError(t, p.PublishStoryExpired(ctx, "story-1"))
	require.NoError(t, p.PublishStoryHighlightCreated(ctx, "hl-1", "author-1"))
	require.NoError(t, p.PublishStoryLfpCreated(ctx, "lfp-1", "author-1", `{"game_id":"dota-2"}`))

	require.Equal(t, []string{
		"StoryCreated",
		"StoryViewed",
		"StoryReacted",
		"StoryExpired",
		"StoryHighlightCreated",
		"StoryLfpCreated",
	}, rec.events)
}

func TestJetStreamPublisher_implementsPublisher(t *testing.T) {
	t.Parallel()
	var _ Publisher = (*JetStreamPublisher)(nil)
}
