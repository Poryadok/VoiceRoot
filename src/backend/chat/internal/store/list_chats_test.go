package store

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestListChatCursorRoundTrip(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	ts := time.Date(2024, 3, 4, 5, 6, 7, 123456789, time.UTC)
	raw := encodeListChatCursor(ts, id)
	gotTS, gotID, err := decodeListChatCursor(raw)
	require.NoError(t, err)
	require.Equal(t, id, gotID)
	require.True(t, ts.Equal(gotTS))
}

func TestListChatCursorInvalid(t *testing.T) {
	_, _, err := decodeListChatCursor("@@@")
	require.ErrorIs(t, err, ErrInvalidListCursor)
}
