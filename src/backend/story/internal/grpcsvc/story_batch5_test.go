package grpcsvc_test

import (
	"context"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	grpcsvc "voice/backend/story/internal/grpcsvc"
	"voice/backend/story/internal/store"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/privacy"

	storyv1 "voice.app/voice/story/v1"
)

type mockSocialGraph struct {
	friends    map[string]bool
	friendsFoF map[string]bool
}

func (m mockSocialGraph) IsFriend(_ context.Context, viewer, author uuid.UUID) (bool, error) {
	return m.AreFriends(context.Background(), viewer, author)
}

func (m mockSocialGraph) AreFriends(_ context.Context, profileA, profileB uuid.UUID) (bool, error) {
	if m.friends == nil {
		return false, nil
	}
	key := profileA.String() + "|" + profileB.String()
	return m.friends[key] || m.friends[profileB.String()+"|"+profileA.String()], nil
}

func (m mockSocialGraph) AreFriendsOfFriends(_ context.Context, profileA, profileB uuid.UUID) (bool, error) {
	if m.friendsFoF == nil {
		return false, nil
	}
	key := profileA.String() + "|" + profileB.String()
	return m.friendsFoF[key] || m.friendsFoF[profileB.String()+"|"+profileA.String()], nil
}

func (m mockSocialGraph) AreCoMembers(context.Context, uuid.UUID, uuid.UUID, []string) (bool, error) {
	return false, nil
}

type mockFeedAuthors struct {
	ids []uuid.UUID
}

func (m mockFeedAuthors) ListFeedAuthorIDs(_ context.Context, viewer uuid.UUID) ([]uuid.UUID, error) {
	if len(m.ids) == 0 {
		return []uuid.UUID{viewer}, nil
	}
	return m.ids, nil
}

func startStoryGRPCFromService(t *testing.T, svc *grpcsvc.StoryGRPC) (storyv1.StoryServiceClient, func()) {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	s := grpc.NewServer()
	storyv1.RegisterStoryServiceServer(s, svc)
	go func() { _ = s.Serve(lis) }()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	return storyv1.NewStoryServiceClient(conn), func() {
		s.Stop()
		_ = conn.Close()
	}
}

func startStoryStore(t *testing.T) *store.StoryStore {
	t.Helper()
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "storybatch5", "")
	_, err := pool.Exec(ctx, migrationSQL(t))
	require.NoError(t, err)
	return &store.StoryStore{Pool: pool}
}

func TestCreateStory_defaultVisibilityFromPrivacy(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, cleanup := startStoryGRPCWithPrivacy(t, privacy.EveryoneWithGuests())
	defer cleanup()

	author := uuid.New()
	ctx := withProfile(context.Background(), uuid.New(), author)
	text := "default vis"
	resp, err := client.CreateStory(ctx, &storyv1.CreateStoryRequest{
		Type: "text", TextContent: &text,
	})
	require.NoError(t, err)
	require.Equal(t, "everyone", resp.GetStory().GetVisibility())
}

func TestCreateStory_closeFriendsVisibilityStored(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	author := uuid.New()
	ctx := withProfile(context.Background(), uuid.New(), author)
	text := "close"
	closeFriends := storyv1.StoryAudience_STORY_AUDIENCE_CLOSE_FRIENDS
	resp, err := client.CreateStory(ctx, &storyv1.CreateStoryRequest{
		Type: "text", TextContent: &text, VisibilityEnum: &closeFriends,
	})
	require.NoError(t, err)
	require.Equal(t, "close_friends", resp.GetStory().GetVisibility())
}

func TestGetStory_closeFriendsVisibleToFoF(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	st := startStoryStore(t)
	author := uuid.New()
	fof := uuid.New()
	svc := grpcsvc.NewStoryGRPC(st)
	social := mockSocialGraph{
		friendsFoF: map[string]bool{fof.String() + "|" + author.String(): true},
	}
	svc.Audience = social
	svc.Friends = social

	client, cleanup := startStoryGRPCFromService(t, svc)
	defer cleanup()

	ctxAuthor := withProfile(context.Background(), uuid.New(), author)
	ctxFoF := withProfile(context.Background(), uuid.New(), fof)
	text := "fof story"
	closeFriends := storyv1.StoryAudience_STORY_AUDIENCE_CLOSE_FRIENDS
	created, err := client.CreateStory(ctxAuthor, &storyv1.CreateStoryRequest{
		Type: "text", TextContent: &text, VisibilityEnum: &closeFriends,
	})
	require.NoError(t, err)

	got, err := client.GetStory(ctxFoF, &storyv1.GetStoryRequest{StoryId: created.GetStory().GetId()})
	require.NoError(t, err)
	require.Equal(t, created.GetStory().GetId(), got.GetStory().GetId())
}

func TestGetStoryFeed_prefiltersNonFriendAuthor(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	st := startStoryStore(t)
	author := uuid.New()
	viewer := uuid.New()
	stranger := uuid.New()

	svc := grpcsvc.NewStoryGRPC(st)
	social := mockSocialGraph{
		friends: map[string]bool{viewer.String() + "|" + author.String(): true},
	}
	svc.Audience = social
	svc.Friends = social
	svc.FeedAuthors = mockFeedAuthors{ids: []uuid.UUID{viewer, author}}

	client, cleanup := startStoryGRPCFromService(t, svc)
	defer cleanup()

	ctxAuthor := withProfile(context.Background(), uuid.New(), author)
	ctxViewer := withProfile(context.Background(), uuid.New(), viewer)
	text := "feed prefilter"
	created, err := client.CreateStory(ctxAuthor, &storyv1.CreateStoryRequest{
		Type: "text", TextContent: &text, Visibility: "everyone",
	})
	require.NoError(t, err)

	strangerText := "stranger story"
	_, err = client.CreateStory(withProfile(context.Background(), uuid.New(), stranger), &storyv1.CreateStoryRequest{
		Type: "text", TextContent: &strangerText, Visibility: "everyone",
	})
	require.NoError(t, err)

	feed, err := client.GetStoryFeed(ctxViewer, &storyv1.GetStoryFeedRequest{})
	require.NoError(t, err)
	var foundAuthor, foundStranger bool
	for _, s := range feed.GetStories() {
		if s.GetId() == created.GetStory().GetId() {
			foundAuthor = true
		}
		if s.GetAuthorProfileId() == stranger.String() {
			foundStranger = true
		}
	}
	require.True(t, foundAuthor)
	require.False(t, foundStranger, "stranger author must be prefiltered from feed")
}

func TestGetStoryFeed_feedGroupsOnePerAuthor(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	author := uuid.New()
	ctx := withProfile(context.Background(), uuid.New(), author)
	for _, txt := range []string{"a", "b"} {
		text := txt
		_, err := client.CreateStory(ctx, &storyv1.CreateStoryRequest{
			Type: "text", TextContent: &text, Visibility: "everyone",
		})
		require.NoError(t, err)
	}

	feed, err := client.GetStoryFeed(ctx, &storyv1.GetStoryFeedRequest{})
	require.NoError(t, err)
	require.Len(t, feed.GetFeedGroups(), 1)
	require.Len(t, feed.GetFeedGroups()[0].GetStories(), 2)
}

func TestGetStory_customVisibilityDeniedToStranger(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	author := uuid.New()
	stranger := uuid.New()
	ctxAuthor := withProfile(context.Background(), uuid.New(), author)
	ctxStranger := withProfile(context.Background(), uuid.New(), stranger)
	text := "custom"
	created, err := client.CreateStory(ctxAuthor, &storyv1.CreateStoryRequest{
		Type: "text", TextContent: &text, Visibility: "custom",
	})
	require.NoError(t, err)

	_, err = client.GetStory(ctxStranger, &storyv1.GetStoryRequest{StoryId: created.GetStory().GetId()})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
