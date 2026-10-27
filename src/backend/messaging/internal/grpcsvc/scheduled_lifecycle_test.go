package grpcsvc

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"testing"
	"time"
	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
)

func TestScheduledValidationFutureHorizonAndOnlineDM(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		req  *messagingv1.SendMessageRequest
		kind chatv1.ChatType
		code codes.Code
	}{
		{"future", &messagingv1.SendMessageRequest{DeliverySchedule: &messagingv1.SendMessageRequest_ScheduledAt{ScheduledAt: timestamppb.New(now.Add(time.Second))}}, chatv1.ChatType_CHAT_TYPE_DM, codes.OK},
		{"exact horizon", &messagingv1.SendMessageRequest{DeliverySchedule: &messagingv1.SendMessageRequest_ScheduledAt{ScheduledAt: timestamppb.New(now.Add(365 * 24 * time.Hour))}}, chatv1.ChatType_CHAT_TYPE_GROUP, codes.OK},
		{"now", &messagingv1.SendMessageRequest{DeliverySchedule: &messagingv1.SendMessageRequest_ScheduledAt{ScheduledAt: timestamppb.New(now)}}, chatv1.ChatType_CHAT_TYPE_DM, codes.InvalidArgument},
		{"past", &messagingv1.SendMessageRequest{DeliverySchedule: &messagingv1.SendMessageRequest_ScheduledAt{ScheduledAt: timestamppb.New(now.Add(-time.Second))}}, chatv1.ChatType_CHAT_TYPE_DM, codes.InvalidArgument},
		{"beyond horizon", &messagingv1.SendMessageRequest{DeliverySchedule: &messagingv1.SendMessageRequest_ScheduledAt{ScheduledAt: timestamppb.New(now.Add(365*24*time.Hour + time.Nanosecond))}}, chatv1.ChatType_CHAT_TYPE_DM, codes.InvalidArgument},
		{"nil timestamp", &messagingv1.SendMessageRequest{DeliverySchedule: &messagingv1.SendMessageRequest_ScheduledAt{}}, chatv1.ChatType_CHAT_TYPE_DM, codes.InvalidArgument},
		{"invalid nanos", &messagingv1.SendMessageRequest{DeliverySchedule: &messagingv1.SendMessageRequest_ScheduledAt{ScheduledAt: &timestamppb.Timestamp{Seconds: now.Unix() + 10, Nanos: 1e9}}}, chatv1.ChatType_CHAT_TYPE_DM, codes.InvalidArgument},
		{"online DM", &messagingv1.SendMessageRequest{DeliverySchedule: &messagingv1.SendMessageRequest_SendWhenOnline{SendWhenOnline: true}}, chatv1.ChatType_CHAT_TYPE_DM, codes.OK},
		{"false online", &messagingv1.SendMessageRequest{DeliverySchedule: &messagingv1.SendMessageRequest_SendWhenOnline{}}, chatv1.ChatType_CHAT_TYPE_DM, codes.InvalidArgument},
		{"group online", &messagingv1.SendMessageRequest{DeliverySchedule: &messagingv1.SendMessageRequest_SendWhenOnline{SendWhenOnline: true}}, chatv1.ChatType_CHAT_TYPE_GROUP, codes.InvalidArgument},
		{"channel online", &messagingv1.SendMessageRequest{DeliverySchedule: &messagingv1.SendMessageRequest_SendWhenOnline{SendWhenOnline: true}}, chatv1.ChatType_CHAT_TYPE_CHANNEL, codes.InvalidArgument},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := validateScheduledDelivery(tt.req, tt.kind, now)
			if status.Code(err) != tt.code {
				t.Fatalf("got %v want %v", err, tt.code)
			}
		})
	}
}
