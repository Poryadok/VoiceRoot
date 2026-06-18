package main

import (
	"context"
	"net"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	commonv1 "voice.app/voice/common/v1"
	storyv1 "voice.app/voice/story/v1"
)

type recordingStoryGRPC struct {
	storyv1.UnimplementedStoryServiceServer
	lastCreate   *storyv1.CreateStoryRequest
	lastFeed     *storyv1.GetStoryFeedRequest
	lastGet      *storyv1.GetStoryRequest
	lastViewed   *storyv1.MarkViewedRequest
	lastReact    *storyv1.ReactToStoryRequest
	lastHighlight *storyv1.GetHighlightsRequest
	lastLFP      *storyv1.CreateLookingForPartyRequest
	lastReply    *storyv1.ReplyToStoryRequest
}

func (s *recordingStoryGRPC) CreateStory(_ context.Context, req *storyv1.CreateStoryRequest) (*storyv1.CreateStoryResponse, error) {
	s.lastCreate = req
	text := req.GetTextContent()
	return &storyv1.CreateStoryResponse{
		Story: &storyv1.Story{
			Id:              "story-1",
			AuthorProfileId: "profile-1",
			Type:            req.GetType(),
			TextContent:     &text,
			Visibility:      req.GetVisibility(),
		},
	}, nil
}

func (s *recordingStoryGRPC) GetStoryFeed(_ context.Context, req *storyv1.GetStoryFeedRequest) (*storyv1.GetStoryFeedResponse, error) {
	s.lastFeed = req
	return &storyv1.GetStoryFeedResponse{
		Stories: []*storyv1.Story{{Id: "story-1", Type: "text"}},
	}, nil
}

func (s *recordingStoryGRPC) GetStory(_ context.Context, req *storyv1.GetStoryRequest) (*storyv1.GetStoryResponse, error) {
	s.lastGet = req
	return &storyv1.GetStoryResponse{
		Story: &storyv1.Story{Id: req.GetStoryId(), Type: "text"},
	}, nil
}

func (s *recordingStoryGRPC) MarkViewed(_ context.Context, req *storyv1.MarkViewedRequest) (*storyv1.MarkViewedResponse, error) {
	s.lastViewed = req
	return &storyv1.MarkViewedResponse{}, nil
}

func (s *recordingStoryGRPC) ReactToStory(_ context.Context, req *storyv1.ReactToStoryRequest) (*storyv1.ReactToStoryResponse, error) {
	s.lastReact = req
	return &storyv1.ReactToStoryResponse{}, nil
}

func (s *recordingStoryGRPC) GetHighlights(_ context.Context, req *storyv1.GetHighlightsRequest) (*storyv1.GetHighlightsResponse, error) {
	s.lastHighlight = req
	return &storyv1.GetHighlightsResponse{
		HighlightList: &storyv1.HighlightList{
			Highlights: []*storyv1.Highlight{{Id: "hl-1", Name: "Wins"}},
		},
	}, nil
}

func (s *recordingStoryGRPC) CreateLookingForParty(_ context.Context, req *storyv1.CreateLookingForPartyRequest) (*storyv1.CreateLookingForPartyResponse, error) {
	s.lastLFP = req
	return &storyv1.CreateLookingForPartyResponse{
		Story: &storyv1.Story{
			Id:                "lfp-1",
			IsLookingForParty: true,
			LfpCriteriaJson:   &req.CriteriaJson,
		},
	}, nil
}

func (s *recordingStoryGRPC) ReplyToStory(_ context.Context, req *storyv1.ReplyToStoryRequest) (*storyv1.ReplyToStoryResponse, error) {
	s.lastReply = req
	return &storyv1.ReplyToStoryResponse{
		ChatId:    "chat-1",
		MessageId: "msg-1",
	}, nil
}

func startBufconnStoryConn(t *testing.T, impl storyv1.StoryServiceServer) (grpc.ClientConnInterface, func()) {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	storyv1.RegisterStoryServiceServer(srv, impl)
	go func() { _ = srv.Serve(lis) }()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	return conn, func() {
		_ = conn.Close()
		srv.Stop()
		_ = lis.Close()
	}
}

func newStoriesContractGateway(t *testing.T, rec *recordingStoryGRPC) http.Handler {
	t.Helper()
	conn, cleanup := startBufconnStoryConn(t, rec)
	t.Cleanup(cleanup)
	return newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{story: storyv1.NewStoryServiceClient(conn)}},
	})
}

// TestTranscodeStories_CreateStory documents POST /api/v1/stories → CreateStory.
func TestTranscodeStories_CreateStory(t *testing.T) {
	t.Parallel()
	rec := &recordingStoryGRPC{}
	h := newStoriesContractGateway(t, rec)

	body := `{"type":"text","text_content":"hello","visibility":"friends"}`
	resp := performRequest(h, http.MethodPost, "/api/v1/stories", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
		"Content-Type":  "application/json",
	})
	require.Equal(t, http.StatusOK, resp.Code)
	require.NotNil(t, rec.lastCreate)
	require.Equal(t, "text", rec.lastCreate.GetType())
	require.Equal(t, "friends", rec.lastCreate.GetVisibility())
}

// TestTranscodeStories_GetFeed documents GET /api/v1/stories/feed → GetStoryFeed.
func TestTranscodeStories_GetFeed(t *testing.T) {
	t.Parallel()
	rec := &recordingStoryGRPC{}
	h := newStoriesContractGateway(t, rec)

	resp := performRequest(h, http.MethodGet, "/api/v1/stories/feed?limit=20", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code)
	require.NotNil(t, rec.lastFeed)
	require.NotNil(t, rec.lastFeed.GetPage())
}

// TestTranscodeStories_GetStory documents GET /api/v1/stories/{id} → GetStory.
func TestTranscodeStories_GetStory(t *testing.T) {
	t.Parallel()
	rec := &recordingStoryGRPC{}
	h := newStoriesContractGateway(t, rec)

	resp := performRequest(h, http.MethodGet, "/api/v1/stories/story-42", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code)
	require.NotNil(t, rec.lastGet)
	require.Equal(t, "story-42", rec.lastGet.GetStoryId())
}

// TestTranscodeStories_MarkViewed documents POST /api/v1/stories/{id}/views → MarkViewed.
func TestTranscodeStories_MarkViewed(t *testing.T) {
	t.Parallel()
	rec := &recordingStoryGRPC{}
	h := newStoriesContractGateway(t, rec)

	resp := performRequest(h, http.MethodPost, "/api/v1/stories/story-42/views", `{}`, map[string]string{
		"Authorization": "Bearer valid-user-token",
		"Content-Type":  "application/json",
	})
	require.Equal(t, http.StatusNoContent, resp.Code)
	require.NotNil(t, rec.lastViewed)
	require.Equal(t, "story-42", rec.lastViewed.GetStoryId())
}

// TestTranscodeStories_React documents POST /api/v1/stories/{id}/reactions → ReactToStory.
func TestTranscodeStories_React(t *testing.T) {
	t.Parallel()
	rec := &recordingStoryGRPC{}
	h := newStoriesContractGateway(t, rec)

	body := `{"emoji":"🔥"}`
	resp := performRequest(h, http.MethodPost, "/api/v1/stories/story-42/reactions", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
		"Content-Type":  "application/json",
	})
	require.Equal(t, http.StatusNoContent, resp.Code)
	require.NotNil(t, rec.lastReact)
	require.Equal(t, "🔥", rec.lastReact.GetEmoji())
}

// TestTranscodeStories_GetHighlights documents GET /api/v1/stories/highlights → GetHighlights.
func TestTranscodeStories_GetHighlights(t *testing.T) {
	t.Parallel()
	rec := &recordingStoryGRPC{}
	h := newStoriesContractGateway(t, rec)

	resp := performRequest(h, http.MethodGet, "/api/v1/stories/highlights?profile_id=profile-1", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code)
	require.NotNil(t, rec.lastHighlight)
	require.Equal(t, "profile-1", rec.lastHighlight.GetProfileId())
}

// TestTranscodeStories_CreateLookingForParty documents POST /api/v1/stories/looking-for-party.
func TestTranscodeStories_CreateLookingForParty(t *testing.T) {
	t.Parallel()
	rec := &recordingStoryGRPC{}
	h := newStoriesContractGateway(t, rec)

	body := `{"criteria_json":"{\"game_id\":\"dota-2\"}"}`
	resp := performRequest(h, http.MethodPost, "/api/v1/stories/looking-for-party", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
		"Content-Type":  "application/json",
	})
	require.Equal(t, http.StatusOK, resp.Code)
	require.NotNil(t, rec.lastLFP)
	require.Contains(t, rec.lastLFP.GetCriteriaJson(), "dota-2")
}

// TestTranscodeStories_FeedPassesCursorPage documents cursor page proto mapping on feed.
func TestTranscodeStories_FeedPassesCursorPage(t *testing.T) {
	t.Parallel()
	rec := &recordingStoryGRPC{}
	h := newStoriesContractGateway(t, rec)

	resp := performRequest(h, http.MethodGet, "/api/v1/stories/feed?cursor=abc&limit=10", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code)
	require.NotNil(t, rec.lastFeed)
	page := rec.lastFeed.GetPage()
	require.NotNil(t, page)
	require.Equal(t, "abc", page.GetCursor())
	require.Equal(t, int32(10), page.GetPageSize())
	_ = commonv1.CursorPageRequest{}
}
