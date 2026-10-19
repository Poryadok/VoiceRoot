package grpcsvc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	notificationv1 "voice.app/voice/notification/v1"
	"voice/backend/notification/internal/dispatch"
	"voice/backend/notification/internal/fcm"
	"voice/backend/notification/internal/store"
	"voice/backend/pkg/releasepolicy"
)

func alphaRegistrationPolicy(t *testing.T) *releasepolicy.Policy {
	t.Helper()
	data, err := os.ReadFile("../../../pkg/releasepolicy/testdata/free-invite-alpha.json")
	require.NoError(t, err)
	sum := sha256.Sum256(data)
	p, err := releasepolicy.Parse(data, hex.EncodeToString(sum[:]), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	require.NoError(t, err)
	return p
}

func TestReleasePolicyRegisterDeviceAuthenticatesAndDeniesBeforeStore(t *testing.T) {
	svc := &NotificationGRPC{ReleasePolicy: alphaRegistrationPolicy(t)}
	ctx := incomingProfileCtx(uuid.New())
	for _, req := range []*notificationv1.RegisterDeviceRequest{{Platform: "android", Token: "valid-token"}, {Platform: "ios", Token: "valid-token", PushService: "apns"}, {PlatformEnum: notificationv1.DevicePlatform_DEVICE_PLATFORM_IOS.Enum(), Token: "valid-token", PushService: "voip_apns"}} {
		_, err := svc.RegisterDevice(ctx, req)
		require.Equal(t, codes.FailedPrecondition, status.Code(err))
		require.Equal(t, "capability_unavailable", status.Convert(err).Message())
		_, err = svc.RegisterDevice(context.Background(), req)
		require.Equal(t, codes.Unauthenticated, status.Code(err), "unauthenticated failure is not a capability assertion")
	}
	_, err := svc.RegisterDevice(ctx, &notificationv1.RegisterDeviceRequest{Platform: "web", Token: "valid-web"})
	require.Equal(t, codes.Unavailable, status.Code(err), "legal web path must reach its missing store rather than be capability denied")
	_, err = svc.UnregisterDevice(ctx, &notificationv1.UnregisterDeviceRequest{DeviceTokenId: uuid.NewString()})
	require.Equal(t, codes.Unavailable, status.Code(err), "unregister remains available to the owning store")
}

func TestReleasePolicyRegisterDeviceCannotDisguiseMobileMetadata(t *testing.T) {
	svc := &NotificationGRPC{ReleasePolicy: alphaRegistrationPolicy(t)}
	for _, req := range []*notificationv1.RegisterDeviceRequest{{Platform: "web", PlatformEnum: notificationv1.DevicePlatform_DEVICE_PLATFORM_ANDROID.Enum(), Token: "tok"}, {Platform: "web", PushService: "apns", Token: "tok"}, {Platform: "desktop", PushService: "voip_apns", Token: "tok"}, {Platform: "web", PlatformEnum: notificationv1.DevicePlatform(99).Enum(), Token: "tok"}, {Platform: "web", PushService: "unknown", Token: "tok"}} {
		_, err := svc.RegisterDevice(incomingProfileCtx(uuid.New()), req)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	}
}

type releaseProviderRecorder struct{ calls int }

func (r *releaseProviderRecorder) Send(context.Context, uuid.UUID, store.DeviceToken, fcm.PushPayload) error {
	r.calls++
	return nil
}

func TestReleasePolicyDeviceRegistrationPostgresNoEffects(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	ctx := context.Background()
	var pool *pgxpool.Pool
	if database := os.Getenv("VOICE_RELEASE_TEST_DATABASE_URL"); database != "" {
		bootstrap, err := pgxpool.New(ctx, database)
		require.NoError(t, err)
		schema := pgx.Identifier{"release_" + strings.ReplaceAll(uuid.NewString(), "-", "")}.Sanitize()
		_, err = bootstrap.Exec(ctx, "CREATE SCHEMA "+schema)
		require.NoError(t, err)
		config, err := pgxpool.ParseConfig(database)
		require.NoError(t, err)
		config.ConnConfig.RuntimeParams["search_path"] = schema
		pool, err = pgxpool.NewWithConfig(ctx, config)
		require.NoError(t, err)
		t.Cleanup(func() {
			pool.Close()
			_, err := bootstrap.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
			require.NoError(t, err)
			bootstrap.Close()
		})
		require.NoError(t, pool.Ping(ctx))
	} else {
		pool = startNotificationPostgresForTest(t, ctx)
	}
	applyNotificationMigration(t, ctx, pool)
	tokens := &store.DeviceTokenStore{Pool: pool}
	profile := uuid.New()
	caller := incomingProfileCtx(profile)
	policy := alphaRegistrationPolicy(t)
	recorder := &releaseProviderRecorder{}
	svc := &NotificationGRPC{ReleasePolicy: policy, Tokens: tokens, Pusher: &dispatch.PushDispatcher{ReleasePolicy: policy, FCM: recorder}}
	_, err := tokens.Register(ctx, profile, "android", "pre-policy-token", "fcm")
	require.NoError(t, err)
	before, err := tokens.ListByProfile(ctx, profile)
	require.NoError(t, err)
	require.Len(t, before, 1)
	for _, req := range []*notificationv1.RegisterDeviceRequest{{Platform: "android", Token: "new-android"}, {Platform: "ios", Token: "new-ios", PushService: "apns"}, {Platform: "ios", Token: "new-voip", PushService: "voip_apns"}} {
		_, err = svc.RegisterDevice(caller, req)
		require.Equal(t, codes.FailedPrecondition, status.Code(err))
	}
	after, err := tokens.ListByProfile(ctx, profile)
	require.NoError(t, err)
	require.Equal(t, before, after, "denial must not write/delete/refresh token rows")
	registered, err := svc.RegisterDevice(caller, &notificationv1.RegisterDeviceRequest{Platform: "web", Token: "web-control", PushService: "fcm"})
	require.NoError(t, err)
	for retry := 0; retry < 2; retry++ {
		_, err = svc.SendNotification(ctx, &notificationv1.SendNotificationRequest{ProfileId: profile.String(), NotificationType: "system", Title: "synthetic"})
		require.NoError(t, err)
	}
	require.Equal(t, 2, recorder.calls, "only the real web positive control reaches provider on both deliveries; old mobile token stays suppressed")
	_, err = svc.UnregisterDevice(caller, &notificationv1.UnregisterDeviceRequest{DeviceTokenId: registered.DeviceTokenId})
	require.NoError(t, err)
	final, err := tokens.ListByProfile(ctx, profile)
	require.NoError(t, err)
	require.Equal(t, before, final, "unregister still works without erasing old mobile state")
}
