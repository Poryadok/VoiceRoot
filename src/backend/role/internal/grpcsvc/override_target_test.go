package grpcsvc

import (
	"context"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"
	"time"
)

func TestTargetValidationErrorKeepsOnlyParentCancellationTyped(t *testing.T) {
	for _, ownerCode := range []codes.Code{codes.Canceled, codes.DeadlineExceeded, codes.Unauthenticated, codes.PermissionDenied, codes.Unimplemented, codes.Unavailable} {
		require.Equal(t, codes.Unavailable, status.Code(targetValidationError(context.Background(), status.Error(ownerCode, "owner failure"))), ownerCode.String())
	}
	for _, ownerCode := range []codes.Code{codes.NotFound, codes.InvalidArgument} {
		require.Equal(t, ownerCode, status.Code(targetValidationError(context.Background(), status.Error(ownerCode, "target rejected"))))
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	require.Equal(t, codes.Canceled, status.Code(targetValidationError(canceled, status.Error(codes.Unavailable, "owner failure"))))
	deadline, done := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer done()
	require.Equal(t, codes.DeadlineExceeded, status.Code(targetValidationError(deadline, status.Error(codes.Unavailable, "owner failure"))))
}
