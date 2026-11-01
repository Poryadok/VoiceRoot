package grpcsvc

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/user/internal/identityassociation"
)

func TestCustodiedSpaceCaptureUsesGuardedOperation(t *testing.T) {
	sentinel := errors.New("custody denied")
	called := 0
	authority, err := NewCustodiedSpaceAccountCaptureAuthority(func(_ context.Context, ids []uuid.UUID) (identityassociation.CaptureBatchResult, error) {
		called++
		require.Empty(t, ids)
		return identityassociation.CaptureBatchResult{}, sentinel
	})
	require.NoError(t, err)
	_, err = authority.CaptureSpaceMemberAccountBindings(context.Background(), captureTestRequest())
	require.ErrorIs(t, err, sentinel)
	require.Equal(t, 1, called)
	_, err = NewCustodiedSpaceAccountCaptureAuthority(nil)
	require.Error(t, err)
}
