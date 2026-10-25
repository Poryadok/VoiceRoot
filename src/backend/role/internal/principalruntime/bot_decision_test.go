package principalruntime

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBotDecisionRequiresExplicitClosedIssuerSet(t *testing.T) {
	f := newRuntimeFixture(t, 2)
	f.config.JWKSURLs["bot"] = f.config.JWKSURLs["space"]
	require.Error(t, f.config.validate(), "Bot trust cannot be added to the default listener set")
	f.config.BotLifecycleEnabled = true
	require.NoError(t, f.config.validate())
	delete(f.config.JWKSURLs, "bot")
	require.Error(t, f.config.validate(), "enabled Bot decision cannot borrow a Space or Gateway endpoint")
}
