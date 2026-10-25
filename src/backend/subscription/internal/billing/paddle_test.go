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

func TestWebhookSecretProductionPolicy(t *testing.T) {
	for _, environment := range []string{"production", " prod ", "PRODUCTION"} {
		for _, secret := range []string{"", " \t", defaultWebhookSecret, " " + defaultWebhookSecret + " "} {
			for _, override := range []string{"", "true"} {
				t.Run(environment+"/"+secret+"/override="+override, func(t *testing.T) {
					t.Setenv("VOICE_ENV", environment)
					t.Setenv("PADDLE_WEBHOOK_SECRET", secret)
					t.Setenv("SUBSCRIPTION_ALLOW_TEST_WEBHOOK_SECRET", override)
					require.ErrorIs(t, ValidateWebhookSecretConfig(), ErrWebhookSecretNotConfigured)

					body := `{"event_id":"evt_unsafe","event_type":"subscription.activated"}`
					// Even a correctly signed payload cannot authorize billing with
					// the public fixture key in a production process.
					require.ErrorIs(t, VerifySignature(body, SignWebhookForTest(body)), ErrWebhookSecretNotConfigured)
				})
			}
		}
	}
}

func TestWebhookSecretProductionConfiguredKey(t *testing.T) {
	t.Setenv("VOICE_ENV", "production")
	t.Setenv("PADDLE_WEBHOOK_SECRET", "configured-private-test-fixture")
	t.Setenv("SUBSCRIPTION_ALLOW_TEST_WEBHOOK_SECRET", "true")
	require.NoError(t, ValidateWebhookSecretConfig())
	body := `{"event_id":"evt_configured","event_type":"subscription.activated"}`
	require.NoError(t, VerifySignature(body, SignWebhookForTest(body)))

	t.Setenv("PADDLE_WEBHOOK_SECRET", defaultWebhookSecret)
	defaultSignature := SignWebhookForTest(body)
	t.Setenv("PADDLE_WEBHOOK_SECRET", "configured-private-test-fixture")
	require.Error(t, VerifySignature(body, defaultSignature))
}

func TestWebhookSecretNonproductionFixture(t *testing.T) {
	for _, environment := range []string{"", "development", "test"} {
		t.Run(environment, func(t *testing.T) {
			t.Setenv("VOICE_ENV", environment)
			t.Setenv("PADDLE_WEBHOOK_SECRET", "")
			t.Setenv("SUBSCRIPTION_ALLOW_TEST_WEBHOOK_SECRET", "")
			require.NoError(t, ValidateWebhookSecretConfig())
			require.Equal(t, defaultWebhookSecret, WebhookSecret())
			body := `{"event_id":"evt_local","event_type":"subscription.activated"}`
			require.NoError(t, VerifySignature(body, SignWebhookForTest(body)))
		})
	}
}

func TestSignWebhookForTest_roundTrip(t *testing.T) {
	payload, err := json.Marshal(map[string]string{"event_id": "evt_round"})
	require.NoError(t, err)
	body := string(payload)
	require.NoError(t, VerifySignature(body, SignWebhookForTest(body)))
}
