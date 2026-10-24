package main

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
	voicestore "voice/backend/voice/internal/store"
)

func TestOrdinaryRuntimeRequiresExplicitModeAndLegacyDrain(t *testing.T) {
	t.Setenv("VOICE_DATABASE_URL", "")
	t.Setenv("VOICE_ORDINARY_SDK_ENABLED", "false")
	r, err := openOrdinaryRuntime(context.Background(), voicestore.NewMemoryCallStore())
	require.NoError(t, err)
	require.Nil(t, r)
	t.Setenv("VOICE_ORDINARY_SDK_ENABLED", "yes")
	_, err = openOrdinaryRuntime(context.Background(), nil)
	require.Error(t, err)
	t.Setenv("VOICE_ORDINARY_SDK_ENABLED", "true")
	t.Setenv("VOICE_ORDINARY_LEGACY_DRAINED", "false")
	_, err = openOrdinaryRuntime(context.Background(), nil)
	require.Error(t, err)
}
