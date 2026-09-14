package clients

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestIsIdempotentDeleteResult_acceptsFileNotFoundAfterRetry(t *testing.T) {
	require.True(t, isIdempotentDeleteResult(status.Error(codes.NotFound, "file not found")))
	require.False(t, isIdempotentDeleteResult(status.Error(codes.Internal, "storage unavailable")))
	require.False(t, isIdempotentDeleteResult(errors.New("transport response lost")))
}
