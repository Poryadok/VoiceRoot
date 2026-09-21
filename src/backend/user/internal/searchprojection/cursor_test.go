package searchprojection

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCursorKeyFromEnvRejectsProtectedPartialConfiguration(t *testing.T) {
	_, err := CursorKeyFromEnv(true, func(string) string { return "" })
	require.Error(t, err)
	key, err := CursorKeyFromEnv(true, func(string) string { return strings.Repeat("k", 32) })
	require.NoError(t, err)
	require.Len(t, key, 32)
}

func TestSignedCursorRejectsTampering(t *testing.T) {
	key := []byte(strings.Repeat("k", 32))
	cursor, err := EncodeCursor(key, "profile-id")
	require.NoError(t, err)
	value, err := DecodeCursor(key, cursor)
	require.NoError(t, err)
	require.Equal(t, "profile-id", value)
	_, err = DecodeCursor(key, cursor+"x")
	require.Error(t, err)
}
