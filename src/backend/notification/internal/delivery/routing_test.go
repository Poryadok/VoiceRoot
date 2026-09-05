package delivery_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/delivery"
)

func TestDecideRouting_OnlineNoPush(t *testing.T) {
	recipient := uuid.New()
	sender := uuid.New()
	decision := delivery.DecideRouting(delivery.DeliveryInput{
		RecipientProfileID: recipient,
		SenderProfileID:    sender,
		ChatID:             uuid.NewString(),
		Type:               delivery.TypeNewMessage,
		IsOnline:           true,
		At:                 time.Now().UTC(),
	})
	require.True(t, decision.InApp, "online users receive in-app notifications")
	require.False(t, decision.Push, "online users must not receive push")
}

func TestDecideRouting_OfflinePush(t *testing.T) {
	recipient := uuid.New()
	sender := uuid.New()
	decision := delivery.DecideRouting(delivery.DeliveryInput{
		RecipientProfileID: recipient,
		SenderProfileID:    sender,
		ChatID:             uuid.NewString(),
		Type:               delivery.TypeNewMessage,
		IsOnline:           false,
		At:                 time.Now().UTC(),
	})
	require.True(t, decision.InApp)
	require.True(t, decision.Push, "offline users receive push for new_message")
}

func TestDecideRouting_IncomingCallOfflinePush(t *testing.T) {
	recipient := uuid.New()
	sender := uuid.New()
	decision := delivery.DecideRouting(delivery.DeliveryInput{
		RecipientProfileID: recipient,
		SenderProfileID:    sender,
		Type:               delivery.TypeIncomingCall,
		IsOnline:           false,
		At:                 time.Now().UTC(),
	})
	require.True(t, decision.Push)
	require.True(t, decision.InApp)
}

func TestDecideRouting_IncomingCallOnlineNoPush(t *testing.T) {
	recipient := uuid.New()
	sender := uuid.New()
	decision := delivery.DecideRouting(delivery.DeliveryInput{
		RecipientProfileID: recipient,
		SenderProfileID:    sender,
		Type:               delivery.TypeIncomingCall,
		IsOnline:           true,
		At:                 time.Now().UTC(),
	})
	require.False(t, decision.Push)
	require.True(t, decision.InApp)
}

func TestDecideRouting_OnlineMatchFoundKeepsPushEligible(t *testing.T) {
	decision := delivery.DecideRouting(delivery.DeliveryInput{
		RecipientProfileID: uuid.New(),
		SenderProfileID:    uuid.New(),
		Type:               delivery.TypeMatchFound,
		IsOnline:           true,
		At:                 time.Now().UTC(),
	})
	require.True(t, decision.InApp)
	require.True(t, decision.Push, "match_found must skip online presence while evaluating push policy")
}

func TestDecideRouting_OnlineVoiceMemberJoinedKeepsPushEligible(t *testing.T) {
	decision := delivery.DecideRouting(delivery.DeliveryInput{
		RecipientProfileID: uuid.New(),
		SenderProfileID:    uuid.New(),
		Type:               delivery.TypeVoiceMemberJoined,
		IsOnline:           true,
		At:                 time.Now().UTC(),
	})
	require.True(t, decision.InApp)
	require.True(t, decision.Push, "voice_member_joined must skip online presence while evaluating push policy")
}

func TestDecideRouting_PresenceExceptionsStillExcludeSender(t *testing.T) {
	for _, typ := range []delivery.NotificationType{delivery.TypeMatchFound, delivery.TypeVoiceMemberJoined} {
		t.Run(string(typ), func(t *testing.T) {
			profileID := uuid.New()
			decision := delivery.DecideRouting(delivery.DeliveryInput{
				RecipientProfileID: profileID,
				SenderProfileID:    profileID,
				Type:               typ,
				IsOnline:           true,
				At:                 time.Now().UTC(),
			})
			require.False(t, decision.InApp)
			require.False(t, decision.Push, "sender exclusion must win over %s presence exception", typ)
		})
	}
}

func TestDecideRouting_SenderExcluded(t *testing.T) {
	sender := uuid.New()
	decision := delivery.DecideRouting(delivery.DeliveryInput{
		RecipientProfileID: sender,
		SenderProfileID:    sender,
		ChatID:             uuid.NewString(),
		Type:               delivery.TypeNewMessage,
		IsOnline:           false,
		At:                 time.Now().UTC(),
	})
	require.False(t, decision.InApp)
	require.False(t, decision.Push, "sender must not receive own notification")
}
