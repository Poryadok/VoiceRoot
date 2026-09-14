package principal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Static rollout contract: production configuration must ship with the
// application cutover, otherwise a seemingly green code-only change would
// leave protected listeners unreachable or use an unmounted private key.
func TestSocialPrivacyPrincipalRollout_StagingAndProdDeclareProtectedResources(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", "..", ".."))
	for _, environment := range []string{"staging", "prod"} {
		t.Run(environment, func(t *testing.T) {
			services, err := os.ReadFile(filepath.Join(root, "deploy", environment, "services.yaml"))
			require.NoError(t, err)
			manifest := string(services)

			for _, required := range []string{
				"SOCIAL_PRINCIPAL_SIGNING_KEYS_DIR",
				"SOCIAL_PRINCIPAL_ACTIVE_KID",
				"USER_PRINCIPAL_GRPC_LISTEN",
				"SPACE_PRINCIPAL_GRPC_LISTEN",
				"USER_PRINCIPAL_REPLAY_REDIS_ADDR",
				"SPACE_PRINCIPAL_REPLAY_REDIS_ADDR",
				"containerPort: 9091",
				"readOnly: true",
			} {
				require.True(t, strings.Contains(manifest, required), "missing rollout declaration %q", required)
			}
		})
	}

	policy, err := os.ReadFile(filepath.Join(root, "deploy", "templates", "network-policy-social-privacy-principal.yaml"))
	require.NoError(t, err)
	require.Contains(t, string(policy), "voice-social")
	require.Contains(t, string(policy), "9091")
	require.False(t, strings.Contains(string(policy), "port: 9090\n    # Social"), "Social must not retain ordinary listener access")
}
