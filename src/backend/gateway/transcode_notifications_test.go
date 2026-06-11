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

	notificationv1 "voice.app/voice/notification/v1"
)

type recordingNotificationGRPC struct {
	notificationv1.UnimplementedNotificationServiceServer
	lastRegister       *notificationv1.RegisterDeviceRequest
	lastUnregister     *notificationv1.UnregisterDeviceRequest
	lastGetSettings    *notificationv1.GetNotificationSettingsRequest
	lastUpdateSettings *notificationv1.UpdateNotificationSettingsRequest
	lastSetQuietHours  *notificationv1.SetQuietHoursRequest
}

func (s *recordingNotificationGRPC) RegisterDevice(_ context.Context, req *notificationv1.RegisterDeviceRequest) (*notificationv1.RegisterDeviceResponse, error) {
	s.lastRegister = req
	return &notificationv1.RegisterDeviceResponse{}, nil
}

func (s *recordingNotificationGRPC) UnregisterDevice(_ context.Context, req *notificationv1.UnregisterDeviceRequest) (*notificationv1.UnregisterDeviceResponse, error) {
	s.lastUnregister = req
	return &notificationv1.UnregisterDeviceResponse{}, nil
}

func (s *recordingNotificationGRPC) GetNotificationSettings(_ context.Context, req *notificationv1.GetNotificationSettingsRequest) (*notificationv1.GetNotificationSettingsResponse, error) {
	s.lastGetSettings = req
	return &notificationv1.GetNotificationSettingsResponse{
		NotificationSettings: &notificationv1.NotificationSettings{Enabled: true, ScopeType: req.GetScopeType()},
	}, nil
}

func (s *recordingNotificationGRPC) UpdateNotificationSettings(_ context.Context, req *notificationv1.UpdateNotificationSettingsRequest) (*notificationv1.UpdateNotificationSettingsResponse, error) {
	s.lastUpdateSettings = req
	return &notificationv1.UpdateNotificationSettingsResponse{NotificationSettings: req.GetSettings()}, nil
}

func (s *recordingNotificationGRPC) SetQuietHours(_ context.Context, req *notificationv1.SetQuietHoursRequest) (*notificationv1.SetQuietHoursResponse, error) {
	s.lastSetQuietHours = req
	return &notificationv1.SetQuietHoursResponse{}, nil
}

func startBufconnNotificationConn(t *testing.T, impl notificationv1.NotificationServiceServer) (grpc.ClientConnInterface, func()) {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	notificationv1.RegisterNotificationServiceServer(srv, impl)
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

func TestTranscodeNotificationsRegisterDevice(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingNotificationGRPC{}
	conn, cleanup := startBufconnNotificationConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{notification: notificationv1.NewNotificationServiceClient(conn)}},
	})

	body := `{"platform":"web","token":"fcm-token","push_service":"fcm"}`
	resp := performRequest(h, http.MethodPost, "/api/v1/notifications/register-device", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code, "body=%s", resp.Body.String())
	require.NotNil(t, grpcRec.lastRegister)
	require.Equal(t, "web", grpcRec.lastRegister.GetPlatform())
	require.Equal(t, "fcm-token", grpcRec.lastRegister.GetToken())
}

func TestTranscodeNotificationsUnregisterDevice(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingNotificationGRPC{}
	conn, cleanup := startBufconnNotificationConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{notification: notificationv1.NewNotificationServiceClient(conn)}},
	})

	body := `{"device_token_id":"token-id-1"}`
	resp := performRequest(h, http.MethodPost, "/api/v1/notifications/unregister-device", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code, "body=%s", resp.Body.String())
	require.NotNil(t, grpcRec.lastUnregister)
	require.Equal(t, "token-id-1", grpcRec.lastUnregister.GetDeviceTokenId())
}

func TestTranscodeNotificationsGetSettings(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingNotificationGRPC{}
	conn, cleanup := startBufconnNotificationConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{notification: notificationv1.NewNotificationServiceClient(conn)}},
	})

	resp := performRequest(h, http.MethodGet, "/api/v1/notifications/settings?scope_type=chat", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code, "body=%s", resp.Body.String())
	require.NotNil(t, grpcRec.lastGetSettings)
	require.Equal(t, "chat", grpcRec.lastGetSettings.GetScopeType())
}

func TestTranscodeNotificationsSetQuietHours(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingNotificationGRPC{}
	conn, cleanup := startBufconnNotificationConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{notification: notificationv1.NewNotificationServiceClient(conn)}},
	})

	body := `{"enabled":true,"start_time":"23:00","end_time":"08:00","timezone":"UTC"}`
	resp := performRequest(h, http.MethodPut, "/api/v1/notifications/quiet-hours", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code, "body=%s", resp.Body.String())
	require.NotNil(t, grpcRec.lastSetQuietHours)
	require.True(t, grpcRec.lastSetQuietHours.GetEnabled())
	require.Equal(t, "23:00", grpcRec.lastSetQuietHours.GetStartTime())
}
