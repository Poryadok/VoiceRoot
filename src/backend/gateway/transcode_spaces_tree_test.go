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
	"google.golang.org/protobuf/types/known/timestamppb"

	chatv1 "voice.app/voice/chat/v1"
	spacev1 "voice.app/voice/space/v1"
)

type recordingSpaceTree struct {
	spacev1.UnimplementedSpaceServiceServer
	lastListSpaceID string
	lastCategory    *spacev1.CreateCategoryRequest
	lastVoiceRoom   *spacev1.CreateVoiceRoomRequest
	lastUpsert      *spacev1.UpsertTreeNodeRequest
	lastReorder     *spacev1.ReorderSpaceTreeRequest
	lastRemove      *spacev1.RemoveTreeNodeRequest
}

func (s *recordingSpaceTree) ListSpaceTree(_ context.Context, req *spacev1.ListSpaceTreeRequest) (*spacev1.ListSpaceTreeResponse, error) {
	s.lastListSpaceID = req.GetSpaceId()
	return &spacev1.ListSpaceTreeResponse{
		Categories: []*spacev1.Category{{Id: "cat-1", SpaceId: req.GetSpaceId(), Name: "General"}},
		Nodes:      []*spacev1.SpaceTreeNode{{Id: "node-1", SpaceId: req.GetSpaceId(), Kind: "text_chat"}},
	}, nil
}

func (s *recordingSpaceTree) CreateCategory(_ context.Context, req *spacev1.CreateCategoryRequest) (*spacev1.CreateCategoryResponse, error) {
	s.lastCategory = req
	return &spacev1.CreateCategoryResponse{
		Category: &spacev1.Category{Id: "cat-new", SpaceId: req.GetSpaceId(), Name: req.GetName()},
	}, nil
}

func (s *recordingSpaceTree) CreateVoiceRoom(_ context.Context, req *spacev1.CreateVoiceRoomRequest) (*spacev1.CreateVoiceRoomResponse, error) {
	s.lastVoiceRoom = req
	return &spacev1.CreateVoiceRoomResponse{
		VoiceRoom: &spacev1.VoiceRoom{Id: "vr-1", SpaceId: req.GetSpaceId(), Name: req.GetName()},
	}, nil
}

func (s *recordingSpaceTree) UpsertTreeNode(_ context.Context, req *spacev1.UpsertTreeNodeRequest) (*spacev1.UpsertTreeNodeResponse, error) {
	s.lastUpsert = req
	return &spacev1.UpsertTreeNodeResponse{
		SpaceTreeNode: &spacev1.SpaceTreeNode{Id: "node-new", SpaceId: req.GetSpaceId(), Kind: req.GetKind()},
	}, nil
}

func (s *recordingSpaceTree) ReorderSpaceTree(_ context.Context, req *spacev1.ReorderSpaceTreeRequest) (*spacev1.ReorderSpaceTreeResponse, error) {
	s.lastReorder = req
	return &spacev1.ReorderSpaceTreeResponse{}, nil
}

func (s *recordingSpaceTree) RemoveTreeNode(_ context.Context, req *spacev1.RemoveTreeNodeRequest) (*spacev1.RemoveTreeNodeResponse, error) {
	s.lastRemove = req
	return &spacev1.RemoveTreeNodeResponse{}, nil
}

type recordingChatsForSpaceTree struct {
	chatv1.UnimplementedChatServiceServer
	lastCreate *chatv1.CreateChatRequest
}

func (c *recordingChatsForSpaceTree) CreateChat(_ context.Context, req *chatv1.CreateChatRequest) (*chatv1.CreateChatResponse, error) {
	c.lastCreate = req
	now := timestamppb.Now()
	name := req.GetName()
	return &chatv1.CreateChatResponse{
		Chat: &chatv1.Chat{
			Id:        "chat-99",
			Type:      req.GetType(),
			Name:      &name,
			CreatedAt: now,
		},
	}, nil
}

func startBufconnSpaceTreeClients(t *testing.T, spaceImpl spacev1.SpaceServiceServer, chatImpl chatv1.ChatServiceServer) (spacev1.SpaceServiceClient, chatv1.ChatServiceClient, func()) {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	spacev1.RegisterSpaceServiceServer(srv, spaceImpl)
	chatv1.RegisterChatServiceServer(srv, chatImpl)
	go func() { _ = srv.Serve(lis) }()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	return spacev1.NewSpaceServiceClient(conn), chatv1.NewChatServiceClient(conn), func() {
		_ = conn.Close()
		srv.Stop()
		_ = lis.Close()
	}
}

func TestTranscodeSpacesListTree(t *testing.T) {
	t.Parallel()
	rec := &recordingSpaceTree{}
	spaceClient, chatClient, cleanup := startBufconnSpaceTreeClients(t, rec, &recordingChatsForSpaceTree{})
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{space: spaceClient, chat: chatClient}},
		restUpstreams: map[string]http.Handler{
			"spaces": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
		},
	})

	resp := performRequest(h, http.MethodGet, "/api/v1/spaces/space-1/tree", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t, "space-1", rec.lastListSpaceID)
}

func TestTranscodeSpacesCreateCategory(t *testing.T) {
	t.Parallel()
	rec := &recordingSpaceTree{}
	spaceClient, chatClient, cleanup := startBufconnSpaceTreeClients(t, rec, &recordingChatsForSpaceTree{})
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{space: spaceClient, chat: chatClient}},
	})

	body := `{"name":"General","sortOrder":0}`
	resp := performRequest(h, http.MethodPost, "/api/v1/spaces/space-1/categories", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t, "space-1", rec.lastCategory.GetSpaceId())
	require.Equal(t, "General", rec.lastCategory.GetName())
}

func TestTranscodeSpacesCreateVoiceRoom(t *testing.T) {
	t.Parallel()
	rec := &recordingSpaceTree{}
	spaceClient, chatClient, cleanup := startBufconnSpaceTreeClients(t, rec, &recordingChatsForSpaceTree{})
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{space: spaceClient, chat: chatClient}},
	})

	body := `{"name":"Lobby"}`
	resp := performRequest(h, http.MethodPost, "/api/v1/spaces/space-1/voice-rooms", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t, "Lobby", rec.lastVoiceRoom.GetName())
}

func TestTranscodeSpacesOrchestrateChat(t *testing.T) {
	t.Parallel()
	rec := &recordingSpaceTree{}
	chatRec := &recordingChatsForSpaceTree{}
	spaceClient, chatClient, cleanup := startBufconnSpaceTreeClients(t, rec, chatRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{space: spaceClient, chat: chatClient}},
	})

	body := `{"type":"CHAT_TYPE_GROUP","name":"general"}`
	resp := performRequest(h, http.MethodPost, "/api/v1/spaces/space-1/chats", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code)
	require.NotNil(t, chatRec.lastCreate)
	require.Equal(t, "space-1", chatRec.lastCreate.GetSpaceId())
	require.Equal(t, "text_chat", rec.lastUpsert.GetKind())
	require.Equal(t, "chat-99", rec.lastUpsert.GetLinkedChat().GetId())
}
