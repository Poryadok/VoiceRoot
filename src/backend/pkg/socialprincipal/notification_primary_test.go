package socialprincipal

import (
	"github.com/stretchr/testify/require"
	"testing"
)

const primaryMethod = "/voice.user.v1.UserService/ResolvePrimaryProfileIDs"

func TestNotificationPrimaryCapabilityIsSeparateAndMutualTLS(t *testing.T) {
	require.Equal(t, primaryMethod, Method("notification-primary"))
	require.True(t, AllowsMethod("notification-primary", primaryMethod))
	require.False(t, AllowsMethod("notification-primary", Method("notification")))
	require.False(t, AllowsMethod("notification", primaryMethod), "existing presence capability cannot gain account lookup")
	require.Equal(t, "notification", expectedIssuer("notification-primary"))
	require.True(t, RequiresMutualTLS("notification-primary"))
}

func TestNotificationPrimaryConfigIsOptInAndFailsClosedWhenPartial(t *testing.T) {
	t.Setenv("S2S_JWKS_URLS_JSON", `{"notification":"https://notification.example/jwks"}`)
	_, enabled, err := LoadFromEnvWithAudience("user", "notification-primary", "USER_NOTIFICATION_PRIMARY_PRINCIPAL_", ":9098")
	require.NoError(t, err)
	require.False(t, enabled)
	t.Setenv("USER_NOTIFICATION_PRIMARY_PRINCIPAL_TLS_CERT_FILE", "cert.pem")
	_, enabled, err = LoadFromEnvWithAudience("user", "notification-primary", "USER_NOTIFICATION_PRIMARY_PRINCIPAL_", ":9098")
	require.Error(t, err)
	require.True(t, enabled, "partial config cannot silently use legacy presence/headers")
}
