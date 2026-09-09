package grpcsvc

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/role/internal/store"
)

func TestRoleRowToProto_UsesPersistedCreatedAt(t *testing.T) {
	persistedCreatedAt := time.Date(2024, time.November, 12, 13, 14, 15, 123456000, time.FixedZone("UTC+3", 3*60*60))

	got := roleRowToProto(&store.RoleRow{
		ID:        uuid.New(),
		SpaceID:   uuid.New(),
		Name:      "Persisted timestamp",
		CreatedAt: persistedCreatedAt,
	})

	require.NotNil(t, got.GetCreatedAt())
	require.Equal(t, persistedCreatedAt.UTC(), got.GetCreatedAt().AsTime())
}
