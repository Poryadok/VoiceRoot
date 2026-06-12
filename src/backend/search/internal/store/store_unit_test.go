package store

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestEscapeLikePattern_literals(t *testing.T) {
	t.Parallel()
	require.Equal(t, `50\%\_off`, escapeLikePattern(`50%_off`))
}

func TestNormalizeLimit_defaults(t *testing.T) {
	t.Parallel()
	require.Equal(t, 20, normalizeLimit(0))
	require.Equal(t, 5, normalizeLimit(5))
}

func TestMessageCursor_roundTrip(t *testing.T) {
	t.Parallel()
	msgID := uuid.New()
	created := time.Now().UTC().Truncate(time.Microsecond)
	raw, err := encodeMessageCursor(messageCursor{CreatedAt: created, MessageID: msgID})
	require.NoError(t, err)
	decoded, err := decodeMessageCursor(raw)
	require.NoError(t, err)
	require.Equal(t, msgID, decoded.MessageID)
	require.True(t, created.Equal(decoded.CreatedAt))
}

func TestMessageCursor_invalid(t *testing.T) {
	t.Parallel()
	_, err := decodeMessageCursor("not-a-cursor")
	require.Error(t, err)
}

func TestSpaceCursor_roundTrip(t *testing.T) {
	t.Parallel()
	spaceID := uuid.New()
	raw, err := encodeSpaceCursor(spaceCursor{Name: "Raiders", SpaceID: spaceID})
	require.NoError(t, err)
	decoded, err := decodeSpaceCursor(raw)
	require.NoError(t, err)
	require.Equal(t, spaceID, decoded.SpaceID)
	require.Equal(t, "Raiders", decoded.Name)
}
