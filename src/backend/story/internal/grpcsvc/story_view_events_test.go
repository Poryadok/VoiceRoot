package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/story/internal/storyevents"
)

type persistedStoryView struct {
	storyID     uuid.UUID
	viewerID    uuid.UUID
	isAnonymous bool
}

type storyViewPersistenceFake struct {
	viewCount int
	views     map[uuid.UUID]persistedStoryView
}

func (f *storyViewPersistenceFake) MarkViewed(_ context.Context, storyID, viewerID uuid.UUID, anonymous bool) error {
	if f.views == nil {
		f.views = make(map[uuid.UUID]persistedStoryView)
	}
	if _, exists := f.views[viewerID]; !exists {
		f.viewCount++
	}
	f.views[viewerID] = persistedStoryView{
		storyID: storyID, viewerID: viewerID, isAnonymous: anonymous,
	}
	return nil
}

type publishedStoryView struct {
	storyID         string
	viewerProfileID string
}

type storyViewPublisherFake struct {
	views []publishedStoryView
}

func (f *storyViewPublisherFake) PublishStoryCreated(context.Context, string, string, string, string, []string) error {
	return nil
}

func (f *storyViewPublisherFake) PublishStoryViewed(_ context.Context, storyID, viewerProfileID string) error {
	f.views = append(f.views, publishedStoryView{storyID: storyID, viewerProfileID: viewerProfileID})
	return nil
}

func (f *storyViewPublisherFake) PublishStoryReacted(context.Context, string, string, string) error {
	return nil
}

func (f *storyViewPublisherFake) PublishStoryExpired(context.Context, string) error { return nil }

func (f *storyViewPublisherFake) PublishStoryHighlightCreated(context.Context, string, string) error {
	return nil
}

func (f *storyViewPublisherFake) PublishStoryLfpCreated(context.Context, string, string, string) error {
	return nil
}

func (f *storyViewPublisherFake) PublishStoryLfpResponse(context.Context, string, string, string, string) error {
	return nil
}

func (f *storyViewPublisherFake) Close() error { return nil }

var _ storyevents.Publisher = (*storyViewPublisherFake)(nil)

func TestPersistAndPublishStoryView_anonymousPersistsButOmitsViewerFromEvent(t *testing.T) {
	t.Parallel()

	storyID := uuid.New()
	viewerID := uuid.New()
	persistence := &storyViewPersistenceFake{}
	publisher := &storyViewPublisherFake{}

	err := persistAndPublishStoryView(
		context.Background(),
		storyID,
		viewerID,
		true,
		persistence.MarkViewed,
		publisher,
	)
	require.NoError(t, err)
	require.Equal(t, 1, persistence.viewCount, "anonymous Premium view must increment the view counter")
	require.Equal(t, persistedStoryView{
		storyID: storyID, viewerID: viewerID, isAnonymous: true,
	}, persistence.views[viewerID], "anonymous Premium view must retain its durable anonymous record")
	require.Equal(t, []publishedStoryView{{
		storyID: storyID.String(), viewerProfileID: "",
	}}, publisher.views, "anonymous story.viewed must omit viewer_profile_id")
}

func TestPersistAndPublishStoryView_regularViewRetainsViewerInEvent(t *testing.T) {
	t.Parallel()

	storyID := uuid.New()
	viewerID := uuid.New()
	persistence := &storyViewPersistenceFake{}
	publisher := &storyViewPublisherFake{}

	err := persistAndPublishStoryView(
		context.Background(),
		storyID,
		viewerID,
		false,
		persistence.MarkViewed,
		publisher,
	)
	require.NoError(t, err)
	require.Equal(t, 1, persistence.viewCount)
	require.Equal(t, persistedStoryView{
		storyID: storyID, viewerID: viewerID, isAnonymous: false,
	}, persistence.views[viewerID])
	require.Equal(t, []publishedStoryView{{
		storyID: storyID.String(), viewerProfileID: viewerID.String(),
	}}, publisher.views, "ordinary story.viewed must retain viewer_profile_id")
}
