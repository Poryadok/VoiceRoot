package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/timestamppb"

	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	socialv1 "voice.app/voice/social/v1"
	userv1 "voice.app/voice/user/v1"
)

// dmSmokeBackend is a minimal in-process stand-in for User, Social, Chat, and Messaging
// services behind the gateway transcoder (app stack DM smoke).
type dmSmokeBackend struct {
	userv1.UnimplementedUserServiceServer
	socialv1.UnimplementedSocialServiceServer
	chatv1.UnimplementedChatServiceServer
	messagingv1.UnimplementedMessagingServiceServer

	sendCalls atomic.Int32
}

func (s *dmSmokeBackend) SearchProfiles(_ context.Context, req *userv1.SearchProfilesRequest) (*userv1.SearchProfilesResponse, error) {
	return &userv1.SearchProfilesResponse{
		ProfileList: &userv1.ProfileList{
			Profiles: []*userv1.Profile{{
				Id:          "profile-search-hit",
				AccountId:   "acc-other",
				DisplayName: req.GetQuery(),
			}},
		},
	}, nil
}

func (s *dmSmokeBackend) GetProfile(_ context.Context, req *userv1.GetProfileRequest) (*userv1.GetProfileResponse, error) {
	pid := req.GetProfileId()
	return &userv1.GetProfileResponse{
		Profile: &userv1.Profile{
			Id:          pid,
			AccountId:   "acc-smoke",
			DisplayName: "SmokeUser",
		},
	}, nil
}

func (s *dmSmokeBackend) ListFriends(context.Context, *socialv1.ListFriendsRequest) (*socialv1.ListFriendsResponse, error) {
	return &socialv1.ListFriendsResponse{FriendList: &socialv1.FriendList{}}, nil
}

func (s *dmSmokeBackend) ListChats(context.Context, *chatv1.ListChatsRequest) (*chatv1.ListChatsResponse, error) {
	return &chatv1.ListChatsResponse{ChatList: &chatv1.ChatList{}}, nil
}

func (s *dmSmokeBackend) GetMessages(context.Context, *messagingv1.GetMessagesRequest) (*messagingv1.GetMessagesResponse, error) {
	return &messagingv1.GetMessagesResponse{MessageList: &messagingv1.MessageList{}}, nil
}

func (s *dmSmokeBackend) SendMessage(_ context.Context, req *messagingv1.SendMessageRequest) (*messagingv1.SendMessageResponse, error) {
	n := s.sendCalls.Add(1)
	return &messagingv1.SendMessageResponse{
		Message: &messagingv1.Message{
			Id:        fmt.Sprintf("msg-smoke-%d", n),
			Chat:      req.GetChat(),
			Content:   req.GetContent(),
			CreatedAt: timestamppb.New(time.Unix(1700001000+int64(n), 0)),
		},
	}, nil
}

func startBufconnDMStack(t *testing.T, backend *dmSmokeBackend) (grpc.ClientConnInterface, func()) {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	userv1.RegisterUserServiceServer(srv, backend)
	socialv1.RegisterSocialServiceServer(srv, backend)
	chatv1.RegisterChatServiceServer(srv, backend)
	messagingv1.RegisterMessagingServiceServer(srv, backend)
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

// TestGatewayPhase1REST_smokeJWT_multiNamespaceAndSendRateLimit exercises JWT auth, transcoding
// to all core REST namespaces (users, friends, chats, messages), and MessagesSend rate limit
// on POST /api/v1/messages/send before the request reaches Messaging gRPC.
func TestGatewayPhase1REST_smokeJWT_multiNamespaceAndSendRateLimit(t *testing.T) {
	t.Parallel()

	backend := &dmSmokeBackend{}
	conn, cleanup := startBufconnDMStack(t, backend)
	t.Cleanup(cleanup)

	// Tight limit so the test stays fast while using the same gateway + sliding-window path as prod.
	limiter := newSlidingWindowLimiter(map[string]rateLimitRule{
		"MessagesSend": {Limit: 2, Window: time.Hour},
	})

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"phase1-jwt": {UserID: "account-1", ProfileID: "profile-1"},
		},
		rateLimiter: limiter,
		transcoder: &transcoder{clients: grpcClients{
			user:      userv1.NewUserServiceClient(conn),
			social:    socialv1.NewSocialServiceClient(conn),
			chat:      chatv1.NewChatServiceClient(conn),
			messaging: messagingv1.NewMessagingServiceClient(conn),
		}},
	})

	auth := map[string]string{"Authorization": "Bearer phase1-jwt"}

	t.Run("users_me", func(t *testing.T) {
		t.Parallel()
		rec := performRequest(h, http.MethodGet, "/api/v1/users/me", "", auth)
		require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
		var body struct {
			Profile struct {
				ID          string `json:"id"`
				DisplayName string `json:"display_name"`
			} `json:"profile"`
		}
		decodeJSON(t, rec.Body, &body)
		require.Equal(t, "profile-1", body.Profile.ID)
		require.Equal(t, "SmokeUser", body.Profile.DisplayName)
	})

	t.Run("friends_list", func(t *testing.T) {
		t.Parallel()
		rec := performRequest(h, http.MethodGet, "/api/v1/friends", "", auth)
		require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	})

	t.Run("chats_list", func(t *testing.T) {
		t.Parallel()
		rec := performRequest(h, http.MethodGet, "/api/v1/chats", "", auth)
		require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	})

	t.Run("users_search", func(t *testing.T) {
		t.Parallel()
		rec := performRequest(h, http.MethodGet, "/api/v1/users/search?q=Phase1Find", "", auth)
		require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	})

	t.Run("messages_list", func(t *testing.T) {
		t.Parallel()
		rec := performRequest(h, http.MethodGet, "/api/v1/messages?chat_id=chat-dm-1", "", auth)
		require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	})

	sendBody := `{"chat":{"id":"chat-dm-1"},"content":"hello-phase1"}`

	rec1 := performRequest(h, http.MethodPost, "/api/v1/messages/send", sendBody, auth)
	require.Equal(t, http.StatusOK, rec1.Code, "body=%s", rec1.Body.String())
	require.EqualValues(t, 1, backend.sendCalls.Load())

	rec2 := performRequest(h, http.MethodPost, "/api/v1/messages/send", sendBody, auth)
	require.Equal(t, http.StatusOK, rec2.Code, "body=%s", rec2.Body.String())
	require.EqualValues(t, 2, backend.sendCalls.Load())

	rec3 := performRequest(h, http.MethodPost, "/api/v1/messages/send", sendBody, auth)
	require.Equal(t, http.StatusTooManyRequests, rec3.Code, "body=%s", rec3.Body.String())
	require.EqualValues(t, 2, backend.sendCalls.Load(), "rate-limited send must not call Messaging SendMessage")

	var errBody map[string]string
	decodeJSON(t, rec3.Body, &errBody)
	require.Equal(t, "rate_limited", errBody["error"])
}
