package grpcsvc

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type searchBlockCheckerStub struct {
	blocked bool
	err     error
}

func (s searchBlockCheckerStub) AccountPairBlocked(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return s.blocked, s.err
}

func TestUserGRPCPairwiseBlockedFailsClosedWhenCheckerUnavailable(t *testing.T) {
	t.Parallel()

	blocked, err := (&UserGRPC{}).pairwiseBlocked(context.Background(), uuid.New(), uuid.New())

	require.Error(t, err)
	require.False(t, blocked)
}

func TestUserGRPCPairwiseBlockedPreservesSocialDecisionAndFailure(t *testing.T) {
	t.Parallel()
	viewer, other := uuid.New(), uuid.New()

	for _, tc := range []struct {
		name    string
		checker AccountBlockChecker
		blocked bool
		wantErr bool
	}{
		{
			name:    "healthy discovery is allowed",
			checker: searchBlockCheckerStub{},
			blocked: false,
		},
		{
			name:    "either-direction Social block denies discovery",
			checker: searchBlockCheckerStub{blocked: true},
			blocked: true,
		},
		{
			name:    "Social backend error denies discovery",
			checker: searchBlockCheckerStub{err: errors.New("social unavailable")},
			wantErr: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			blocked, err := (&UserGRPC{Blocks: tc.checker}).pairwiseBlocked(context.Background(), viewer, other)

			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.blocked, blocked)
		})
	}
}
