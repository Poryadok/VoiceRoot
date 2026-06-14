package billing

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestParseWebhook_requiresEventFields(t *testing.T) {
	_, err := ParseWebhook(`{"event_type":"subscription.activated"}`)
	require.Error(t, err)

	_, err = ParseWebhook(`{"event_id":"evt_1"}`)
	require.Error(t, err)

	ev, err := ParseWebhook(`{"event_id":"evt_1","event_type":"subscription.activated","data":{}}`)
	require.NoError(t, err)
	require.Equal(t, "evt_1", ev.EventID)
}

func TestVerifySignature_acceptsValidHeader(t *testing.T) {
	body := `{"event_id":"evt_1","event_type":"subscription.activated"}`
	sig := SignWebhookForTest(body)
	require.NoError(t, VerifySignature(body, sig))
}

func TestVerifySignature_rejectsTamperedBody(t *testing.T) {
	body := `{"event_id":"evt_1","event_type":"subscription.activated"}`
	sig := SignWebhookForTest(body)
	require.Error(t, VerifySignature(body+` `, sig))
}

func TestAccountIDFromCustomData(t *testing.T) {
	id := uuid.New()
	got, err := AccountIDFromCustomData(map[string]string{"account_id": id.String()})
	require.NoError(t, err)
	require.Equal(t, id, got)

	_, err = AccountIDFromCustomData(map[string]string{})
	require.Error(t, err)
}

func TestSpaceProFromCustomData(t *testing.T) {
	spaceID := uuid.New()
	purchaserID := uuid.New()
	gotSpace, gotPurchaser, err := SpaceProFromCustomData(map[string]string{
		"space_id":     spaceID.String(),
		"purchaser_id": purchaserID.String(),
	})
	require.NoError(t, err)
	require.Equal(t, spaceID, gotSpace)
	require.Equal(t, purchaserID, gotPurchaser)
}

func TestSignWebhookForTest_roundTrip(t *testing.T) {
	payload, err := json.Marshal(map[string]string{"event_id": "evt_round"})
	require.NoError(t, err)
	body := string(payload)
	require.NoError(t, VerifySignature(body, SignWebhookForTest(body)))
}
