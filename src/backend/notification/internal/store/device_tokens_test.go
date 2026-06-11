package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/store"
)

func TestDeviceTokenStore_NilPool(t *testing.T) {
	t.Parallel()

	var s *store.DeviceTokenStore
	ctx := context.Background()
	profileID := uuid.New()

	_, err := s.Register(ctx, profileID, "web", "tok", "fcm")
	require.ErrorIs(t, err, store.ErrNotImplemented)

	err = s.Unregister(ctx, profileID, uuid.New())
	require.ErrorIs(t, err, store.ErrNotImplemented)

	rows, err := s.ListByProfile(ctx, profileID)
	require.NoError(t, err)
	require.Nil(t, rows)

	err = s.DeleteByToken(ctx, "tok")
	require.ErrorIs(t, err, store.ErrNotImplemented)
}
