package consumer_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/notification/internal/consumer"
	"voice/backend/notification/internal/delivery"
	eventsv1 "voice.app/voice/events/v1"
)

func TestVoiceEventHandler_OfflineCalleeGetsPush(t *testing.T) {
	callee := uuid.NewString()
	initiator := uuid.NewString()
	h := &consumer.VoiceEventHandler{Router: delivery.DecideRouting}
	decisions := h.HandleCallIncoming(context.Background(), &eventsv1.CallIncoming{
		RoomId:              uuid.NewString(),
		ChatId:              uuid.NewString(),
		InitiatorProfileId:  initiator,
		CalleeProfileId:     callee,
		MediaKind:           "audio",
		LivekitRoomName:     "lk",
		ExpiresAt:           timestamppb.New(time.Now().UTC().Add(30 * time.Second)),
	}, false)
	require.Len(t, decisions, 1)
	require.True(t, decisions[callee].Push)
	require.True(t, decisions[callee].InApp)
}

func TestVoiceEventHandler_OnlineCalleeNoPush(t *testing.T) {
	callee := uuid.NewString()
	initiator := uuid.NewString()
	h := &consumer.VoiceEventHandler{Router: delivery.DecideRouting}
	decisions := h.HandleCallIncoming(context.Background(), &eventsv1.CallIncoming{
		RoomId:             uuid.NewString(),
		ChatId:             uuid.NewString(),
		InitiatorProfileId: initiator,
		CalleeProfileId:    callee,
		MediaKind:          "video",
	}, true)
	require.Len(t, decisions, 1)
	require.False(t, decisions[callee].Push)
	require.True(t, decisions[callee].InApp)
}

func TestVoiceEventHandler_InitiatorExcluded(t *testing.T) {
	profile := uuid.NewString()
	h := &consumer.VoiceEventHandler{Router: delivery.DecideRouting}
	decisions := h.HandleCallIncoming(context.Background(), &eventsv1.CallIncoming{
		RoomId:             uuid.NewString(),
		InitiatorProfileId: profile,
		CalleeProfileId:    profile,
	}, false)
	require.Len(t, decisions, 1)
	require.False(t, decisions[profile].Push)
	require.False(t, decisions[profile].InApp)
}
