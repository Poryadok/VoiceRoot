package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStartOwnershipRecoveryRuntime_MissingProtectedDependenciesStaysDisabled(t *testing.T) {
	require.Nil(t, startOwnershipRecoveryRuntime(context.Background(), nil, nil, nil))
}
