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
	chatID := uuid.MustParse("018f1234-5678-7abc-8def-123456789abd")
	b := EncodeBeforeCursor(chatID, id)
	before, after, err := DecodeHistoryCursor(b, chatID)
	require.NoError(t, err)
	require.NotNil(t, before)
	require.Nil(t, after)
	require.Equal(t, id, *before)

	a := EncodeAfterCursor(chatID, id)
	before, after, err = DecodeHistoryCursor(a, chatID)
	require.NoError(t, err)
	require.Nil(t, before)
	require.NotNil(t, after)
	require.Equal(t, id, *after)

	before, after, err = DecodeHistoryCursor("", chatID)
	require.NoError(t, err)
	require.Nil(t, before)
	require.Nil(t, after)

	_, _, err = DecodeHistoryCursor(b, uuid.New())
	require.ErrorIs(t, err, ErrInvalidHistoryCursor)

	_, _, err = DecodeHistoryCursor("not-base64", chatID)
	require.ErrorIs(t, err, ErrInvalidHistoryCursor)

	_, _, err = DecodeHistoryCursor("e30", chatID) // {}
	require.ErrorIs(t, err, ErrInvalidHistoryCursor)

	badUUID := encodeCursorPayload(t, chatID.String(), "not-uuid", "")
	_, _, err = DecodeHistoryCursor(badUUID, chatID)
	require.ErrorIs(t, err, ErrInvalidHistoryCursor)

	bothFields := encodeCursorPayload(t, chatID.String(), id.String(), id.String())
	_, _, err = DecodeHistoryCursor(bothFields, chatID)
	require.ErrorIs(t, err, ErrInvalidHistoryCursor)

	emptyFields := encodeCursorPayload(t, chatID.String(), "", "")
	_, _, err = DecodeHistoryCursor(emptyFields, chatID)
	require.ErrorIs(t, err, ErrInvalidHistoryCursor)

	badAfter := encodeCursorPayload(t, chatID.String(), "", "not-uuid")
	_, _, err = DecodeHistoryCursor(badAfter, chatID)
	require.ErrorIs(t, err, ErrInvalidHistoryCursor)
}

func TestDecodeHistoryCursorRejectsLegacyPayloadWithoutChatID(t *testing.T) {
	chatID := uuid.New()
	messageID := uuid.New()
	legacyCursor := encodeLegacyCursorPayload(t, messageID.String(), "")

	_, _, err := DecodeHistoryCursor(legacyCursor, chatID)
	require.ErrorIs(t, err, ErrInvalidHistoryCursor)
}

func encodeCursorPayload(t *testing.T, chatID, b, a string) string {
	t.Helper()
	p := historyCursorPayload{C: chatID, B: b, A: a}
	raw, err := json.Marshal(p)
	require.NoError(t, err)
	return base64.RawURLEncoding.EncodeToString(raw)
}

func encodeLegacyCursorPayload(t *testing.T, b, a string) string {
	t.Helper()
	type legacyHistoryCursorPayload struct {
		B string `json:"b,omitempty"`
		A string `json:"a,omitempty"`
	}
	raw, err := json.Marshal(legacyHistoryCursorPayload{B: b, A: a})
	require.NoError(t, err)
	return base64.RawURLEncoding.EncodeToString(raw)
}
