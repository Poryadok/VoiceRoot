package grpcsvc

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/notification/internal/fcm"
	"voice/backend/notification/internal/store"

	notificationv1 "voice.app/voice/notification/v1"
)

type recordingFCMSender struct {
	mu   sync.Mutex
	sent []fcm.PushPayload
}

func (r *recordingFCMSender) Send(ctx context.Context, profileID uuid.UUID, token store.DeviceToken, payload fcm.PushPayload) error {
	_ = ctx
	_ = profileID
	_ = token
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sent = append(r.sent, payload)
	return nil
}

func TestSendNotification_DeliversToEachRegisteredToken(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startNotificationPostgresForTest(t, ctx)
	applyNotificationMigration(t, ctx, pool)

	tokens := &store.DeviceTokenStore{Pool: pool}
	profileID := uuid.New()
	_, err := tokens.Register(ctx, profileID, "android", "android-tok", "fcm")
	require.NoError(t, err)
	_, err = tokens.Register(ctx, profileID, "web", "web-tok", "fcm")
	require.NoError(t, err)

	rec := &recordingFCMSender{}
	svc := &NotificationGRPC{FCM: rec, Tokens: tokens}
	_, err = svc.SendNotification(ctx, &notificationv1.SendNotificationRequest{
		ProfileId:        profileID.String(),
		NotificationType: "new_message",
		Body:             "hello",
	})
	require.NoError(t, err)
	require.Len(t, rec.sent, 2)
}

func TestSendNotification_MatchFoundRoutesToFCM(t *testing.T) {
	rec := &recordingFCMSender{}
	svc := &NotificationGRPC{
		FCM: rec,
		Tokens: &store.DeviceTokenStore{},
	}
	profileID := uuid.NewString()
	_, err := svc.SendNotification(context.Background(), &notificationv1.SendNotificationRequest{
		ProfileId:        profileID,
		NotificationType: "match_found",
		Title:            "Match found",
		Body:             "Your squad is ready",
		PayloadJson:      `{"match_id":"` + uuid.NewString() + `"}`,
	})
	require.NoError(t, err)
	require.Len(t, rec.sent, 1, "match_found must route through FCM sender")
	require.Contains(t, rec.sent[0].Data["type"], "match_found")
}

func TestSendNotification_MissingFields_InvalidArgument(t *testing.T) {
	svc := &NotificationGRPC{FCM: &recordingFCMSender{}}
	_, err := svc.SendNotification(context.Background(), &notificationv1.SendNotificationRequest{})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestSendNotification_FCMUnavailable(t *testing.T) {
	svc := &NotificationGRPC{Tokens: &store.DeviceTokenStore{}}
	_, err := svc.SendNotification(context.Background(), &notificationv1.SendNotificationRequest{
		ProfileId:        uuid.NewString(),
		NotificationType: "new_message",
	})
	require.Error(t, err)
	require.Equal(t, codes.Unavailable, status.Code(err))
}

func TestSendNotification_InvalidProfileID(t *testing.T) {
	svc := &NotificationGRPC{FCM: &recordingFCMSender{}, Tokens: &store.DeviceTokenStore{}}
	_, err := svc.SendNotification(context.Background(), &notificationv1.SendNotificationRequest{
		ProfileId:        "bad-id",
		NotificationType: "new_message",
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestSendNotification_InvalidTokenDeletedFromStore(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startNotificationPostgresForTest(t, ctx)
	applyNotificationMigration(t, ctx, pool)

	tokens := &store.DeviceTokenStore{Pool: pool}
	profileID := uuid.New()
	staleToken := "stale-grpc-" + uuid.NewString()
	_, err := tokens.Register(ctx, profileID, "web", staleToken, "fcm")
	require.NoError(t, err)

	svc := &NotificationGRPC{
		FCM:    invalidTokenSender{},
		Tokens: tokens,
	}
	_, err = svc.SendNotification(ctx, &notificationv1.SendNotificationRequest{
		ProfileId:        profileID.String(),
		NotificationType: "new_message",
		Body:             "ping",
	})
	require.NoError(t, err)

	rows, err := tokens.ListByProfile(ctx, profileID)
	require.NoError(t, err)
	require.Empty(t, rows, "invalid FCM token must be removed during SendNotification")
}

type invalidTokenSender struct{}

func (invalidTokenSender) Send(context.Context, uuid.UUID, store.DeviceToken, fcm.PushPayload) error {
	return fcm.ErrInvalidToken
}

func TestSendNotification_FCMErrorWithoutTokens(t *testing.T) {
	svc := &NotificationGRPC{
		FCM:    failingFCMSender{},
		Tokens: &store.DeviceTokenStore{},
	}
	_, err := svc.SendNotification(context.Background(), &notificationv1.SendNotificationRequest{
		ProfileId:        uuid.NewString(),
		NotificationType: "new_message",
	})
	require.Error(t, err)
	require.Equal(t, codes.Internal, status.Code(err))
}

func TestSendNotification_FCMHardErrorStopsDelivery(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startNotificationPostgresForTest(t, ctx)
	applyNotificationMigration(t, ctx, pool)

	tokens := &store.DeviceTokenStore{Pool: pool}
	profileID := uuid.New()
	_, err := tokens.Register(ctx, profileID, "web", "web-hard-fail", "fcm")
	require.NoError(t, err)

	svc := &NotificationGRPC{FCM: failingFCMSender{}, Tokens: tokens}
	_, err = svc.SendNotification(ctx, &notificationv1.SendNotificationRequest{
		ProfileId:        profileID.String(),
		NotificationType: "new_message",
	})
	require.Error(t, err)
	require.Equal(t, codes.Internal, status.Code(err))
}

func TestSendBulkNotification_PropagatesError(t *testing.T) {
	svc := &NotificationGRPC{FCM: &recordingFCMSender{}}
	_, err := svc.SendBulkNotification(context.Background(), &notificationv1.SendBulkNotificationRequest{
		ProfileIds:       []string{"bad-profile"},
		NotificationType: "system",
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

type failingFCMSender struct{}

func (failingFCMSender) Send(context.Context, uuid.UUID, store.DeviceToken, fcm.PushPayload) error {
	return errors.New("fcm unavailable")
}
