package dispatch_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/delivery"
	"voice/backend/notification/internal/dispatch"
	"voice/backend/notification/internal/fcm"
	"voice/backend/notification/internal/grouping"
	"voice/backend/notification/internal/push"
	"voice/backend/notification/internal/store"
)

type recordingFCM struct {
	sent []push.Payload
}

func (r *recordingFCM) Send(_ context.Context, _ uuid.UUID, _ store.DeviceToken, payload fcm.PushPayload) error {
	r.sent = append(r.sent, push.Payload{
		Title:       payload.Title,
		Body:        payload.Body,
		CollapseTag: payload.CollapseTag,
		Counter:     payload.Counter,
		Data:        payload.Data,
	})
	return nil
}

type fakeTokenRepo struct {
	byProfile map[uuid.UUID][]store.DeviceToken
}

func (f *fakeTokenRepo) ListByProfile(_ context.Context, profileID uuid.UUID) ([]store.DeviceToken, error) {
	return f.byProfile[profileID], nil
}

func (f *fakeTokenRepo) DeleteByToken(context.Context, string) error { return nil }

func TestShouldDeliverPushToToken(t *testing.T) {
	require.True(t, dispatch.ShouldDeliverPushToToken("new_message", "fcm"))
	require.True(t, dispatch.ShouldDeliverPushToToken("new_message", "apns"))
	require.False(t, dispatch.ShouldDeliverPushToToken("new_message", "voip_apns"))
	require.True(t, dispatch.ShouldDeliverPushToToken("incoming_call", "voip_apns"))
	require.False(t, dispatch.ShouldDeliverPushToToken("incoming_call", "fcm"))
}

func TestMessagePusher_NoTokensNoSend(t *testing.T) {
	rec := &recordingFCM{}
	profileID := uuid.New()
	err := (&dispatch.MessagePusher{
		Tokens:   &fakeTokenRepo{byProfile: map[uuid.UUID][]store.DeviceToken{}},
		Pusher:   &dispatch.PushDispatcher{FCM: rec},
		Grouping: grouping.NewMemoryStore(),
	}).SendPush(context.Background(), map[string]delivery.DeliveryDecision{
		profileID.String(): {Push: true},
	}, delivery.DeliveryInput{
		ChatID: "chat-1",
		Type:   delivery.TypeNewMessage,
	}, push.Payload{
		Body: "Hello",
		Data: map[string]string{"type": "new_message"},
	}, "Hello")
	require.NoError(t, err)
	require.Empty(t, rec.sent)
}

func TestMessagePusher_SendsFCMWithGrouping(t *testing.T) {
	rec := &recordingFCM{}
	profileID := uuid.New()
	err := (&dispatch.MessagePusher{
		Tokens: &fakeTokenRepo{byProfile: map[uuid.UUID][]store.DeviceToken{
			profileID: {{Token: "tok-fcm", PushService: "fcm"}},
		}},
		Pusher:   &dispatch.PushDispatcher{FCM: rec},
		Grouping: grouping.NewMemoryStore(),
	}).SendPush(context.Background(), map[string]delivery.DeliveryDecision{
		profileID.String(): {Push: true},
	}, delivery.DeliveryInput{
		ChatID: "chat-1",
		Type:   delivery.TypeNewMessage,
	}, push.Payload{
		Body: "Hello",
		Data: map[string]string{"type": "new_message"},
	}, "Hello")
	require.NoError(t, err)
	require.Len(t, rec.sent, 1)
	require.Equal(t, 1, rec.sent[0].Counter)
	require.NotEmpty(t, rec.sent[0].CollapseTag)
}

func TestMessagePusher_SkipsVoIPToken(t *testing.T) {
	rec := &recordingFCM{}
	profileID := uuid.New()
	err := (&dispatch.MessagePusher{
		Tokens: &fakeTokenRepo{byProfile: map[uuid.UUID][]store.DeviceToken{
			profileID: {{Token: "voip-tok", PushService: "voip_apns"}},
		}},
		Pusher:   &dispatch.PushDispatcher{FCM: rec},
		Grouping: grouping.NewMemoryStore(),
	}).SendPush(context.Background(), map[string]delivery.DeliveryDecision{
		profileID.String(): {Push: true},
	}, delivery.DeliveryInput{
		Type: delivery.TypeNewMessage,
	}, push.Payload{
		Data: map[string]string{"type": "new_message"},
	}, "Hello")
	require.NoError(t, err)
	require.Empty(t, rec.sent)
}

func TestMatchmakingPusher_FallbackWhenNoTokens(t *testing.T) {
	rec := &recordingFCM{}
	pusher := &dispatch.MatchmakingPusher{
		Tokens: &fakeTokenRepo{byProfile: map[uuid.UUID][]store.DeviceToken{}},
		Pusher: &dispatch.PushDispatcher{FCM: rec},
	}
	err := pusher.SendPush(context.Background(), map[string]delivery.DeliveryDecision{
		uuid.NewString(): {Push: true},
	}, push.Payload{
		Data: map[string]string{"type": "match_found"},
	})
	require.NoError(t, err)
	require.Len(t, rec.sent, 1)
}

func TestMatchmakingPusher_SkipsVoIPToken(t *testing.T) {
	rec := &recordingFCM{}
	profileID := uuid.New()
	pusher := &dispatch.MatchmakingPusher{
		Tokens: &fakeTokenRepo{byProfile: map[uuid.UUID][]store.DeviceToken{
			profileID: {
				{Token: "fcm-tok", PushService: "fcm"},
				{Token: "voip-tok", PushService: "voip_apns"},
			},
		}},
		Pusher: &dispatch.PushDispatcher{FCM: rec},
	}
	err := pusher.SendPush(context.Background(), map[string]delivery.DeliveryDecision{
		profileID.String(): {Push: true},
	}, push.Payload{
		Data: map[string]string{"type": "match_found"},
	})
	require.NoError(t, err)
	require.Len(t, rec.sent, 1)
}

func TestMessagePusher_EnrichDecision_OfflineGetsPush(t *testing.T) {
	profileID := uuid.New()
	decision, err := (&dispatch.MessagePusher{}).EnrichDecision(
		context.Background(),
		profileID.String(),
		uuid.New(),
		"chat-1",
		delivery.TypeNewMessage,
	)
	require.NoError(t, err)
	require.True(t, decision.Push)
}

type mutedPolicy struct{}

func (mutedPolicy) LoadPolicy(context.Context, uuid.UUID, string, delivery.NotificationType, time.Time) (delivery.SettingsSnapshot, delivery.QuietHoursSnapshot, error) {
	return delivery.SettingsSnapshot{ChatMuted: true}, delivery.QuietHoursSnapshot{}, nil
}

func TestMessagePusher_MutedChatSkipsPush(t *testing.T) {
	rec := &recordingFCM{}
	profileID := uuid.New()
	err := (&dispatch.MessagePusher{
		Tokens: &fakeTokenRepo{byProfile: map[uuid.UUID][]store.DeviceToken{
			profileID: {{Token: "tok", PushService: "fcm"}},
		}},
		Pusher:   &dispatch.PushDispatcher{FCM: rec},
		Grouping: grouping.NewMemoryStore(),
		Policy:   mutedPolicy{},
	}).SendPush(context.Background(), map[string]delivery.DeliveryDecision{
		profileID.String(): {Push: true},
	}, delivery.DeliveryInput{
		ChatID: "chat-1",
		Type:   delivery.TypeNewMessage,
	}, push.Payload{
		Data: map[string]string{"type": "new_message"},
	}, "Hello")
	require.NoError(t, err)
	require.Empty(t, rec.sent)
}
