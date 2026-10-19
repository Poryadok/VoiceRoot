package spacelifecycle

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

type publicationDrain struct{ calls atomic.Int64 }

func (d *publicationDrain) ResumeCapturedDrain(context.Context, CapturedTarget) error {
	d.calls.Add(1)
	return nil
}

type publicationFence struct{ calls atomic.Int64 }

func (f *publicationFence) SetSpaceLifecycle(context.Context, string, int64, bool) error {
	f.calls.Add(1)
	return nil
}

func TestRecoveryPublicationRetainsFinalBindingsDuringStartupMutation(t *testing.T) {
	finalDrain, finalFence := &publicationDrain{}, &publicationFence{}
	otherDrain, otherFence := &publicationDrain{}, &publicationFence{}
	service := &Service{Effects: &Effects{NativeDrain: finalDrain, ProjectionFence: finalFence}}
	recovery := service.RecoverySnapshot()
	// A slow/optional startup path must not retarget a published worker.
	service.Effects.NativeDrain = otherDrain
	service.Effects.ProjectionFence = otherFence
	require.NoError(t, recovery.Effects.NativeDrain.ResumeCapturedDrain(t.Context(), CapturedTarget{}))
	require.NoError(t, recovery.Effects.ProjectionFence.SetSpaceLifecycle(t.Context(), "space", 1, false))
	require.Equal(t, int64(1), finalDrain.calls.Load())
	require.Equal(t, int64(1), finalFence.calls.Load())
	require.Zero(t, otherDrain.calls.Load())
	require.Zero(t, otherFence.calls.Load())

	var workers sync.WaitGroup
	workers.Add(1)
	go func() {
		defer workers.Done()
		for range 1000 {
			_ = recovery.Effects.NativeDrain.ResumeCapturedDrain(t.Context(), CapturedTarget{})
			_ = recovery.Effects.ProjectionFence.SetSpaceLifecycle(t.Context(), "space", 1, false)
		}
	}()
	for range 1000 {
		service.Effects.NativeDrain = finalDrain
		service.Effects.ProjectionFence = finalFence
		service.Effects.NativeDrain = otherDrain
		service.Effects.ProjectionFence = otherFence
	}
	workers.Wait()
	require.Equal(t, int64(1001), finalDrain.calls.Load())
	require.Equal(t, int64(1001), finalFence.calls.Load())
}
