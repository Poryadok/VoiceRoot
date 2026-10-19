//go:build linux

package nativeapp

import (
	"context"
	"github.com/google/uuid"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNativeStartupQueuedCriticalFailureCannotOpen(t *testing.T) {
	b, stamp, observer := gateFixture(t)
	gate := newForwardGate(b, stamp, observer)
	failures := make(chan error, 1)
	failures <- errUnavailable
	require.ErrorIs(t, awaitOpen(context.Background(), gate, failures), errUnavailable,
		"an earlier failed bootstrap listener cannot be hidden by a successful OPEN observation")
}

func TestNativeCriticalFailureClosesAdmissionBeforeRootConsumesError(t *testing.T) {
	b, stamp, observer := gateFixture(t)
	gate := newForwardGate(b, stamp, observer)
	runtime := &preparedRuntime{gate: gate, errors: make(chan error, 1)}
	runtime.fail()
	release, err := gate.EnterForward(context.Background(), uuid.New(), [32]byte{1})
	require.ErrorIs(t, err, errUnavailable)
	require.Nil(t, release)
	require.Len(t, runtime.errors, 1)
}
