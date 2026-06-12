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
