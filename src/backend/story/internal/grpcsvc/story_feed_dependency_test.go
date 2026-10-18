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
	"voice/backend/story/internal/store"

	commonv1 "voice.app/voice/common/v1"
	storyv1 "voice.app/voice/story/v1"
)

type feedAuthorDependency struct {
	ids []uuid.UUID
	err error
}

func (f *feedAuthorDependency) ListFeedAuthorIDs(context.Context, uuid.UUID) ([]uuid.UUID, error) {
	return f.ids, f.err
}

func TestGetStoryFeed_AuthorLookupFailureDoesNotBroadenFeed(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	st := startStoryStore(t)
	viewer, foreign := uuid.New(), uuid.New()
	for _, author := range []uuid.UUID{viewer, viewer, foreign} {
		text := "visible story"
		_, err := st.CreateStory(context.Background(), store.CreateStoryInput{
			AuthorProfileID: author, Type: "text", TextContent: &text, Visibility: "everyone",
		})
		require.NoError(t, err)
	}
	authors := &feedAuthorDependency{ids: []uuid.UUID{viewer}}
	svc := grpcsvc.NewStoryGRPC(st)
	svc.FeedAuthors = authors
	svc.Privacy = mockStoryPrivacy{audience: privacy.EveryoneWithGuests()}
	client, cleanup := startStoryGRPCFromService(t, svc)
	defer cleanup()
	ctx := withProfile(context.Background(), uuid.New(), viewer)

	for _, tc := range []struct {
		name string
		err  error
		ids  []uuid.UUID
	}{
		{name: "unavailable with no authors", err: errors.New("private-social-address")},
		{name: "error with partial own authors", err: errors.New("private-social-address"), ids: []uuid.UUID{viewer}},
		{name: "deadline with partial foreign authors", err: status.Error(codes.DeadlineExceeded, "private-social-address"), ids: []uuid.UUID{viewer, foreign}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			authors.ids, authors.err = tc.ids, tc.err
			feed, err := client.GetStoryFeed(ctx, &storyv1.GetStoryFeedRequest{})
			require.Equal(t, codes.Unavailable, status.Code(err))
			require.Nil(t, feed, "a dependency failure must not return a global or partial feed")
			require.NotContains(t, err.Error(), "private-social-address")
		})
	}

	t.Run("recovery retains author-scoped pagination", func(t *testing.T) {
		authors.ids, authors.err = []uuid.UUID{viewer}, nil
		first, err := client.GetStoryFeed(ctx, &storyv1.GetStoryFeedRequest{
			Page: &commonv1.CursorPageRequest{PageSize: 1},
		})
		require.NoError(t, err)
		require.Len(t, first.GetStories(), 1)
		require.Equal(t, viewer.String(), first.GetStories()[0].GetAuthorProfileId())
		require.True(t, first.GetPage().GetHasMore())
		require.NotEmpty(t, first.GetNextCursor())
		second, err := client.GetStoryFeed(ctx, &storyv1.GetStoryFeedRequest{
			Page: &commonv1.CursorPageRequest{PageSize: 1, Cursor: first.GetNextCursor()},
		})
		require.NoError(t, err)
		require.Len(t, second.GetStories(), 1)
		require.Equal(t, viewer.String(), second.GetStories()[0].GetAuthorProfileId())
		require.NotEqual(t, first.GetStories()[0].GetId(), second.GetStories()[0].GetId())
		require.False(t, second.GetPage().GetHasMore())
	})
}
