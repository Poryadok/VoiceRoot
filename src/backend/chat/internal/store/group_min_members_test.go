package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGroupAddMinMembers_DefaultThreeInternalTwo(t *testing.T) {
	t.Parallel()
	require.Equal(t, MinGroupMembers, groupAddMinMembers(context.Background()))
	require.Equal(t, 2, groupAddMinMembers(WithGroupMinMembers(context.Background(), 2)))
	require.Equal(t, MinGroupMembers, groupAddMinMembers(WithGroupMinMembers(context.Background(), 0)))
}
