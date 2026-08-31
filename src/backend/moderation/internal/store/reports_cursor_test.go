package store

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestReportListCursor_roundTrip(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 6, 14, 12, 0, 0, 123456789, time.UTC)
	id := uuid.MustParse("11111111-1111-4111-8111-111111111111")

	encoded := encodeReportListCursor(2, createdAt, id)
	priority, ts, decodedID, err := decodeReportListCursor(encoded)
	require.NoError(t, err)
	require.Equal(t, 2, priority)
	require.True(t, ts.Equal(createdAt))
	require.Equal(t, id, decodedID)
}

func TestReportListCursor_empty(t *testing.T) {
	t.Parallel()

	_, _, id, err := decodeReportListCursor("")
	require.NoError(t, err)
	require.Equal(t, uuid.Nil, id)
}

func TestReportListCursor_invalid(t *testing.T) {
	t.Parallel()

	_, _, _, err := decodeReportListCursor("not-a-cursor")
	require.ErrorIs(t, err, ErrInvalidReportListCursor)
}
