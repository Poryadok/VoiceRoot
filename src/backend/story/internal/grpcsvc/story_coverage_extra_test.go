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

type mockStoryPrivacy struct {
	audience privacy.Audience
}

func (m mockStoryPrivacy) ShowStoriesAudience(context.Context, uuid.UUID) (privacy.Audience, error) {
	return m.audience, nil
}

func startStoryGRPCWithPrivacy(t *testing.T, audience privacy.Audience) (storyv1.StoryServiceClient, func()) {
	t.Helper()
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "storyprivacy", "")
	_, err := pool.Exec(ctx, migrationSQL(t))
	require.NoError(t, err)

	st := &store.StoryStore{Pool: pool}
	svc := grpcsvc.NewStoryGRPC(st)
	checker := mockFriendChecker{}
	svc.Friends = checker
	svc.Audience = checker
	svc.Privacy = mockStoryPrivacy{audience: audience}

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

func TestCreateLookingForParty_allowsMatchingVisibility(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, cleanup := startStoryGRPCWithPrivacy(t, privacy.EveryoneWithGuests())
	defer cleanup()

	author := uuid.New()
	ctx := withProfile(context.Background(), uuid.New(), author)
	resp, err := client.CreateLookingForParty(ctx, &storyv1.CreateLookingForPartyRequest{
		CriteriaJson: `{"game_id":"dota-2","visibility":"everyone"}`,
	})
	require.NoError(t, err)
	require.True(t, resp.GetStory().GetIsLookingForParty())
}

func TestUpdateHighlight_visibilityOnly(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	profile := uuid.New()
	ctx := withProfile(context.Background(), uuid.New(), profile)
	hl, err := client.CreateHighlight(ctx, &storyv1.CreateHighlightRequest{Name: "Mix"})
	require.NoError(t, err)

	vis := "friends"
	updated, err := client.UpdateHighlight(ctx, &storyv1.UpdateHighlightRequest{
		HighlightId: hl.GetHighlight().GetId(),
		Visibility:  &vis,
	})
	require.NoError(t, err)
	require.Equal(t, "friends", updated.GetHighlight().GetVisibility())
}

func TestGetStory_notFound(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	ctx := withProfile(context.Background(), uuid.New(), uuid.New())
	_, err := client.GetStory(ctx, &storyv1.GetStoryRequest{StoryId: uuid.NewString()})
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestReactToStory_requiresEmoji(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	author := uuid.New()
	viewer := uuid.New()
	ctxAuthor := withProfile(context.Background(), uuid.New(), author)
	ctxViewer := withProfile(context.Background(), uuid.New(), viewer)
	text := "react"
	created, err := client.CreateStory(ctxAuthor, &storyv1.CreateStoryRequest{
		Type: "text", TextContent: &text, Visibility: "everyone",
	})
	require.NoError(t, err)

	_, err = client.ReactToStory(ctxViewer, &storyv1.ReactToStoryRequest{
		StoryId: created.GetStory().GetId(),
		Emoji:   "  ",
	})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestGetHighlights_everyoneVisibleToStranger(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	author := uuid.New()
	stranger := uuid.New()
	ctxAuthor := withProfile(context.Background(), uuid.New(), author)
	ctxStranger := withProfile(context.Background(), uuid.New(), stranger)

	_, err := client.CreateHighlight(ctxAuthor, &storyv1.CreateHighlightRequest{
		Name:       "Public",
		Visibility: "everyone",
	})
	require.NoError(t, err)

	list, err := client.GetHighlights(ctxStranger, &storyv1.GetHighlightsRequest{ProfileId: author.String()})
	require.NoError(t, err)
	require.Len(t, list.GetHighlightList().GetHighlights(), 1)
}

func TestHighlight_addRemoveDeleteFlow(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	profile := uuid.New()
	ctx := withProfile(context.Background(), uuid.New(), profile)
	text := "hl story"
	story, err := client.CreateStory(ctx, &storyv1.CreateStoryRequest{
		Type: "text", TextContent: &text, Visibility: "friends",
	})
	require.NoError(t, err)

	hl, err := client.CreateHighlight(ctx, &storyv1.CreateHighlightRequest{Name: "Flow"})
	require.NoError(t, err)

	_, err = client.AddToHighlight(ctx, &storyv1.AddToHighlightRequest{
		HighlightId: hl.GetHighlight().GetId(),
		StoryId:     story.GetStory().GetId(),
	})
	require.NoError(t, err)

	_, err = client.RemoveFromHighlight(ctx, &storyv1.RemoveFromHighlightRequest{
		HighlightId: hl.GetHighlight().GetId(),
		StoryId:     story.GetStory().GetId(),
	})
	require.NoError(t, err)

	_, err = client.DeleteHighlight(ctx, &storyv1.DeleteHighlightRequest{HighlightId: hl.GetHighlight().GetId()})
	require.NoError(t, err)
}

func TestCreateStory_invalidMediaFileID(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	profile := uuid.New()
	ctx := withProfile(context.Background(), uuid.New(), profile)
	bad := "not-uuid"
	_, err := client.CreateStory(ctx, &storyv1.CreateStoryRequest{
		Type:        "photo",
		MediaFileId: &bad,
		Visibility:  "everyone",
	})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestDeleteStory_notFound(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	ctx := withProfile(context.Background(), uuid.New(), uuid.New())
	_, err := client.DeleteStory(ctx, &storyv1.DeleteStoryRequest{StoryId: uuid.NewString()})
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestGetArchive_ownProfile(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	profile := uuid.New()
	ctx := withProfile(context.Background(), uuid.New(), profile)
	arch, err := client.GetArchive(ctx, &storyv1.GetArchiveRequest{})
	require.NoError(t, err)
	require.NotNil(t, arch.GetStoryList())
}

func TestUpdateHighlight_requiresNameOrVisibility(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	profile := uuid.New()
	ctx := withProfile(context.Background(), uuid.New(), profile)
	hl, err := client.CreateHighlight(ctx, &storyv1.CreateHighlightRequest{Name: "Keep"})
	require.NoError(t, err)

	_, err = client.UpdateHighlight(ctx, &storyv1.UpdateHighlightRequest{
		HighlightId: hl.GetHighlight().GetId(),
	})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestCreateLookingForParty_defaultVisibilityEveryone(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, cleanup := startStoryGRPCWithPrivacy(t, privacy.FriendsAndFoF())
	defer cleanup()

	author := uuid.New()
	ctx := withProfile(context.Background(), uuid.New(), author)
	resp, err := client.CreateLookingForParty(ctx, &storyv1.CreateLookingForPartyRequest{
		CriteriaJson: `{"game_id":"dota-2"}`,
	})
	require.NoError(t, err)
	require.Equal(t, "everyone", resp.GetStory().GetVisibility())
}

func TestGetStoryReactions_notFound(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	ctx := withProfile(context.Background(), uuid.New(), uuid.New())
	_, err := client.GetStoryReactions(ctx, &storyv1.GetStoryReactionsRequest{StoryId: uuid.NewString()})
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestCreateHighlight_visibilityEnum(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	profile := uuid.New()
	ctx := withProfile(context.Background(), uuid.New(), profile)
	friends := storyv1.StoryAudience_STORY_AUDIENCE_FRIENDS
	resp, err := client.CreateHighlight(ctx, &storyv1.CreateHighlightRequest{
		Name:           "Enum vis",
		VisibilityEnum: &friends,
	})
	require.NoError(t, err)
	require.Equal(t, "friends", resp.GetHighlight().GetVisibility())
}
