package grpcsvc

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/notification/internal/apns"
	"voice/backend/notification/internal/authctx"
	"voice/backend/notification/internal/dispatch"
	"voice/backend/notification/internal/fcm"
	"voice/backend/notification/internal/push"
	"voice/backend/notification/internal/store"

	notificationv1 "voice.app/voice/notification/v1"
)

// NotificationGRPC implements NotificationService (Phase-6 stub).
type NotificationGRPC struct {
	notificationv1.UnimplementedNotificationServiceServer
	Tokens   *store.DeviceTokenStore
	Settings *store.SettingsStore
	Pusher   *dispatch.PushDispatcher
	Analytics interface {
		Publish(ctx context.Context, subject, sourceService, eventType string, props map[string]any) error
	}
}

func shouldDeliverPushToToken(notificationType, pushService string) bool {
	return dispatch.ShouldDeliverPushToToken(notificationType, pushService)
}

func (s *NotificationGRPC) pusher() *dispatch.PushDispatcher {
	if s == nil {
		return nil
	}
	if s.Pusher != nil {
		return s.Pusher
	}
	return nil
}

func (s *NotificationGRPC) RegisterDevice(ctx context.Context, req *notificationv1.RegisterDeviceRequest) (*notificationv1.RegisterDeviceResponse, error) {
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	if req.GetToken() == "" || req.GetPlatform() == "" {
		return nil, status.Error(codes.InvalidArgument, "platform and token required")
	}
	pushService := req.GetPushService()
	if pushService == "" {
		pushService = "fcm"
	}
	if s.Tokens == nil {
		return nil, status.Error(codes.Unavailable, "token store unavailable")
	}
	id, err := s.Tokens.Register(ctx, profileID, req.GetPlatform(), req.GetToken(), pushService)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "register device: %v", err)
	}
	return &notificationv1.RegisterDeviceResponse{DeviceTokenId: id.String()}, nil
}

func (s *NotificationGRPC) UnregisterDevice(ctx context.Context, req *notificationv1.UnregisterDeviceRequest) (*notificationv1.UnregisterDeviceResponse, error) {
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	if req.GetDeviceTokenId() == "" {
		return nil, status.Error(codes.InvalidArgument, "device_token_id required")
	}
	if s.Tokens == nil {
		return nil, status.Error(codes.Unavailable, "token store unavailable")
	}
	deviceTokenID, err := parseUUID(req.GetDeviceTokenId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid device_token_id")
	}
	if err := s.Tokens.Unregister(ctx, profileID, deviceTokenID); err != nil {
		if err == store.ErrDeviceTokenNotFound {
			return nil, status.Error(codes.NotFound, "device token not found")
		}
		return nil, status.Errorf(codes.Internal, "unregister device: %v", err)
	}
	return &notificationv1.UnregisterDeviceResponse{}, nil
}

func (s *NotificationGRPC) SendNotification(ctx context.Context, req *notificationv1.SendNotificationRequest) (*notificationv1.SendNotificationResponse, error) {
	if req.GetProfileId() == "" || req.GetNotificationType() == "" {
		return nil, status.Error(codes.InvalidArgument, "profile_id and notification_type required")
	}
	pusher := s.pusher()
	if pusher == nil {
		return nil, status.Error(codes.Unavailable, "push sender unavailable")
	}
	profileID, err := parseUUID(req.GetProfileId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid profile_id")
	}
	payload := push.Payload{
		Title: req.GetTitle(),
		Body:  req.GetBody(),
		Data: map[string]string{
			"type": req.GetNotificationType(),
		},
	}
	tokens, err := s.Tokens.ListByProfile(ctx, profileID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list tokens: %v", err)
	}
	if len(tokens) == 0 {
		fallbackService := "fcm"
		if req.GetNotificationType() == "incoming_call" {
			fallbackService = "voip_apns"
		}
		if err := pusher.Send(ctx, profileID, store.DeviceToken{PushService: fallbackService}, payload); err != nil {
			return nil, status.Errorf(codes.Internal, "send push: %v", err)
		}
		return &notificationv1.SendNotificationResponse{}, nil
	}
	for _, tok := range tokens {
		if !shouldDeliverPushToToken(req.GetNotificationType(), tok.PushService) {
			continue
		}
		if err := pusher.Send(ctx, profileID, tok, payload); err != nil {
			if err == fcm.ErrInvalidToken || err == apns.ErrInvalidToken {
				_ = s.Tokens.DeleteByToken(ctx, tok.Token)
				continue
			}
			return nil, status.Errorf(codes.Internal, "send push: %v", err)
		}
	}
	if s.Analytics != nil {
		_ = s.Analytics.Publish(ctx, "analytics.notification.push_sent", "notification", "push_sent", map[string]any{
			"profile_id":        profileID.String(),
			"notification_type": req.GetNotificationType(),
			"token_count":       len(tokens),
		})
	}
	return &notificationv1.SendNotificationResponse{}, nil
}

func (s *NotificationGRPC) GetNotificationSettings(ctx context.Context, req *notificationv1.GetNotificationSettingsRequest) (*notificationv1.GetNotificationSettingsResponse, error) {
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	scope := req.GetScopeType()
	if scope == "" {
		scope = "global"
	}
	if s.Settings == nil {
		return nil, status.Error(codes.Unavailable, "settings store unavailable")
	}
	var scopeID *uuid.UUID
	if req.ScopeId != nil && req.GetScopeId() != "" {
		parsed, err := parseUUID(req.GetScopeId())
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid scope_id")
		}
		scopeID = &parsed
	}
	row, err := s.Settings.GetSettings(ctx, profileID, scope, scopeID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get settings: %v", err)
	}
	return &notificationv1.GetNotificationSettingsResponse{
		NotificationSettings: settingsToProto(row),
	}, nil
}

func (s *NotificationGRPC) UpdateNotificationSettings(ctx context.Context, req *notificationv1.UpdateNotificationSettingsRequest) (*notificationv1.UpdateNotificationSettingsResponse, error) {
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	if req.GetSettings() == nil {
		return nil, status.Error(codes.InvalidArgument, "settings required")
	}
	if s.Settings == nil {
		return nil, status.Error(codes.Unavailable, "settings store unavailable")
	}
	row, err := settingsFromProto(profileID, req.GetSettings())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err := s.Settings.UpsertSettings(ctx, row); err != nil {
		return nil, status.Errorf(codes.Internal, "update settings: %v", err)
	}
	return &notificationv1.UpdateNotificationSettingsResponse{
		NotificationSettings: settingsToProto(row),
	}, nil
}

func (s *NotificationGRPC) GetQuietHours(ctx context.Context, _ *notificationv1.GetQuietHoursRequest) (*notificationv1.GetQuietHoursResponse, error) {
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	if s.Settings == nil {
		return nil, status.Error(codes.Unavailable, "settings store unavailable")
	}
	row, err := s.Settings.GetQuietHours(ctx, profileID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get quiet hours: %v", err)
	}
	return &notificationv1.GetQuietHoursResponse{
		QuietHours: quietHoursToProto(row),
	}, nil
}

func (s *NotificationGRPC) SetQuietHours(ctx context.Context, req *notificationv1.SetQuietHoursRequest) (*notificationv1.SetQuietHoursResponse, error) {
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	if s.Settings == nil {
		return nil, status.Error(codes.Unavailable, "settings store unavailable")
	}
	if err := s.Settings.SetQuietHours(ctx, store.QuietHours{
		ProfileID:        profileID,
		Enabled:          req.GetEnabled(),
		StartTime:        req.GetStartTime(),
		EndTime:          req.GetEndTime(),
		Timezone:         req.GetTimezone(),
		OverrideMentions: req.GetOverrideMentions(),
	}); err != nil {
		return nil, status.Errorf(codes.Internal, "set quiet hours: %v", err)
	}
	return &notificationv1.SetQuietHoursResponse{}, nil
}

func (s *NotificationGRPC) SendBulkNotification(ctx context.Context, req *notificationv1.SendBulkNotificationRequest) (*notificationv1.SendBulkNotificationResponse, error) {
	for _, pid := range req.GetProfileIds() {
		_, err := s.SendNotification(ctx, &notificationv1.SendNotificationRequest{
			ProfileId:            pid,
			NotificationType:     req.GetNotificationType(),
			Title:                req.GetTitle(),
			Body:                 req.GetBody(),
			PayloadJson:          req.GetPayloadJson(),
			NotificationCategory: req.NotificationCategory,
		})
		if err != nil {
			return nil, err
		}
	}
	return &notificationv1.SendBulkNotificationResponse{}, nil
}

func (s *NotificationGRPC) RelayNotification(ctx context.Context, req *notificationv1.RelayNotificationRequest) (*notificationv1.RelayNotificationResponse, error) {
	_ = ctx
	_ = req
	return nil, status.Error(codes.Unimplemented, "federation relay not implemented")
}

func settingsToProto(row store.NotificationSettings) *notificationv1.NotificationSettings {
	out := &notificationv1.NotificationSettings{
		ProfileId: row.ProfileID.String(),
		ScopeType: row.ScopeType,
		Enabled:   row.Enabled,
	}
	if row.ScopeID != nil {
		scopeID := row.ScopeID.String()
		out.ScopeId = &scopeID
	}
	if len(row.SuppressTypes) > 0 {
		if b, err := json.Marshal(row.SuppressTypes); err == nil {
			out.SuppressTypesJson = string(b)
		}
	}
	if row.MuteUntil != nil {
		out.MuteUntil = timestamppb.New(row.MuteUntil.UTC())
	}
	return out
}

func quietHoursToProto(row store.QuietHours) *notificationv1.QuietHours {
	tz := row.Timezone
	if tz == "" {
		tz = "UTC"
	}
	return &notificationv1.QuietHours{
		Enabled:          row.Enabled,
		StartTime:        row.StartTime,
		EndTime:          row.EndTime,
		Timezone:         tz,
		OverrideMentions: row.OverrideMentions,
	}
}

func settingsFromProto(profileID uuid.UUID, in *notificationv1.NotificationSettings) (store.NotificationSettings, error) {
	scope := in.GetScopeType()
	if scope == "" {
		scope = "global"
	}
	row := store.NotificationSettings{
		ProfileID: profileID,
		ScopeType: scope,
		Enabled:   in.GetEnabled(),
	}
	if in.ScopeId != nil && in.GetScopeId() != "" {
		parsed, err := parseUUID(in.GetScopeId())
		if err != nil {
			return store.NotificationSettings{}, err
		}
		row.ScopeID = &parsed
	}
	if in.GetSuppressTypesJson() != "" {
		if err := json.Unmarshal([]byte(in.GetSuppressTypesJson()), &row.SuppressTypes); err != nil {
			return store.NotificationSettings{}, err
		}
	}
	if in.MuteUntil != nil {
		t := in.GetMuteUntil().AsTime().UTC()
		row.MuteUntil = &t
	}
	return row, nil
}
