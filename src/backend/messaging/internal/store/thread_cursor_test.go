package store

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestThreadCursorRoundTrip(t *testing.T) {
	t.Parallel()
	when := time.Date(2026, 9, 13, 12, 0, 0, 123456000, time.UTC)
	id := uuid.MustParse("018f1234-5678-7abc-8def-123456789abc")

	cursor, err := DecodeThreadCursor(EncodeThreadCursor(when, id))
	require.NoError(t, err)
	require.NotNil(t, cursor)
	require.Equal(t, when, cursor.LastReplyAt)
	require.Equal(t, id, cursor.ThreadID)
}

func TestDecodeThreadCursorRejectsMalformedValues(t *testing.T) {
	t.Parallel()

	cursor, err := DecodeThreadCursor("")
	require.NoError(t, err)
	require.Nil(t, cursor)

	for _, raw := range []string{"not-base64", "e30"} {
		_, err := DecodeThreadCursor(raw)
		require.ErrorIs(t, err, ErrInvalidThreadCursor)
	}
}
