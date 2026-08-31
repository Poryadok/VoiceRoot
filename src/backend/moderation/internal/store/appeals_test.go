package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAppealListCursor_roundTrip(t *testing.T) {
	createdAt := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	id := uuid.New()
	encoded := encodeAppealListCursor(createdAt, id)
	decodedAt, decodedID, err := decodeAppealListCursor(encoded)
	require.NoError(t, err)
	require.True(t, createdAt.Equal(decodedAt))
	require.Equal(t, id, decodedID)
}

func TestAppealStore_ListAppealsPage_notConfigured(t *testing.T) {
	var s AppealStore
	_, err := s.ListAppealsPage(context.Background(), "pending", "", 10)
	require.ErrorIs(t, err, errStoreNotConfigured)
}

func TestDecodeAppealListCursor_invalid(t *testing.T) {
	_, _, err := decodeAppealListCursor("not-a-cursor")
	require.ErrorIs(t, err, ErrInvalidAppealListCursor)
}
