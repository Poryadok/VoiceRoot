package store

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestDecodeHistoryCursor(t *testing.T) {
	id := uuid.MustParse("018f1234-5678-7abc-8def-123456789abc")
	b := EncodeBeforeCursor(id)
	before, after, err := DecodeHistoryCursor(b)
	require.NoError(t, err)
	require.NotNil(t, before)
	require.Nil(t, after)
	require.Equal(t, id, *before)

	a := EncodeAfterCursor(id)
	before, after, err = DecodeHistoryCursor(a)
	require.NoError(t, err)
	require.Nil(t, before)
	require.NotNil(t, after)
	require.Equal(t, id, *after)

	before, after, err = DecodeHistoryCursor("")
	require.NoError(t, err)
	require.Nil(t, before)
	require.Nil(t, after)

	_, _, err = DecodeHistoryCursor("not-base64")
	require.ErrorIs(t, err, ErrInvalidHistoryCursor)

	_, _, err = DecodeHistoryCursor("e30") // {}
	require.ErrorIs(t, err, ErrInvalidHistoryCursor)

	badUUID := encodeCursorPayload(t, "not-uuid", "")
	_, _, err = DecodeHistoryCursor(badUUID)
	require.ErrorIs(t, err, ErrInvalidHistoryCursor)

	bothFields := encodeCursorPayload(t, id.String(), id.String())
	_, _, err = DecodeHistoryCursor(bothFields)
	require.ErrorIs(t, err, ErrInvalidHistoryCursor)

	emptyFields := encodeCursorPayload(t, "", "")
	_, _, err = DecodeHistoryCursor(emptyFields)
	require.ErrorIs(t, err, ErrInvalidHistoryCursor)

	badAfter := encodeCursorPayload(t, "", "not-uuid")
	_, _, err = DecodeHistoryCursor(badAfter)
	require.ErrorIs(t, err, ErrInvalidHistoryCursor)
}

func encodeCursorPayload(t *testing.T, b, a string) string {
	t.Helper()
	p := historyCursorPayload{B: b, A: a}
	raw, err := json.Marshal(p)
	require.NoError(t, err)
	return base64.RawURLEncoding.EncodeToString(raw)
}
