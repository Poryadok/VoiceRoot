package s2s

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGameAuthAndExecutionPermitClientsRequireExplicitPinnedMTLSConfig(t *testing.T) {
	_, authErr := NewGameAuthKeys(GameAuthKeysConfig{})
	require.Error(t, authErr)
	_, permitErr := NewGameMessageExecutionPermitClient(GameMessageExecutionPermitConfig{})
	require.Error(t, permitErr)
	_, wrongSchemeErr := NewGameMessageExecutionPermitClient(GameMessageExecutionPermitConfig{Endpoint: "http://voice-auth:8443/api/v1/auth/sdk/game-message/execution-permits", TLSCertFile: "cert", TLSKeyFile: "key", CAFile: "ca"})
	require.Error(t, wrongSchemeErr)
}
