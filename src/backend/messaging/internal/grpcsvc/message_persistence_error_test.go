package grpcsvc

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestMessagePersistencePreservesWrappedAttemptContextStatus(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want codes.Code
	}{{context.DeadlineExceeded, codes.DeadlineExceeded}, {context.Canceled, codes.Canceled}} {
		require.Equal(t, tc.want, status.Code(messageContextStatus(fmt.Errorf("commit outcome uncertain: %w", tc.err))))
	}
	typed := status.Error(codes.Unavailable, "dependency failed")
	require.Same(t, typed, messageContextStatus(typed))
}
