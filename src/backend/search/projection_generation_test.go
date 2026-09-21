package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDesiredProjectionGenerationIsOptionalAndFailClosed(t *testing.T) {
	t.Setenv("SEARCH_USER_PROJECTION_DESIRED_GENERATION", "")
	_, set, err := desiredProjectionGeneration()
	require.NoError(t, err)
	require.False(t, set)

	t.Setenv("SEARCH_USER_PROJECTION_DESIRED_GENERATION", "2")
	generation, set, err := desiredProjectionGeneration()
	require.NoError(t, err)
	require.True(t, set)
	require.Equal(t, uint64(2), generation)

	t.Setenv("SEARCH_USER_PROJECTION_DESIRED_GENERATION", "0")
	_, _, err = desiredProjectionGeneration()
	require.Error(t, err)
}
