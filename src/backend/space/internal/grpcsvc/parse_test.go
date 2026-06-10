package grpcsvc

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestParseUUIDField(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		_, err := parseUUIDField("space_id", "  ")
		require.Error(t, err)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("invalid", func(t *testing.T) {
		t.Parallel()
		_, err := parseUUIDField("space_id", "not-a-uuid")
		require.Error(t, err)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("valid trims whitespace", func(t *testing.T) {
		t.Parallel()
		id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
		got, err := parseUUIDField("space_id", "  "+id.String()+"  ")
		require.NoError(t, err)
		require.Equal(t, id, got)
	})
}
