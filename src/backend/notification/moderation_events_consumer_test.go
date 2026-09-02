package main

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/consumer"
	"voice/backend/notification/internal/delivery"
	"voice/backend/notification/internal/dispatch"
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

type alwaysOnlinePresence struct{}

func (alwaysOnlinePresence) IsOnline(context.Context, uuid.UUID) (bool, error) {
	return true, nil
}

func TestEnrichSanctionDecisions_OnlineRecipientStillPush(t *testing.T) {
	profileID := uuid.New()
	raw := map[string]delivery.DeliveryDecision{
		profileID.String(): {InApp: true, Push: true},
	}

	// Contrast: normal EnrichDecisions with online presence drops Push.
	withPresence, err := dispatch.EnrichDecisions(
		context.Background(),
		alwaysOnlinePresence{},
		delivery.PermissivePolicyLoader{},
		raw,
		uuid.Nil,
		"",
		delivery.TypeSystem,
	)
	require.NoError(t, err)
	require.Equal(t, delivery.DeliveryDecision{InApp: true, Push: false}, withPresence[profileID.String()])

	// Sanction path skips presence so online users still get push delivery.
	enriched, err := enrichSanctionDecisions(context.Background(), delivery.PermissivePolicyLoader{}, raw)
	require.NoError(t, err)
	require.Equal(t, delivery.DeliveryDecision{InApp: true, Push: true}, enriched[profileID.String()])
}

func TestRouteModerationNotification_OnlineRecipientDecisionStaysPushable(t *testing.T) {
	accountID := uuid.New()
	profileID := uuid.New()
	handler := &consumer.ModerationEventHandler{}
	profiles := &stubAccountProfiles{ids: []uuid.UUID{profileID}}
	env := &eventsv1.ModerationStreamEvent{
		Payload: &eventsv1.ModerationStreamEvent_SanctionApplied{
			SanctionApplied: &eventsv1.SanctionApplied{
				SanctionId:      uuid.NewString(),
				TargetAccountId: accountID.String(),
				Type:            "temp_ban",
			},
		},
	}
	decisions, payload, ok, err := routeModerationNotification(context.Background(), handler, profiles, env)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "system", payload.Data["type"])

	enriched, err := enrichSanctionDecisions(context.Background(), delivery.PermissivePolicyLoader{}, decisions)
	require.NoError(t, err)
	require.True(t, enriched[profileID.String()].Push, "online presence-skip must keep sanction push")
	require.True(t, enriched[profileID.String()].InApp)
}
