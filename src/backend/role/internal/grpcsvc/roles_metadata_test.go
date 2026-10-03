package grpcsvc

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/role/internal/store"
)

func TestRoleRowToProtoPreservesMetadata(t *testing.T) {
	color := "#123abc"
	row := &store.RoleRow{ID: uuid.New(), SpaceID: uuid.New(), Name: "Mentionable", Color: &color, IsMentionable: true}

	got := roleRowToProto(row)
	require.Equal(t, color, got.GetColor())
	require.True(t, got.GetIsMentionable())

	row.Color = nil
	row.IsMentionable = false
	got = roleRowToProto(row)
	require.Empty(t, got.GetColor())
	require.False(t, got.GetIsMentionable())
}
