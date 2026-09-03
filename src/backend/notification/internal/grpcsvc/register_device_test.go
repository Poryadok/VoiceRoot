package grpcsvc

import (
	"context"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"voice/backend/notification/internal/authctx"
	"voice/backend/notification/internal/store"

	notificationv1 "voice.app/voice/notification/v1"
)

func withProfileCtx(ctx context.Context, profileID uuid.UUID) context.Context {
	return metadata.AppendToOutgoingContext(ctx, authctx.HeaderProfileID, profileID.String())
}

func startNotificationGRPCTestServer(t *testing.T, pool *pgxpool.Pool) (notificationv1.NotificationServiceClient, func()) {
	t.Helper()
	const bufSize = 1 << 20
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	notificationv1.RegisterNotificationServiceServer(srv, &NotificationGRPC{
		Tokens: &store.DeviceTokenStore{Pool: pool},
	})
	go func() {
		if err := srv.Serve(lis); err != nil {
			t.Logf("grpc serve: %v", err)
		}
	}()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	cleanup := func() {
		_ = conn.Close()
		srv.Stop()
	}
	return notificationv1.NewNotificationServiceClient(conn), cleanup
}

func TestRegisterDevice_MissingProfile_Unauthenticated(t *testing.T) {
	client, cleanup := startNotificationGRPCTestServer(t, nil)
	t.Cleanup(cleanup)

	_, err := client.RegisterDevice(context.Background(), &notificationv1.RegisterDeviceRequest{
		Platform: "android",
		Token:    "tok",
	})
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestRegisterDevice_TokenStoreUnavailable(t *testing.T) {
	svc := &NotificationGRPC{}
	_, err := svc.RegisterDevice(incomingProfileCtx(uuid.New()), &notificationv1.RegisterDeviceRequest{
		Platform: "android",
		Token:    "tok",
	})
	require.Error(t, err)
	require.Equal(t, codes.Unavailable, status.Code(err))
}

func TestRegisterDevice_InvalidArgument(t *testing.T) {
	client, cleanup := startNotificationGRPCTestServer(t, nil)
	t.Cleanup(cleanup)

	profileID := uuid.New()
	_, err := client.RegisterDevice(withProfileCtx(context.Background(), profileID), &notificationv1.RegisterDeviceRequest{
		Platform: "android",
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestRegisterDevice_NilRequest_InvalidArgument(t *testing.T) {
	svc := &NotificationGRPC{}
	_, err := svc.RegisterDevice(incomingProfileCtx(uuid.New()), nil)
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestRegisterDevice_DatabaseError_Internal(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startNotificationPostgresForTest(t, ctx)
	applyNotificationMigration(t, ctx, pool)
	pool.Close()

	svc := &NotificationGRPC{Tokens: &store.DeviceTokenStore{Pool: pool}}
	_, err := svc.RegisterDevice(incomingProfileCtx(uuid.New()), &notificationv1.RegisterDeviceRequest{
		Platform: "android",
		Token:    "tok-after-close",
	})
	require.Error(t, err)
	require.Equal(t, codes.Internal, status.Code(err))
}

func TestRegisterDevice_Success(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startNotificationPostgresForTest(t, ctx)
	applyNotificationMigration(t, ctx, pool)

	client, cleanup := startNotificationGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	profileID := uuid.New()
	_, err := client.RegisterDevice(withProfileCtx(ctx, profileID), &notificationv1.RegisterDeviceRequest{
		Platform:    "web",
		Token:       "fcm-web-token",
		PushService: "fcm",
	})
	require.NoError(t, err)
}

func TestRegisterDevice_PlatformEnumResolution(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startNotificationPostgresForTest(t, ctx)
	applyNotificationMigration(t, ctx, pool)

	client, cleanup := startNotificationGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	tests := []struct {
		name         string
		platform     string
		platformEnum *notificationv1.DevicePlatform
		token        string
		wantPlatform string
		wantCode     codes.Code
	}{
		{
			name:         "android enum only persists canonical platform",
			platformEnum: notificationv1.DevicePlatform_DEVICE_PLATFORM_ANDROID.Enum(),
			token:        "enum-android-token",
			wantPlatform: "android",
			wantCode:     codes.OK,
		},
		{
			name:         "ios enum only persists canonical platform",
			platformEnum: notificationv1.DevicePlatform_DEVICE_PLATFORM_IOS.Enum(),
			token:        "enum-ios-token",
			wantPlatform: "ios",
			wantCode:     codes.OK,
		},
		{
			name:         "web enum only persists canonical platform",
			platformEnum: notificationv1.DevicePlatform_DEVICE_PLATFORM_WEB.Enum(),
			token:        "enum-web-token",
			wantPlatform: "web",
			wantCode:     codes.OK,
		},
		{
			name:         "desktop enum only persists canonical platform",
			platformEnum: notificationv1.DevicePlatform_DEVICE_PLATFORM_DESKTOP.Enum(),
			token:        "enum-desktop-token",
			wantPlatform: "desktop",
			wantCode:     codes.OK,
		},
		{
			name:         "legacy string only remains accepted without new validation",
			platform:     "ios",
			token:        "legacy-string-token",
			wantPlatform: "ios",
			wantCode:     codes.OK,
		},
		{
			name:     "legacy invalid string preserves database validation",
			platform: "windows",
			token:    "legacy-invalid-string-token",
			wantCode: codes.Internal,
		},
		{
			name:         "valid enum takes priority over conflicting legacy string",
			platform:     "android",
			platformEnum: notificationv1.DevicePlatform_DEVICE_PLATFORM_IOS.Enum(),
			token:        "enum-priority-token",
			wantPlatform: "ios",
			wantCode:     codes.OK,
		},
		{
			name:         "unspecified enum falls back to legacy string",
			platform:     "web",
			platformEnum: notificationv1.DevicePlatform_DEVICE_PLATFORM_UNSPECIFIED.Enum(),
			token:        "unspecified-fallback-token",
			wantPlatform: "web",
			wantCode:     codes.OK,
		},
		{
			name:         "unknown enum falls back to legacy string",
			platform:     "desktop",
			platformEnum: notificationv1.DevicePlatform(99).Enum(),
			token:        "unknown-fallback-token",
			wantPlatform: "desktop",
			wantCode:     codes.OK,
		},
		{
			name:         "unknown enum rejects invalid legacy fallback",
			platform:     "windows",
			platformEnum: notificationv1.DevicePlatform(99).Enum(),
			token:        "unknown-invalid-fallback-token",
			wantCode:     codes.InvalidArgument,
		},
		{
			name:         "unspecified enum without legacy string is invalid",
			platformEnum: notificationv1.DevicePlatform_DEVICE_PLATFORM_UNSPECIFIED.Enum(),
			token:        "unspecified-invalid-token",
			wantCode:     codes.InvalidArgument,
		},
		{
			name:         "unknown enum without legacy string is invalid",
			platformEnum: notificationv1.DevicePlatform(99).Enum(),
			token:        "unknown-invalid-token",
			wantCode:     codes.InvalidArgument,
		},
		{
			name:     "no platform inputs is invalid",
			token:    "missing-platform-token",
			wantCode: codes.InvalidArgument,
		},
		{
			name:         "token remains required when valid enum supplies platform",
			platformEnum: notificationv1.DevicePlatform_DEVICE_PLATFORM_ANDROID.Enum(),
			wantCode:     codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profileID := uuid.New()
			_, err := client.RegisterDevice(withProfileCtx(ctx, profileID), &notificationv1.RegisterDeviceRequest{
				Platform:     tt.platform,
				PlatformEnum: tt.platformEnum,
				Token:        tt.token,
			})
			require.Equal(t, tt.wantCode, status.Code(err))

			tokens := &store.DeviceTokenStore{Pool: pool}
			rows, err := tokens.ListByProfile(ctx, profileID)
			require.NoError(t, err)
			if tt.wantCode != codes.OK {
				require.Empty(t, rows)
				return
			}

			require.Len(t, rows, 1)
			require.Equal(t, tt.wantPlatform, rows[0].Platform)
			require.Equal(t, tt.token, rows[0].Token)
		})
	}
}

func TestUnregisterDevice_TokenStoreUnavailable(t *testing.T) {
	svc := &NotificationGRPC{}
	_, err := svc.UnregisterDevice(incomingProfileCtx(uuid.New()), &notificationv1.UnregisterDeviceRequest{
		DeviceTokenId: uuid.NewString(),
	})
	require.Error(t, err)
	require.Equal(t, codes.Unavailable, status.Code(err))
}

func TestRegisterDevice_APNSServicePersisted(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startNotificationPostgresForTest(t, ctx)
	applyNotificationMigration(t, ctx, pool)

	client, cleanup := startNotificationGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	profileID := uuid.New()
	_, err := client.RegisterDevice(withProfileCtx(ctx, profileID), &notificationv1.RegisterDeviceRequest{
		Platform:    "ios",
		Token:       "apns-device-token",
		PushService: "apns",
	})
	require.NoError(t, err)

	tokens := &store.DeviceTokenStore{Pool: pool}
	rows, err := tokens.ListByProfile(ctx, profileID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "apns", rows[0].PushService)
	require.Equal(t, "ios", rows[0].Platform)
}

func TestRegisterDevice_DefaultPushService(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startNotificationPostgresForTest(t, ctx)
	applyNotificationMigration(t, ctx, pool)

	client, cleanup := startNotificationGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	profileID := uuid.New()
	_, err := client.RegisterDevice(withProfileCtx(ctx, profileID), &notificationv1.RegisterDeviceRequest{
		Platform: "android",
		Token:    "default-push-service-token",
	})
	require.NoError(t, err)

	tokens := &store.DeviceTokenStore{Pool: pool}
	rows, err := tokens.ListByProfile(ctx, profileID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "fcm", rows[0].PushService)
}

func TestUnregisterDevice_MissingProfile_Unauthenticated(t *testing.T) {
	client, cleanup := startNotificationGRPCTestServer(t, nil)
	t.Cleanup(cleanup)

	_, err := client.UnregisterDevice(context.Background(), &notificationv1.UnregisterDeviceRequest{
		DeviceTokenId: uuid.NewString(),
	})
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestUnregisterDevice_InvalidArgument(t *testing.T) {
	client, cleanup := startNotificationGRPCTestServer(t, nil)
	t.Cleanup(cleanup)

	profileID := uuid.New()
	_, err := client.UnregisterDevice(withProfileCtx(context.Background(), profileID), &notificationv1.UnregisterDeviceRequest{})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestUnregisterDevice_InvalidUUID(t *testing.T) {
	client, cleanup := startNotificationGRPCTestServer(t, nil)
	t.Cleanup(cleanup)

	profileID := uuid.New()
	_, err := client.UnregisterDevice(withProfileCtx(context.Background(), profileID), &notificationv1.UnregisterDeviceRequest{
		DeviceTokenId: "not-uuid",
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestUnregisterDevice_Success(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startNotificationPostgresForTest(t, ctx)
	applyNotificationMigration(t, ctx, pool)

	client, cleanup := startNotificationGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	profileID := uuid.New()
	_, err := client.RegisterDevice(withProfileCtx(ctx, profileID), &notificationv1.RegisterDeviceRequest{
		Platform: "web",
		Token:    "unregister-success-token",
	})
	require.NoError(t, err)

	tokens := &store.DeviceTokenStore{Pool: pool}
	rows, err := tokens.ListByProfile(ctx, profileID)
	require.NoError(t, err)
	require.Len(t, rows, 1)

	_, err = client.UnregisterDevice(withProfileCtx(ctx, profileID), &notificationv1.UnregisterDeviceRequest{
		DeviceTokenId: rows[0].ID.String(),
	})
	require.NoError(t, err)

	rows, err = tokens.ListByProfile(ctx, profileID)
	require.NoError(t, err)
	require.Empty(t, rows)
}

func TestUnregisterDevice_DatabaseError_Internal(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startNotificationPostgresForTest(t, ctx)
	applyNotificationMigration(t, ctx, pool)
	pool.Close()

	svc := &NotificationGRPC{Tokens: &store.DeviceTokenStore{Pool: pool}}
	_, err := svc.UnregisterDevice(incomingProfileCtx(uuid.New()), &notificationv1.UnregisterDeviceRequest{
		DeviceTokenId: uuid.NewString(),
	})
	require.Error(t, err)
	require.Equal(t, codes.Internal, status.Code(err))
}

func TestUnregisterDevice_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startNotificationPostgresForTest(t, ctx)
	applyNotificationMigration(t, ctx, pool)

	client, cleanup := startNotificationGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	profileID := uuid.New()
	_, err := client.UnregisterDevice(withProfileCtx(ctx, profileID), &notificationv1.UnregisterDeviceRequest{
		DeviceTokenId: uuid.NewString(),
	})
	require.Error(t, err)
	require.Equal(t, codes.NotFound, status.Code(err))
}
