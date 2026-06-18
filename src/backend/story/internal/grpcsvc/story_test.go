package grpcsvc_test

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"

	grpcsvc "voice/backend/story/internal/grpcsvc"
	"voice/backend/story/internal/store"
	"voice/backend/pkg/integrationtest"

	storyv1 "voice.app/voice/story/v1"
)

func migrationSQL(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations", "story_db")
	b1, err := os.ReadFile(filepath.Join(dir, "000001_init.up.sql"))
	require.NoError(t, err)
	b2, err := os.ReadFile(filepath.Join(dir, "000002_visibility_audience.up.sql"))
	require.NoError(t, err)
	return string(b1) + "\n" + string(b2)
}

func startStoryGRPC(t *testing.T) (storyv1.StoryServiceClient, *store.StoryStore, func()) {
	t.Helper()
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "storygrpc", "")
	_, err := pool.Exec(ctx, migrationSQL(t))
	require.NoError(t, err)

	st := &store.StoryStore{Pool: pool}
	svc := grpcsvc.NewStoryGRPC(st)

	lis := bufconn.Listen(1024 * 1024)
	s := grpc.NewServer()
	storyv1.RegisterStoryServiceServer(s, svc)
	go func() { _ = s.Serve(lis) }()

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	cleanup := func() {
		s.Stop()
		_ = conn.Close()
	}
	return storyv1.NewStoryServiceClient(conn), st, cleanup
}

func withProfile(ctx context.Context, accountID, profileID uuid.UUID) context.Context {
	md := metadata.Pairs(
		"x-voice-user-id", accountID.String(),
		"x-voice-profile-id", profileID.String(),
	)
	return metadata.NewOutgoingContext(ctx, md)
}

func TestCreateStory_text(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	profile := uuid.New()
	ctx := withProfile(context.Background(), uuid.New(), profile)
	text := "phase 17"
	resp, err := client.CreateStory(ctx, &storyv1.CreateStoryRequest{
		Type:        "text",
		TextContent: &text,
		Visibility:  "friends",
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.GetStory().GetId())
	require.Equal(t, profile.String(), resp.GetStory().GetAuthorProfileId())
	require.Equal(t, "text", resp.GetStory().GetType())
}

func TestGetStoryFeed_privacyFilterFriendsOnly(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	author := uuid.New()
	stranger := uuid.New()
	ctxAuthor := withProfile(context.Background(), uuid.New(), author)
	text := "friends only"
	created, err := client.CreateStory(ctxAuthor, &storyv1.CreateStoryRequest{
		Type:        "text",
		TextContent: &text,
		Visibility:  "friends",
	})
	require.NoError(t, err)

	ctxStranger := withProfile(context.Background(), uuid.New(), stranger)
	feed, err := client.GetStoryFeed(ctxStranger, &storyv1.GetStoryFeedRequest{})
	require.NoError(t, err)
	for _, s := range feed.GetStories() {
		require.NotEqual(t, created.GetStory().GetId(), s.GetId(),
			"non-friend must not see friends-only story in feed")
	}
}

func TestMarkViewed_recordsView(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	author := uuid.New()
	viewer := uuid.New()
	ctxAuthor := withProfile(context.Background(), uuid.New(), author)
	text := "view me"
	created, err := client.CreateStory(ctxAuthor, &storyv1.CreateStoryRequest{
		Type:        "text",
		TextContent: &text,
		Visibility:  "everyone",
	})
	require.NoError(t, err)

	ctxViewer := withProfile(context.Background(), uuid.New(), viewer)
	_, err = client.MarkViewed(ctxViewer, &storyv1.MarkViewedRequest{
		StoryId: created.GetStory().GetId(),
	})
	require.NoError(t, err)

	viewers, err := client.GetViewers(ctxAuthor, &storyv1.GetViewersRequest{
		StoryId: created.GetStory().GetId(),
	})
	require.NoError(t, err)
	require.Contains(t, viewers.GetViewerList().GetViewerProfileIds(), viewer.String())
}

func TestCreateLookingForParty_setsLFPFlag(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	profile := uuid.New()
	ctx := withProfile(context.Background(), uuid.New(), profile)
	criteria := `{"game_id":"dota-2","mode":"5v5"}`
	resp, err := client.CreateLookingForParty(ctx, &storyv1.CreateLookingForPartyRequest{
		CriteriaJson: criteria,
	})
	require.NoError(t, err)
	require.True(t, resp.GetStory().GetIsLookingForParty())
	require.JSONEq(t, criteria, resp.GetStory().GetLfpCriteriaJson())
}

func TestGetHighlights_returnsProfileCollections(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	profile := uuid.New()
	ctx := withProfile(context.Background(), uuid.New(), profile)
	text := "for highlight"
	created, err := client.CreateStory(ctx, &storyv1.CreateStoryRequest{
		Type:        "text",
		TextContent: &text,
		Visibility:  "friends",
	})
	require.NoError(t, err)

	hl, err := client.CreateHighlight(ctx, &storyv1.CreateHighlightRequest{Name: "Wins"})
	require.NoError(t, err)
	require.NotEmpty(t, hl.GetHighlight().GetId())

	_, err = client.AddToHighlight(ctx, &storyv1.AddToHighlightRequest{
		HighlightId: hl.GetHighlight().GetId(),
		StoryId:     created.GetStory().GetId(),
	})
	require.NoError(t, err)

	list, err := client.GetHighlights(ctx, &storyv1.GetHighlightsRequest{
		ProfileId: profile.String(),
	})
	require.NoError(t, err)
	require.Len(t, list.GetHighlightList().GetHighlights(), 1)
	require.Contains(t, list.GetHighlightList().GetHighlights()[0].GetStoryIds(), created.GetStory().GetId())
}
