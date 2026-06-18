package webhook_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"voice/backend/bot/internal/webhook"
)

func TestSignAndVerify(t *testing.T) {
	secret := "test-secret"
	body := []byte(`{"type":"slash_command"}`)
	ts := time.Now().Unix()
	sig := webhook.Sign(secret, ts, body)
	require.True(t, webhook.Verify(secret, sig, formatTS(ts), body, time.Now()))
	require.False(t, webhook.Verify(secret, sig, formatTS(ts), []byte("tampered"), time.Now()))
}

func TestVerify_rejectsOldSecretAfterRotation(t *testing.T) {
	oldSecret := "old-secret"
	newSecret := "new-secret"
	body := []byte(`{"type":"slash_command"}`)
	ts := time.Now().Unix()
	sig := webhook.Sign(oldSecret, ts, body)
	require.False(t, webhook.Verify(newSecret, sig, formatTS(ts), body, time.Now()))
	require.True(t, webhook.Verify(newSecret, webhook.Sign(newSecret, ts, body), formatTS(ts), body, time.Now()))
}

func formatTS(ts int64) string {
	return strconv.FormatInt(ts, 10)
}
