package r2avatar

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestObjectKey_shape(t *testing.T) {
	pid := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	k := ObjectKey(pid, ".png")
	require.Contains(t, k, "avatars/11111111-1111-1111-1111-111111111111/")
	require.True(t, strings.HasSuffix(k, ".png"), k)
}

func TestObjectKey_extWithoutDot(t *testing.T) {
	pid := uuid.New()
	k := ObjectKey(pid, "webp")
	require.True(t, strings.HasSuffix(k, ".webp"), k)
}

func TestJoinPublicURL(t *testing.T) {
	require.Equal(t, "https://cdn.example/avatars/p/1.png",
		JoinPublicURL("https://cdn.example/", "/avatars/p/1.png"))
	require.Equal(t, "https://cdn.example/avatars/p/1.png",
		JoinPublicURL("https://cdn.example", "avatars/p/1.png"))
}
