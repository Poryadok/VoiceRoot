package main

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/timestamppb"

	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	socialv1 "voice.app/voice/social/v1"
	userv1 "voice.app/voice/user/v1"
)

type recordingUserGRPC struct {
	userv1.UnimplementedUserServiceServer
	lastMD metadata.MD
}

func (s *recordingUserGRPC) GetProfile(ctx context.Context, req *userv1.GetProfileRequest) (*userv1.GetProfileResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	s.lastMD = md
	pid := req.GetProfileId()
	if pid == "" {
		return nil, status.Error(codes.InvalidArgument, "profile_id required")
	}
	return &userv1.GetProfileResponse{
		Profile: &userv1.Profile{
			Id:          pid,
			AccountId:   "account-1",
			DisplayName: "Alice",
		},
	}, nil
}

func (s *recordingUserGRPC) SearchProfiles(ctx context.Context, req *userv1.SearchProfilesRequest) (*userv1.SearchProfilesResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	s.lastMD = md
	return &userv1.SearchProfilesResponse{
		ProfileList: &userv1.ProfileList{
			Profiles: []*userv1.Profile{
				{
					Id:             "profile-search-hit",
					AccountId:      "account-2",
					Username:       "carol",
					Discriminator:  "0001",
					DisplayName:    "Carol " + req.GetQuery(),
				},
			},
		},
	}, nil
}

type avatarPresignRecorder struct {
	userv1.UnimplementedUserServiceServer
	last *userv1.CreateAvatarPresignedUploadRequest
}

func (s *avatarPresignRecorder) CreateAvatarPresignedUpload(_ context.Context, req *userv1.CreateAvatarPresignedUploadRequest) (*userv1.CreateAvatarPresignedUploadResponse, error) {
	s.last = req
	return &userv1.CreateAvatarPresignedUploadResponse{
		HttpMethod:      "PUT",
		UploadUrl:       "https://r2.example/presigned",
		RequiredHeaders: map[string]string{"Content-Type": "image/png"},
		MaxBytes:        5242880,
		ExpiresAt:       timestamppb.New(time.Unix(1700000000, 0)),
		PublicUrl:       "https://cdn.example/avatars/x.png",
		ObjectKey:       "avatars/x.png",
	}, nil
}

func TestTranscodeUsersAvatarPresignedUpload(t *testing.T) {
	t.Parallel()

	rec := &avatarPresignRecorder{}
	conn, cleanup := startBufconnUserConn(t, rec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{user: userv1.NewUserServiceClient(conn)}},
	})

	body := `{"content_type":"image/png","content_length":2048}`
	resp := performRequest(h, http.MethodPost, "/api/v1/users/me/avatar/presigned-upload", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%q", resp.Code, http.StatusOK, resp.Body.String())
	}
	if rec.last == nil {
		t.Fatal("CreateAvatarPresignedUpload not invoked")
	}
	if rec.last.GetProfileId() != "profile-1" {
		t.Fatalf("profile_id = %q, want profile-1", rec.last.GetProfileId())
	}
	if rec.last.GetContentType() != "image/png" || rec.last.GetContentLength() != 2048 {
		t.Fatalf("unexpected gRPC request: %+v", rec.last)
	}
	var out struct {
		UploadURL  string `json:"upload_url"`
		PublicURL  string `json:"public_url"`
		HTTPMethod string `json:"http_method"`
	}
	decodeJSON(t, resp.Body, &out)
	if out.UploadURL != "https://r2.example/presigned" || out.PublicURL != "https://cdn.example/avatars/x.png" || out.HTTPMethod != "PUT" {
		t.Fatalf("response body = %+v", out)
	}
}

func TestTranscodeUsersSearchProfiles(t *testing.T) {
	t.Parallel()

	rec := &recordingUserGRPC{}
	conn, cleanup := startBufconnUserConn(t, rec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{user: userv1.NewUserServiceClient(conn)}},
	})

	resp := performRequest(h, http.MethodGet, "/api/v1/users/search?q=Phase1Find", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code, "body=%s", resp.Body.String())

	var body struct {
		ProfileList struct {
			Profiles []struct {
				ID          string `json:"id"`
				DisplayName string `json:"display_name"`
			} `json:"profiles"`
		} `json:"profile_list"`
	}
	decodeJSON(t, resp.Body, &body)
	require.Len(t, body.ProfileList.Profiles, 1)
	require.Equal(t, "profile-search-hit", body.ProfileList.Profiles[0].ID)
	require.Contains(t, body.ProfileList.Profiles[0].DisplayName, "Phase1Find")

	if got := rec.lastMD.Get("x-voice-user-id"); len(got) != 1 || got[0] != "account-1" {
		t.Fatalf("x-voice-user-id = %v, want account-1", got)
	}
}

func TestTranscodeUsersSearchProfilesNotFoundWithoutUpstream(t *testing.T) {
	t.Parallel()

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
	})

	resp := performRequest(h, http.MethodGet, "/api/v1/users/search?q=test", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusNotFound, resp.Code, "body=%s", resp.Body.String())
}

func TestTranscodeUsersMePropagatesVoiceHeaders(t *testing.T) {
	t.Parallel()

	rec := &recordingUserGRPC{}
	conn, cleanup := startBufconnUserConn(t, rec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {
				UserID:    "account-1",
				ProfileID: "profile-1",
			},
		},
		transcoder: &transcoder{clients: grpcClients{user: userv1.NewUserServiceClient(conn)}},
	})

	resp := performRequest(h, http.MethodGet, "/api/v1/users/me", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%q", resp.Code, http.StatusOK, resp.Body.String())
	}
	if got := rec.lastMD.Get("x-voice-user-id"); len(got) != 1 || got[0] != "account-1" {
		t.Fatalf("x-voice-user-id = %v, want account-1", got)
	}
	if got := rec.lastMD.Get("x-voice-profile-id"); len(got) != 1 || got[0] != "profile-1" {
		t.Fatalf("x-voice-profile-id = %v, want profile-1", got)
	}
}

func startBufconnUserConn(t *testing.T, impl userv1.UserServiceServer) (grpc.ClientConnInterface, func()) {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	userv1.RegisterUserServiceServer(srv, impl)
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

func startBufconnSocialConn(t *testing.T, impl socialv1.SocialServiceServer) (grpc.ClientConnInterface, func()) {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	socialv1.RegisterSocialServiceServer(srv, impl)
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

func startBufconnChatConn(t *testing.T, impl chatv1.ChatServiceServer) (grpc.ClientConnInterface, func()) {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	chatv1.RegisterChatServiceServer(srv, impl)
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

func startBufconnMessagingConn(t *testing.T, impl messagingv1.MessagingServiceServer) (grpc.ClientConnInterface, func()) {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	messagingv1.RegisterMessagingServiceServer(srv, impl)
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

type denyMessagingGRPC struct {
	messagingv1.UnimplementedMessagingServiceServer
}

func (denyMessagingGRPC) SendMessage(context.Context, *messagingv1.SendMessageRequest) (*messagingv1.SendMessageResponse, error) {
	return nil, status.Error(codes.PermissionDenied, "not allowed")
}

func TestTranscodeGRPCErrorMapping(t *testing.T) {
	t.Parallel()

	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	messagingv1.RegisterMessagingServiceServer(srv, denyMessagingGRPC{})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(func() {
		srv.Stop()
		_ = lis.Close()
	})
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{messaging: messagingv1.NewMessagingServiceClient(conn)}},
	})

	rec := performRequest(h, http.MethodPost, "/api/v1/messages/send", `{"chat":{"id":"chat-1"},"content":"hi"}`, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%q", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	var got struct {
		ErrorCode string `json:"error_code"`
		Message   string `json:"message"`
	}
	decodeJSON(t, rec.Body, &got)
	if got.ErrorCode != "permission_denied" || got.Message == "" {
		t.Fatalf("error body = %+v", got)
	}
}

func TestTranscodePrecedenceOverRESTProxy(t *testing.T) {
	t.Parallel()

	rec := &recordingUserGRPC{}
	conn, cleanup := startBufconnUserConn(t, rec)
	t.Cleanup(cleanup)

	proxyCalled := false
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{user: userv1.NewUserServiceClient(conn)}},
		restUpstreams: map[string]http.Handler{
			"users": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				proxyCalled = true
				w.WriteHeader(http.StatusAccepted)
			}),
		},
	})

	resp := performRequest(h, http.MethodGet, "/api/v1/users/me", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}
	if proxyCalled {
		t.Fatal("REST proxy must not run when gRPC transcoder handles the route")
	}
}

type recordingSocialFriends struct {
	socialv1.UnimplementedSocialServiceServer
	last *socialv1.ListFriendsRequest
}

func (s *recordingSocialFriends) ListFriends(_ context.Context, req *socialv1.ListFriendsRequest) (*socialv1.ListFriendsResponse, error) {
	s.last = req
	return &socialv1.ListFriendsResponse{FriendList: &socialv1.FriendList{}}, nil
}

func TestTranscodeFriendsListPrecedenceOverRESTProxy(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingSocialFriends{}
	conn, cleanup := startBufconnSocialConn(t, grpcRec)
	t.Cleanup(cleanup)

	proxyCalled := false
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{social: socialv1.NewSocialServiceClient(conn)}},
		restUpstreams: map[string]http.Handler{
			"friends": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				proxyCalled = true
				w.WriteHeader(http.StatusAccepted)
			}),
		},
	})

	resp := performRequest(h, http.MethodGet, "/api/v1/friends", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%q", resp.Code, http.StatusOK, resp.Body.String())
	}
	if proxyCalled {
		t.Fatal("REST proxy must not run when gRPC transcoder handles /api/v1/friends")
	}
	if grpcRec.last == nil {
		t.Fatal("ListFriends not invoked")
	}
}

type recordingChatsList struct {
	chatv1.UnimplementedChatServiceServer
	last *chatv1.ListChatsRequest
}

func (s *recordingChatsList) ListChats(_ context.Context, req *chatv1.ListChatsRequest) (*chatv1.ListChatsResponse, error) {
	s.last = req
	return &chatv1.ListChatsResponse{ChatList: &chatv1.ChatList{}}, nil
}

func TestTranscodeChatsListPrecedenceOverRESTProxy(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingChatsList{}
	conn, cleanup := startBufconnChatConn(t, grpcRec)
	t.Cleanup(cleanup)

	proxyCalled := false
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{chat: chatv1.NewChatServiceClient(conn)}},
		restUpstreams: map[string]http.Handler{
			"chats": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				proxyCalled = true
				w.WriteHeader(http.StatusAccepted)
			}),
		},
	})

	resp := performRequest(h, http.MethodGet, "/api/v1/chats", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%q", resp.Code, http.StatusOK, resp.Body.String())
	}
	if proxyCalled {
		t.Fatal("REST proxy must not run when gRPC transcoder handles /api/v1/chats")
	}
	if grpcRec.last == nil {
		t.Fatal("ListChats not invoked")
	}
}

type recordingChatsCreateDM struct {
	chatv1.UnimplementedChatServiceServer
	last *chatv1.CreateDMRequest
}

func (s *recordingChatsCreateDM) CreateDM(_ context.Context, req *chatv1.CreateDMRequest) (*chatv1.CreateDMResponse, error) {
	s.last = req
	now := timestamppb.New(time.Unix(1700000001, 0))
	return &chatv1.CreateDMResponse{
		Chat: &chatv1.Chat{
			Id:                 "chat-dm-1",
			Type:               chatv1.ChatType_CHAT_TYPE_DM,
			CreatorProfileId:   "profile-1",
			CreatedAt:          now,
			UpdatedAt:          now,
			LastMessageAt:      now,
		},
	}, nil
}

func TestTranscodeChatsCreateDMPrecedenceOverRESTProxy(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingChatsCreateDM{}
	conn, cleanup := startBufconnChatConn(t, grpcRec)
	t.Cleanup(cleanup)

	proxyCalled := false
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{chat: chatv1.NewChatServiceClient(conn)}},
		restUpstreams: map[string]http.Handler{
			"chats": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				proxyCalled = true
				w.WriteHeader(http.StatusAccepted)
			}),
		},
	})

	body := `{"other_profile_id":"profile-other"}`
	resp := performRequest(h, http.MethodPost, "/api/v1/chats/dm", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%q", resp.Code, http.StatusOK, resp.Body.String())
	}
	if proxyCalled {
		t.Fatal("REST proxy must not run when gRPC transcoder handles POST /api/v1/chats/dm")
	}
	if grpcRec.last == nil || grpcRec.last.GetOtherProfileId() != "profile-other" {
		t.Fatalf("CreateDM request = %+v", grpcRec.last)
	}
}

type recordingMessagesGet struct {
	messagingv1.UnimplementedMessagingServiceServer
	last *messagingv1.GetMessagesRequest
}

func (s *recordingMessagesGet) GetMessages(_ context.Context, req *messagingv1.GetMessagesRequest) (*messagingv1.GetMessagesResponse, error) {
	s.last = req
	return &messagingv1.GetMessagesResponse{MessageList: &messagingv1.MessageList{}}, nil
}

func TestTranscodeMessagesGetPrecedenceOverRESTProxy(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingMessagesGet{}
	conn, cleanup := startBufconnMessagingConn(t, grpcRec)
	t.Cleanup(cleanup)

	proxyCalled := false
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{messaging: messagingv1.NewMessagingServiceClient(conn)}},
		restUpstreams: map[string]http.Handler{
			"messages": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				proxyCalled = true
				w.WriteHeader(http.StatusAccepted)
			}),
		},
	})

	resp := performRequest(h, http.MethodGet, "/api/v1/messages?chat_id=chat-99", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%q", resp.Code, http.StatusOK, resp.Body.String())
	}
	if proxyCalled {
		t.Fatal("REST proxy must not run when gRPC transcoder handles GET /api/v1/messages")
	}
	if grpcRec.last == nil || grpcRec.last.GetChat().GetId() != "chat-99" {
		t.Fatalf("GetMessages request = %+v", grpcRec.last)
	}
}

func TestRESTProxyUsedWhenTranscoderDoesNotHandleChatsNestedPath(t *testing.T) {
	t.Parallel()

	// Chat transcoder only handles specific shapes; multi-segment paths fall through to REST proxy.
	grpcRec := &recordingChatsList{}
	conn, cleanup := startBufconnChatConn(t, grpcRec)
	t.Cleanup(cleanup)

	var proxyPath string
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{chat: chatv1.NewChatServiceClient(conn)}},
		restUpstreams: map[string]http.Handler{
			"chats": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				proxyPath = r.URL.Path
				w.WriteHeader(http.StatusTeapot)
			}),
		},
	})

	resp := performRequest(h, http.MethodGet, "/api/v1/chats/future/nested", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	if resp.Code != http.StatusTeapot {
		t.Fatalf("status = %d, want %d (proxy)", resp.Code, http.StatusTeapot)
	}
	if proxyPath != "/api/v1/chats/future/nested" {
		t.Fatalf("proxy path = %q", proxyPath)
	}
	if grpcRec.last != nil {
		t.Fatal("ListChats must not run for paths the transcoder does not handle")
	}
}
