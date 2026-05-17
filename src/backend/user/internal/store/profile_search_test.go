package store

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestEscapeLikePattern(t *testing.T) {
	require.Equal(t, `foo\%bar\_\\x`, escapeLikePattern(`foo%bar_\x`))
}

func TestSearchCursorRoundTrip(t *testing.T) {
	id := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	c := ProfileSearchCursor{UsernameLower: "alice", Discriminator: "0001", ID: id}
	s, err := EncodeSearchCursor(c)
	require.NoError(t, err)
	got, err := DecodeSearchCursor(s)
	require.NoError(t, err)
	require.Equal(t, c.UsernameLower, got.UsernameLower)
	require.Equal(t, c.Discriminator, got.Discriminator)
	require.Equal(t, c.ID, got.ID)
}

func TestDecodeSearchCursor_empty(t *testing.T) {
	got, err := DecodeSearchCursor("")
	require.NoError(t, err)
	require.Nil(t, got)
}

func TestDecodeSearchCursor_invalid(t *testing.T) {
	_, err := DecodeSearchCursor("not-base64!!!")
	require.Error(t, err)
}
