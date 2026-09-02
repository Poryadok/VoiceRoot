package main

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/consumer"
	"voice/backend/notification/internal/delivery"
	eventsv1 "voice.app/voice/events/v1"
)

type stubAccountProfiles struct {
	ids []uuid.UUID
	err error
	got uuid.UUID
}

func (s *stubAccountProfiles) ProfileIDsForAccount(_ context.Context, accountID uuid.UUID) ([]uuid.UUID, error) {
	s.got = accountID
	if s.err != nil {
		return nil, s.err
	}
	return s.ids, nil
}

func TestRouteModerationNotification_SanctionApplied(t *testing.T) {
	accountID := uuid.New()
	profileID := uuid.New()
	handler := &consumer.ModerationEventHandler{}
	profiles := &stubAccountProfiles{ids: []uuid.UUID{profileID}}
	env := &eventsv1.ModerationStreamEvent{
		Payload: &eventsv1.ModerationStreamEvent_SanctionApplied{
			SanctionApplied: &eventsv1.SanctionApplied{
				SanctionId:      uuid.NewString(),
				TargetAccountId: accountID.String(),
				Type:            "warning",
			},
		},
	}
	decisions, payload, ok, err := routeModerationNotification(context.Background(), handler, profiles, env)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, accountID, profiles.got)
	require.Equal(t, map[string]delivery.DeliveryDecision{
		profileID.String(): {InApp: true, Push: true},
	}, decisions)
	require.Equal(t, "system", payload.Data["type"])
	require.Equal(t, "You received a warning from moderation", payload.Body)
}

func TestRouteModerationNotification_ShadowBanSkipped(t *testing.T) {
	handler := &consumer.ModerationEventHandler{}
	profiles := &stubAccountProfiles{ids: []uuid.UUID{uuid.New()}}
	env := &eventsv1.ModerationStreamEvent{
		Payload: &eventsv1.ModerationStreamEvent_SanctionApplied{
			SanctionApplied: &eventsv1.SanctionApplied{
				SanctionId:      uuid.NewString(),
				TargetAccountId: uuid.NewString(),
				Type:            "shadow_ban",
			},
		},
	}
	decisions, _, ok, err := routeModerationNotification(context.Background(), handler, profiles, env)
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, decisions)
	require.Equal(t, uuid.Nil, profiles.got)
}

func TestRouteModerationNotification_NoResolver(t *testing.T) {
	handler := &consumer.ModerationEventHandler{}
	env := &eventsv1.ModerationStreamEvent{
		Payload: &eventsv1.ModerationStreamEvent_SanctionApplied{
			SanctionApplied: &eventsv1.SanctionApplied{
				SanctionId:      uuid.NewString(),
				TargetAccountId: uuid.NewString(),
				Type:            "temp_ban",
			},
		},
	}
	decisions, _, ok, err := routeModerationNotification(context.Background(), handler, nil, env)
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, decisions)
}

func TestRouteModerationNotification_UnknownPayload(t *testing.T) {
	handler := &consumer.ModerationEventHandler{}
	env := &eventsv1.ModerationStreamEvent{}
	_, _, ok, err := routeModerationNotification(context.Background(), handler, &stubAccountProfiles{}, env)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestSanctionPushBody(t *testing.T) {
	require.Equal(t, "You have been banned from matchmaking", sanctionPushBody("mm_ban"))
	require.Equal(t, "Your account has been permanently banned", sanctionPushBody("perm_ban"))
}
