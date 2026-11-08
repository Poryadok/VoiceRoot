package socialprincipal

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAuthProfileAuthorityPurposeIsolation(t *testing.T) {
	for _, method := range []string{"/voice.user.v1.AuthProfileAuthorityService/MatchAuthProfileReferences", "/voice.user.v1.AuthProfileAuthorityService/ResolveAuthOwnershipTarget"} {
		require.True(t, AllowsMethod("auth-profile-authority", method))
		for _, other := range []string{"auth", "auth-admission", "user", "space-account-capture", "file", "search"} {
			require.False(t, AllowsMethod(other, method))
		}
	}
	require.True(t, RequiresMutualTLS("auth-profile-authority"))
	require.Equal(t, []string{"auth"}, capabilityIssuers("auth-profile-authority"))
	require.False(t, AllowsMethod("auth-profile-authority", Method("auth")))
}
