package store

import (
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
}
