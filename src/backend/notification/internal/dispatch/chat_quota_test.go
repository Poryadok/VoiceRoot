package dispatch_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"voice/backend/notification/internal/chatquota"
	"voice/backend/notification/internal/delivery"
	"voice/backend/notification/internal/dispatch"
	"voice/backend/notification/internal/fcm"
	"voice/backend/notification/internal/grouping"
	"voice/backend/notification/internal/push"
	"voice/backend/notification/internal/store"
)

type admissionSpy struct {
	calls int
	allow bool
	err   error
}

type quotaPartialFCM struct {
	calls  int
	failed bool
}

func (s *quotaPartialFCM) Send(context.Context, uuid.UUID, store.DeviceToken, fcm.PushPayload) error {
	s.calls++
	if s.calls == 2 && !s.failed {
		s.failed = true
		return errors.New("temporary provider failure")
	}
	return nil
}

func TestChatPushQuotaPartialFanoutRetrySurvivesFullBucket(t *testing.T) {
	ctx := context.Background()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer func() { _ = client.Close() }()
	quota, err := chatquota.New(client, chatquota.Config{Limit: 1, Window: time.Minute})
	require.NoError(t, err)
	recipient, chat, message := uuid.New(), uuid.NewString(), uuid.NewString()
	sender := &quotaPartialFCM{}
	p := &dispatch.MessagePusher{Admission: quota, Tokens: &fakeTokenRepo{byProfile: map[uuid.UUID][]store.DeviceToken{recipient: {{Token: "a", PushService: "fcm"}, {Token: "b", PushService: "fcm"}}}}, Pusher: &dispatch.PushDispatcher{FCM: sender}}
	send := func(id string) error {
		return p.SendPreparedPush(ctx, map[string]delivery.DeliveryDecision{recipient.String(): {Push: true}}, delivery.DeliveryInput{ChatID: chat, Type: delivery.TypeNewMessage}, push.Payload{Data: map[string]string{"chat_id": chat, "message_id": id}}, "")
	}
	require.Error(t, send(message))
	require.Equal(t, 2, sender.calls)
	require.NoError(t, send(uuid.NewString()))
	require.Equal(t, 2, sender.calls, "full quota suppresses new message")
	require.NoError(t, send(message))
	require.Equal(t, 4, sender.calls, "original admission still permits unsent device; delivered device may repeat")
	p.Policy = mutedPolicy{}
	require.NoError(t, p.SendPush(ctx, map[string]delivery.DeliveryDecision{recipient.String(): {Push: true}}, delivery.DeliveryInput{ChatID: chat, Type: delivery.TypeNewMessage}, push.Payload{Data: map[string]string{"chat_id": chat, "message_id": message}}, ""))
	require.Equal(t, 4, sender.calls, "cached admission never bypasses new mute")
}

func (s *admissionSpy) Admit(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (bool, error) {
	s.calls++
	return s.allow, s.err
}

func TestChatPushQuotaSuppressionErrorAndIdentityHaveNoGroupingOrProviderEffect(t *testing.T) {
	for _, tc := range []struct {
		name    string
		allow   bool
		err     error
		message string
		wantErr bool
	}{
		{"suppressed", false, nil, uuid.NewString(), false},
		{"redis error", false, errors.New("Redis unavailable"), uuid.NewString(), true},
		{"missing identity", true, nil, "", true},
		{"noncanonical identity", true, nil, "AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			recipient, chat := uuid.New(), uuid.NewString()
			spy := &admissionSpy{allow: tc.allow, err: tc.err}
			rec := &recordingFCM{}
			groups := grouping.NewMemoryStore()
			p := &dispatch.MessagePusher{Admission: spy, Grouping: groups, Tokens: &fakeTokenRepo{byProfile: map[uuid.UUID][]store.DeviceToken{recipient: {{Token: "a", PushService: "fcm"}, {Token: "b", PushService: "fcm"}}}}, Pusher: &dispatch.PushDispatcher{FCM: rec}}
			err := p.SendPreparedPush(ctx, map[string]delivery.DeliveryDecision{recipient.String(): {Push: true}}, delivery.DeliveryInput{ChatID: chat, Type: delivery.TypeNewMessage}, push.Payload{Data: map[string]string{"chat_id": chat, "message_id": tc.message}}, "body")
			require.Equal(t, tc.wantErr, err != nil)
			require.Empty(t, rec.sent)
			state, err := groups.Get(ctx, delivery.GroupingKey(recipient, chat))
			require.NoError(t, err)
			require.Nil(t, state)
			if tc.message == "" {
				require.Zero(t, spy.calls)
			}
		})
	}
}

func TestChatPushQuotaLogicalVariantsDevicesAndRetries(t *testing.T) {
	ctx := context.Background()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer func() { _ = client.Close() }()
	quota, err := chatquota.New(client, chatquota.Config{Limit: 1, Window: time.Minute})
	require.NoError(t, err)
	recipient, chat, message := uuid.New(), uuid.NewString(), uuid.NewString()
	rec := &recordingFCM{}
	groups := grouping.NewMemoryStore()
	p := &dispatch.MessagePusher{Admission: quota, Grouping: groups, Tokens: &fakeTokenRepo{byProfile: map[uuid.UUID][]store.DeviceToken{recipient: {{Token: "a", PushService: "fcm"}, {Token: "b", PushService: "fcm"}}}}, Pusher: &dispatch.PushDispatcher{FCM: rec}}
	send := func(typ delivery.NotificationType, msg string) {
		require.NoError(t, p.SendPreparedPush(ctx, map[string]delivery.DeliveryDecision{recipient.String(): {Push: true}}, delivery.DeliveryInput{ChatID: chat, Type: typ}, push.Payload{Silent: true, Data: map[string]string{"chat_id": chat, "message_id": msg}}, "body"))
	}
	send(delivery.TypeNewMessage, message)
	require.Len(t, rec.sent, 2)
	require.Equal(t, 1, rec.sent[0].Counter)
	require.Equal(t, rec.sent[0].Counter, rec.sent[1].Counter)
	require.True(t, rec.sent[1].Silent)
	send(delivery.TypeMention, message)
	require.Len(t, rec.sent, 4, "same message admission is shared across variants and devices")
	send(delivery.TypeReply, uuid.NewString())
	require.Len(t, rec.sent, 4, "another message in same collapse key spends independently")
	require.NoError(t, quota.Close())
	err = p.SendPreparedPush(ctx, map[string]delivery.DeliveryDecision{recipient.String(): {Push: true}}, delivery.DeliveryInput{ChatID: chat, Type: delivery.TypeNewMessage}, push.Payload{Data: map[string]string{"chat_id": chat, "message_id": uuid.NewString()}}, "")
	require.Error(t, err)
	require.Len(t, rec.sent, 4)
}

func TestChatPushQuotaNoEligibleDeviceAndLifecycleSpendNothing(t *testing.T) {
	for _, kind := range []string{"no tokens", "wrong type", "frozen chat", "blocked game", "frozen dispatcher"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			recipient := uuid.New()
			spy := &admissionSpy{allow: true}
			rec := &recordingFCM{}
			tokens := []store.DeviceToken{{Token: "a", PushService: "fcm"}}
			if kind == "no tokens" {
				tokens = nil
			}
			if kind == "wrong type" {
				tokens[0].PushService = "voip_apns"
			}
			p := &dispatch.MessagePusher{Admission: spy, Tokens: &fakeTokenRepo{byProfile: map[uuid.UUID][]store.DeviceToken{recipient: tokens}}, Pusher: &dispatch.PushDispatcher{FCM: rec}}
			in := delivery.DeliveryInput{ChatID: uuid.NewString(), Type: delivery.TypeNewMessage}
			if kind == "frozen chat" {
				p.LifecycleDelivery = suppressedLifecycleDelivery{}
			}
			if kind == "blocked game" {
				in.GameApplicationID = uuid.New()
				in.GameEnvironmentID = uuid.New()
				in.GameCategory = "game_activity"
				p.GameConsent = &gamePushConsentSequence{answers: []bool{false}}
				p.GameBlocks = unblockedGamePush{}
			}
			if kind == "frozen dispatcher" {
				p.Pusher.LifecycleDelivery = quotaLifecycleFence{}
			}
			require.NoError(t, p.SendPreparedPush(ctx, map[string]delivery.DeliveryDecision{recipient.String(): {Push: true}}, in, push.Payload{Data: map[string]string{"chat_id": in.ChatID, "message_id": uuid.NewString()}}, ""))
			require.Zero(t, spy.calls)
			require.Empty(t, rec.sent)
		})
	}
}

type quotaLifecycleFence struct{}

func (quotaLifecycleFence) WithChatDelivery(context.Context, string, func(context.Context) error) error {
	return nil
}
func (quotaLifecycleFence) WithSpaceDelivery(context.Context, string, func(context.Context) error) error {
	return nil
}

func TestChatPushQuotaOutOfScopeNeverConsultsAdmission(t *testing.T) {
	ctx := context.Background()
	recipient := uuid.New()
	spy := &admissionSpy{err: errors.New("Redis down")}
	rec := &recordingFCM{}
	p := &dispatch.MessagePusher{Admission: spy, Tokens: &fakeTokenRepo{byProfile: map[uuid.UUID][]store.DeviceToken{recipient: {{Token: "a", PushService: "fcm"}}}}, Pusher: &dispatch.PushDispatcher{FCM: rec}}
	for _, typ := range []delivery.NotificationType{"system", "subscription_grace_reminder", "match_found", "voice_member_joined", "incoming_call"} {
		require.NoError(t, p.SendPreparedPush(ctx, map[string]delivery.DeliveryDecision{recipient.String(): {Push: true}}, delivery.DeliveryInput{Type: typ}, push.Payload{}, ""))
	}
	require.Zero(t, spy.calls)
	// Direct common dispatcher is independent even while ordinary admission fails.
	require.NoError(t, p.Pusher.Send(ctx, recipient, store.DeviceToken{PushService: "fcm"}, push.Payload{Data: map[string]string{"type": "system"}}))
}
