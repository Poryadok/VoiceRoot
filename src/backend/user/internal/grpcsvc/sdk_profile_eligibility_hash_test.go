package grpcsvc

import (
	"testing"

	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/principal"

	"github.com/stretchr/testify/require"
)

// This golden is shared with Auth's Java request-bound principal client test.
func TestSdkProfileEligibilityRequestHashMatchesAuthJavaGolden(t *testing.T) {
	req := &userv1.GetSdkProfileEligibilityRequest{
		AccountId: "0bd64a51-8720-4846-a4ea-3ff980a41d83",
		ProfileId: "d371d30f-7059-46e7-9880-3fbc4c61f8e9",
	}

	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	require.Equal(t, "sha256:755e97464a52e2dfddfdec444f215743dfe49e1da2f5719a08e2bb90b4ae3709", hash)
}
