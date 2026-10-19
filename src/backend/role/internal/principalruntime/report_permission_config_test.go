package principalruntime

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestReportPermissionRequiresExactOptionalIssuerSet(t *testing.T) {
	f := newRuntimeFixture(t, 2)
	f.config.JWKSURLs["messaging"] = f.config.JWKSURLs["space"]
	f.config.JWKSURLs["moderation"] = f.config.JWKSURLs["space"]
	require.Error(t, f.config.validate(), "new callers cannot silently enter default listener")
	f.config.ReportPermissionEnabled = true
	require.NoError(t, f.config.validate())
	delete(f.config.JWKSURLs, "moderation")
	require.Error(t, f.config.validate(), "enabled report route requires BOTH exact owners")
	f.config.JWKSURLs["moderation"] = f.config.JWKSURLs["space"]
	f.config.JWKSURLs["chat"] = f.config.JWKSURLs["space"]
	require.Error(t, f.config.validate(), "feature cannot introduce another issuer")
}

func TestReportPermissionEnvironmentIsClosed(t *testing.T) {
	validRuntimeEnv(t)
	t.Setenv("ROLE_REPORT_PERMISSION_ENABLED", "true")
	_, _, err := LoadFromEnv()
	require.Error(t, err, "enabled report route cannot borrow existing service trust")
	t.Setenv("S2S_JWKS_URLS_JSON", `{"space":"https://space.internal/jwks","gameintegration":"https://gis.internal/jwks","voice":"https://voice.internal/jwks","messaging":"https://messaging.internal/jwks","moderation":"https://moderation.internal/jwks"}`)
	cfg, enabled, err := LoadFromEnv()
	require.NoError(t, err)
	require.True(t, enabled)
	require.True(t, cfg.ReportPermissionEnabled)
	for _, value := range []string{"false", "", "TRUE", "1", " true "} {
		t.Setenv("ROLE_REPORT_PERMISSION_ENABLED", value)
		_, _, err := LoadFromEnv()
		require.Error(t, err, "strict boolean and exact issuer set required")
	}
}
