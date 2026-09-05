package dispatch_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/delivery"
	"voice/backend/notification/internal/dispatch"
)

type onlinePresenceChecker struct{}

func (onlinePresenceChecker) IsOnline(context.Context, uuid.UUID) (bool, error) {
	return true, nil
}

type suppressTypePolicyLoader struct {
	typ delivery.NotificationType
}

func (p suppressTypePolicyLoader) LoadPolicy(context.Context, uuid.UUID, string, delivery.NotificationType, time.Time) (delivery.SettingsSnapshot, delivery.QuietHoursSnapshot, error) {
	return delivery.SettingsSnapshot{SuppressTypes: []delivery.NotificationType{p.typ}}, delivery.QuietHoursSnapshot{}, nil
}

func TestEnrichDecisions_OnlineMatchFoundKeepsPushEligible(t *testing.T) {
	recipient := uuid.New()

	decisions, err := dispatch.EnrichDecisions(
		t.Context(),
		onlinePresenceChecker{},
		delivery.PermissivePolicyLoader{},
		map[string]delivery.DeliveryDecision{recipient.String(): {}},
		uuid.Nil,
		"",
		delivery.TypeMatchFound,
	)
	require.NoError(t, err)
	require.True(t, decisions[recipient.String()].InApp)
	require.True(t, decisions[recipient.String()].Push, "match_found must skip online presence while evaluating push policy")
}

func TestEnrichDecisions_OnlineVoiceMemberJoinedKeepsPushEligible(t *testing.T) {
	recipient := uuid.New()

	decisions, err := dispatch.EnrichDecisions(
		t.Context(),
		onlinePresenceChecker{},
		delivery.PermissivePolicyLoader{},
		map[string]delivery.DeliveryDecision{recipient.String(): {}},
		uuid.New(),
		"",
		delivery.TypeVoiceMemberJoined,
	)
	require.NoError(t, err)
	require.True(t, decisions[recipient.String()].InApp)
	require.True(t, decisions[recipient.String()].Push, "voice_member_joined must skip online presence while evaluating push policy")
}

func TestEnrichDecisions_PresenceExceptionsStillApplySuppressTypes(t *testing.T) {
	for _, typ := range []delivery.NotificationType{delivery.TypeMatchFound, delivery.TypeVoiceMemberJoined} {
		t.Run(string(typ), func(t *testing.T) {
			recipient := uuid.New()
			decisions, err := dispatch.EnrichDecisions(
				t.Context(),
				onlinePresenceChecker{},
				suppressTypePolicyLoader{typ: typ},
				map[string]delivery.DeliveryDecision{recipient.String(): {}},
				uuid.Nil,
				"",
				typ,
			)
			require.NoError(t, err)
			require.False(t, decisions[recipient.String()].InApp)
			require.False(t, decisions[recipient.String()].Push, "suppress_types must still filter %s after presence exception", typ)
		})
	}
}
