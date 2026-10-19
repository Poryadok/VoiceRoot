package automod

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
)

func TestBootGate_NewProcessCannotAcquireEffectAuthority(t *testing.T) {
	_, err := NewBootGate().AcquireEffects(context.Background())
	require.ErrorIs(t, err, ErrBootClosed, "local process startup never reuses a prior ready decision")
}

func TestBootGate_OrderedReceiptsAndCapturedChecksCannotSurviveRecovery(t *testing.T) {
	ctx := context.Background()
	gate := NewBootGate()
	id, generation := gate.Identity()
	receipt := func(phase BootPhase) BootReceipt {
		return BootReceipt{BootID: id, Generation: generation, Phase: phase, ProofID: uuid.New()}
	}
	require.ErrorIs(t, gate.Advance(receipt(BootEffectsReady)), ErrBootReceipt)
	first := receipt(BootLocalIssuer)
	require.NoError(t, gate.Advance(first))
	require.NoError(t, gate.Advance(first))
	require.ErrorIs(t, gate.Advance(receipt(BootLocalIssuer)), ErrBootReceipt, "same phase cannot replace its committed proof")
	_, err := gate.AcquirePrivateRepair(ctx)
	require.ErrorIs(t, err, ErrBootClosed)
	require.NoError(t, gate.Advance(receipt(BootAuthority)))
	require.NoError(t, gate.Advance(receipt(BootTerminalImported)))
	private, err := gate.AcquirePrivateRepair(ctx)
	require.NoError(t, err)
	require.NoError(t, private(ctx))
	_, err = gate.AcquireEffects(ctx)
	require.ErrorIs(t, err, ErrBootClosed, "terminal import alone never opens normal effects")
	for phase := BootPrivateRepair; phase <= BootEffectsReady; phase++ {
		require.NoError(t, gate.Advance(receipt(phase)))
	}
	effect, err := gate.AcquireEffects(ctx)
	require.NoError(t, err)
	require.NoError(t, effect(ctx))
	var group sync.WaitGroup
	for i := 0; i < 12; i++ {
		group.Add(1)
		go func() { defer group.Done(); _ = effect(ctx) }()
	}
	gate.Close()
	group.Wait()
	require.ErrorIs(t, effect(ctx), ErrBootClosed)
	require.ErrorIs(t, private(ctx), ErrBootClosed)
	require.ErrorIs(t, gate.Advance(first), ErrBootClosed)
	newGate := NewBootGate()
	newID, newGeneration := newGate.Identity()
	require.NotEqual(t, id, newID)
	for phase := BootLocalIssuer; phase <= BootEffectsReady; phase++ {
		require.NoError(t, newGate.Advance(BootReceipt{BootID: newID, Generation: newGeneration, Phase: phase, ProofID: uuid.New()}))
	}
	_, err = newGate.AcquireEffects(ctx)
	require.NoError(t, err)
	require.ErrorIs(t, effect(ctx), ErrBootClosed, "new ready boot cannot authorize an old in-flight operation")
}
