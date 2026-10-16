package adapters

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "voice.app/voice/events/v1"
	idhash "voice/backend/analytics/internal/hash"
)

func TestMapperFromRoleSubjectIgnoresTypedSpaceVoiceInvalidationAndKeepsLegacyJSON(t *testing.T) {
	m := Mapper{HashKey: "test-key"}
	typed, err := proto.Marshal(&eventsv1.RoleStreamEvent{
		EventId: "66666666-6666-6666-6666-666666666666",
		Payload: &eventsv1.RoleStreamEvent_VoiceRoomPolicyInvalidated{
			VoiceRoomPolicyInvalidated: &eventsv1.VoiceRoomPolicyInvalidated{
				SpaceId: "11111111-1111-1111-1111-111111111111", PolicyEpoch: 4,
			},
		},
	})
	require.NoError(t, err)
	require.Nil(t, m.FromRoleSubject("role.voice_policy_invalidated", typed))

	legacy := m.FromRoleSubject("role.voice_override_removed", []byte(`{"space_id":"11111111-1111-1111-1111-111111111111","role_id":"22222222-2222-2222-2222-222222222222"}`))
	require.NotNil(t, legacy)
	require.Equal(t, "role_voice_override_removed", legacy.GetEventType())
	require.Equal(t, "role", legacy.GetSourceService())
}

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

func TestMapperFromChatLifecyclePreservesOnlyEstablishedAnalytics(t *testing.T) {
	m := Mapper{HashKey: "test-key"}
	created := m.FromChat(&eventsv1.ChatStreamEvent{Payload: &eventsv1.ChatStreamEvent_ChatCreated{ChatCreated: &eventsv1.ChatCreated{ChatId: "chat-1", Type: "group"}}})
	require.NotNil(t, created)
	require.Equal(t, "chat_created", created.GetEventType())
	for _, event := range []*eventsv1.ChatStreamEvent{
		{Payload: &eventsv1.ChatStreamEvent_ChatUpdated{ChatUpdated: &eventsv1.ChatUpdated{ChatId: "chat-1", ChangedFields: []string{"name"}}}},
		{Payload: &eventsv1.ChatStreamEvent_ChatDeleted{ChatDeleted: &eventsv1.ChatDeleted{ChatId: "chat-1"}}},
		{Payload: &eventsv1.ChatStreamEvent_ChatMemberChanged{ChatMemberChanged: &eventsv1.ChatMemberChanged{ChatId: "chat-1", ProfileId: "profile-1", Change: "removed"}}},
	} {
		require.Nil(t, m.FromChat(event), "no analytics event is defined for this Chat lifecycle fact")
	}
	joined := m.FromChat(&eventsv1.ChatStreamEvent{Payload: &eventsv1.ChatStreamEvent_ChatMemberChanged{ChatMemberChanged: &eventsv1.ChatMemberChanged{ChatId: "chat-1", ProfileId: "profile-1", Change: "joined"}}})
	require.NotNil(t, joined)
	require.Equal(t, "space_joined", joined.GetEventType(), "preserve the existing joined mapping")
}
