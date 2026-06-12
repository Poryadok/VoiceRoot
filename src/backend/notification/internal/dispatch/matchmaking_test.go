package dispatch

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/delivery"
	"voice/backend/notification/internal/fcm"
	"voice/backend/notification/internal/store"
)

type recordingFCM struct {
	sent []fcm.PushPayload
}

func (r *recordingFCM) Send(_ context.Context, _ uuid.UUID, _ store.DeviceToken, payload fcm.PushPayload) error {
	r.sent = append(r.sent, payload)
	return nil
}

func TestMatchmakingPusher_SendPush(t *testing.T) {
	rec := &recordingFCM{}
	pusher := &MatchmakingPusher{Tokens: &store.DeviceTokenStore{}, FCM: rec}
	profileID := uuid.NewString()
	err := pusher.SendPush(context.Background(), map[string]delivery.DeliveryDecision{
		profileID: {Push: true, InApp: true},
	}, fcm.PushPayload{
		Title: "Search ended",
		Body:  "Try again",
		Data:  map[string]string{"type": "search_timeout"},
	})
	require.NoError(t, err)
	require.Len(t, rec.sent, 1)
	require.Equal(t, "search_timeout", rec.sent[0].Data["type"])
}
