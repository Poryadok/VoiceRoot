package grpcsvc

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsSpamPattern_detectsLinkFlooding(t *testing.T) {
	require.True(t, isSpamPattern("http://a http://b http://c"))
	require.False(t, isSpamPattern("hello world"))
}
