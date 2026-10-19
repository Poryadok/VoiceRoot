package dispatch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

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
