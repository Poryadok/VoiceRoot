package socialprincipal

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSpaceAccountCaptureCapabilityIsExactAndRequiresMutualTLS(t *testing.T) {
	const method = "/voice.user.v1.UserService/CaptureSpaceMemberAccountBindings"
	require.Equal(t, method, Method("space-account-capture"))
	require.True(t, AllowsMethod("space-account-capture", method))
	require.True(t, RequiresMutualTLS("space-account-capture"))
	require.Equal(t, "space", expectedIssuer("space-account-capture"))
	for _, other := range []string{"GetProfile", "ResolveAccountIDForProfile", "GetSpaceAdmissionProfileEligibility", "GetSdkProfileEligibility", "RecordSdkAuthorTombstone"} {
		require.False(t, AllowsMethod("space-account-capture", "/voice.user.v1.UserService/"+other))
	}
	for _, capability := range []string{"user", "space", "auth", "auth-admission", "file", "search", "notification", "messaging"} {
		require.False(t, AllowsMethod(capability, method), capability)
	}
}
