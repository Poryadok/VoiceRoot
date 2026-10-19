package dispatch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/delivery"
	"voice/backend/notification/internal/grouping"
	"voice/backend/notification/internal/push"
	"voice/backend/notification/internal/store"
	"voice/backend/pkg/releasepolicy"
)

func alphaReleasePolicy(t *testing.T) *releasepolicy.Policy {
	t.Helper()
	data, err := os.ReadFile("../../../pkg/releasepolicy/testdata/free-invite-alpha.json")
	require.NoError(t, err)
	sum := sha256.Sum256(data)
	p, err := releasepolicy.Parse(data, hex.EncodeToString(sum[:]), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	require.NoError(t, err)
	return p
}

func TestReleasePolicyFinalDispatchSuppressesOldMobileTokensAndPreservesWeb(t *testing.T) {
	fcm, apns, voip := &recordingFCMSender{}, &recordingAPNSTokens{}, &recordingVoIPSender{}
	suppressed := 0
	d := &PushDispatcher{FCM: fcm, APNs: apns, VoIP: voip, ReleasePolicy: alphaReleasePolicy(t), OnCapabilitySuppressed: func(context.Context) { suppressed++ }}
	ctx, id, payload := context.Background(), uuid.New(), push.Payload{Data: map[string]string{"type": "new_message"}}
	web := store.DeviceToken{Platform: "web", PushService: "fcm", Token: "web-control"}
	require.NoError(t, d.Send(ctx, id, web, payload), "positive observer control must really dispatch")
	for retry := 0; retry < 2; retry++ {
		for _, token := range []store.DeviceToken{{Platform: "android", PushService: "fcm", Token: "old-android"}, {Platform: "ios", PushService: "apns", Token: "old-ios"}, {Platform: "ios", PushService: "voip_apns", Token: "old-voip"}, {Platform: "ios", PushService: "fcm", Token: "old-ios-fcm"}} {
			require.NoError(t, d.Send(ctx, id, token, payload), "suppression completes queued/retried work without provider call")
		}
	}
	require.Len(t, fcm.sent, 1)
	require.Empty(t, apns.sent)
	require.Empty(t, voip.sent)
	require.Equal(t, 8, suppressed)
	require.NoError(t, d.Send(ctx, id, web, payload))
	require.Len(t, fcm.sent, 2, "observer remains healthy after all denials")
}

func TestReleasePolicyFinalDispatchRejectsUnknownOrConflictingMetadata(t *testing.T) {
	fcm, apns, voip := &recordingFCMSender{}, &recordingAPNSTokens{}, &recordingVoIPSender{}
	d := &PushDispatcher{FCM: fcm, APNs: apns, VoIP: voip, ReleasePolicy: alphaReleasePolicy(t)}
	for _, token := range []store.DeviceToken{{PushService: "fcm"}, {Platform: "windows", PushService: "fcm"}, {Platform: "web", PushService: "apns"}, {Platform: "android", PushService: "voip_apns"}, {Platform: "web", PushService: "unknown"}, {Platform: "web"}} {
		require.Error(t, d.Send(context.Background(), uuid.New(), token, push.Payload{}), "unknown/contradictory metadata cannot fall through to FCM")
	}
	require.Empty(t, fcm.sent)
	require.Empty(t, apns.sent)
	require.Empty(t, voip.sent)
}

type releasePreparationFence struct {
	active bool
	calls  int
}

func (f *releasePreparationFence) WithChatDelivery(ctx context.Context, _ string, next func(context.Context) error) error {
	f.calls++
	f.active = true
	defer func() { f.active = false }()
	return next(ctx)
}
func (f *releasePreparationFence) WithSpaceDelivery(ctx context.Context, _ string, next func(context.Context) error) error {
	return next(ctx)
}

func TestReleasePolicyEligibilityPrecedesQuotaPreparationUnderLifecycle(t *testing.T) {
	fence := &releasePreparationFence{}
	sender := &recordingFCMSender{}
	prepared := 0
	d := &PushDispatcher{ReleasePolicy: alphaReleasePolicy(t), FCM: sender, LifecycleDelivery: fence}
	payload := push.Payload{Data: map[string]string{"chat_id": uuid.NewString()}}
	prepare := func(context.Context) (push.Payload, bool, error) {
		require.True(t, fence.active)
		prepared++
		return payload, true, nil
	}
	require.NoError(t, d.PrepareAndSend(context.Background(), uuid.New(), store.DeviceToken{Platform: "android", PushService: "fcm"}, payload, prepare))
	require.Zero(t, prepared, "disabled legacy mobile token must not reserve quota/grouping")
	require.Error(t, d.PrepareAndSend(context.Background(), uuid.New(), store.DeviceToken{Platform: "web", PushService: "apns"}, payload, prepare))
	require.Zero(t, prepared, "conflicting target must not reserve quota/grouping")
	require.NoError(t, d.PrepareAndSend(context.Background(), uuid.New(), store.DeviceToken{Platform: "web", PushService: "fcm"}, payload, prepare))
	require.Equal(t, 1, prepared)
	require.Len(t, sender.sent, 1)
	require.Equal(t, 3, fence.calls)
}

type releaseQuotaSpy struct {
	calls int
	allow bool
	err   error
}

func (s *releaseQuotaSpy) Admit(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (bool, error) {
	s.calls++
	return s.allow, s.err
}

type releaseTokenRepo struct{ tokens []store.DeviceToken }

func (s releaseTokenRepo) ListByProfile(context.Context, uuid.UUID) ([]store.DeviceToken, error) {
	return s.tokens, nil
}
func (releaseTokenRepo) DeleteByToken(context.Context, string) error {
	return errors.New("unexpected token deletion")
}

func TestReleasePolicyMessageQuotaUnionNoDisabledEffectsAndWebControls(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		web, allow           bool
		quotaErr             error
		wantCalls, wantSends int
		wantErr              bool
	}{
		{"mobile-only", false, true, nil, 0, 0, false},
		{"mobile-only ignores Redis outage", false, false, errors.New("Redis unavailable"), 0, 0, false},
		{"mixed allow", true, true, nil, 1, 1, false},
		{"mixed deny", true, false, nil, 1, 0, false},
		{"mixed Redis outage", true, false, errors.New("Redis unavailable"), 1, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			recipient := uuid.New()
			chat := uuid.NewString()
			tokens := []store.DeviceToken{{Platform: "android", PushService: "fcm", Token: "old-mobile"}}
			if tc.web {
				tokens = append(tokens, store.DeviceToken{Platform: "web", PushService: "fcm", Token: "web-control"})
			}
			quota := &releaseQuotaSpy{allow: tc.allow, err: tc.quotaErr}
			sender := &recordingFCMSender{}
			groups := grouping.NewMemoryStore()
			p := &MessagePusher{Tokens: releaseTokenRepo{tokens}, Admission: quota, Grouping: groups, Pusher: &PushDispatcher{ReleasePolicy: alphaReleasePolicy(t), FCM: sender}}
			err := p.SendPreparedPush(ctx, map[string]delivery.DeliveryDecision{recipient.String(): {Push: true}}, delivery.DeliveryInput{ChatID: chat, Type: delivery.TypeNewMessage}, push.Payload{Data: map[string]string{"chat_id": chat, "message_id": uuid.NewString()}}, "synthetic")
			require.Equal(t, tc.wantErr, err != nil)
			require.Equal(t, tc.wantCalls, quota.calls)
			require.Len(t, sender.sent, tc.wantSends)
			state, err := groups.Get(ctx, delivery.GroupingKey(recipient, chat))
			require.NoError(t, err)
			if tc.wantSends == 0 {
				require.Nil(t, state, "denied/disabled delivery cannot alter grouping")
			} else {
				require.NotNil(t, state)
			}
		})
	}
}
