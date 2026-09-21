package searchprojection

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCursorKeyFromEnvRejectsProtectedPartialConfiguration(t *testing.T) {
	_, err := CursorKeyFromEnv(true, func(string) string { return "" })
	require.Error(t, err)
	key, err := CursorKeyFromEnv(true, func(string) string { return strings.Repeat("k", 32) })
	require.NoError(t, err)
	require.Len(t, key, 32)
}

func TestSnapshotCursorBindsHighWatermarkAndExpires(t *testing.T) {
	key := []byte(strings.Repeat("k", 32))
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	cursor, err := EncodeSnapshotCursor(key, 42, 17, now)
	require.NoError(t, err)
	decoded, err := DecodeSnapshotCursor(key, cursor, now.Add(time.Minute))
	require.NoError(t, err)
	require.Equal(t, uint64(42), decoded.High)
	require.Equal(t, uint64(17), decoded.LastOffset)
	_, err = DecodeSnapshotCursor(key, cursor, now.Add(snapshotCursorTTL+time.Second))
	require.Error(t, err)
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
