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
	"voice/backend/story/internal/storyevents"
	"voice/backend/pkg/integrationtest"

	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	storyv1 "voice.app/voice/story/v1"
)

type mockChatClient struct{}

func (mockChatClient) CreateDM(_ context.Context, _ *chatv1.CreateDMRequest) (*chatv1.CreateDMResponse, error) {
	return &chatv1.CreateDMResponse{Chat: &chatv1.Chat{Id: "dm-chat-1"}}, nil
}

type mockMessagingClient struct{}

func (mockMessagingClient) SendMessage(_ context.Context, _ *messagingv1.SendMessageRequest) (*messagingv1.SendMessageResponse, error) {
	return &messagingv1.SendMessageResponse{Message: &messagingv1.Message{Id: "dm-msg-1"}}, nil
}

type mockPremiumChecker struct{}

func (mockPremiumChecker) HasActivePremium(context.Context, uuid.UUID) (bool, error) {
	return true, nil
}

func startStoryGRPCWithClients(t *testing.T) (storyv1.StoryServiceClient, func()) {
	t.Helper()
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "storyclients", "")
	_, err := pool.Exec(ctx, migrationSQL(t))
	require.NoError(t, err)

	st := &store.StoryStore{Pool: pool}
	svc := grpcsvc.NewStoryGRPC(st)
	svc.Friends = mockFriendChecker{}
	svc.Chat = mockChatClient{}
	svc.Messaging = mockMessagingClient{}
	svc.Subscriptions = mockPremiumChecker{}
	svc.Events = storyevents.NoopPublisher{}

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

func TestReplyToStory_sendsPrivateDM(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, cleanup := startStoryGRPCWithClients(t)
	defer cleanup()

	author := uuid.New()
	replier := uuid.New()
	ctxAuthor := withProfile(context.Background(), uuid.New(), author)
	ctxReplier := withProfile(context.Background(), uuid.New(), replier)
	text := "reply target"
	created, err := client.CreateStory(ctxAuthor, &storyv1.CreateStoryRequest{
		Type: "text", TextContent: &text, Visibility: "everyone",
	})
	require.NoError(t, err)

	resp, err := client.ReplyToStory(ctxReplier, &storyv1.ReplyToStoryRequest{
		StoryId: created.GetStory().GetId(),
		Text:    "private reply",
	})
	require.NoError(t, err)
	require.Equal(t, "dm-chat-1", resp.GetChatId())
	require.Equal(t, "dm-msg-1", resp.GetMessageId())
}

func TestReplyToStory_ownStoryRejected(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, cleanup := startStoryGRPCWithClients(t)
	defer cleanup()

	author := uuid.New()
	ctx := withProfile(context.Background(), uuid.New(), author)
	text := "mine"
	created, err := client.CreateStory(ctx, &storyv1.CreateStoryRequest{
		Type: "text", TextContent: &text, Visibility: "everyone",
	})
	require.NoError(t, err)

	_, err = client.ReplyToStory(ctx, &storyv1.ReplyToStoryRequest{
		StoryId: created.GetStory().GetId(),
		Text:    "nope",
	})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestMarkViewed_anonymousAllowedWithPremium(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, cleanup := startStoryGRPCWithClients(t)
	defer cleanup()

	author := uuid.New()
	viewer := uuid.New()
	ctxAuthor := withProfile(context.Background(), uuid.New(), author)
	ctxViewer := withProfile(context.Background(), uuid.New(), viewer)
	text := "premium anon"
	created, err := client.CreateStory(ctxAuthor, &storyv1.CreateStoryRequest{
		Type: "text", TextContent: &text, Visibility: "everyone",
	})
	require.NoError(t, err)

	_, err = client.MarkViewed(ctxViewer, &storyv1.MarkViewedRequest{
		StoryId:   created.GetStory().GetId(),
		Anonymous: true,
	})
	require.NoError(t, err)
}

func TestCreateHighlight_persistsVisibility(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	profile := uuid.New()
	ctx := withProfile(context.Background(), uuid.New(), profile)
	resp, err := client.CreateHighlight(ctx, &storyv1.CreateHighlightRequest{
		Name:       "VIP",
		Visibility: "friends",
	})
	require.NoError(t, err)
	require.Equal(t, "friends", resp.GetHighlight().GetVisibility())
}

func TestGetStoryFeed_invalidCursor(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	author := uuid.New()
	ctx := withProfile(context.Background(), uuid.New(), author)
	_, err := client.GetStoryFeed(ctx, &storyv1.GetStoryFeedRequest{
		Page: &commonv1.CursorPageRequest{Cursor: "not-a-valid-cursor"},
	})
	require.Error(t, err)
}

func TestCreateStory_typeEnumAndMediaFile(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	profile := uuid.New()
	ctx := withProfile(context.Background(), uuid.New(), profile)
	mediaID := uuid.New().String()
	text := "photo story"
	enum := storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO
	audience := storyv1.StoryAudience_STORY_AUDIENCE_PUBLIC
	resp, err := client.CreateStory(ctx, &storyv1.CreateStoryRequest{
		TypeEnum:      &enum,
		VisibilityEnum: &audience,
		MediaFileId:   &mediaID,
		TextContent:   &text,
	})
	require.NoError(t, err)
	require.Equal(t, "photo", resp.GetStory().GetType())
	require.Equal(t, "everyone", resp.GetStory().GetVisibility())
}

func TestReplyToStory_notConfigured(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	client, _, cleanup := startStoryGRPC(t)
	defer cleanup()

	author := uuid.New()
	replier := uuid.New()
	ctxAuthor := withProfile(context.Background(), uuid.New(), author)
	ctxReplier := withProfile(context.Background(), uuid.New(), replier)
	text := "no clients"
	created, err := client.CreateStory(ctxAuthor, &storyv1.CreateStoryRequest{
		Type: "text", TextContent: &text, Visibility: "everyone",
	})
	require.NoError(t, err)

	_, err = client.ReplyToStory(ctxReplier, &storyv1.ReplyToStoryRequest{
		StoryId: created.GetStory().GetId(),
		Text:    "hi",
	})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}
