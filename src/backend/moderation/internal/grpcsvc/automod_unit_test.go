package grpcsvc

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsSpamPattern_detectsLinkFlooding(t *testing.T) {
	require.True(t, isSpamPattern("http://a http://b http://c"))
	require.False(t, isSpamPattern("hello world"))
}

func TestModerationGRPC_reportThreshold(t *testing.T) {
	s := &ModerationGRPC{PlatformAudienceSize: 1000}
	require.Equal(t, 10, s.reportThreshold())
	s.PlatformAudienceSize = 2000
	require.Equal(t, 20, s.reportThreshold())
	s.PlatformAudienceSize = 50
	require.Equal(t, 10, s.reportThreshold())
}
