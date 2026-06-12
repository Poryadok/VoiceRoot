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
	searchv1 "voice.app/voice/search/v1"

	chatv1 "voice.app/voice/chat/v1"
)

type recordingSearchGRPC struct {
	searchv1.UnimplementedSearchServiceServer
	lastInChat  *searchv1.SearchInChatRequest
	lastGlobal  *searchv1.SearchGlobalRequest
	lastUsers   *searchv1.SearchUsersRequest
	lastSpaces  *searchv1.SearchSpacesRequest
}

func (s *recordingSearchGRPC) SearchInChat(_ context.Context, req *searchv1.SearchInChatRequest) (*searchv1.SearchInChatResponse, error) {
	s.lastInChat = req
	return &searchv1.SearchInChatResponse{
		SearchResults: &searchv1.SearchResults{
			Hits: []*searchv1.SearchHit{{
				MessageId: "msg-1",
				Snippet:   "matched snippet",
				Score:     1.0,
			}},
		},
	}, nil
}

func (s *recordingSearchGRPC) SearchGlobal(_ context.Context, req *searchv1.SearchGlobalRequest) (*searchv1.SearchGlobalResponse, error) {
	s.lastGlobal = req
	return &searchv1.SearchGlobalResponse{
		GlobalSearchResults: &searchv1.GlobalSearchResults{
			Messages: []*searchv1.SearchHit{{MessageId: "msg-global", Snippet: "global hit"}},
			ProfileIds: []string{"profile-1"},
			MatchedChats: []*chatv1.ChatRef{{Id: "chat-1"}},
			SpaceIds: []string{"space-1"},
		},
	}, nil
}

func (s *recordingSearchGRPC) SearchUsers(_ context.Context, req *searchv1.SearchUsersRequest) (*searchv1.SearchUsersResponse, error) {
	s.lastUsers = req
	return &searchv1.SearchUsersResponse{
		UserSearchResults: &searchv1.UserSearchResults{ProfileIds: []string{"profile-2"}},
	}, nil
}

func (s *recordingSearchGRPC) SearchSpaces(_ context.Context, req *searchv1.SearchSpacesRequest) (*searchv1.SearchSpacesResponse, error) {
	s.lastSpaces = req
	return &searchv1.SearchSpacesResponse{
		SpaceSearchResults: &searchv1.SpaceSearchResults{SpaceIds: []string{"space-public"}},
	}, nil
}

func startBufconnSearchConn(t *testing.T, impl searchv1.SearchServiceServer) (grpc.ClientConnInterface, func()) {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	searchv1.RegisterSearchServiceServer(srv, impl)
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

func TestTranscodeSearchInChat(t *testing.T) {
	t.Parallel()
	grpcRec := &recordingSearchGRPC{}
	conn, cleanup := startBufconnSearchConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{search: searchv1.NewSearchServiceClient(conn)}},
	})

	rec := performRequest(h, http.MethodGet, "/api/v1/search/in-chat?chat_id=chat-1&q=hello", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	require.NotNil(t, grpcRec.lastInChat)
	require.Equal(t, "chat-1", grpcRec.lastInChat.GetChat().GetId())
	require.Equal(t, "hello", grpcRec.lastInChat.GetQuery())
}

func TestTranscodeSearchGlobal_DefaultPageSize20(t *testing.T) {
	t.Parallel()
	grpcRec := &recordingSearchGRPC{}
	conn, cleanup := startBufconnSearchConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{search: searchv1.NewSearchServiceClient(conn)}},
	})

	rec := performRequest(h, http.MethodGet, "/api/v1/search/global?q=raid", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	require.NotNil(t, grpcRec.lastGlobal)
	require.Equal(t, "raid", grpcRec.lastGlobal.GetQuery())
	if grpcRec.lastGlobal.GetPage() == nil {
		t.Fatal("expected page request with default page_size")
	}
	require.Equal(t, int32(20), grpcRec.lastGlobal.GetPage().GetPageSize())
}

func TestTranscodeSearchUsers(t *testing.T) {
	t.Parallel()
	grpcRec := &recordingSearchGRPC{}
	conn, cleanup := startBufconnSearchConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{search: searchv1.NewSearchServiceClient(conn)}},
	})

	rec := performRequest(h, http.MethodGet, "/api/v1/search/users?q=carol", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	require.NotNil(t, grpcRec.lastUsers)
	require.Equal(t, "carol", grpcRec.lastUsers.GetQuery())
}

func TestTranscodeSearchUnavailableWhenClientNil(t *testing.T) {
	t.Parallel()
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{}},
	})
	rec := performRequest(h, http.MethodGet, "/api/v1/search/global?q=test", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestTranscodeSearchSpaces(t *testing.T) {
	t.Parallel()
	grpcRec := &recordingSearchGRPC{}
	conn, cleanup := startBufconnSearchConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{search: searchv1.NewSearchServiceClient(conn)}},
	})

	rec := performRequest(h, http.MethodGet, "/api/v1/search/spaces?q=guild&page_size=20", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	require.NotNil(t, grpcRec.lastSpaces)
	require.Equal(t, "guild", grpcRec.lastSpaces.GetQuery())
	require.Equal(t, int32(20), grpcRec.lastSpaces.GetPage().GetPageSize())
}

func TestTranscodeSearchGlobal_ReturnsSections(t *testing.T) {
	t.Parallel()
	grpcRec := &recordingSearchGRPC{}
	conn, cleanup := startBufconnSearchConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{search: searchv1.NewSearchServiceClient(conn)}},
	})

	rec := performRequest(h, http.MethodGet, "/api/v1/search/global?q=raid", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, rec.Code)

	var body struct {
		GlobalSearchResults struct {
			Messages     []map[string]any `json:"messages"`
			ProfileIds   []string         `json:"profile_ids"`
			MatchedChats []map[string]any `json:"matched_chats"`
			SpaceIds     []string         `json:"space_ids"`
		} `json:"global_search_results"`
	}
	decodeJSON(t, rec.Body, &body)
	require.NotEmpty(t, body.GlobalSearchResults.Messages)
	require.NotEmpty(t, body.GlobalSearchResults.ProfileIds)
	require.NotEmpty(t, body.GlobalSearchResults.MatchedChats, "matchedChats=%v", body.GlobalSearchResults.MatchedChats)
	require.NotEmpty(t, body.GlobalSearchResults.SpaceIds)
	_ = commonv1.CursorPageRequest{}
}
