package dispatch

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/apns"
	"voice/backend/notification/internal/delivery"
	"voice/backend/notification/internal/push"
	"voice/backend/notification/internal/store"
)

type callPusherTestStore struct {
	profileID uuid.UUID
	tokens    []store.DeviceToken
}

func (s *callPusherTestStore) ListByProfile(_ context.Context, id uuid.UUID) ([]store.DeviceToken, error) {
	if id != s.profileID {
		return nil, nil
	}
	return s.tokens, nil
}

func (s *callPusherTestStore) DeleteByToken(context.Context, string) error {
	return nil
}

func TestCallPusher_FiltersVoIPTokensOnly(t *testing.T) {
	profileID := uuid.New()
	voipRec := &recordingVoIPSender{}
	pusher := &PushDispatcher{VoIP: voipRec}
	cp := &CallPusher{
		Tokens: &callPusherTestStore{
			profileID: profileID,
			tokens: []store.DeviceToken{
				{Token: "regular", PushService: "apns"},
				{Token: "voip-device", PushService: "voip_apns"},
			},
		},
		Pusher: pusher,
	}
	err := cp.SendPush(context.Background(), map[string]delivery.DeliveryDecision{
		profileID.String(): {Push: true},
	}, push.Payload{Data: map[string]string{"type": "incoming_call", "room_id": "r1"}})
	require.NoError(t, err)
	require.Len(t, voipRec.sent, 1)
	require.Equal(t, "voip-device", voipRec.sent[0].Token)
}

func TestCallPusher_DeletesInvalidVoIPToken(t *testing.T) {
	profileID := uuid.New()
	deleted := ""
	cp := &CallPusher{
		Tokens: &deletingTokenStore{
			callPusherTestStore: callPusherTestStore{
				profileID: profileID,
				tokens:    []store.DeviceToken{{Token: "bad-voip", PushService: "voip_apns"}},
			},
			deleted: &deleted,
		},
		Pusher: &PushDispatcher{VoIP: invalidVoIPSender{}},
	}
	err := cp.SendPush(context.Background(), map[string]delivery.DeliveryDecision{
		profileID.String(): {Push: true},
	}, push.Payload{Data: map[string]string{"type": "incoming_call"}})
	require.NoError(t, err)
	require.Equal(t, "bad-voip", deleted)
}

type invalidVoIPSender struct{}

func (invalidVoIPSender) Send(context.Context, uuid.UUID, store.DeviceToken, push.Payload) error {
	return apns.ErrInvalidToken
}

type deletingTokenStore struct {
	callPusherTestStore
	deleted *string
}

func (s *deletingTokenStore) DeleteByToken(_ context.Context, token string) error {
	*s.deleted = token
	return nil
}

func TestCallPusher_NoVoIPTokensStillAttemptsFallback(t *testing.T) {
	profileID := uuid.New()
	voipRec := &recordingVoIPSender{}
	cp := &CallPusher{
		Tokens: &callPusherTestStore{
			profileID: profileID,
			tokens:    []store.DeviceToken{{Token: "apns-only", PushService: "apns"}},
		},
		Pusher: &PushDispatcher{VoIP: voipRec},
	}
	err := cp.SendPush(context.Background(), map[string]delivery.DeliveryDecision{
		profileID.String(): {Push: true},
	}, push.Payload{Data: map[string]string{"type": "incoming_call"}})
	require.NoError(t, err)
	require.Len(t, voipRec.sent, 1)
	require.Empty(t, voipRec.sent[0].Token)
}

func TestCallPusher_SkipsWhenPushFalse(t *testing.T) {
	voipRec := &recordingVoIPSender{}
	cp := &CallPusher{
		Tokens: &callPusherTestStore{profileID: uuid.New()},
		Pusher: &PushDispatcher{VoIP: voipRec},
	}
	err := cp.SendPush(context.Background(), map[string]delivery.DeliveryDecision{
		uuid.NewString(): {Push: false},
	}, push.Payload{})
	require.NoError(t, err)
	require.Empty(t, voipRec.sent)
}
