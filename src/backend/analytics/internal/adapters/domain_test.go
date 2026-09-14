package adapters

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "voice.app/voice/events/v1"
	idhash "voice/backend/analytics/internal/hash"
)

func TestMapperFromMessageSent(t *testing.T) {
	m := Mapper{HashKey: "test-key"}
	ev := m.FromMessage(&eventsv1.MessageStreamEvent{
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_MessageSent{
			MessageSent: &eventsv1.MessageSent{
				ChatId:          "chat-1",
				MessageId:       "msg-1",
				SenderProfileId: "profile-1",
			},
		},
	})
	require.NotNil(t, ev)
	require.Equal(t, "message_sent", ev.GetEventType())
	require.Equal(t, "messaging", ev.GetSourceService())
	require.NotEmpty(t, ev.GetProfileIdHashed())
}

func TestMapperFromSubscriptionPlanStarted(t *testing.T) {
	m := Mapper{HashKey: "test-key"}
	ev := m.FromSubscription(&eventsv1.SubscriptionStreamEvent{
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.SubscriptionStreamEvent_PlanStarted{
			PlanStarted: &eventsv1.PlanStarted{AccountId: "acc-1", Plan: "premium"},
		},
	})
	require.NotNil(t, ev)
	require.Equal(t, "plan_started", ev.GetEventType())
	require.Equal(t, idhash.ID("test-key", "acc-1"), ev.GetUserIdHashed())
}

func TestMapperFromSubscriptionPlanExpired(t *testing.T) {
	m := Mapper{HashKey: "test-key"}
	occurredAt := timestamppb.Now()
	ev := m.FromSubscription(&eventsv1.SubscriptionStreamEvent{
		OccurredAt: occurredAt,
		Payload: &eventsv1.SubscriptionStreamEvent_PlanExpired{
			PlanExpired: &eventsv1.PlanExpired{AccountId: "acc-1", Plan: "premium"},
		},
	})

	require.NotNil(t, ev)
	require.Equal(t, "plan_expired", ev.GetEventType())
	require.Equal(t, "subscription", ev.GetSourceService())
	require.Equal(t, occurredAt.AsTime(), ev.GetTimestamp().AsTime())
	require.Equal(t, idhash.ID("test-key", "acc-1"), ev.GetUserIdHashed())
	var properties map[string]string
	require.NoError(t, json.Unmarshal([]byte(ev.GetPropertiesJson()), &properties))
	require.Equal(t, "premium", properties["plan"])
}

func TestMapperFromSubscriptionDowngrade(t *testing.T) {
	m := Mapper{HashKey: "test-key"}
	occurredAt := timestamppb.Now()
	ev := m.FromSubscription(&eventsv1.SubscriptionStreamEvent{
		OccurredAt: occurredAt,
		Payload: &eventsv1.SubscriptionStreamEvent_Downgrade{
			Downgrade: &eventsv1.Downgrade{AccountId: "acc-1", Plan: "premium"},
		},
	})

	require.NotNil(t, ev)
	require.Equal(t, "downgrade", ev.GetEventType())
	require.Equal(t, "subscription", ev.GetSourceService())
	require.Equal(t, occurredAt.AsTime(), ev.GetTimestamp().AsTime())
	require.Equal(t, idhash.ID("test-key", "acc-1"), ev.GetUserIdHashed())
	var properties map[string]string
	require.NoError(t, json.Unmarshal([]byte(ev.GetPropertiesJson()), &properties))
	require.Equal(t, "premium", properties["plan"])
}

func TestMapperFromUserRegistered(t *testing.T) {
	m := Mapper{HashKey: "test-key"}
	ev := m.FromUser(&eventsv1.UserStreamEvent{
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.UserStreamEvent_UserRegistered{
			UserRegistered: &eventsv1.UserRegistered{
				AccountId: "acc-1",
				Type:      "user",
				Method:    "email",
			},
		},
	})
	require.NotNil(t, ev)
	require.Equal(t, "user_registered", ev.GetEventType())
	require.NotEmpty(t, ev.GetUserIdHashed())
}
