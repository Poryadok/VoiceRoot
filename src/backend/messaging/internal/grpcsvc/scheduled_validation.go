package grpcsvc

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"time"
	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/messaging/internal/store"
)

func validateScheduledDelivery(req *messagingv1.SendMessageRequest, chatType chatv1.ChatType, now time.Time) (store.ScheduledMessageScheduleKind, *time.Time, error) {
	invalid := func() (store.ScheduledMessageScheduleKind, *time.Time, error) {
		return "", nil, status.Error(codes.InvalidArgument, "invalid delivery schedule")
	}
	switch schedule := req.GetDeliverySchedule().(type) {
	case *messagingv1.SendMessageRequest_ScheduledAt:
		if schedule == nil || schedule.ScheduledAt == nil || schedule.ScheduledAt.CheckValid() != nil {
			return invalid()
		}
		at := schedule.ScheduledAt.AsTime().UTC()
		if !at.After(now) || at.After(now.Add(365*24*time.Hour)) {
			return invalid()
		}
		return store.ScheduledMessageScheduleAt, &at, nil
	case *messagingv1.SendMessageRequest_SendWhenOnline:
		if schedule == nil || !schedule.SendWhenOnline || chatType != chatv1.ChatType_CHAT_TYPE_DM {
			return invalid()
		}
		return store.ScheduledMessageScheduleWhenOnline, nil, nil
	default:
		return invalid()
	}
}
