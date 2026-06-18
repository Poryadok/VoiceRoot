package grpcsvc_test

import (
	"context"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	grpcsvc "voice/backend/story/internal/grpcsvc"
	"voice/backend/story/internal/store"
	"voice/backend/pkg/integrationtest"

	storyv1 "voice.app/voice/story/v1"
)

type mockFriendChecker struct{}

func (mockFriendChecker) IsFriend(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}

func (mockFriendChecker) AreFriends(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}

func (mockFriendChecker) AreFriendsOfFriends(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

func (mockFriendChecker) AreCoMembers(context.Context, uuid.UUID, uuid.UUID, []string) (bool, error) {
	return false, nil
}

func startStoryGRPCWithFriends(t *testing.T) (storyv1.StoryServiceClient, func()) {
	t.Helper()
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "storyfriends", "")
	_, err := pool.Exec(ctx, migrationSQL(t))
	require.NoError(t, err)

	st := &store.StoryStore{Pool: pool}
	svc := grpcsvc.NewStoryGRPC(st)
	checker := mockFriendChecker{}
	svc.Friends = checker
	svc.Audience = checker

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

func TestGetStoryFeed_friendsStoryVisibleWhenFriends(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, cleanup := startStoryGRPCWithFriends(t)
	defer cleanup()

	author := uuid.New()
	viewer := uuid.New()
	ctxAuthor := withProfile(context.Background(), uuid.New(), author)
	ctxViewer := withProfile(context.Background(), uuid.New(), viewer)
	text := "friends feed"
	created, err := client.CreateStory(ctxAuthor, &storyv1.CreateStoryRequest{
		Type: "text", TextContent: &text, Visibility: "friends",
	})
	require.NoError(t, err)

	feed, err := client.GetStoryFeed(ctxViewer, &storyv1.GetStoryFeedRequest{})
	require.NoError(t, err)
	found := false
	for _, s := range feed.GetStories() {
		if s.GetId() == created.GetStory().GetId() {
			found = true
		}
	}
	require.True(t, found, "friend checker should expose friends-only story in feed")
}
