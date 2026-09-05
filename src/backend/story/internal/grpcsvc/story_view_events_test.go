package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/story/internal/storyevents"
)

type recordedStoryView struct {
	storyID   uuid.UUID
	viewerID  uuid.UUID
	anonymous bool
}

type storyViewPersistenceFake struct {
	views []recordedStoryView
}

func (f *storyViewPersistenceFake) MarkViewed(_ context.Context, storyID, viewerID uuid.UUID, anonymous bool) error {
	f.views = append(f.views, recordedStoryView{
		storyID: storyID, viewerID: viewerID, anonymous: anonymous,
	})
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
	require.Equal(t, []recordedStoryView{{
		storyID: storyID, viewerID: viewerID, anonymous: true,
	}}, persistence.views, "anonymous Premium view must still be persisted")
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
	require.Equal(t, []recordedStoryView{{
		storyID: storyID, viewerID: viewerID, anonymous: false,
	}}, persistence.views)
	require.Equal(t, []publishedStoryView{{
		storyID: storyID.String(), viewerProfileID: viewerID.String(),
	}}, publisher.views, "ordinary story.viewed must retain viewer_profile_id")
}
