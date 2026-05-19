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

	messagingv1 "voice.app/voice/messaging/v1"
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
		HttpMethod string `json:"http_method"`
	}
	decodeJSON(t, resp.Body, &out)
	if out.UploadURL != "https://r2.example/presigned" || out.PublicURL != "https://cdn.example/avatars/x.png" || out.HttpMethod != "PUT" {
		t.Fatalf("response body = %+v", out)
	}
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
