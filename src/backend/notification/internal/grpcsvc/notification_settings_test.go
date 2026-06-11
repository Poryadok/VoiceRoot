package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"voice/backend/notification/internal/authctx"
	"voice/backend/notification/internal/fcm"
	"voice/backend/notification/internal/store"

	notificationv1 "voice.app/voice/notification/v1"
)

func incomingProfileCtx(profileID uuid.UUID) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		authctx.HeaderProfileID, profileID.String(),
	))
}

func TestGetNotificationSettings_DefaultScope(t *testing.T) {
	profileID := uuid.New()
	svc := &NotificationGRPC{}

	resp, err := svc.GetNotificationSettings(incomingProfileCtx(profileID), &notificationv1.GetNotificationSettingsRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp.GetNotificationSettings())
	require.Equal(t, profileID.String(), resp.GetNotificationSettings().GetProfileId())
	require.Equal(t, "global", resp.GetNotificationSettings().GetScopeType())
	require.True(t, resp.GetNotificationSettings().GetEnabled())
}

func TestGetNotificationSettings_CustomScope(t *testing.T) {
	profileID := uuid.New()
	chatID := uuid.NewString()
	svc := &NotificationGRPC{}

	scope := "chat"
	resp, err := svc.GetNotificationSettings(incomingProfileCtx(profileID), &notificationv1.GetNotificationSettingsRequest{
		ScopeType: &scope,
		ScopeId:   &chatID,
	})
	require.NoError(t, err)
	require.Equal(t, "chat", resp.GetNotificationSettings().GetScopeType())
	require.Equal(t, chatID, resp.GetNotificationSettings().GetScopeId())
}

func TestGetNotificationSettings_Unauthenticated(t *testing.T) {
	svc := &NotificationGRPC{}
	_, err := svc.GetNotificationSettings(context.Background(), &notificationv1.GetNotificationSettingsRequest{})
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestSetQuietHours_Success(t *testing.T) {
	svc := &NotificationGRPC{}
	_, err := svc.SetQuietHours(incomingProfileCtx(uuid.New()), &notificationv1.SetQuietHoursRequest{
		Enabled:   true,
		StartTime: "23:00",
		EndTime:   "08:00",
		Timezone:  "UTC",
	})
	require.NoError(t, err)
}

func TestSetQuietHours_Unauthenticated(t *testing.T) {
	svc := &NotificationGRPC{}
	_, err := svc.SetQuietHours(context.Background(), &notificationv1.SetQuietHoursRequest{})
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestUpdateNotificationSettings_Success(t *testing.T) {
	svc := &NotificationGRPC{}
	settings := &notificationv1.NotificationSettings{
		ProfileId: uuid.NewString(),
		ScopeType: "global",
		Enabled:   false,
	}
	resp, err := svc.UpdateNotificationSettings(context.Background(), &notificationv1.UpdateNotificationSettingsRequest{
		Settings: settings,
	})
	require.NoError(t, err)
	require.Equal(t, settings, resp.GetNotificationSettings())
}

func TestUpdateNotificationSettings_MissingSettings(t *testing.T) {
	svc := &NotificationGRPC{}
	_, err := svc.UpdateNotificationSettings(context.Background(), &notificationv1.UpdateNotificationSettingsRequest{})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestSendBulkNotification_ForwardsEachProfile(t *testing.T) {
	rec := &recordingFCMSender{}
	svc := &NotificationGRPC{FCM: rec}

	ids := []string{uuid.NewString(), uuid.NewString()}
	_, err := svc.SendBulkNotification(context.Background(), &notificationv1.SendBulkNotificationRequest{
		ProfileIds:       ids,
		NotificationType: "system",
		Title:            "Maintenance",
		Body:             "Back soon",
	})
	require.NoError(t, err)
	require.Len(t, rec.sent, 2)
}

func TestRelayNotification_Unimplemented(t *testing.T) {
	svc := &NotificationGRPC{}
	_, err := svc.RelayNotification(context.Background(), &notificationv1.RelayNotificationRequest{})
	require.Error(t, err)
	require.Equal(t, codes.Unimplemented, status.Code(err))
}

func TestSendNotification_NoopFCMDoesNotFail(t *testing.T) {
	svc := &NotificationGRPC{
		FCM:    &fcm.NoopSender{},
		Tokens: &store.DeviceTokenStore{},
	}
	_, err := svc.SendNotification(context.Background(), &notificationv1.SendNotificationRequest{
		ProfileId:        uuid.NewString(),
		NotificationType: "new_message",
		Title:            "Hi",
		Body:             "Hello",
	})
	require.NoError(t, err, "notification service must continue when FCM is noop")
}
